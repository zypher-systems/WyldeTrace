package handlers

import (
	"errors"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
	"github.com/wyldetechnologies/wyldetrace/internal/scanner"
	"github.com/wyldetechnologies/wyldetrace/internal/scope"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

type claimHost struct {
	Address   string `json:"address"`
	Hostname  string `json:"hostname"`
	MAC       string `json:"mac"`
	OpenPorts string `json:"open_ports"`
}

type claimRequest struct {
	SubnetID  uuid.UUID   `json:"subnet_id"`
	Addresses []string    `json:"addresses"`
	Hosts     []claimHost `json:"hosts"`
	Status    string      `json:"status"`
	Notes     string      `json:"notes"`
}

type claimResult struct {
	Address string `json:"address"`
	Action  string `json:"action"`
	Detail  string `json:"detail,omitempty"`
	ID      string `json:"id,omitempty"`
}

type claimResponse struct {
	Created   int           `json:"created"`
	Updated   int           `json:"updated"`
	Skipped   int           `json:"skipped"`
	Conflicts int           `json:"conflicts"`
	Results   []claimResult `json:"results"`
}

func (a *API) ClaimDiscovered(w http.ResponseWriter, r *http.Request) {
	var req claimRequest
	if err := httpx.Decode(r, &req); err != nil || req.SubnetID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, "subnet_id required")
		return
	}
	resp, code, msg := a.doClaim(r, req)
	if msg != "" {
		httpx.Error(w, code, msg)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (a *API) doClaim(r *http.Request, req claimRequest) (claimResponse, int, string) {
	hosts := map[string]claimHost{}
	for _, addr := range req.Addresses {
		hosts[addr] = claimHost{Address: addr}
	}
	for _, h := range req.Hosts {
		if h.Address == "" {
			continue
		}
		hosts[h.Address] = h
	}
	if len(hosts) == 0 {
		return claimResponse{}, http.StatusBadRequest, "addresses or hosts required"
	}
	if req.Status == "" {
		req.Status = "assigned"
	}
	if !validIPStatus[req.Status] {
		return claimResponse{}, http.StatusBadRequest, "invalid status"
	}
	if req.Notes == "" {
		req.Notes = "claimed from discovery sweep"
	}

	subnet, err := a.Store.Queries.GetSubnet(r.Context(), req.SubnetID)
	if err != nil || subnet.DeletedAt.Valid {
		return claimResponse{}, http.StatusNotFound, "subnet not found"
	}
	sc, err := scope.Load(r.Context(), a.Store)
	if err != nil {
		return claimResponse{}, http.StatusUnauthorized, "unauthorized"
	}
	if !sc.AllowsNullUUID(subnet.LocationID) {
		return claimResponse{}, http.StatusForbidden, "location out of scope"
	}
	prefix, err := ipam.ParsePrefix(subnet.Cidr)
	if err != nil {
		return claimResponse{}, http.StatusBadRequest, "invalid subnet cidr"
	}

	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	resp := claimResponse{Results: make([]claimResult, 0, len(hosts))}
	var restoreHist []sqlcdb.RestoreIPRow
	txErr := a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		for _, h := range hosts {
			addr, err := netip.ParseAddr(h.Address)
			if err != nil {
				resp.Skipped++
				resp.Results = append(resp.Results, claimResult{Address: h.Address, Action: "skipped", Detail: "invalid address"})
				continue
			}
			if !prefix.Contains(addr) {
				resp.Conflicts++
				resp.Results = append(resp.Results, claimResult{Address: addr.String(), Action: "conflict", Detail: "not in subnet " + prefix.String()})
				continue
			}

			existing, err := q.GetIPByAddressAny(r.Context(), sqlcdb.GetIPByAddressAnyParams{
				Address: addr.String(), VrfID: subnet.VrfID,
			})
			if err == nil {
				if existing.SubnetID != req.SubnetID && !existing.DeletedAt.Valid {
					resp.Conflicts++
					resp.Results = append(resp.Results, claimResult{
						Address: addr.String(),
						Action:  "conflict",
						Detail:  "address already assigned to another subnet",
						ID:      existing.ID.String(),
					})
					continue
				}
				hostname := existing.Hostname
				if h.Hostname != "" {
					hostname = h.Hostname
				}
				mac := firstNonEmpty(h.MAC, existing.MacAddress)
				ports := firstNonEmpty(h.OpenPorts, existing.OpenPorts)
				notes := mergeNotes(existing.Notes, req.Notes)
				if existing.DeletedAt.Valid {
					restored, rerr := q.RestoreIP(r.Context(), sqlcdb.RestoreIPParams{
						ID: existing.ID, SubnetID: req.SubnetID, VrfID: subnet.VrfID, Hostname: hostname, Status: req.Status,
						DeviceTypeID: existing.DeviceTypeID, LocationID: subnet.LocationID, Notes: notes,
						ReservedUntil: existing.ReservedUntil, LastSeenAt: now, MacAddress: mac, OpenPorts: ports,
						InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
						CustomFields: existing.CustomFields,
					})
					if rerr != nil {
						resp.Skipped++
						resp.Results = append(resp.Results, claimResult{Address: addr.String(), Action: "skipped", Detail: rerr.Error()})
						continue
					}
					restoreHist = append(restoreHist, restored)
					resp.Updated++
					resp.Results = append(resp.Results, claimResult{Address: addr.String(), Action: "restored", ID: restored.ID.String()})
					continue
				}
				updated, uerr := q.UpdateIP(r.Context(), sqlcdb.UpdateIPParams{
					ID:            existing.ID,
					Address:       existing.Address,
					SubnetID:      existing.SubnetID,
					VrfID:         subnet.VrfID,
					Hostname:      hostname,
					Status:        req.Status,
					DeviceTypeID:  existing.DeviceTypeID,
					LocationID:    existing.LocationID,
					Notes:         notes,
					ReservedUntil: existing.ReservedUntil,
					LastSeenAt:    now,
					MacAddress:    mac,
					OpenPorts:     ports,
					InterfaceName: existing.InterfaceName,
					SwitchID:      existing.SwitchID,
					CustomFields:  existing.CustomFields,
				})
				if uerr != nil {
					resp.Skipped++
					resp.Results = append(resp.Results, claimResult{Address: addr.String(), Action: "skipped", Detail: uerr.Error()})
					continue
				}
				resp.Updated++
				resp.Results = append(resp.Results, claimResult{Address: addr.String(), Action: "updated", ID: updated.ID.String()})
				continue
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				resp.Skipped++
				resp.Results = append(resp.Results, claimResult{Address: addr.String(), Action: "skipped", Detail: err.Error()})
				continue
			}

			created, cerr := q.CreateIP(r.Context(), sqlcdb.CreateIPParams{
				Address:       addr.String(),
				SubnetID:      req.SubnetID,
				VrfID:         subnet.VrfID,
				Hostname:      h.Hostname,
				Status:        req.Status,
				DeviceTypeID:  pgtype.UUID{},
				LocationID:    subnet.LocationID,
				Notes:         req.Notes,
				ReservedUntil: pgtype.Timestamptz{},
				LastSeenAt:    now,
				MacAddress:    h.MAC,
				OpenPorts:     h.OpenPorts,
				CustomFields:  emptyJSON(),
			})
			if cerr != nil {
				resp.Conflicts++
				resp.Results = append(resp.Results, claimResult{Address: addr.String(), Action: "conflict", Detail: cerr.Error()})
				continue
			}
			resp.Created++
			resp.Results = append(resp.Results, claimResult{Address: addr.String(), Action: "created", ID: created.ID.String()})
		}
		return nil
	})
	if txErr != nil {
		return claimResponse{}, http.StatusInternalServerError, txErr.Error()
	}
	for _, restored := range restoreHist {
		a.recordIPHistory(r.Context(), "restore", restored.ID, restored.Address, restored.SubnetID, restored.Hostname, restored.Status, restored.DeviceTypeID, restored.LocationID, restored.Notes, restored.MacAddress, restored.OpenPorts, restored.ReservedUntil, restored.LastSeenAt, map[string]any{"via": "claim"})
	}

	a.audit(r.Context(), "scanner.claim", "subnet", req.SubnetID.String(), map[string]any{
		"created": resp.Created, "updated": resp.Updated, "conflicts": resp.Conflicts,
	})
	if a.Hooks != nil && (resp.Created > 0 || resp.Updated > 0) {
		claimed := make([]string, 0)
		for _, r := range resp.Results {
			if r.Action == "created" || r.Action == "updated" || r.Action == "restored" {
				claimed = append(claimed, r.Address)
			}
		}
		a.Hooks.Dispatch(r.Context(), webhook.EventIPClaimed, map[string]any{
			"subnet_id": req.SubnetID.String(), "cidr": subnet.Cidr,
			"created": resp.Created, "updated": resp.Updated,
			"count": len(claimed), "hosts": claimed,
		})
	}
	return resp, http.StatusOK, ""
}

func mergeNotes(existing, incoming string) string {
	if existing == "" {
		return incoming
	}
	if incoming == "" || existing == incoming {
		return existing
	}
	return existing + "; " + incoming
}

// ClaimHostsFromResults helper for converting sweep results.
func ClaimHostsFromResults(results []scanner.Result) []claimHost {
	out := make([]claimHost, 0)
	for _, r := range results {
		if !r.Alive {
			continue
		}
		out = append(out, claimHost{
			Address:   r.IP,
			Hostname:  r.Hostname,
			MAC:       r.MAC,
			OpenPorts: scanner.FormatPorts(r.OpenPorts),
		})
	}
	return out
}
