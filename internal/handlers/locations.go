package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
)

type locationDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type locationRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func toLocationDTO(l sqlcdb.Location) locationDTO {
	return locationDTO{
		ID:          l.ID,
		Name:        l.Name,
		Description: l.Description,
		CreatedAt:   l.CreatedAt.Time,
		UpdatedAt:   l.UpdatedAt.Time,
	}
}

func (a *API) ListLocations(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	items, err := a.Store.Queries.ListLocations(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]locationDTO, 0, len(items))
	for _, item := range items {
		id := item.ID
		if !sc.Allows(&id) {
			continue
		}
		out = append(out, toLocationDTO(item))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) GetLocation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.GetLocation(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "location not found")
		return
	}
	if !a.requireLocationAccess(w, r, db.UUIDToNull(id)) {
		return
	}
	subs, err := a.Store.Queries.ListSubnetsByLocation(r.Context(), db.UUIDToNull(id))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	counts, _ := a.Store.Queries.ListSubnetIPCounts(r.Context())
	usedBy := map[uuid.UUID]int{}
	for _, c := range counts {
		usedBy[c.SubnetID] = int(c.UsedCount)
	}
	vids := a.vlanVIDMap(r.Context())
	outSubs := make([]subnetDTO, 0, len(subs))
	siteUsed, siteTotal := 0, 0
	siteDense := true
	for _, s := range subs {
		dto := toSubnetDTO(s.ID, s.Cidr, s.Name, s.Description, s.LocationID, s.VrfID, s.VlanID, s.ParentID, s.Tags, s.CustomFields, s.CreatedAt, s.UpdatedAt, vids)
		if prefix, err := ipam.ParsePrefix(s.Cidr); err == nil {
			used := usedBy[s.ID]
			dto.Used = &used
			dto.Sparse = ipam.IsSparsePrefix(prefix)
			siteUsed += used
			if dto.Sparse {
				siteDense = false
			} else if usable, uerr := ipam.TotalUsable(prefix); uerr == nil {
				free := usable - used
				if free < 0 {
					free = 0
				}
				pct := 0.0
				if usable > 0 {
					pct = float64(used) / float64(usable) * 100
				}
				dto.Total = &usable
				dto.Free = &free
				dto.Percent = &pct
				siteTotal += usable
			}
		}
		outSubs = append(outSubs, dto)
	}
	utilOut := map[string]any{
		"used": siteUsed, "subnet_count": len(outSubs),
	}
	if siteDense && len(outSubs) > 0 {
		siteFree := siteTotal - siteUsed
		if siteFree < 0 {
			siteFree = 0
		}
		sitePct := 0.0
		if siteTotal > 0 {
			sitePct = float64(siteUsed) / float64(siteTotal) * 100
		}
		utilOut["total"] = siteTotal
		utilOut["free"] = siteFree
		utilOut["percent"] = sitePct
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"location":    toLocationDTO(item),
		"subnets":     outSubs,
		"utilization": utilOut,
	})
}

func (a *API) CreateLocation(w http.ResponseWriter, r *http.Request) {
	var req locationRequest
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	item, err := a.Store.Queries.CreateLocation(r.Context(), sqlcdb.CreateLocationParams{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, toLocationDTO(item))
}

func (a *API) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req locationRequest
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	item, err := a.Store.Queries.UpdateLocation(r.Context(), sqlcdb.UpdateLocationParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, toLocationDTO(item))
}

func (a *API) DeleteLocation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.Queries.DeleteLocation(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
