package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
)

// AllocateChild carves the first free aligned child under a parent at prefix_length.
func (a *API) AllocateChild(w http.ResponseWriter, r *http.Request) {
	parentID, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !a.requireSubnetAccess(w, r, parentID) {
		return
	}
	var req struct {
		PrefixLength int    `json:"prefix_length"`
		Name         string `json:"name"`
		Description  string `json:"description"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.PrefixLength <= 0 {
		httpx.Error(w, http.StatusBadRequest, "prefix_length required")
		return
	}
	parent, err := a.Store.Queries.GetSubnet(r.Context(), parentID)
	if err != nil || parent.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	parentPrefix, err := ipam.ParsePrefix(parent.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid parent cidr")
		return
	}
	overlapping, err := a.Store.Queries.FindOverlappingSubnets(r.Context(), sqlcdb.FindOverlappingSubnetsParams{
		VrfID: parent.VrfID, Cidr: parent.Cidr, ExcludeID: db.UUIDToNull(parentID),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	occupied := make([]netip.Prefix, 0, len(overlapping))
	for _, o := range overlapping {
		p, err := ipam.ParsePrefix(o.Cidr)
		if err != nil {
			continue
		}
		occupied = append(occupied, p)
	}
	child, err := ipam.FirstFreeChild(parentPrefix, req.PrefixLength, occupied)
	if err != nil {
		httpx.Error(w, http.StatusConflict, err.Error())
		return
	}
	name := req.Name
	if name == "" {
		name = child.String()
	}
	var created sqlcdb.CreateSubnetRow
	err = a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		item, cerr := q.CreateSubnet(r.Context(), sqlcdb.CreateSubnetParams{
			Cidr: child.String(), Name: name, Description: req.Description,
			LocationID: parent.LocationID, VrfID: parent.VrfID, VlanID: parent.VlanID,
			ParentID: db.UUIDToNull(parentID), Tags: parent.Tags, CustomFields: emptyJSON(),
		})
		if cerr != nil {
			return cerr
		}
		created = item
		return nil
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "subnet.allocate_child", "subnet", created.ID.String(), map[string]any{
		"parent_id": parentID.String(), "cidr": child.String(),
	})
	vids := a.vlanVIDMap(r.Context())
	httpx.JSON(w, http.StatusCreated, toSubnetDTO(created.ID, created.Cidr, created.Name, created.Description, created.LocationID, created.VrfID, created.VlanID, created.ParentID, created.Tags, created.CustomFields, created.CreatedAt, created.UpdatedAt, vids))
}

/*
SplitSubnet design choice:
  Create two child halves (parent.Bits()+1), reassign live IPs by containment,
  then soft-delete the parent. Parent row is retained for audit/restore; children
  inherit location/VRF/VLAN/tags. Direct children of the parent are re-parented
  under the containing half when possible.
*/
func (a *API) SplitSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !a.requireSubnetAccess(w, r, id) {
		return
	}
	var req struct {
		LeftName  string `json:"left_name"`
		RightName string `json:"right_name"`
	}
	_ = httpx.Decode(r, &req)

	parent, err := a.Store.Queries.GetSubnet(r.Context(), id)
	if err != nil || parent.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	parentPrefix, err := ipam.ParsePrefix(parent.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid cidr")
		return
	}
	left, right, err := ipam.SplitHalves(parentPrefix)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// Ensure halves do not collide with existing siblings in VRF (excluding parent).
	for _, half := range []netip.Prefix{left, right} {
		if err := a.ensureNoOverlap(r.Context(), parent.VrfID, half, id); err != nil {
			httpx.Error(w, http.StatusConflict, err.Error())
			return
		}
	}
	leftName := req.LeftName
	if leftName == "" {
		leftName = parent.Name + " (a)"
	}
	rightName := req.RightName
	if rightName == "" {
		rightName = parent.Name + " (b)"
	}

	var leftRow, rightRow sqlcdb.CreateSubnetRow
	err = a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		var cerr error
		leftRow, cerr = q.CreateSubnet(r.Context(), sqlcdb.CreateSubnetParams{
			Cidr: left.String(), Name: leftName, Description: parent.Description,
			LocationID: parent.LocationID, VrfID: parent.VrfID, VlanID: parent.VlanID,
			ParentID: parent.ParentID, Tags: parent.Tags, CustomFields: parent.CustomFields,
		})
		if cerr != nil {
			return cerr
		}
		rightRow, cerr = q.CreateSubnet(r.Context(), sqlcdb.CreateSubnetParams{
			Cidr: right.String(), Name: rightName, Description: parent.Description,
			LocationID: parent.LocationID, VrfID: parent.VrfID, VlanID: parent.VlanID,
			ParentID: parent.ParentID, Tags: parent.Tags, CustomFields: parent.CustomFields,
		})
		if cerr != nil {
			return cerr
		}
		ips, cerr := q.ListIPsBySubnet(r.Context(), id)
		if cerr != nil {
			return cerr
		}
		for _, ip := range ips {
			addr, aerr := netip.ParseAddr(ip.Address)
			if aerr != nil {
				continue
			}
			target := leftRow.ID
			if right.Contains(addr) {
				target = rightRow.ID
			} else if !left.Contains(addr) {
				return fmt.Errorf("ip %s not contained in either half", ip.Address)
			}
			if err := q.MoveIPToSubnet(r.Context(), sqlcdb.MoveIPToSubnetParams{ID: ip.ID, SubnetID: target}); err != nil {
				return err
			}
		}
		kids, cerr := q.ListChildSubnets(r.Context(), db.UUIDToNull(id))
		if cerr != nil {
			return cerr
		}
		for _, kid := range kids {
			kp, kerr := ipam.ParsePrefix(kid.Cidr)
			if kerr != nil {
				continue
			}
			var newParent uuid.UUID
			switch {
			case ipam.ContainsPrefix(left, kp):
				newParent = leftRow.ID
			case ipam.ContainsPrefix(right, kp):
				newParent = rightRow.ID
			default:
				continue
			}
			if err := q.SetSubnetParent(r.Context(), sqlcdb.SetSubnetParentParams{
				ID: kid.ID, ParentID: db.UUIDToNull(newParent),
			}); err != nil {
				return err
			}
		}
		_, cerr = q.SoftDeleteSubnet(r.Context(), id)
		return cerr
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "subnet.split", "subnet", id.String(), map[string]any{
		"left": leftRow.ID.String(), "right": rightRow.ID.String(),
		"left_cidr": left.String(), "right_cidr": right.String(),
	})
	vids := a.vlanVIDMap(r.Context())
	httpx.JSON(w, http.StatusOK, map[string]any{
		"parent_id": id,
		"left":      toSubnetDTO(leftRow.ID, leftRow.Cidr, leftRow.Name, leftRow.Description, leftRow.LocationID, leftRow.VrfID, leftRow.VlanID, leftRow.ParentID, leftRow.Tags, leftRow.CustomFields, leftRow.CreatedAt, leftRow.UpdatedAt, vids),
		"right":     toSubnetDTO(rightRow.ID, rightRow.Cidr, rightRow.Name, rightRow.Description, rightRow.LocationID, rightRow.VrfID, rightRow.VlanID, rightRow.ParentID, rightRow.Tags, rightRow.CustomFields, rightRow.CreatedAt, rightRow.UpdatedAt, vids),
	})
}

// MergeSubnets merges the target with a sibling into their covering CIDR.
// Design: resize the primary subnet to the covering CIDR, move sibling IPs onto it,
// reparent sibling children, then soft-delete the sibling.
func (a *API) MergeSubnets(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !a.requireSubnetAccess(w, r, id) {
		return
	}
	var req struct {
		SiblingID uuid.UUID `json:"sibling_id"`
		Name      string    `json:"name"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.SiblingID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, "sibling_id required")
		return
	}
	if !a.requireSubnetAccess(w, r, req.SiblingID) {
		return
	}
	primary, err := a.Store.Queries.GetSubnet(r.Context(), id)
	if err != nil || primary.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	sibling, err := a.Store.Queries.GetSubnet(r.Context(), req.SiblingID)
	if err != nil || sibling.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "sibling not found")
		return
	}
	if primary.VrfID != sibling.VrfID {
		httpx.Error(w, http.StatusBadRequest, "subnets must be in the same VRF")
		return
	}
	aPrefix, err := ipam.ParsePrefix(primary.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid cidr")
		return
	}
	bPrefix, err := ipam.ParsePrefix(sibling.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid sibling cidr")
		return
	}
	cover, err := ipam.CoveringPrefix(aPrefix, bPrefix)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.ensureNoOverlap(r.Context(), primary.VrfID, cover, id, req.SiblingID); err != nil {
		httpx.Error(w, http.StatusConflict, err.Error())
		return
	}
	name := req.Name
	if name == "" {
		name = primary.Name
	}
	parentID := primary.ParentID
	if !parentID.Valid {
		parentID = sibling.ParentID
	}

	var merged sqlcdb.UpdateSubnetRow
	err = a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		item, cerr := q.UpdateSubnet(r.Context(), sqlcdb.UpdateSubnetParams{
			ID: id, Cidr: cover.String(), Name: name, Description: primary.Description,
			LocationID: primary.LocationID, VrfID: primary.VrfID, VlanID: primary.VlanID,
			ParentID: parentID, Tags: primary.Tags, CustomFields: primary.CustomFields,
		})
		if cerr != nil {
			return cerr
		}
		merged = item
		ips, cerr := q.ListIPsBySubnet(r.Context(), req.SiblingID)
		if cerr != nil {
			return cerr
		}
		for _, ip := range ips {
			if err := q.MoveIPToSubnet(r.Context(), sqlcdb.MoveIPToSubnetParams{ID: ip.ID, SubnetID: id}); err != nil {
				return err
			}
		}
		kids, cerr := q.ListChildSubnets(r.Context(), db.UUIDToNull(req.SiblingID))
		if cerr != nil {
			return cerr
		}
		for _, kid := range kids {
			if err := q.SetSubnetParent(r.Context(), sqlcdb.SetSubnetParentParams{
				ID: kid.ID, ParentID: db.UUIDToNull(id),
			}); err != nil {
				return err
			}
		}
		_, cerr = q.SoftDeleteSubnet(r.Context(), req.SiblingID)
		return cerr
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "subnet.merge", "subnet", id.String(), map[string]any{
		"sibling_id": req.SiblingID.String(), "cidr": cover.String(),
	})
	vids := a.vlanVIDMap(r.Context())
	httpx.JSON(w, http.StatusOK, toSubnetDTO(merged.ID, merged.Cidr, merged.Name, merged.Description, merged.LocationID, merged.VrfID, merged.VlanID, merged.ParentID, merged.Tags, merged.CustomFields, merged.CreatedAt, merged.UpdatedAt, vids))
}

// ResizeSubnet changes a subnet CIDR when all IPs remain contained and no VRF overlap.
func (a *API) ResizeSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !a.requireSubnetAccess(w, r, id) {
		return
	}
	var req struct {
		CIDR string `json:"cidr"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	newPrefix, err := ipam.ParsePrefix(req.CIDR)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid cidr")
		return
	}
	existing, err := a.Store.Queries.GetSubnet(r.Context(), id)
	if err != nil || existing.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	if err := a.ensureNoOverlap(r.Context(), existing.VrfID, newPrefix, id); err != nil {
		httpx.Error(w, http.StatusConflict, err.Error())
		return
	}
	ips, err := a.Store.Queries.ListIPsBySubnet(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, ip := range ips {
		addr, aerr := netip.ParseAddr(ip.Address)
		if aerr != nil || !newPrefix.Contains(addr) {
			httpx.Error(w, http.StatusBadRequest, fmt.Sprintf("ip %s would fall outside %s", ip.Address, newPrefix))
			return
		}
	}
	parentID, err := a.resolveSubnetParent(r.Context(), newPrefix, existing.VrfID, nil, id)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var updated sqlcdb.UpdateSubnetRow
	err = a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		item, cerr := q.UpdateSubnet(r.Context(), sqlcdb.UpdateSubnetParams{
			ID: id, Cidr: newPrefix.String(), Name: existing.Name, Description: existing.Description,
			LocationID: existing.LocationID, VrfID: existing.VrfID, VlanID: existing.VlanID,
			ParentID: parentID, Tags: existing.Tags, CustomFields: existing.CustomFields,
		})
		if cerr != nil {
			return cerr
		}
		updated = item
		return a.reparentContainedQ(r.Context(), q, id, newPrefix, existing.VrfID)
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "subnet.resize", "subnet", id.String(), map[string]any{
		"from": existing.Cidr, "to": newPrefix.String(),
	})
	vids := a.vlanVIDMap(r.Context())
	httpx.JSON(w, http.StatusOK, toSubnetDTO(updated.ID, updated.Cidr, updated.Name, updated.Description, updated.LocationID, updated.VrfID, updated.VlanID, updated.ParentID, updated.Tags, updated.CustomFields, updated.CreatedAt, updated.UpdatedAt, vids))
}

func (a *API) ensureNoOverlap(ctx context.Context, vrfID uuid.UUID, prefix netip.Prefix, exclude ...uuid.UUID) error {
	ex := map[uuid.UUID]bool{}
	for _, id := range exclude {
		ex[id] = true
	}
	overlapping, err := a.Store.Queries.FindOverlappingSubnets(ctx, sqlcdb.FindOverlappingSubnetsParams{
		VrfID: vrfID, Cidr: prefix.String(), ExcludeID: pgtype.UUID{},
	})
	if err != nil {
		return err
	}
	for _, o := range overlapping {
		if ex[o.ID] {
			continue
		}
		other, err := ipam.ParsePrefix(o.Cidr)
		if err != nil {
			continue
		}
		switch ipam.Classify(prefix, other) {
		case ipam.RelEqual, ipam.RelOverlap:
			return fmt.Errorf("overlaps existing subnet %s", other)
		case ipam.RelContains, ipam.RelContained:
			// nesting is allowed for hierarchy; overlap of equal/partial is not
		}
	}
	return nil
}
