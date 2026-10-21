package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/scheduler"
)

type discoveryJobDTO struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	SubnetID        uuid.UUID       `json:"subnet_id"`
	IntervalMinutes int32           `json:"interval_minutes"`
	Enabled         bool            `json:"enabled"`
	Enrich          bool            `json:"enrich"`
	LastRunAt       *time.Time      `json:"last_run_at"`
	LastStatus      string          `json:"last_status"`
	LastSummary     json.RawMessage `json:"last_summary"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type discoveryJobRequest struct {
	Name            string    `json:"name"`
	SubnetID        uuid.UUID `json:"subnet_id"`
	IntervalMinutes int32     `json:"interval_minutes"`
	Enabled         *bool     `json:"enabled"`
	Enrich          *bool     `json:"enrich"`
}

func toDiscoveryJobDTO(j sqlcdb.DiscoveryJob) discoveryJobDTO {
	summary := j.LastSummary
	if summary == nil {
		summary = []byte("{}")
	}
	return discoveryJobDTO{
		ID:              j.ID,
		Name:            j.Name,
		SubnetID:        j.SubnetID,
		IntervalMinutes: j.IntervalMinutes,
		Enabled:         j.Enabled,
		Enrich:          j.Enrich,
		LastRunAt:       timestamptzPtr(j.LastRunAt),
		LastStatus:      j.LastStatus,
		LastSummary:     summary,
		CreatedAt:       j.CreatedAt.Time,
		UpdatedAt:       j.UpdatedAt.Time,
	}
}

func (a *API) ListDiscoveryJobs(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	items, err := a.Store.Queries.ListDiscoveryJobs(r.Context(), sqlcdb.ListDiscoveryJobsParams{
		Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]discoveryJobDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toDiscoveryJobDTO(item))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CreateDiscoveryJob(w http.ResponseWriter, r *http.Request) {
	var req discoveryJobRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.SubnetID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, "name and subnet_id are required")
		return
	}
	if req.IntervalMinutes < 1 {
		req.IntervalMinutes = 60
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	enrich := true
	if req.Enrich != nil {
		enrich = *req.Enrich
	}
	if !a.requireSubnetAccess(w, r, req.SubnetID) {
		return
	}
	item, err := a.Store.Queries.CreateDiscoveryJob(r.Context(), sqlcdb.CreateDiscoveryJobParams{
		Name:            req.Name,
		SubnetID:        req.SubnetID,
		IntervalMinutes: req.IntervalMinutes,
		Enabled:         enabled,
		Enrich:          enrich,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "discovery_job.create", "discovery_job", item.ID.String(), map[string]any{
		"name": item.Name, "subnet_id": item.SubnetID.String(),
	})
	httpx.JSON(w, http.StatusCreated, toDiscoveryJobDTO(item))
}

func (a *API) UpdateDiscoveryJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetDiscoveryJob(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "job not found")
		return
	}
	if !a.requireSubnetAccess(w, r, existing.SubnetID) {
		return
	}
	var req discoveryJobRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.SubnetID == uuid.Nil {
		req.SubnetID = existing.SubnetID
	}
	if req.IntervalMinutes < 1 {
		req.IntervalMinutes = existing.IntervalMinutes
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	enrich := existing.Enrich
	if req.Enrich != nil {
		enrich = *req.Enrich
	}
	if req.SubnetID != existing.SubnetID && !a.requireSubnetAccess(w, r, req.SubnetID) {
		return
	}
	item, err := a.Store.Queries.UpdateDiscoveryJob(r.Context(), sqlcdb.UpdateDiscoveryJobParams{
		ID:              id,
		Name:            req.Name,
		SubnetID:        req.SubnetID,
		IntervalMinutes: req.IntervalMinutes,
		Enabled:         enabled,
		Enrich:          enrich,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "discovery_job.update", "discovery_job", item.ID.String(), map[string]any{
		"name": item.Name, "enabled": item.Enabled,
	})
	httpx.JSON(w, http.StatusOK, toDiscoveryJobDTO(item))
}

func (a *API) DeleteDiscoveryJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetDiscoveryJob(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "job not found")
		return
	}
	if !a.requireSubnetAccess(w, r, existing.SubnetID) {
		return
	}
	if err := a.Store.Queries.DeleteDiscoveryJob(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "discovery_job.delete", "discovery_job", id.String(), map[string]any{})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) RunDiscoveryJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetDiscoveryJob(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "job not found")
		return
	}
	if !a.requireSubnetAccess(w, r, existing.SubnetID) {
		return
	}
	if a.Scheduler == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "scheduler unavailable")
		return
	}
	if err := a.Scheduler.RunNow(r.Context(), id); err != nil {
		if errors.Is(err, scheduler.ErrBusy) {
			httpx.Error(w, http.StatusConflict, "job already running")
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	job, err := a.Store.Queries.GetDiscoveryJob(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "discovery_job.run", "discovery_job", id.String(), map[string]any{
		"status": job.LastStatus,
	})
	httpx.JSON(w, http.StatusOK, toDiscoveryJobDTO(job))
}

type discoveryJobRunDTO struct {
	ID         uuid.UUID       `json:"id"`
	JobID      uuid.UUID       `json:"job_id"`
	Status     string          `json:"status"`
	Summary    json.RawMessage `json:"summary"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt time.Time       `json:"finished_at"`
}

func (a *API) ListDiscoveryJobRuns(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	job, err := a.Store.Queries.GetDiscoveryJob(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "job not found")
		return
	}
	if !a.requireSubnetAccess(w, r, job.SubnetID) {
		return
	}
	limit := int32(25)
	items, err := a.Store.Queries.ListDiscoveryJobRuns(r.Context(), sqlcdb.ListDiscoveryJobRunsParams{
		JobID: id, RowLimit: limit,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]discoveryJobRunDTO, 0, len(items))
	for _, item := range items {
		summary := item.Summary
		if summary == nil {
			summary = []byte("{}")
		}
		out = append(out, discoveryJobRunDTO{
			ID: item.ID, JobID: item.JobID, Status: item.Status, Summary: summary,
			StartedAt: item.StartedAt.Time, FinishedAt: item.FinishedAt.Time,
		})
	}
	httpx.JSON(w, http.StatusOK, out)
}
