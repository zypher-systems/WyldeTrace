package handlers

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
	"github.com/wyldetechnologies/wyldetrace/internal/middleware"
)

type ipRequestDTO struct {
	ID               uuid.UUID  `json:"id"`
	RequesterID      uuid.UUID  `json:"requester_id"`
	RequesterEmail   string     `json:"requester_email,omitempty"`
	SubnetID         uuid.UUID  `json:"subnet_id"`
	SubnetCIDR       string     `json:"subnet_cidr,omitempty"`
	SubnetName       string     `json:"subnet_name,omitempty"`
	RangeID          *uuid.UUID `json:"range_id,omitempty"`
	Hostname         string     `json:"hostname"`
	Notes            string     `json:"notes"`
	RequestedAddress string     `json:"requested_address,omitempty"`
	Status           string     `json:"status"`
	AllocatedIPID    *uuid.UUID `json:"allocated_ip_id,omitempty"`
	AllocatedAddress string     `json:"allocated_address,omitempty"`
	ReviewedBy       *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewNotes      string     `json:"review_notes,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
}

type createIPRequestBody struct {
	SubnetID         uuid.UUID  `json:"subnet_id"`
	RangeID          *uuid.UUID `json:"range_id"`
	Hostname         string     `json:"hostname"`
	Notes            string     `json:"notes"`
	RequestedAddress string     `json:"requested_address"`
}

type approveIPRequestBody struct {
	Address     string `json:"address"`
	Hostname    string `json:"hostname"`
	Notes       string `json:"notes"`
	ReviewNotes string `json:"review_notes"`
}

type denyIPRequestBody struct {
	ReviewNotes string `json:"review_notes"`
}

func (a *API) ListIPRequests(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pq := httpx.ParsePage(r)
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	var requesterFilter pgtype.UUID
	var subnetFilter pgtype.UUID
	if claims.Role.Rank() < auth.RoleEditor.Rank() {
		requesterFilter = db.UUIDToNull(claims.UserID)
	} else if raw := strings.TrimSpace(r.URL.Query().Get("requester_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid requester_id")
			return
		}
		requesterFilter = db.UUIDToNull(id)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("subnet_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid subnet_id")
			return
		}
		subnetFilter = db.UUIDToNull(id)
	}
	total, err := a.Store.Queries.CountIPRequests(r.Context(), sqlcdb.CountIPRequestsParams{
		Status: status, RequesterID: requesterFilter, SubnetID: subnetFilter,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListIPRequests(r.Context(), sqlcdb.ListIPRequestsParams{
		Status: status, RequesterID: requesterFilter, SubnetID: subnetFilter,
		RowLimit: pq.PageSize, RowOffset: pq.Offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]ipRequestDTO, 0, len(items))
	for _, item := range items {
		out = append(out, a.ipRequestDTOFromList(r, item))
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[ipRequestDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func (a *API) CountPendingIPRequests(w http.ResponseWriter, r *http.Request) {
	n, err := a.Store.Queries.CountPendingIPRequests(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"pending": n})
}

func (a *API) CreateIPRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok || claims.IsAPIToken {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !claims.Role.CanViewReports() {
		httpx.Error(w, http.StatusForbidden, "insufficient role")
		return
	}
	var req createIPRequestBody
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.SubnetID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, "subnet_id required")
		return
	}
	subnet, err := a.Store.Queries.GetSubnet(r.Context(), req.SubnetID)
	if err != nil || subnet.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	if claims.Role.Rank() >= auth.RoleReadOnly.Rank() {
		if !a.requireSubnetAccess(w, r, req.SubnetID) {
			return
		}
	}
	var rangeID pgtype.UUID
	if req.RangeID != nil && *req.RangeID != uuid.Nil {
		rng, err := a.Store.Queries.GetIPRange(r.Context(), *req.RangeID)
		if err != nil || rng.SubnetID != req.SubnetID {
			httpx.Error(w, http.StatusBadRequest, "range_id must belong to subnet")
			return
		}
		rangeID = db.UUIDToNull(*req.RangeID)
	}
	var requestedAddr *netip.Addr
	addrStr := strings.TrimSpace(req.RequestedAddress)
	if addrStr != "" {
		ip, err := netip.ParseAddr(addrStr)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid requested_address")
			return
		}
		prefix, err := ipam.ParsePrefix(subnet.Cidr)
		if err != nil || !prefix.Contains(ip) {
			httpx.Error(w, http.StatusBadRequest, "requested_address must be inside subnet")
			return
		}
		requestedAddr = &ip
	}
	item, err := a.Store.Queries.CreateIPRequest(r.Context(), sqlcdb.CreateIPRequestParams{
		RequesterID:      claims.UserID,
		SubnetID:         req.SubnetID,
		RangeID:          rangeID,
		Hostname:         strings.TrimSpace(req.Hostname),
		Notes:            strings.TrimSpace(req.Notes),
		RequestedAddress: requestedAddr,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "ip_request.create", "ip_request", item.ID.String(), map[string]any{
		"subnet_id": req.SubnetID.String(), "hostname": item.Hostname,
	})
	httpx.JSON(w, http.StatusCreated, a.ipRequestDTOFromCreate(r, item))
}

func (a *API) GetIPRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.GetIPRequest(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "request not found")
		return
	}
	if claims.Role.Rank() < auth.RoleEditor.Rank() && item.RequesterID != claims.UserID {
		httpx.Error(w, http.StatusForbidden, "insufficient role")
		return
	}
	httpx.JSON(w, http.StatusOK, a.ipRequestDTOFromGet(r, item))
}

func (a *API) ApproveIPRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req approveIPRequestBody
	if err := httpx.Decode(r, &req); err != nil && r.ContentLength > 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := a.Store.Queries.GetIPRequest(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "request not found")
		return
	}
	if item.Status != "pending" {
		httpx.Error(w, http.StatusConflict, "request is not pending")
		return
	}
	if !a.requireSubnetAccess(w, r, item.SubnetID) {
		return
	}
	subnet, err := a.Store.Queries.GetSubnet(r.Context(), item.SubnetID)
	if err != nil || subnet.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	hostname := strings.TrimSpace(req.Hostname)
	if hostname == "" {
		hostname = item.Hostname
	}
	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = item.Notes
	}

	var created sqlcdb.CreateIPRow
	allocErr := error(nil)
	for attempt := 0; attempt < 5; attempt++ {
		allocErr = a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
			addr, err := pickAddressForRequest(r, q, item, subnet, strings.TrimSpace(req.Address))
			if err != nil {
				return err
			}
			created, err = q.CreateIP(r.Context(), sqlcdb.CreateIPParams{
				Address:       addr.String(),
				SubnetID:      item.SubnetID,
				VrfID:         subnet.VrfID,
				Hostname:      hostname,
				Status:        "assigned",
				DeviceTypeID:  pgtype.UUID{},
				LocationID:    subnet.LocationID,
				Notes:         notes,
				ReservedUntil: pgtype.Timestamptz{},
				LastSeenAt:    pgtype.Timestamptz{},
				MacAddress:    "",
				OpenPorts:     "",
				CustomFields:  emptyJSON(),
			})
			if err != nil {
				return err
			}
			_, err = q.ApproveIPRequest(r.Context(), sqlcdb.ApproveIPRequestParams{
				ID:            item.ID,
				AllocatedIpID: db.UUIDToNull(created.ID),
				ReviewedBy:    db.UUIDToNull(claims.UserID),
				ReviewNotes:   strings.TrimSpace(req.ReviewNotes),
			})
			return err
		})
		if allocErr == nil {
			break
		}
		low := strings.ToLower(allocErr.Error())
		if !strings.Contains(low, "unique") && !strings.Contains(low, "duplicate") {
			break
		}
	}
	if allocErr != nil {
		msg := allocErr.Error()
		low := strings.ToLower(msg)
		if strings.Contains(low, "no free") || strings.Contains(low, "already in use") {
			httpx.Error(w, http.StatusConflict, msg)
			return
		}
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	updated, _ := a.Store.Queries.GetIPRequest(r.Context(), id)
	a.audit(r.Context(), "ip_request.approve", "ip_request", id.String(), map[string]any{
		"address": created.Address, "ip_id": created.ID.String(),
	})
	dto := a.ipRequestDTOFromGet(r, updated)
	dto.AllocatedAddress = created.Address
	httpx.JSON(w, http.StatusOK, dto)
}

func (a *API) DenyIPRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req denyIPRequestBody
	_ = httpx.Decode(r, &req)
	item, err := a.Store.Queries.GetIPRequest(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "request not found")
		return
	}
	if item.Status != "pending" {
		httpx.Error(w, http.StatusConflict, "request is not pending")
		return
	}
	if !a.requireSubnetAccess(w, r, item.SubnetID) {
		return
	}
	updated, err := a.Store.Queries.DenyIPRequest(r.Context(), sqlcdb.DenyIPRequestParams{
		ID:          item.ID,
		ReviewedBy:  db.UUIDToNull(claims.UserID),
		ReviewNotes: strings.TrimSpace(req.ReviewNotes),
	})
	if err != nil {
		httpx.Error(w, http.StatusConflict, "request is not pending")
		return
	}
	a.audit(r.Context(), "ip_request.deny", "ip_request", id.String(), map[string]any{})
	httpx.JSON(w, http.StatusOK, a.ipRequestDTOFromDeny(r, updated))
}

func (a *API) DeleteIPRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.GetIPRequest(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "request not found")
		return
	}
	if item.Status != "pending" {
		httpx.Error(w, http.StatusConflict, "only pending requests can be cancelled")
		return
	}
	if claims.Role.Rank() < auth.RoleEditor.Rank() && item.RequesterID != claims.UserID {
		httpx.Error(w, http.StatusForbidden, "insufficient role")
		return
	}
	if err := a.Store.Queries.DeleteIPRequest(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "ip_request.cancel", "ip_request", id.String(), map[string]any{})
	w.WriteHeader(http.StatusNoContent)
}

func pickAddressForRequest(
	r *http.Request,
	q *sqlcdb.Queries,
	item sqlcdb.GetIPRequestRow,
	subnet sqlcdb.GetSubnetRow,
	override string,
) (netip.Addr, error) {
	prefix, err := ipam.ParsePrefix(subnet.Cidr)
	if err != nil {
		return netip.Addr{}, err
	}
	usedAddrs, err := q.ListOccupiedAddressesBySubnet(r.Context(), item.SubnetID)
	if err != nil {
		return netip.Addr{}, err
	}
	used := ipam.UsedSet(usedAddrs)

	want := strings.TrimSpace(override)
	if want == "" {
		want = strings.TrimSpace(item.RequestedAddress)
	}
	if want != "" {
		addr, err := netip.ParseAddr(want)
		if err != nil || !prefix.Contains(addr) {
			return netip.Addr{}, fmt.Errorf("address must be inside subnet")
		}
		if _, taken := used[addr]; taken {
			return netip.Addr{}, fmt.Errorf("address already in use")
		}
		if item.RangeID.Valid {
			rng, err := q.GetIPRange(r.Context(), uuid.UUID(item.RangeID.Bytes))
			if err != nil {
				return netip.Addr{}, err
			}
			if !addrInRange(addr, rng.StartAddress, rng.EndAddress) {
				return netip.Addr{}, fmt.Errorf("address outside requested range")
			}
		}
		return addr, nil
	}

	if item.RangeID.Valid {
		rng, err := q.GetIPRange(r.Context(), uuid.UUID(item.RangeID.Bytes))
		if err != nil {
			return netip.Addr{}, err
		}
		return nextFreeInRange(rng.StartAddress, rng.EndAddress, used)
	}
	if ipam.IsSparsePrefix(prefix) {
		return ipam.NextFreeSparse(prefix, used, netip.Addr{})
	}
	return ipam.NextFree(prefix, used)
}

func addrInRange(addr netip.Addr, startStr, endStr string) bool {
	start, err1 := netip.ParseAddr(startStr)
	end, err2 := netip.ParseAddr(endStr)
	if err1 != nil || err2 != nil {
		return false
	}
	return !addr.Less(start) && !end.Less(addr)
}

func nextFreeInRange(startStr, endStr string, used map[netip.Addr]struct{}) (netip.Addr, error) {
	start, err := netip.ParseAddr(startStr)
	if err != nil {
		return netip.Addr{}, err
	}
	end, err := netip.ParseAddr(endStr)
	if err != nil {
		return netip.Addr{}, err
	}
	for addr := start; ; {
		if _, taken := used[addr]; !taken {
			return addr, nil
		}
		if addr == end {
			break
		}
		next := addr.Next()
		if !next.IsValid() || next.Less(addr) {
			break
		}
		addr = next
	}
	return netip.Addr{}, fmt.Errorf("no free addresses in range")
}

func (a *API) enrichIPRequestDTO(r *http.Request, dto *ipRequestDTO) {
	if u, err := a.Store.Queries.GetUserByID(r.Context(), dto.RequesterID); err == nil {
		dto.RequesterEmail = u.Email
	}
	if s, err := a.Store.Queries.GetSubnet(r.Context(), dto.SubnetID); err == nil {
		dto.SubnetCIDR = s.Cidr
		dto.SubnetName = s.Name
	}
	if dto.AllocatedIPID != nil {
		if ip, err := a.Store.Queries.GetIP(r.Context(), *dto.AllocatedIPID); err == nil {
			dto.AllocatedAddress = ip.Address
		}
	}
}

func fillIPRequestDTO(
	id, requesterID, subnetID uuid.UUID,
	rangeID, allocatedIPID, reviewedBy pgtype.UUID,
	hostname, notes, requestedAddress, status, reviewNotes string,
	createdAt, updatedAt, reviewedAt pgtype.Timestamptz,
) ipRequestDTO {
	dto := ipRequestDTO{
		ID: id, RequesterID: requesterID, SubnetID: subnetID,
		Hostname: hostname, Notes: notes, RequestedAddress: requestedAddress,
		Status: status, ReviewNotes: reviewNotes,
		CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time,
		RangeID: nullUUIDPtr(rangeID), AllocatedIPID: nullUUIDPtr(allocatedIPID),
		ReviewedBy: nullUUIDPtr(reviewedBy), ReviewedAt: timestamptzPtr(reviewedAt),
	}
	return dto
}

func (a *API) ipRequestDTOFromList(r *http.Request, item sqlcdb.ListIPRequestsRow) ipRequestDTO {
	dto := fillIPRequestDTO(
		item.ID, item.RequesterID, item.SubnetID, item.RangeID, item.AllocatedIpID, item.ReviewedBy,
		item.Hostname, item.Notes, item.RequestedAddress, item.Status, item.ReviewNotes,
		item.CreatedAt, item.UpdatedAt, item.ReviewedAt,
	)
	a.enrichIPRequestDTO(r, &dto)
	return dto
}

func (a *API) ipRequestDTOFromGet(r *http.Request, item sqlcdb.GetIPRequestRow) ipRequestDTO {
	dto := fillIPRequestDTO(
		item.ID, item.RequesterID, item.SubnetID, item.RangeID, item.AllocatedIpID, item.ReviewedBy,
		item.Hostname, item.Notes, item.RequestedAddress, item.Status, item.ReviewNotes,
		item.CreatedAt, item.UpdatedAt, item.ReviewedAt,
	)
	a.enrichIPRequestDTO(r, &dto)
	return dto
}

func (a *API) ipRequestDTOFromCreate(r *http.Request, item sqlcdb.CreateIPRequestRow) ipRequestDTO {
	dto := fillIPRequestDTO(
		item.ID, item.RequesterID, item.SubnetID, item.RangeID, item.AllocatedIpID, item.ReviewedBy,
		item.Hostname, item.Notes, item.RequestedAddress, item.Status, item.ReviewNotes,
		item.CreatedAt, item.UpdatedAt, item.ReviewedAt,
	)
	a.enrichIPRequestDTO(r, &dto)
	return dto
}

func (a *API) ipRequestDTOFromDeny(r *http.Request, item sqlcdb.DenyIPRequestRow) ipRequestDTO {
	dto := fillIPRequestDTO(
		item.ID, item.RequesterID, item.SubnetID, item.RangeID, item.AllocatedIpID, item.ReviewedBy,
		item.Hostname, item.Notes, item.RequestedAddress, item.Status, item.ReviewNotes,
		item.CreatedAt, item.UpdatedAt, item.ReviewedAt,
	)
	a.enrichIPRequestDTO(r, &dto)
	return dto
}
