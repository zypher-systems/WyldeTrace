package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/scanner"
)

type findingDTO struct {
	ID            uuid.UUID  `json:"id"`
	SubnetID      uuid.UUID  `json:"subnet_id"`
	Address       string     `json:"address"`
	Kind          string     `json:"kind"`
	Hostname      string     `json:"hostname"`
	MACAddress    string     `json:"mac_address"`
	OpenPorts     string     `json:"open_ports"`
	InterfaceName string     `json:"interface_name,omitempty"`
	SwitchID      string     `json:"switch_id,omitempty"`
	DBIPID        *uuid.UUID `json:"db_ip_id,omitempty"`
	DBStatus      string     `json:"db_status"`
	JobID         *uuid.UUID `json:"job_id,omitempty"`
	FirstSeenAt   time.Time  `json:"first_seen_at"`
	LastSeenAt    time.Time  `json:"last_seen_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	ResolveAction string     `json:"resolve_action,omitempty"`
	SubnetCIDR     string `json:"subnet_cidr,omitempty"`
	SubnetName     string `json:"subnet_name,omitempty"`
	LocationID     string `json:"location_id,omitempty"`
	LocationName   string `json:"location_name,omitempty"`
}

func findingDTOFrom(
	id, subnetID uuid.UUID,
	address, kind, hostname, mac, ports, iface, switchID string,
	dbIPID, jobID pgtype.UUID,
	dbStatus string,
	firstSeen, lastSeen, resolvedAt pgtype.Timestamptz,
	resolveAction string,
) findingDTO {
	return findingDTO{
		ID: id, SubnetID: subnetID, Address: address, Kind: kind,
		Hostname: hostname, MACAddress: mac, OpenPorts: ports,
		InterfaceName: iface, SwitchID: switchID,
		DBIPID: nullUUIDPtr(dbIPID), DBStatus: dbStatus, JobID: nullUUIDPtr(jobID),
		FirstSeenAt: firstSeen.Time, LastSeenAt: lastSeen.Time,
		ResolvedAt: timestamptzPtr(resolvedAt), ResolveAction: resolveAction,
	}
}

func nullUUIDPtr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	return &id
}

func (a *API) ListInbox(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	pq := httpx.ParsePage(r)
	sq := httpx.ParseSort(r, map[string]bool{
		"last_seen_at": true, "first_seen_at": true, "address": true, "kind": true,
	}, "last_seen_at", "desc")
	kind := r.URL.Query().Get("kind")
	var subnetID pgtype.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("subnet_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid subnet_id")
			return
		}
		if !a.requireSubnetAccess(w, r, id) {
			return
		}
		subnetID = db.UUIDToNull(id)
	}
	var filterLoc pgtype.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("location_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid location_id")
			return
		}
		if !sc.Allows(&id) {
			httpx.Error(w, http.StatusForbidden, "location out of scope")
			return
		}
		filterLoc = db.UUIDToNull(id)
	}
	params := sqlcdb.CountOpenFindingsParams{
		Kind: kind, SubnetID: subnetID, LocationID: filterLoc,
		Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(),
	}
	total, err := a.Store.Queries.CountOpenFindings(r.Context(), params)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListOpenFindings(r.Context(), sqlcdb.ListOpenFindingsParams{
		Kind: kind, SubnetID: subnetID, LocationID: filterLoc,
		Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(),
		SortBy: sq.By, SortDir: sq.Dir, RowLimit: pq.PageSize, RowOffset: pq.Offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]findingDTO, 0, len(items))
	for _, item := range items {
		subnet, err := a.Store.Queries.GetSubnet(r.Context(), item.SubnetID)
		if err != nil {
			continue
		}
		dto := findingDTOFrom(item.ID, item.SubnetID, item.Address, item.Kind, item.Hostname, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.DbIpID, item.JobID, item.DbStatus, item.FirstSeenAt, item.LastSeenAt, item.ResolvedAt, item.ResolveAction)
		dto.SubnetCIDR = subnet.Cidr
		dto.SubnetName = subnet.Name
		if subnet.LocationID.Valid {
			locID := uuid.UUID(subnet.LocationID.Bytes)
			dto.LocationID = locID.String()
			if loc, lerr := a.Store.Queries.GetLocation(r.Context(), locID); lerr == nil {
				dto.LocationName = loc.Name
			}
		}
		out = append(out, dto)
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[findingDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func (a *API) DismissFinding(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	f, err := a.Store.Queries.GetFinding(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "finding not found")
		return
	}
	if !a.requireSubnetAccess(w, r, f.SubnetID) {
		return
	}
	item, err := a.Store.Queries.ResolveFinding(r.Context(), sqlcdb.ResolveFindingParams{
		ID: id, ResolveAction: "dismissed",
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "inbox.dismiss", "discovery_finding", id.String(), map[string]any{
		"address": item.Address, "kind": item.Kind,
	})
	httpx.JSON(w, http.StatusOK, findingDTOFrom(item.ID, item.SubnetID, item.Address, item.Kind, item.Hostname, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.DbIpID, item.JobID, item.DbStatus, item.FirstSeenAt, item.LastSeenAt, item.ResolvedAt, item.ResolveAction))
}

func (a *API) ClaimFinding(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	f, err := a.Store.Queries.GetFinding(r.Context(), id)
	if err != nil || f.ResolvedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "finding not found")
		return
	}
	if f.Kind != "new_alive" {
		httpx.Error(w, http.StatusBadRequest, "only new_alive findings can be claimed")
		return
	}
	resp, code, msg := a.doClaim(r, claimRequest{
		SubnetID: f.SubnetID,
		Hosts: []claimHost{{
			Address: f.Address, Hostname: f.Hostname, MAC: f.MacAddress, OpenPorts: f.OpenPorts,
		}},
		Status: "assigned",
		Notes:  "claimed from inbox",
	})
	if msg != "" {
		httpx.Error(w, code, msg)
		return
	}
	_, _ = a.Store.Queries.ResolveFinding(r.Context(), sqlcdb.ResolveFindingParams{
		ID: id, ResolveAction: "claimed",
	})
	_ = a.Store.Queries.ResolveFindingsByAddress(r.Context(), sqlcdb.ResolveFindingsByAddressParams{
		ResolveAction: "claimed", SubnetID: f.SubnetID, Address: f.Address,
	})
	a.audit(r.Context(), "inbox.claim", "discovery_finding", id.String(), map[string]any{
		"address": f.Address, "created": resp.Created, "updated": resp.Updated,
	})
	httpx.JSON(w, http.StatusOK, resp)
}

func (a *API) MarkFindingAvailable(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	f, err := a.Store.Queries.GetFinding(r.Context(), id)
	if err != nil || f.ResolvedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "finding not found")
		return
	}
	if f.Kind != "stale" {
		httpx.Error(w, http.StatusBadRequest, "only stale findings can be marked available")
		return
	}
	if !a.requireSubnetAccess(w, r, f.SubnetID) {
		return
	}
	if f.DbIpID.Valid {
		ipID := uuid.UUID(f.DbIpID.Bytes)
		existing, err := a.Store.Queries.GetIP(r.Context(), ipID)
		if err == nil && !existing.DeletedAt.Valid {
			_, err = a.Store.Queries.UpdateIP(r.Context(), sqlcdb.UpdateIPParams{
				ID: existing.ID, Address: existing.Address, SubnetID: existing.SubnetID, VrfID: existing.VrfID,
				Hostname: existing.Hostname, Status: "available",
				DeviceTypeID: existing.DeviceTypeID, LocationID: existing.LocationID,
				Notes: existing.Notes, ReservedUntil: pgtype.Timestamptz{},
				LastSeenAt: existing.LastSeenAt, MacAddress: existing.MacAddress, OpenPorts: existing.OpenPorts,
				InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
				CustomFields: existing.CustomFields,
			})
			if err != nil {
				httpx.Error(w, http.StatusBadRequest, err.Error())
				return
			}
		}
	}
	item, err := a.Store.Queries.ResolveFinding(r.Context(), sqlcdb.ResolveFindingParams{
		ID: id, ResolveAction: "marked_available",
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "inbox.mark_available", "discovery_finding", id.String(), map[string]any{
		"address": item.Address,
	})
	httpx.JSON(w, http.StatusOK, findingDTOFrom(item.ID, item.SubnetID, item.Address, item.Kind, item.Hostname, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.DbIpID, item.JobID, item.DbStatus, item.FirstSeenAt, item.LastSeenAt, item.ResolvedAt, item.ResolveAction))
}

type bulkInboxRequest struct {
	IDs    []uuid.UUID `json:"ids"`
	Action string      `json:"action"` // dismiss | claim | mark-available
}

func (a *API) BulkInbox(w http.ResponseWriter, r *http.Request) {
	var req bulkInboxRequest
	if err := httpx.Decode(r, &req); err != nil || len(req.IDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "ids required")
		return
	}
	switch req.Action {
	case "dismiss", "claim", "mark-available":
	default:
		httpx.Error(w, http.StatusBadRequest, "action must be dismiss, claim, or mark-available")
		return
	}
	okN, skipN := 0, 0
	errors := []string{}
	for _, id := range req.IDs {
		f, err := a.Store.Queries.GetFinding(r.Context(), id)
		if err != nil || f.ResolvedAt.Valid {
			skipN++
			continue
		}
		if !a.requireSubnetAccess(w, r, f.SubnetID) {
			return
		}
		switch req.Action {
		case "dismiss":
			if _, err := a.Store.Queries.ResolveFinding(r.Context(), sqlcdb.ResolveFindingParams{
				ID: id, ResolveAction: "dismissed",
			}); err != nil {
				skipN++
				errors = append(errors, id.String()+": "+err.Error())
				continue
			}
			okN++
		case "claim":
			if f.Kind != "new_alive" {
				skipN++
				continue
			}
			_, code, msg := a.doClaim(r, claimRequest{
				SubnetID: f.SubnetID,
				Hosts: []claimHost{{
					Address: f.Address, Hostname: f.Hostname, MAC: f.MacAddress, OpenPorts: f.OpenPorts,
				}},
				Status: "assigned",
				Notes:  "claimed from inbox bulk",
			})
			if msg != "" {
				skipN++
				errors = append(errors, f.Address+": "+msg)
				_ = code
				continue
			}
			_, _ = a.Store.Queries.ResolveFinding(r.Context(), sqlcdb.ResolveFindingParams{
				ID: id, ResolveAction: "claimed",
			})
			okN++
		case "mark-available":
			if f.Kind != "stale" {
				skipN++
				continue
			}
			if f.DbIpID.Valid {
				ipID := uuid.UUID(f.DbIpID.Bytes)
				existing, err := a.Store.Queries.GetIP(r.Context(), ipID)
				if err == nil && !existing.DeletedAt.Valid {
					_, _ = a.Store.Queries.UpdateIP(r.Context(), sqlcdb.UpdateIPParams{
						ID: existing.ID, Address: existing.Address, SubnetID: existing.SubnetID, VrfID: existing.VrfID,
						Hostname: existing.Hostname, Status: "available",
						DeviceTypeID: existing.DeviceTypeID, LocationID: existing.LocationID,
						Notes: existing.Notes, ReservedUntil: pgtype.Timestamptz{},
						LastSeenAt: existing.LastSeenAt, MacAddress: existing.MacAddress, OpenPorts: existing.OpenPorts,
						InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
						CustomFields: existing.CustomFields,
					})
				}
			}
			if _, err := a.Store.Queries.ResolveFinding(r.Context(), sqlcdb.ResolveFindingParams{
				ID: id, ResolveAction: "marked_available",
			}); err != nil {
				skipN++
				continue
			}
			okN++
		}
	}
	a.audit(r.Context(), "inbox.bulk", "discovery_finding", "", map[string]any{
		"action": req.Action, "ok": okN, "skipped": skipN, "count": len(req.IDs),
	})
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": okN, "skipped": skipN, "errors": errors})
}

func (a *API) upsertFinding(ctx context.Context, subnetID uuid.UUID, address, kind, hostname, mac, ports string, dbIPID pgtype.UUID, dbStatus string, jobID pgtype.UUID) {
	existing, err := a.Store.Queries.GetOpenFinding(ctx, sqlcdb.GetOpenFindingParams{
		SubnetID: subnetID, Address: address, Kind: kind,
	})
	if err == nil {
		_, _ = a.Store.Queries.TouchFinding(ctx, sqlcdb.TouchFindingParams{
			ID: existing.ID, Hostname: hostname, MacAddress: mac, OpenPorts: ports,
			InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
			DbIpID: dbIPID, DbStatus: dbStatus, JobID: jobID,
		})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return
	}
	_, _ = a.Store.Queries.InsertFinding(ctx, sqlcdb.InsertFindingParams{
		SubnetID: subnetID, Address: address, Kind: kind,
		Hostname: hostname, MacAddress: mac, OpenPorts: ports,
		DbIpID: dbIPID, DbStatus: dbStatus, JobID: jobID,
	})
}

func (a *API) recordReconcileFindings(ctx context.Context, subnetID uuid.UUID, rec *reconcileReport, jobID pgtype.UUID) {
	if a == nil || rec == nil {
		return
	}
	for _, h := range rec.NewAlive {
		a.upsertFinding(ctx, subnetID, h.IP, "new_alive", h.Hostname, h.MAC, scanner.FormatPorts(h.Ports), pgtype.UUID{}, "", jobID)
	}
	for _, h := range rec.Stale {
		var ipID pgtype.UUID
		if h.DBID != "" {
			if id, err := uuid.Parse(h.DBID); err == nil {
				ipID = pgtype.UUID{Bytes: id, Valid: true}
			}
		}
		a.upsertFinding(ctx, subnetID, h.IP, "stale", h.Hostname, "", "", ipID, h.Status, jobID)
	}
}
