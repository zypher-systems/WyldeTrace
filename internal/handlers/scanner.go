package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/metrics"
	"github.com/wyldetechnologies/wyldetrace/internal/scanner"
)

type sweepRequest struct {
	CIDR     string     `json:"cidr"`
	SubnetID *uuid.UUID `json:"subnet_id"`
	Workers  int        `json:"workers"`
	Enrich   *bool      `json:"enrich"`
}

type reconcileHost struct {
	IP         string   `json:"ip"`
	Hostname   string   `json:"hostname,omitempty"`
	MAC        string   `json:"mac,omitempty"`
	Vendor     string   `json:"vendor,omitempty"`
	DeviceHint string   `json:"device_hint,omitempty"`
	Ports      []int    `json:"open_ports,omitempty"`
	Hints      []string `json:"hints,omitempty"`
	Status     string   `json:"db_status,omitempty"`
	DBID       string   `json:"db_id,omitempty"`
}

type reconcileReport struct {
	NewAlive []reconcileHost `json:"new_alive"`
	Matched  []reconcileHost `json:"matched"`
	Stale    []reconcileHost `json:"stale"`
}

type sweepResponse struct {
	*scanner.SweepReport
	Reconcile *reconcileReport `json:"reconcile,omitempty"`
}

func (a *API) Sweep(w http.ResponseWriter, r *http.Request) {
	var req sweepRequest
	if err := httpx.Decode(r, &req); err != nil || req.CIDR == "" {
		httpx.Error(w, http.StatusBadRequest, "cidr required")
		return
	}
	cidr, err := scanner.NormalizeCIDR(req.CIDR)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid cidr")
		return
	}
	enrich := true
	if req.Enrich != nil {
		enrich = *req.Enrich
	}

	// Sweeps must target an inventory subnet the caller can access.
	subnetID := req.SubnetID
	if subnetID == nil {
		sub, gerr := a.Store.Queries.GetSubnetByCIDR(r.Context(), sqlcdb.GetSubnetByCIDRParams{
			Cidr: cidr, VrfID: db.DefaultVRFID,
		})
		if gerr != nil || sub.DeletedAt.Valid {
			httpx.Error(w, http.StatusBadRequest, "cidr must match an inventory subnet (pass subnet_id)")
			return
		}
		id := sub.ID
		subnetID = &id
	}
	if !a.requireSubnetAccess(w, r, *subnetID) {
		return
	}
	sub, err := a.Store.Queries.GetSubnet(r.Context(), *subnetID)
	if err != nil || sub.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	if norm, nerr := scanner.NormalizeCIDR(sub.Cidr); nerr != nil || norm != cidr {
		httpx.Error(w, http.StatusBadRequest, "cidr must match the selected subnet")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	sweepStart := time.Now()
	report, err := scanner.Sweep(ctx, cidr, scanner.SweepOptions{Workers: req.Workers, Enrich: enrich})
	metrics.SweepDuration.Observe(time.Since(sweepStart).Seconds())
	if err != nil && report == nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := sweepResponse{SweepReport: report}
	rec, rerr := a.buildReconcile(ctx, *subnetID, report)
	if rerr != nil {
		httpx.Error(w, http.StatusBadRequest, rerr.Error())
		return
	}
	resp.Reconcile = rec
	a.recordReconcileFindings(ctx, *subnetID, rec, pgtype.UUID{})

	a.audit(r.Context(), "scanner.sweep", "subnet", cidr, map[string]any{
		"alive":  report.Alive,
		"total":  report.Total,
		"enrich": enrich,
	})
	httpx.JSON(w, http.StatusOK, resp)
}

func (a *API) buildReconcile(ctx context.Context, subnetID uuid.UUID, report *scanner.SweepReport) (*reconcileReport, error) {
	if _, err := a.Store.Queries.GetSubnet(ctx, subnetID); err != nil {
		return nil, err
	}
	ips, err := a.Store.Queries.ListIPsBySubnet(ctx, subnetID)
	if err != nil {
		return nil, err
	}
	byAddr := map[string]struct {
		id     uuid.UUID
		status string
		host   string
	}{}
	for _, ip := range ips {
		byAddr[ip.Address] = struct {
			id     uuid.UUID
			status string
			host   string
		}{id: ip.ID, status: ip.Status, host: ip.Hostname}
	}

	alive := map[string]scanner.Result{}
	for _, r := range report.Results {
		if r.Alive {
			alive[r.IP] = r
		}
	}

	rec := &reconcileReport{
		NewAlive: []reconcileHost{},
		Matched:  []reconcileHost{},
		Stale:    []reconcileHost{},
	}
	for ip, r := range alive {
		h := reconcileHost{
			IP:         ip,
			Hostname:   r.Hostname,
			MAC:        r.MAC,
			Vendor:     r.Vendor,
			DeviceHint: r.DeviceHint,
			Ports:      r.OpenPorts,
			Hints:      r.Hints,
		}
		if db, ok := byAddr[ip]; ok {
			h.Status = db.status
			h.DBID = db.id.String()
			if h.Hostname == "" {
				h.Hostname = db.host
			}
			rec.Matched = append(rec.Matched, h)
		} else {
			rec.NewAlive = append(rec.NewAlive, h)
		}
	}
	for addr, db := range byAddr {
		if _, ok := alive[addr]; ok {
			continue
		}
		if db.status == "available" {
			continue
		}
		// skip expired reservations from stale noise
		rec.Stale = append(rec.Stale, reconcileHost{
			IP:       addr,
			Hostname: db.host,
			Status:   db.status,
			DBID:     db.id.String(),
		})
	}
	return rec, nil
}
