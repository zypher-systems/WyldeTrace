package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type deviceTypeDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type deviceTypeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func toDeviceTypeDTO(d sqlcdb.DeviceType) deviceTypeDTO {
	return deviceTypeDTO{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		CreatedAt:   d.CreatedAt.Time,
		UpdatedAt:   d.UpdatedAt.Time,
	}
}

func (a *API) ListDeviceTypes(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.Queries.ListDeviceTypes(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]deviceTypeDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toDeviceTypeDTO(item))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CreateDeviceType(w http.ResponseWriter, r *http.Request) {
	var req deviceTypeRequest
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	item, err := a.Store.Queries.CreateDeviceType(r.Context(), sqlcdb.CreateDeviceTypeParams{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, toDeviceTypeDTO(item))
}

func (a *API) UpdateDeviceType(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req deviceTypeRequest
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	item, err := a.Store.Queries.UpdateDeviceType(r.Context(), sqlcdb.UpdateDeviceTypeParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, toDeviceTypeDTO(item))
}

func (a *API) DeleteDeviceType(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.Queries.DeleteDeviceType(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
