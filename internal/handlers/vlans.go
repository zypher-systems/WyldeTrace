package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type vlanDTO struct {
	ID          uuid.UUID  `json:"id"`
	VID         int32      `json:"vid"`
	Name        string     `json:"name"`
	LocationID  *uuid.UUID `json:"location_id"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type vlanRequest struct {
	VID         int32      `json:"vid"`
	Name        string     `json:"name"`
	LocationID  *uuid.UUID `json:"location_id"`
	Description string     `json:"description"`
}

func toVLANDTO(v sqlcdb.Vlan) vlanDTO {
	return vlanDTO{
		ID:          v.ID,
		VID:         v.Vid,
		Name:        v.Name,
		LocationID:  db.NullUUID(v.LocationID),
		Description: v.Description,
		CreatedAt:   v.CreatedAt.Time,
		UpdatedAt:   v.UpdatedAt.Time,
	}
}

func (a *API) ListVLANs(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	items, err := a.Store.Queries.ListVLANs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]vlanDTO, 0, len(items))
	for _, item := range items {
		if item.LocationID.Valid && !sc.AllowsNullUUID(item.LocationID) {
			continue
		}
		out = append(out, toVLANDTO(item))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) GetVLAN(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.GetVLAN(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "vlan not found")
		return
	}
	if item.LocationID.Valid && !a.requireLocationAccess(w, r, item.LocationID) {
		return
	}
	httpx.JSON(w, http.StatusOK, toVLANDTO(item))
}

func (a *API) CreateVLAN(w http.ResponseWriter, r *http.Request) {
	var req vlanRequest
	if err := httpx.Decode(r, &req); err != nil || req.VID < 1 || req.VID > 4094 {
		httpx.Error(w, http.StatusBadRequest, "invalid payload (vid 1-4094 required)")
		return
	}
	if !a.requireLocationAccess(w, r, db.PtrUUIDToNull(req.LocationID)) {
		return
	}
	item, err := a.Store.Queries.CreateVLAN(r.Context(), sqlcdb.CreateVLANParams{
		Vid: req.VID, Name: req.Name, LocationID: db.PtrUUIDToNull(req.LocationID), Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, toVLANDTO(item))
}

func (a *API) UpdateVLAN(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req vlanRequest
	if err := httpx.Decode(r, &req); err != nil || req.VID < 1 || req.VID > 4094 {
		httpx.Error(w, http.StatusBadRequest, "invalid payload (vid 1-4094 required)")
		return
	}
	if !a.requireLocationAccess(w, r, db.PtrUUIDToNull(req.LocationID)) {
		return
	}
	item, err := a.Store.Queries.UpdateVLAN(r.Context(), sqlcdb.UpdateVLANParams{
		ID: id, Vid: req.VID, Name: req.Name, LocationID: db.PtrUUIDToNull(req.LocationID), Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, toVLANDTO(item))
}

func (a *API) DeleteVLAN(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetVLAN(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "vlan not found")
		return
	}
	if existing.LocationID.Valid && !a.requireLocationAccess(w, r, existing.LocationID) {
		return
	}
	if err := a.Store.Queries.DeleteVLAN(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
