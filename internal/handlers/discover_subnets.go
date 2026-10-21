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
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
	"github.com/wyldetechnologies/wyldetrace/internal/scanner"
)

type discoverSubnetsRequest struct {
	CIDR        string     `json:"cidr"`
	ChildPrefix int        `json:"child_prefix"`
	SampleHosts int        `json:"sample_hosts"`
	Workers     int        `json:"workers"`
	LocationID  *uuid.UUID `json:"location_id"`
}

type discoverCandidate struct {
	scanner.ChildProbe
	ExistsInDB bool       `json:"exists_in_db"`
	ExistingID *uuid.UUID `json:"existing_id,omitempty"`
	Suggested  bool       `json:"suggested"`
}

type discoverSubnetsResponse struct {
	ParentCIDR  string              `json:"parent_cidr"`
	ChildPrefix int                 `json:"child_prefix"`
	Total       int                 `json:"total"`
	LikelyUsed  int                 `json:"likely_used"`
	Suggested   int                 `json:"suggested"`
	Candidates  []discoverCandidate `json:"candidates"`
}

type commitDiscoverRequest struct {
	ParentCIDR string     `json:"parent_cidr"`
	LocationID *uuid.UUID `json:"location_id"`
	CIDRs      []struct {
		CIDR string `json:"cidr"`
		Name string `json:"name"`
	} `json:"cidrs"`
}

func (a *API) DiscoverSubnets(w http.ResponseWriter, r *http.Request) {
	var req discoverSubnetsRequest
	if err := httpx.Decode(r, &req); err != nil || req.CIDR == "" {
		httpx.Error(w, http.StatusBadRequest, "cidr required")
		return
	}
	if !a.requireLocationAccess(w, r, db.PtrUUIDToNull(req.LocationID)) {
		return
	}
	parent, err := scanner.NormalizeCIDR(req.CIDR)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid cidr")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	report, err := scanner.DiscoverChildren(ctx, parent, scanner.DiscoverOptions{
		ChildPrefix: req.ChildPrefix,
		SampleHosts: req.SampleHosts,
		Workers:     req.Workers,
	})
	if err != nil && report == nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, _ := a.Store.Queries.ListSubnets(r.Context())
	byCIDR := map[string]uuid.UUID{}
	for _, s := range existing {
		byCIDR[s.Cidr] = s.ID
	}

	out := discoverSubnetsResponse{
		ParentCIDR:  report.ParentCIDR,
		ChildPrefix: report.ChildPrefix,
		Total:       report.Total,
		LikelyUsed:  report.LikelyUsed,
		Candidates:  make([]discoverCandidate, 0, len(report.Children)),
	}
	for _, c := range report.Children {
		cand := discoverCandidate{ChildProbe: c, Suggested: c.LikelyUsed}
		if id, ok := byCIDR[c.CIDR]; ok {
			cand.ExistsInDB = true
			cand.ExistingID = &id
			cand.Suggested = false
		}
		if cand.Suggested {
			out.Suggested++
		}
		out.Candidates = append(out.Candidates, cand)
	}

	a.audit(r.Context(), "scanner.discover_subnets", "subnet", parent, map[string]any{
		"child_prefix": report.ChildPrefix,
		"likely_used":  report.LikelyUsed,
		"suggested":    out.Suggested,
	})
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CommitDiscoveredSubnets(w http.ResponseWriter, r *http.Request) {
	var req commitDiscoverRequest
	if err := httpx.Decode(r, &req); err != nil || len(req.CIDRs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "cidrs required")
		return
	}
	if !a.requireLocationAccess(w, r, db.PtrUUIDToNull(req.LocationID)) {
		return
	}
	var parentID uuid.UUID
	if req.ParentCIDR != "" {
		if p, err := ipam.ParsePrefix(req.ParentCIDR); err == nil {
			existing, _ := a.Store.Queries.ListSubnets(r.Context())
			for _, s := range existing {
				if s.Cidr == p.String() {
					parentID = s.ID
					break
				}
			}
		}
	}

	created := 0
	skipped := 0
	errors := []string{}
	for _, c := range req.CIDRs {
		prefix, err := ipam.ParsePrefix(c.CIDR)
		if err != nil {
			errors = append(errors, c.CIDR+": invalid")
			skipped++
			continue
		}
		name := c.Name
		if name == "" {
			name = prefix.String()
		}
		parentNull := db.PtrUUIDToNull(nil)
		if parentID != uuid.Nil {
			pid := parentID
			parentNull = db.PtrUUIDToNull(&pid)
		}
		_, err = a.Store.Queries.CreateSubnet(r.Context(), sqlcdb.CreateSubnetParams{
			Cidr:         prefix.String(),
			Name:         name,
			Description:  "created from supernet discovery",
			LocationID:   db.PtrUUIDToNull(req.LocationID),
			VrfID:        db.DefaultVRFID,
			VlanID:       pgtype.UUID{},
			ParentID:     parentNull,
			Tags:         []string{"discovered"},
			CustomFields: []byte(`{}`),
		})
		if err != nil {
			errors = append(errors, prefix.String()+": "+err.Error())
			skipped++
			continue
		}
		created++
	}
	a.audit(r.Context(), "scanner.discover_subnets.commit", "subnet", req.ParentCIDR, map[string]any{
		"created": created, "skipped": skipped,
	})
	httpx.JSON(w, http.StatusOK, map[string]any{
		"created": created, "skipped": skipped, "errors": errors,
	})
}
