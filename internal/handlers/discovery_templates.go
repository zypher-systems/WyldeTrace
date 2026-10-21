package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type discoveryJobTemplateDTO struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	IntervalMinutes int32     `json:"interval_minutes"`
	Enrich          bool      `json:"enrich"`
	EnabledDefault  bool      `json:"enabled_default"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type discoveryJobTemplateRequest struct {
	Name            string `json:"name"`
	IntervalMinutes int32  `json:"interval_minutes"`
	Enrich          *bool  `json:"enrich"`
	EnabledDefault  *bool  `json:"enabled_default"`
}

func toDiscoveryJobTemplateDTO(t sqlcdb.DiscoveryJobTemplate) discoveryJobTemplateDTO {
	return discoveryJobTemplateDTO{
		ID: t.ID, Name: t.Name, IntervalMinutes: t.IntervalMinutes,
		Enrich: t.Enrich, EnabledDefault: t.EnabledDefault,
		CreatedAt: t.CreatedAt.Time, UpdatedAt: t.UpdatedAt.Time,
	}
}

func (a *API) ListDiscoveryJobTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.Queries.ListDiscoveryJobTemplates(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]discoveryJobTemplateDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toDiscoveryJobTemplateDTO(item))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CreateDiscoveryJobTemplate(w http.ResponseWriter, r *http.Request) {
	var req discoveryJobTemplateRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "name required")
		return
	}
	if req.IntervalMinutes < 1 {
		req.IntervalMinutes = 60
	}
	enrich := true
	if req.Enrich != nil {
		enrich = *req.Enrich
	}
	enabled := true
	if req.EnabledDefault != nil {
		enabled = *req.EnabledDefault
	}
	item, err := a.Store.Queries.CreateDiscoveryJobTemplate(r.Context(), sqlcdb.CreateDiscoveryJobTemplateParams{
		Name: req.Name, IntervalMinutes: req.IntervalMinutes, Enrich: enrich, EnabledDefault: enabled,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "discovery_job_template.create", "discovery_job_template", item.ID.String(), map[string]any{
		"name": item.Name,
	})
	httpx.JSON(w, http.StatusCreated, toDiscoveryJobTemplateDTO(item))
}

func (a *API) UpdateDiscoveryJobTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetDiscoveryJobTemplate(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "template not found")
		return
	}
	var req discoveryJobTemplateRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}
	interval := req.IntervalMinutes
	if interval < 1 {
		interval = existing.IntervalMinutes
	}
	enrich := existing.Enrich
	if req.Enrich != nil {
		enrich = *req.Enrich
	}
	enabled := existing.EnabledDefault
	if req.EnabledDefault != nil {
		enabled = *req.EnabledDefault
	}
	item, err := a.Store.Queries.UpdateDiscoveryJobTemplate(r.Context(), sqlcdb.UpdateDiscoveryJobTemplateParams{
		ID: id, Name: name, IntervalMinutes: interval, Enrich: enrich, EnabledDefault: enabled,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "discovery_job_template.update", "discovery_job_template", item.ID.String(), map[string]any{
		"name": item.Name,
	})
	httpx.JSON(w, http.StatusOK, toDiscoveryJobTemplateDTO(item))
}

func (a *API) DeleteDiscoveryJobTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.Queries.DeleteDiscoveryJobTemplate(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "discovery_job_template.delete", "discovery_job_template", id.String(), map[string]any{})
	w.WriteHeader(http.StatusNoContent)
}

type createJobFromTemplateRequest struct {
	TemplateID uuid.UUID `json:"template_id"`
	SubnetID   uuid.UUID `json:"subnet_id"`
	Name       string    `json:"name"`
}

func (a *API) CreateDiscoveryJobFromTemplate(w http.ResponseWriter, r *http.Request) {
	var req createJobFromTemplateRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	tpl, err := a.Store.Queries.GetDiscoveryJobTemplate(r.Context(), req.TemplateID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "template not found")
		return
	}
	if req.SubnetID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, "subnet_id required")
		return
	}
	subnet, err := a.Store.Queries.GetSubnet(r.Context(), req.SubnetID)
	if err != nil || subnet.DeletedAt.Valid {
		httpx.Error(w, http.StatusBadRequest, "subnet not found")
		return
	}
	if !a.requireLocationAccess(w, r, subnet.LocationID) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("%s · %s", tpl.Name, subnet.Cidr)
	}
	item, err := a.Store.Queries.CreateDiscoveryJob(r.Context(), sqlcdb.CreateDiscoveryJobParams{
		Name:            name,
		SubnetID:        req.SubnetID,
		IntervalMinutes: tpl.IntervalMinutes,
		Enabled:         tpl.EnabledDefault,
		Enrich:          tpl.Enrich,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "discovery_job.create", "discovery_job", item.ID.String(), map[string]any{
		"name": item.Name, "subnet_id": item.SubnetID.String(), "template_id": tpl.ID.String(),
	})
	httpx.JSON(w, http.StatusCreated, toDiscoveryJobDTO(item))
}
