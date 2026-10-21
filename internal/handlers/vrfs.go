package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type vrfDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	RD          string    `json:"rd"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type vrfRequest struct {
	Name        string `json:"name"`
	RD          string `json:"rd"`
	Description string `json:"description"`
}

func toVRFIDTO(v sqlcdb.Vrf) vrfDTO {
	return vrfDTO{
		ID:          v.ID,
		Name:        v.Name,
		RD:          v.Rd,
		Description: v.Description,
		CreatedAt:   v.CreatedAt.Time,
		UpdatedAt:   v.UpdatedAt.Time,
	}
}

func (a *API) ListVRFs(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.Queries.ListVRFs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]vrfDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toVRFIDTO(item))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) GetVRF(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.GetVRF(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "vrf not found")
		return
	}
	httpx.JSON(w, http.StatusOK, toVRFIDTO(item))
}

func (a *API) CreateVRF(w http.ResponseWriter, r *http.Request) {
	var req vrfRequest
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	item, err := a.Store.Queries.CreateVRF(r.Context(), sqlcdb.CreateVRFParams{
		Name: req.Name, Rd: req.RD, Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, toVRFIDTO(item))
}

func (a *API) UpdateVRF(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req vrfRequest
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	item, err := a.Store.Queries.UpdateVRF(r.Context(), sqlcdb.UpdateVRFParams{
		ID: id, Name: req.Name, Rd: req.RD, Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, toVRFIDTO(item))
}

func (a *API) DeleteVRF(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if id == db.DefaultVRFID {
		httpx.Error(w, http.StatusBadRequest, "cannot delete the default VRF")
		return
	}
	if err := a.Store.Queries.DeleteVRF(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
