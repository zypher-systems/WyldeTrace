package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
)

type subnetDTO struct {
	ID           uuid.UUID      `json:"id"`
	CIDR         string         `json:"cidr"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	LocationID   *uuid.UUID     `json:"location_id"`
	VrfID        uuid.UUID      `json:"vrf_id"`
	VlanID       *uuid.UUID     `json:"vlan_id"`
	VlanVID      *int32         `json:"vlan_vid"`
	ParentID     *uuid.UUID     `json:"parent_id"`
	Tags         []string       `json:"tags"`
	CustomFields map[string]any `json:"custom_fields"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Used         *int           `json:"used,omitempty"`
	Total        *int           `json:"total,omitempty"`
	Free         *int           `json:"free,omitempty"`
	Percent      *float64       `json:"percent,omitempty"`
	Sparse       bool           `json:"sparse,omitempty"`
	NextIP       string         `json:"next_ip,omitempty"`
}

type subnetRequest struct {
	CIDR         string         `json:"cidr"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	LocationID   *uuid.UUID     `json:"location_id"`
	VrfID        *uuid.UUID     `json:"vrf_id"`
	VlanVID      *int32         `json:"vlan_vid"`
	ParentID     *uuid.UUID     `json:"parent_id"`
	Tags         []string       `json:"tags"`
	CustomFields map[string]any `json:"custom_fields"`
}

type subnetTreeNode struct {
	subnetDTO
	Children []subnetTreeNode `json:"children"`
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if out == nil {
		return []string{}
	}
	return out
}

func emptyJSON() json.RawMessage { return json.RawMessage(`{}`) }

func encodeCustomFields(m map[string]any) json.RawMessage {
	if m == nil {
		return emptyJSON()
	}
	b, err := json.Marshal(m)
	if err != nil {
		return emptyJSON()
	}
	return b
}

func decodeCustomFields(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func toSubnetDTO(id uuid.UUID, cidr, name, description string, locationID pgtype.UUID, vrfID uuid.UUID, vlanID, parentID pgtype.UUID, tags []string, customFields json.RawMessage, createdAt, updatedAt pgtype.Timestamptz, vlanVIDs map[uuid.UUID]int32) subnetDTO {
	dto := subnetDTO{
		ID:           id,
		CIDR:         cidr,
		Name:         name,
		Description:  description,
		LocationID:   db.NullUUID(locationID),
		VrfID:        vrfID,
		VlanID:       db.NullUUID(vlanID),
		ParentID:     db.NullUUID(parentID),
		Tags:         normalizeTags(tags),
		CustomFields: decodeCustomFields(customFields),
		CreatedAt:    createdAt.Time,
		UpdatedAt:    updatedAt.Time,
	}
	if dto.VlanID != nil && vlanVIDs != nil {
		if vid, ok := vlanVIDs[*dto.VlanID]; ok {
			dto.VlanVID = &vid
		}
	}
	return dto
}

func (a *API) vlanVIDMap(ctx context.Context) map[uuid.UUID]int32 {
	items, err := a.Store.Queries.ListVLANs(ctx)
	out := map[uuid.UUID]int32{}
	if err != nil {
		return out
	}
	for _, v := range items {
		out[v.ID] = v.Vid
	}
	return out
}

func resolveVrfID(id *uuid.UUID) uuid.UUID {
	if id == nil || *id == uuid.Nil {
		return db.DefaultVRFID
	}
	return *id
}

// resolveVLANFK finds or creates a VLAN row for vid+location and returns its UUID FK.
func (a *API) resolveVLANFK(ctx context.Context, vid *int32, locationID pgtype.UUID) (pgtype.UUID, error) {
	if vid == nil {
		return pgtype.UUID{}, nil
	}
	if *vid < 1 || *vid > 4094 {
		return pgtype.UUID{}, fmt.Errorf("vlan_vid must be between 1 and 4094")
	}
	existing, err := a.Store.Queries.GetVLANByVidLocation(ctx, sqlcdb.GetVLANByVidLocationParams{
		Vid: *vid, LocationID: locationID,
	})
	if err == nil {
		return db.UUIDToNull(existing.ID), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, err
	}
	created, err := a.Store.Queries.CreateVLAN(ctx, sqlcdb.CreateVLANParams{
		Vid: *vid, Name: fmt.Sprintf("VLAN %d", *vid), LocationID: locationID, Description: "",
	})
	if err != nil {
		return pgtype.UUID{}, err
	}
	return db.UUIDToNull(created.ID), nil
}

func (a *API) ListSubnetTags(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	tags, err := a.Store.Queries.ListSubnetTags(r.Context(), sqlcdb.ListSubnetTagsParams{
		Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tags == nil {
		tags = []string{}
	}
	httpx.JSON(w, http.StatusOK, tags)
}

func parseSubnetTagFilter(r *http.Request) (tags []string, match string) {
	match = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tag_match")))
	if match != "or" {
		match = "and"
	}
	seen := map[string]bool{}
	add := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			t := strings.TrimSpace(strings.ToLower(part))
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			tags = append(tags, t)
		}
	}
	add(r.URL.Query().Get("tags"))
	add(r.URL.Query().Get("tag"))
	if tags == nil {
		tags = []string{}
	}
	return tags, match
}

func (a *API) ListSubnets(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	pq := httpx.ParsePage(r)
	sq := httpx.ParseSort(r, map[string]bool{
		"cidr": true, "name": true, "created_at": true, "updated_at": true,
	}, "cidr", "asc")
	tags, tagMatch := parseSubnetTagFilter(r)
	total, err := a.Store.Queries.CountSubnetsFiltered(r.Context(), sqlcdb.CountSubnetsFilteredParams{
		Query: pq.Q, Tags: tags, TagMatch: tagMatch, Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListSubnetsPaged(r.Context(), sqlcdb.ListSubnetsPagedParams{
		Query: pq.Q, Tags: tags, TagMatch: tagMatch, Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(),
		SortBy: sq.By, SortDir: sq.Dir, RowOffset: pq.Offset, RowLimit: pq.PageSize,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	counts, _ := a.Store.Queries.ListSubnetIPCounts(r.Context())
	usedBy := map[uuid.UUID]int{}
	for _, c := range counts {
		usedBy[c.SubnetID] = int(c.UsedCount)
	}
	vids := a.vlanVIDMap(r.Context())

	out := make([]subnetDTO, 0, len(items))
	for _, item := range items {
		dto := toSubnetDTO(item.ID, item.Cidr, item.Name, item.Description, item.LocationID, item.VrfID, item.VlanID, item.ParentID, item.Tags, item.CustomFields, item.CreatedAt, item.UpdatedAt, vids)
		if prefix, err := ipam.ParsePrefix(item.Cidr); err == nil {
			used := usedBy[item.ID]
			dto.Used = &used
			dto.Sparse = ipam.IsSparsePrefix(prefix)
			if !dto.Sparse {
				usable, uerr := ipam.TotalUsable(prefix)
				if uerr == nil {
					free := usable - used
					if free < 0 {
						free = 0
					}
					pct := 0.0
					if usable > 0 {
						pct = float64(used) / float64(usable) * 100
					}
					dto.Total = &usable
					dto.Free = &free
					dto.Percent = &pct
				}
			}
		}
		out = append(out, dto)
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[subnetDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func (a *API) GetSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.GetSubnet(r.Context(), id)
	if err != nil || item.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	if !a.requireLocationAccess(w, r, item.LocationID) {
		return
	}
	vids := a.vlanVIDMap(r.Context())
	dto := toSubnetDTO(item.ID, item.Cidr, item.Name, item.Description, item.LocationID, item.VrfID, item.VlanID, item.ParentID, item.Tags, item.CustomFields, item.CreatedAt, item.UpdatedAt, vids)
	if util, err := a.subnetUtilization(r.Context(), id, item.Cidr); err == nil {
		dto.Used = &util.Used
		dto.Total = util.Total
		dto.Free = util.Free
		dto.Percent = util.Percent
		dto.Sparse = util.Sparse
		dto.NextIP = util.NextIP
	}
	httpx.JSON(w, http.StatusOK, dto)
}

func (a *API) GetSubnetUtilization(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.GetSubnet(r.Context(), id)
	if err != nil || item.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	if !a.requireLocationAccess(w, r, item.LocationID) {
		return
	}
	util, err := a.subnetUtilization(r.Context(), id, item.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, util)
}

type ipMapCell struct {
	Address  string  `json:"address"`
	Status   string  `json:"status"`
	Hostname string  `json:"hostname,omitempty"`
	ID       *string `json:"id,omitempty"`
	Special  bool    `json:"special,omitempty"`
}

type ipMapResponse struct {
	CIDR       string      `json:"cidr"`
	PrefixLen  int         `json:"prefix_len"`
	Supported  bool        `json:"supported"`
	Message    string      `json:"message,omitempty"`
	Columns    int         `json:"columns"`
	Cells      []ipMapCell `json:"cells"`
	Children   []subnetDTO `json:"children,omitempty"`
	Percent    *float64    `json:"percent,omitempty"`
	Used       int         `json:"used"`
	Total      *int        `json:"total,omitempty"`
	Sparse     bool        `json:"sparse,omitempty"`
}

func (a *API) GetSubnetIPMap(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.GetSubnet(r.Context(), id)
	if err != nil || item.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	if !a.requireLocationAccess(w, r, item.LocationID) {
		return
	}
	prefix, err := ipam.ParsePrefix(item.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet cidr")
		return
	}
	util, _ := a.subnetUtilization(r.Context(), id, item.Cidr)
	addrs, err := ipam.MapAddresses(prefix)
	if err != nil {
		kids, _ := a.Store.Queries.ListChildSubnets(r.Context(), db.UUIDToNull(id))
		vids := a.vlanVIDMap(r.Context())
		children := make([]subnetDTO, 0, len(kids))
		for _, k := range kids {
			children = append(children, toSubnetDTO(k.ID, k.Cidr, k.Name, k.Description, k.LocationID, k.VrfID, k.VlanID, k.ParentID, k.Tags, k.CustomFields, k.CreatedAt, k.UpdatedAt, vids))
		}
		msg := err.Error()
		if prefix.Addr().Is6() {
			msg += ". Sparse IPv6 prefixes are not gridded — register addresses directly, use IP ranges, or open a /120 (or denser) child."
		} else {
			msg += ". Open a child /24 or smaller to view the address grid."
		}
		httpx.JSON(w, http.StatusOK, ipMapResponse{
			CIDR: item.Cidr, PrefixLen: prefix.Bits(), Supported: false,
			Message: msg, Children: children,
			Percent: util.Percent, Used: util.Used, Total: util.Total, Sparse: util.Sparse,
		})
		return
	}
	rows, err := a.Store.Queries.ListIPMapBySubnet(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	byAddr := map[string]sqlcdb.ListIPMapBySubnetRow{}
	for _, row := range rows {
		byAddr[row.Address] = row
	}
	// Prefer 20-wide rows (0–19, 20–39, …) so a /24 fits without horizontal waste.
	cols := 20
	if len(addrs) <= 20 {
		cols = max(1, len(addrs))
	} else if len(addrs) <= 64 {
		cols = 10
	}
	cells := make([]ipMapCell, 0, len(addrs))
	for _, addr := range addrs {
		cell := ipMapCell{Address: addr.String(), Status: "empty", Special: ipam.IsNetworkOrBroadcast(prefix, addr)}
		if cell.Special {
			cell.Status = "special"
		}
		if row, ok := byAddr[addr.String()]; ok {
			idStr := row.ID.String()
			cell.ID = &idStr
			cell.Hostname = row.Hostname
			if row.DeletedAt.Valid {
				cell.Status = "deleted"
			} else {
				cell.Status = row.Status
			}
		}
		cells = append(cells, cell)
	}
	httpx.JSON(w, http.StatusOK, ipMapResponse{
		CIDR: item.Cidr, PrefixLen: prefix.Bits(), Supported: true,
		Columns: cols, Cells: cells,
		Percent: util.Percent, Used: util.Used, Total: util.Total, Sparse: util.Sparse,
	})
}

func (a *API) AllocateNextIP(w http.ResponseWriter, r *http.Request) {
	subnetID, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet id")
		return
	}
	if !a.requireSubnetAccess(w, r, subnetID) {
		return
	}
	var req struct {
		Hostname string `json:"hostname"`
		Notes    string `json:"notes"`
		Status   string `json:"status"`
		Start    string `json:"start"` // optional hint cursor (sparse allocate)
	}
	if err := httpx.Decode(r, &req); err != nil && r.ContentLength > 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Status == "" {
		req.Status = "assigned"
	}
	if !validIPStatus[req.Status] {
		httpx.Error(w, http.StatusBadRequest, "invalid status")
		return
	}

	subnet, err := a.Store.Queries.GetSubnet(r.Context(), subnetID)
	if err != nil || subnet.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	prefix, err := ipam.ParsePrefix(subnet.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet cidr")
		return
	}
	var startHint netip.Addr
	if req.Start != "" {
		hint, herr := netip.ParseAddr(req.Start)
		if herr != nil || !prefix.Contains(hint) {
			httpx.Error(w, http.StatusBadRequest, "start must be an address inside the subnet")
			return
		}
		startHint = hint
	}
	var reservedUntil pgtype.Timestamptz
	if req.Status == "reserved" {
		reservedUntil = pgtype.Timestamptz{Time: time.Now().UTC().Add(30 * 24 * time.Hour), Valid: true}
	}

	var item sqlcdb.CreateIPRow
	for attempt := 0; attempt < 5; attempt++ {
		err = a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
			usedAddrs, err := q.ListOccupiedAddressesBySubnet(r.Context(), subnetID)
			if err != nil {
				return err
			}
			used := ipam.UsedSet(usedAddrs)
			var next netip.Addr
			if ipam.IsSparsePrefix(prefix) {
				next, err = ipam.NextFreeSparse(prefix, used, startHint)
			} else {
				next, err = ipam.NextFree(prefix, used)
			}
			if err != nil {
				return err
			}
			created, err := q.CreateIP(r.Context(), sqlcdb.CreateIPParams{
				Address:       next.String(),
				SubnetID:      subnetID,
				VrfID:         subnet.VrfID,
				Hostname:      req.Hostname,
				Status:        req.Status,
				DeviceTypeID:  pgtype.UUID{},
				LocationID:    subnet.LocationID,
				Notes:         req.Notes,
				ReservedUntil: reservedUntil,
				LastSeenAt:    pgtype.Timestamptz{},
				MacAddress:    "",
				OpenPorts:     "",
				CustomFields:  emptyJSON(),
			})
			if err != nil {
				return err
			}
			item = created
			return nil
		})
		if err == nil {
			break
		}
		if !strings.Contains(strings.ToLower(err.Error()), "unique") &&
			!strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			break
		}
	}
	if err != nil {
		if strings.Contains(err.Error(), "no free") {
			httpx.Error(w, http.StatusConflict, err.Error())
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "ip.allocate_next", "ip_address", item.ID.String(), map[string]any{
		"address": item.Address, "subnet_id": subnetID.String(),
	})
	httpx.JSON(w, http.StatusCreated, toIPDTOFromRow(item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.ReservedUntil, item.LastSeenAt, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.CustomFields, item.CreatedAt, item.UpdatedAt))
}

func (a *API) ListSubnetChildren(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !a.requireSubnetAccess(w, r, id) {
		return
	}
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	items, err := a.Store.Queries.ListChildSubnets(r.Context(), db.UUIDToNull(id))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	vids := a.vlanVIDMap(r.Context())
	out := make([]subnetDTO, 0, len(items))
	for _, item := range items {
		if !sc.AllowsNullUUID(item.LocationID) {
			continue
		}
		out = append(out, toSubnetDTO(item.ID, item.Cidr, item.Name, item.Description, item.LocationID, item.VrfID, item.VlanID, item.ParentID, item.Tags, item.CustomFields, item.CreatedAt, item.UpdatedAt, vids))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) SubnetTree(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	items, err := a.Store.Queries.ListSubnets(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	vids := a.vlanVIDMap(r.Context())
	nodes := make(map[uuid.UUID]*subnetTreeNode, len(items))
	for _, item := range items {
		if !sc.AllowsNullUUID(item.LocationID) {
			continue
		}
		dto := toSubnetDTO(item.ID, item.Cidr, item.Name, item.Description, item.LocationID, item.VrfID, item.VlanID, item.ParentID, item.Tags, item.CustomFields, item.CreatedAt, item.UpdatedAt, vids)
		nodes[item.ID] = &subnetTreeNode{subnetDTO: dto, Children: []subnetTreeNode{}}
	}
	childrenOf := map[uuid.UUID][]uuid.UUID{}
	var rootIDs []uuid.UUID
	for id, node := range nodes {
		if node.ParentID != nil {
			if _, ok := nodes[*node.ParentID]; ok {
				childrenOf[*node.ParentID] = append(childrenOf[*node.ParentID], id)
				continue
			}
		}
		rootIDs = append(rootIDs, id)
	}
	var build func(id uuid.UUID) subnetTreeNode
	build = func(id uuid.UUID) subnetTreeNode {
		n := *nodes[id]
		kids := childrenOf[id]
		n.Children = make([]subnetTreeNode, 0, len(kids))
		for _, cid := range kids {
			n.Children = append(n.Children, build(cid))
		}
		return n
	}
	roots := make([]subnetTreeNode, 0, len(rootIDs))
	for _, id := range rootIDs {
		roots = append(roots, build(id))
	}
	httpx.JSON(w, http.StatusOK, roots)
}

func (a *API) CreateSubnet(w http.ResponseWriter, r *http.Request) {
	var req subnetRequest
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if !a.requireLocationAccess(w, r, db.PtrUUIDToNull(req.LocationID)) {
		return
	}
	prefix, err := ipam.ParsePrefix(req.CIDR)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid cidr")
		return
	}
	vrfID := resolveVrfID(req.VrfID)
	parentID, err := a.resolveSubnetParent(r.Context(), prefix, vrfID, req.ParentID, uuid.Nil)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.validateSubnetCustomFields(r, req.CustomFields); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	locID := db.PtrUUIDToNull(req.LocationID)
	vlanFK, err := a.resolveVLANFK(r.Context(), req.VlanVID, locID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := a.Store.Queries.CreateSubnet(r.Context(), sqlcdb.CreateSubnetParams{
		Cidr:         prefix.String(),
		Name:         req.Name,
		Description:  req.Description,
		LocationID:   locID,
		VrfID:        vrfID,
		VlanID:       vlanFK,
		ParentID:     parentID,
		Tags:         normalizeTags(req.Tags),
		CustomFields: encodeCustomFields(req.CustomFields),
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.reparentContained(r.Context(), item.ID, prefix, vrfID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "subnet.create", "subnet", item.ID.String(), map[string]any{"cidr": prefix.String(), "name": req.Name})
	fresh, _ := a.Store.Queries.GetSubnet(r.Context(), item.ID)
	vids := a.vlanVIDMap(r.Context())
	httpx.JSON(w, http.StatusCreated, toSubnetDTO(fresh.ID, fresh.Cidr, fresh.Name, fresh.Description, fresh.LocationID, fresh.VrfID, fresh.VlanID, fresh.ParentID, fresh.Tags, fresh.CustomFields, fresh.CreatedAt, fresh.UpdatedAt, vids))
}

func (a *API) UpdateSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetSubnet(r.Context(), id)
	if err != nil || existing.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	if !a.requireLocationAccess(w, r, existing.LocationID) {
		return
	}
	var req subnetRequest
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if !a.requireLocationAccess(w, r, db.PtrUUIDToNull(req.LocationID)) {
		return
	}
	prefix, err := ipam.ParsePrefix(req.CIDR)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid cidr")
		return
	}
	vrfID := existing.VrfID
	if req.VrfID != nil {
		vrfID = resolveVrfID(req.VrfID)
	}
	parentID, err := a.resolveSubnetParent(r.Context(), prefix, vrfID, req.ParentID, id)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	tags := normalizeTags(req.Tags)
	if req.Tags == nil {
		tags = normalizeTags(existing.Tags)
	}
	cf := encodeCustomFields(req.CustomFields)
	if req.CustomFields == nil {
		cf = existing.CustomFields
		if len(cf) == 0 {
			cf = json.RawMessage(`{}`)
		}
	} else if err := a.validateSubnetCustomFields(r, req.CustomFields); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	locID := db.PtrUUIDToNull(req.LocationID)
	vlanFK, err := a.resolveVLANFK(r.Context(), req.VlanVID, locID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := a.Store.Queries.UpdateSubnet(r.Context(), sqlcdb.UpdateSubnetParams{
		ID:           id,
		Cidr:         prefix.String(),
		Name:         req.Name,
		Description:  req.Description,
		LocationID:   locID,
		VrfID:        vrfID,
		VlanID:       vlanFK,
		ParentID:     parentID,
		Tags:         tags,
		CustomFields: cf,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.reparentContained(r.Context(), item.ID, prefix, vrfID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	fresh, _ := a.Store.Queries.GetSubnet(r.Context(), item.ID)
	vids := a.vlanVIDMap(r.Context())
	httpx.JSON(w, http.StatusOK, toSubnetDTO(fresh.ID, fresh.Cidr, fresh.Name, fresh.Description, fresh.LocationID, fresh.VrfID, fresh.VlanID, fresh.ParentID, fresh.Tags, fresh.CustomFields, fresh.CreatedAt, fresh.UpdatedAt, vids))
}

func (a *API) DeleteSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !a.requireSubnetAccess(w, r, id) {
		return
	}
	item, err := a.Store.Queries.SoftDeleteSubnet(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "subnet.delete", "subnet", id.String(), map[string]any{
		"soft": true, "cidr": item.Cidr, "name": item.Name,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) ListDeletedSubnets(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	pq := httpx.ParsePage(r)
	total, err := a.Store.Queries.CountDeletedSubnets(r.Context(), pq.Q)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListDeletedSubnets(r.Context(), sqlcdb.ListDeletedSubnetsParams{
		Query: pq.Q, RowLimit: pq.PageSize, RowOffset: pq.Offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	type deletedSubnetDTO struct {
		subnetDTO
		DeletedAt time.Time `json:"deleted_at"`
	}
	vids := a.vlanVIDMap(r.Context())
	out := make([]deletedSubnetDTO, 0, len(items))
	for _, item := range items {
		if !sc.AllowsNullUUID(item.LocationID) {
			continue
		}
		dto := toSubnetDTO(item.ID, item.Cidr, item.Name, item.Description, item.LocationID, item.VrfID, item.VlanID, item.ParentID, item.Tags, item.CustomFields, item.CreatedAt, item.UpdatedAt, vids)
		out = append(out, deletedSubnetDTO{subnetDTO: dto, DeletedAt: item.DeletedAt.Time})
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[deletedSubnetDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func (a *API) RestoreDeletedSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetSubnet(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	if !existing.DeletedAt.Valid {
		httpx.Error(w, http.StatusBadRequest, "subnet is not deleted")
		return
	}
	if !a.requireLocationAccess(w, r, existing.LocationID) {
		return
	}
	item, err := a.Store.Queries.RestoreSubnet(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "subnet.restore", "subnet", id.String(), map[string]any{"cidr": item.Cidr})
	vids := a.vlanVIDMap(r.Context())
	httpx.JSON(w, http.StatusOK, toSubnetDTO(item.ID, item.Cidr, item.Name, item.Description, item.LocationID, item.VrfID, item.VlanID, item.ParentID, item.Tags, item.CustomFields, item.CreatedAt, item.UpdatedAt, vids))
}

// usedAddressesForUtil returns reserved_until-aware assigned/reserved/dhcp addresses.
// Allocate-next continues to use ListOccupiedAddressesBySubnet (any live row).
func (a *API) usedAddressesForUtil(ctx context.Context, q *sqlcdb.Queries, subnetID uuid.UUID) ([]string, error) {
	if q == nil {
		q = a.Store.Queries
	}
	return q.ListUsedAddressesBySubnet(ctx, subnetID)
}

func (a *API) subnetUtilization(ctx context.Context, subnetID uuid.UUID, cidr string) (ipam.Utilization, error) {
	prefix, err := ipam.ParsePrefix(cidr)
	if err != nil {
		return ipam.Utilization{}, err
	}
	used, err := a.usedAddressesForUtil(ctx, nil, subnetID)
	if err != nil {
		return ipam.Utilization{}, err
	}
	return ipam.ComputeUtilization(prefix, used), nil
}

func (a *API) resolveSubnetParent(ctx context.Context, prefix netip.Prefix, vrfID uuid.UUID, requested *uuid.UUID, exclude uuid.UUID) (pgtype.UUID, error) {
	excludeParam := pgtype.UUID{}
	if exclude != uuid.Nil {
		excludeParam = db.UUIDToNull(exclude)
	}
	overlapping, err := a.Store.Queries.FindOverlappingSubnets(ctx, sqlcdb.FindOverlappingSubnetsParams{
		VrfID:     vrfID,
		Cidr:      prefix.String(),
		ExcludeID: excludeParam,
	})
	if err != nil {
		return pgtype.UUID{}, err
	}
	for _, o := range overlapping {
		other, err := ipam.ParsePrefix(o.Cidr)
		if err != nil {
			continue
		}
		switch ipam.Classify(prefix, other) {
		case ipam.RelEqual:
			return pgtype.UUID{}, fmt.Errorf("subnet %s already exists", prefix)
		case ipam.RelOverlap:
			return pgtype.UUID{}, fmt.Errorf("subnet %s partially overlaps existing %s", prefix, other)
		}
	}

	if requested != nil {
		parent, err := a.Store.Queries.GetSubnet(ctx, *requested)
		if err != nil || parent.DeletedAt.Valid {
			return pgtype.UUID{}, fmt.Errorf("parent subnet not found")
		}
		if parent.VrfID != vrfID {
			return pgtype.UUID{}, fmt.Errorf("parent subnet must be in the same VRF")
		}
		parentPrefix, err := ipam.ParsePrefix(parent.Cidr)
		if err != nil {
			return pgtype.UUID{}, err
		}
		if !ipam.ContainsPrefix(parentPrefix, prefix) || parentPrefix == prefix {
			return pgtype.UUID{}, fmt.Errorf("subnet must be strictly contained in parent %s", parent.Cidr)
		}
		return db.UUIDToNull(*requested), nil
	}

	parent, err := a.Store.Queries.FindTightestParent(ctx, sqlcdb.FindTightestParentParams{
		VrfID: vrfID, Cidr: prefix.String(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, nil
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	return db.UUIDToNull(parent.ID), nil
}

// reparentContained links existing child CIDRs under a newly created/updated supernet.
func (a *API) reparentContained(ctx context.Context, parentID uuid.UUID, prefix netip.Prefix, vrfID uuid.UUID) error {
	return a.reparentContainedQ(ctx, a.Store.Queries, parentID, prefix, vrfID)
}

func (a *API) reparentContainedQ(ctx context.Context, q *sqlcdb.Queries, parentID uuid.UUID, prefix netip.Prefix, vrfID uuid.UUID) error {
	overlapping, err := q.FindOverlappingSubnets(ctx, sqlcdb.FindOverlappingSubnetsParams{
		VrfID:     vrfID,
		Cidr:      prefix.String(),
		ExcludeID: db.UUIDToNull(parentID),
	})
	if err != nil {
		return err
	}
	for _, o := range overlapping {
		other, err := ipam.ParsePrefix(o.Cidr)
		if err != nil {
			continue
		}
		if ipam.Classify(prefix, other) != ipam.RelContains {
			continue
		}
		// Only reparent if currently root or already under a looser ancestor.
		if o.ParentID.Valid {
			currentParent, err := q.GetSubnet(ctx, uuid.UUID(o.ParentID.Bytes))
			if err == nil {
				curPrefix, err := ipam.ParsePrefix(currentParent.Cidr)
				if err == nil && curPrefix.Bits() > prefix.Bits() {
					// already has a tighter parent
					continue
				}
			}
		}
		if err := q.SetSubnetParent(ctx, sqlcdb.SetSubnetParentParams{
			ID:       o.ID,
			ParentID: db.UUIDToNull(parentID),
		}); err != nil {
			return err
		}
	}
	return nil
}
