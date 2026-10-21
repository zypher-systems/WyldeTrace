package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/middleware"
)

type ipHistoryDTO struct {
	ID         uuid.UUID       `json:"id"`
	IPID       *uuid.UUID      `json:"ip_id"`
	Address    string          `json:"address"`
	SubnetID   *uuid.UUID      `json:"subnet_id"`
	Hostname   string          `json:"hostname"`
	Status     string          `json:"status"`
	MACAddress string          `json:"mac_address"`
	OpenPorts  string          `json:"open_ports"`
	Action     string          `json:"action"`
	ActorEmail string          `json:"actor_email"`
	Details    json.RawMessage `json:"details"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (a *API) recordIPHistory(
	ctx context.Context,
	action string,
	id uuid.UUID,
	address string,
	subnetID uuid.UUID,
	hostname, status string,
	deviceTypeID, locationID pgtype.UUID,
	notes, mac, ports string,
	reservedUntil, lastSeenAt pgtype.Timestamptz,
	details any,
) {
	email := ""
	if claims, ok := middleware.ClaimsFromContext(ctx); ok {
		email = claims.Email
	}
	raw, _ := json.Marshal(details)
	if raw == nil {
		raw = []byte("{}")
	}
	_, err := a.Store.Queries.CreateIPHistory(ctx, sqlcdb.CreateIPHistoryParams{
		IpID:          db.UUIDToNull(id),
		Address:       address,
		SubnetID:      db.UUIDToNull(subnetID),
		Hostname:      hostname,
		Status:        status,
		DeviceTypeID:  deviceTypeID,
		LocationID:    locationID,
		Notes:         notes,
		MacAddress:    mac,
		OpenPorts:     ports,
		ReservedUntil: reservedUntil,
		LastSeenAt:    lastSeenAt,
		Action:        action,
		ActorEmail:    email,
		Details:       raw,
	})
	if err != nil {
		log.Printf("ip_history: %v", err)
	}
}

func toIPHistoryDTO(
	id uuid.UUID,
	ipID pgtype.UUID,
	address string,
	subnetID pgtype.UUID,
	hostname, status, macAddress, openPorts, action, actorEmail string,
	details json.RawMessage,
	createdAt pgtype.Timestamptz,
) ipHistoryDTO {
	return ipHistoryDTO{
		ID:         id,
		IPID:       db.NullUUID(ipID),
		Address:    address,
		SubnetID:   db.NullUUID(subnetID),
		Hostname:   hostname,
		Status:     status,
		MACAddress: macAddress,
		OpenPorts:  openPorts,
		Action:     action,
		ActorEmail: actorEmail,
		Details:    details,
		CreatedAt:  createdAt.Time,
	}
}

func (a *API) ListIPHistory(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	scoped := !sc.Unrestricted
	locs := sc.LocationIDsOrEmpty()

	addr := r.URL.Query().Get("address")
	ipIDStr := r.URL.Query().Get("ip_id")
	action := r.URL.Query().Get("action")
	actor := r.URL.Query().Get("actor")
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")
	pq := httpx.ParsePage(r)
	sq := httpx.ParseSort(r, map[string]bool{
		"created_at": true, "address": true, "action": true, "actor_email": true,
	}, "created_at", "desc")

	// Legacy: address or ip_id without pagination envelope.
	if (addr != "" || ipIDStr != "") && action == "" && actor == "" && fromRaw == "" && toRaw == "" &&
		r.URL.Query().Get("page") == "" {
		limit := int32(50)
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				limit = int32(n)
			}
		}
		switch {
		case addr != "":
			items, err := a.Store.Queries.ListIPHistoryByAddress(r.Context(), sqlcdb.ListIPHistoryByAddressParams{
				Address: addr, RowLimit: limit, Scoped: scoped, LocationIds: locs,
			})
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, err.Error())
				return
			}
			out := make([]ipHistoryDTO, 0, len(items))
			for _, item := range items {
				out = append(out, toIPHistoryDTO(
					item.ID, item.IpID, item.Address, item.SubnetID, item.Hostname, item.Status,
					item.MacAddress, item.OpenPorts, item.Action, item.ActorEmail, item.Details, item.CreatedAt,
				))
			}
			httpx.JSON(w, http.StatusOK, out)
			return
		default:
			id, perr := uuid.Parse(ipIDStr)
			if perr != nil {
				httpx.Error(w, http.StatusBadRequest, "invalid ip_id")
				return
			}
			items, err := a.Store.Queries.ListIPHistoryByIPID(r.Context(), sqlcdb.ListIPHistoryByIPIDParams{
				IpID: db.UUIDToNull(id), RowLimit: limit, Scoped: scoped, LocationIds: locs,
			})
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, err.Error())
				return
			}
			out := make([]ipHistoryDTO, 0, len(items))
			for _, item := range items {
				out = append(out, toIPHistoryDTO(
					item.ID, item.IpID, item.Address, item.SubnetID, item.Hostname, item.Status,
					item.MacAddress, item.OpenPorts, item.Action, item.ActorEmail, item.Details, item.CreatedAt,
				))
			}
			httpx.JSON(w, http.StatusOK, out)
			return
		}
	}

	fromTS, err := parseOptionalTime(fromRaw)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid from")
		return
	}
	toTS, err := parseOptionalTime(toRaw)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid to")
		return
	}
	if ipIDStr != "" && addr == "" {
		id, perr := uuid.Parse(ipIDStr)
		if perr != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid ip_id")
			return
		}
		if ip, gerr := a.Store.Queries.GetIP(r.Context(), id); gerr == nil {
			if !sc.AllowsNullUUID(ip.LocationID) {
				httpx.Error(w, http.StatusForbidden, "ip out of scope")
				return
			}
			addr = ip.Address
		}
	}

	total, err := a.Store.Queries.CountIPHistoryFiltered(r.Context(), sqlcdb.CountIPHistoryFilteredParams{
		Address: addr, Action: action, Actor: actor, FromTs: fromTS, ToTs: toTS,
		Scoped: scoped, LocationIds: locs,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListIPHistoryFiltered(r.Context(), sqlcdb.ListIPHistoryFilteredParams{
		Address: addr, Action: action, Actor: actor, FromTs: fromTS, ToTs: toTS,
		SortBy: sq.By, SortDir: sq.Dir,
		RowLimit: pq.PageSize, RowOffset: pq.Offset, Scoped: scoped, LocationIds: locs,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]ipHistoryDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toIPHistoryDTO(
			item.ID, item.IpID, item.Address, item.SubnetID, item.Hostname, item.Status,
			item.MacAddress, item.OpenPorts, item.Action, item.ActorEmail, item.Details, item.CreatedAt,
		))
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[ipHistoryDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func parseOptionalTime(raw string) (pgtype.Timestamptz, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pgtype.Timestamptz{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return pgtype.Timestamptz{}, err
		}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}, nil
}
