package handlers

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
	"github.com/wyldetechnologies/wyldetrace/internal/oui"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

var validIPStatus = map[string]bool{
	"available": true,
	"assigned":  true,
	"reserved":  true,
	"dhcp":      true,
}

type ipDTO struct {
	ID            uuid.UUID      `json:"id"`
	Address       string         `json:"address"`
	SubnetID      uuid.UUID      `json:"subnet_id"`
	Hostname      string         `json:"hostname"`
	Status        string         `json:"status"`
	DeviceTypeID  *uuid.UUID     `json:"device_type_id"`
	LocationID    *uuid.UUID     `json:"location_id"`
	Notes         string         `json:"notes"`
	ReservedUntil *time.Time     `json:"reserved_until"`
	LastSeenAt    *time.Time     `json:"last_seen_at"`
	MACAddress    string         `json:"mac_address"`
	OpenPorts     string         `json:"open_ports"`
	InterfaceName string         `json:"interface_name,omitempty"`
	SwitchID      string         `json:"switch_id,omitempty"`
	CustomFields  map[string]any `json:"custom_fields"`
	Vendor        string         `json:"vendor,omitempty"`
	DeviceHint    string         `json:"device_hint,omitempty"`
	DeletedAt     *time.Time     `json:"deleted_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type ipRequest struct {
	Address       string         `json:"address"`
	Hostname      string         `json:"hostname"`
	Status        string         `json:"status"`
	DeviceTypeID  *uuid.UUID     `json:"device_type_id"`
	LocationID    *uuid.UUID     `json:"location_id"`
	Notes         string         `json:"notes"`
	SubnetID      *uuid.UUID     `json:"subnet_id"`
	ReservedUntil *time.Time     `json:"reserved_until"`
	MACAddress    string         `json:"mac_address"`
	OpenPorts     string         `json:"open_ports"`
	CustomFields  map[string]any `json:"custom_fields"`
}

func timestamptzPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func ptrToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func toIPDTOFromRow(
	id uuid.UUID,
	address string,
	subnetID uuid.UUID,
	hostname, status string,
	deviceTypeID, locationID pgtype.UUID,
	notes string,
	reservedUntil, lastSeenAt pgtype.Timestamptz,
	mac, ports, iface, switchID string,
	customFields json.RawMessage,
	createdAt, updatedAt pgtype.Timestamptz,
) ipDTO {
	return toIPDTOFromRowDeleted(id, address, subnetID, hostname, status, deviceTypeID, locationID, notes, reservedUntil, lastSeenAt, mac, ports, iface, switchID, customFields, createdAt, updatedAt, pgtype.Timestamptz{})
}

func toIPDTOFromRowDeleted(
	id uuid.UUID,
	address string,
	subnetID uuid.UUID,
	hostname, status string,
	deviceTypeID, locationID pgtype.UUID,
	notes string,
	reservedUntil, lastSeenAt pgtype.Timestamptz,
	mac, ports, iface, switchID string,
	customFields json.RawMessage,
	createdAt, updatedAt, deletedAt pgtype.Timestamptz,
) ipDTO {
	sug := oui.SuggestDeviceType(mac, parsePortsCSV(ports), nil)
	return ipDTO{
		ID:            id,
		Address:       address,
		SubnetID:      subnetID,
		Hostname:      hostname,
		Status:        status,
		DeviceTypeID:  db.NullUUID(deviceTypeID),
		LocationID:    db.NullUUID(locationID),
		Notes:         notes,
		ReservedUntil: timestamptzPtr(reservedUntil),
		LastSeenAt:    timestamptzPtr(lastSeenAt),
		MACAddress:    mac,
		OpenPorts:     ports,
		InterfaceName: iface,
		SwitchID:      switchID,
		CustomFields:  decodeCustomFields(customFields),
		Vendor:        sug.Vendor,
		DeviceHint:    sug.DeviceTypeHint,
		DeletedAt:     timestamptzPtr(deletedAt),
		CreatedAt:     createdAt.Time,
		UpdatedAt:     updatedAt.Time,
	}
}

func parsePortsCSV(s string) []int {
	if s == "" {
		return nil
	}
	var out []int
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			n := 0
			ok := false
			for _, c := range part {
				if c < '0' || c > '9' {
					ok = false
					break
				}
				n = n*10 + int(c-'0')
				ok = true
			}
			if ok {
				out = append(out, n)
			}
			start = i + 1
		}
	}
	return out
}

func (a *API) ListIPs(w http.ResponseWriter, r *http.Request) {
	a.listIPsPaged(w, r, pgtype.UUID{})
}

func (a *API) ListIPsBySubnet(w http.ResponseWriter, r *http.Request) {
	subnetID, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet id")
		return
	}
	a.listIPsPaged(w, r, db.UUIDToNull(subnetID))
}

func (a *API) listIPsPaged(w http.ResponseWriter, r *http.Request, subnetID pgtype.UUID) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	if subnetID.Valid {
		if !a.requireSubnetAccess(w, r, uuid.UUID(subnetID.Bytes)) {
			return
		}
	}
	pq := httpx.ParsePage(r)
	sq := httpx.ParseSort(r, map[string]bool{
		"address": true, "hostname": true, "status": true,
		"created_at": true, "updated_at": true, "last_seen_at": true,
	}, "address", "asc")
	total, err := a.Store.Queries.CountIPsFiltered(r.Context(), sqlcdb.CountIPsFilteredParams{
		SubnetID:    subnetID,
		Status:      pq.Status,
		Query:       pq.Q,
		Scoped:      !sc.Unrestricted,
		LocationIds: sc.LocationIDsOrEmpty(),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListIPsPaged(r.Context(), sqlcdb.ListIPsPagedParams{
		SubnetID:    subnetID,
		Status:      pq.Status,
		Query:       pq.Q,
		Scoped:      !sc.Unrestricted,
		LocationIds: sc.LocationIDsOrEmpty(),
		SortBy:      sq.By,
		SortDir:     sq.Dir,
		RowOffset:   pq.Offset,
		RowLimit:    pq.PageSize,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]ipDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toIPDTOFromRow(item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.ReservedUntil, item.LastSeenAt, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.CustomFields, item.CreatedAt, item.UpdatedAt))
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[ipDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func (a *API) CreateIP(w http.ResponseWriter, r *http.Request) {
	subnetID, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet id")
		return
	}
	if !a.requireSubnetAccess(w, r, subnetID) {
		return
	}
	var req ipRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	addr, err := netip.ParseAddr(req.Address)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid address")
		return
	}
	subnet, err := a.Store.Queries.GetSubnet(r.Context(), subnetID)
	if err != nil || subnet.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	prefix, err := ipam.ParsePrefix(subnet.Cidr)
	if err != nil || !prefix.Contains(addr) {
		httpx.Error(w, http.StatusBadRequest, "address must be inside the subnet")
		return
	}
	if req.Status == "" {
		req.Status = "assigned"
	}
	if !validIPStatus[req.Status] {
		httpx.Error(w, http.StatusBadRequest, "invalid status")
		return
	}
	if req.Status == "reserved" && req.ReservedUntil == nil {
		t := time.Now().UTC().Add(30 * 24 * time.Hour)
		req.ReservedUntil = &t
	}
	if err := a.validateCustomFields(r, "ip", req.CustomFields); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := a.Store.Queries.CreateIP(r.Context(), sqlcdb.CreateIPParams{
		Address:       addr.String(),
		SubnetID:      subnetID,
		VrfID:         subnet.VrfID,
		Hostname:      req.Hostname,
		Status:        req.Status,
		DeviceTypeID:  db.PtrUUIDToNull(req.DeviceTypeID),
		LocationID:    db.PtrUUIDToNull(req.LocationID),
		Notes:         req.Notes,
		ReservedUntil: ptrToTimestamptz(req.ReservedUntil),
		LastSeenAt:    pgtype.Timestamptz{},
		MacAddress:    req.MACAddress,
		OpenPorts:     req.OpenPorts,
		CustomFields:  encodeCustomFields(req.CustomFields),
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.recordIPHistory(r.Context(), "create", item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.MacAddress, item.OpenPorts, item.ReservedUntil, item.LastSeenAt, nil)
	a.audit(r.Context(), "ip.create", "ip_address", item.ID.String(), map[string]any{
		"address": item.Address, "status": item.Status, "subnet_id": subnetID.String(),
	})
	if item.Hostname != "" {
		a.syncDNSForIP(r.Context(), "create", item.Hostname, item.Address)
	}
	httpx.JSON(w, http.StatusCreated, toIPDTOFromRow(item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.ReservedUntil, item.LastSeenAt, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.CustomFields, item.CreatedAt, item.UpdatedAt))
}

func (a *API) UpdateIP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetIP(r.Context(), id)
	if err != nil || existing.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "ip not found")
		return
	}
	if !a.requireSubnetAccess(w, r, existing.SubnetID) {
		return
	}
	var req ipRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	addr, err := netip.ParseAddr(req.Address)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid address")
		return
	}
	if !validIPStatus[req.Status] {
		httpx.Error(w, http.StatusBadRequest, "invalid status")
		return
	}
	subnetID := existing.SubnetID
	vrfID := existing.VrfID
	if req.SubnetID != nil {
		subnetID = *req.SubnetID
		if !a.requireSubnetAccess(w, r, subnetID) {
			return
		}
		sub, serr := a.Store.Queries.GetSubnet(r.Context(), subnetID)
		if serr != nil || sub.DeletedAt.Valid {
			httpx.Error(w, http.StatusBadRequest, "subnet not found")
			return
		}
		vrfID = sub.VrfID
	}
	cf := existing.CustomFields
	if req.CustomFields != nil {
		if err := a.validateCustomFields(r, "ip", req.CustomFields); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		cf = encodeCustomFields(req.CustomFields)
	}
	a.recordIPHistory(r.Context(), "update", existing.ID, existing.Address, existing.SubnetID, existing.Hostname, existing.Status, existing.DeviceTypeID, existing.LocationID, existing.Notes, existing.MacAddress, existing.OpenPorts, existing.ReservedUntil, existing.LastSeenAt, map[string]any{"to": req.Address})
	item, err := a.Store.Queries.UpdateIP(r.Context(), sqlcdb.UpdateIPParams{
		ID:            id,
		Address:       addr.String(),
		SubnetID:      subnetID,
		VrfID:         vrfID,
		Hostname:      req.Hostname,
		Status:        req.Status,
		DeviceTypeID:  db.PtrUUIDToNull(req.DeviceTypeID),
		LocationID:    db.PtrUUIDToNull(req.LocationID),
		Notes:         req.Notes,
		ReservedUntil: ptrToTimestamptz(req.ReservedUntil),
		LastSeenAt:    existing.LastSeenAt,
		MacAddress:    firstNonEmpty(req.MACAddress, existing.MacAddress),
		OpenPorts:     firstNonEmpty(req.OpenPorts, existing.OpenPorts),
		InterfaceName: existing.InterfaceName,
		SwitchID:      existing.SwitchID,
		CustomFields:  cf,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "ip.update", "ip_address", item.ID.String(), map[string]any{
		"address": item.Address, "status": item.Status,
	})
	if existing.Hostname != item.Hostname || existing.Address != item.Address {
		if existing.Hostname != "" && (existing.Hostname != item.Hostname || existing.Address != item.Address) {
			a.syncDNSForIP(r.Context(), "delete", existing.Hostname, existing.Address)
		}
		if item.Hostname != "" {
			a.syncDNSForIP(r.Context(), "update", item.Hostname, item.Address)
		}
	}
	httpx.JSON(w, http.StatusOK, toIPDTOFromRow(item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.ReservedUntil, item.LastSeenAt, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.CustomFields, item.CreatedAt, item.UpdatedAt))
}

func (a *API) DeleteIP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetIP(r.Context(), id)
	if err != nil || existing.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "ip not found")
		return
	}
	if !a.requireSubnetAccess(w, r, existing.SubnetID) {
		return
	}
	item, err := a.Store.Queries.SoftDeleteIP(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.recordIPHistory(r.Context(), "delete", item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.MacAddress, item.OpenPorts, item.ReservedUntil, item.LastSeenAt, nil)
	a.audit(r.Context(), "ip.delete", "ip_address", id.String(), map[string]any{"soft": true, "address": item.Address})
	if item.Hostname != "" {
		a.syncDNSForIP(r.Context(), "delete", item.Hostname, item.Address)
	}
	if a.Hooks != nil {
		a.Hooks.Dispatch(r.Context(), webhook.EventIPDeleted, map[string]any{
			"id": item.ID.String(), "address": item.Address,
			"subnet_id": item.SubnetID.String(), "hostname": item.Hostname, "status": item.Status,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) ListDeletedIPs(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	pq := httpx.ParsePage(r)
	total, err := a.Store.Queries.CountDeletedIPs(r.Context(), pq.Q)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListDeletedIPs(r.Context(), sqlcdb.ListDeletedIPsParams{
		Query: pq.Q, RowLimit: pq.PageSize, RowOffset: pq.Offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]ipDTO, 0, len(items))
	for _, item := range items {
		if !sc.AllowsNullUUID(item.LocationID) {
			continue
		}
		out = append(out, toIPDTOFromRowDeleted(item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.ReservedUntil, item.LastSeenAt, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.CustomFields, item.CreatedAt, item.UpdatedAt, item.DeletedAt))
	}
	// Note: total may include out-of-scope rows when scoped; acceptable for MVP.
	httpx.JSON(w, http.StatusOK, httpx.PageResult[ipDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func (a *API) RestoreDeletedIP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetIP(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ip not found")
		return
	}
	if !existing.DeletedAt.Valid {
		httpx.Error(w, http.StatusBadRequest, "ip is not deleted")
		return
	}
	if !a.requireSubnetAccess(w, r, existing.SubnetID) {
		return
	}
	item, err := a.Store.Queries.RestoreIP(r.Context(), sqlcdb.RestoreIPParams{
		ID:            id,
		SubnetID:      existing.SubnetID,
		VrfID:         existing.VrfID,
		Hostname:      existing.Hostname,
		Status:        existing.Status,
		DeviceTypeID:  existing.DeviceTypeID,
		LocationID:    existing.LocationID,
		Notes:         existing.Notes,
		ReservedUntil: existing.ReservedUntil,
		LastSeenAt:    existing.LastSeenAt,
		MacAddress:    existing.MacAddress,
		OpenPorts:     existing.OpenPorts,
		InterfaceName: existing.InterfaceName,
		SwitchID:      existing.SwitchID,
		CustomFields:  existing.CustomFields,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.recordIPHistory(r.Context(), "restore", item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.MacAddress, item.OpenPorts, item.ReservedUntil, item.LastSeenAt, nil)
	a.audit(r.Context(), "ip.restore", "ip_address", id.String(), map[string]any{"address": item.Address})
	httpx.JSON(w, http.StatusOK, toIPDTOFromRow(item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.ReservedUntil, item.LastSeenAt, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.CustomFields, item.CreatedAt, item.UpdatedAt))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
