package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/scope"
)

func (a *API) requireScope(w http.ResponseWriter, r *http.Request) (scope.Scope, bool) {
	s, err := scope.Load(r.Context(), a.Store)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return scope.Scope{}, false
	}
	return s, true
}

func (a *API) requireLocationAccess(w http.ResponseWriter, r *http.Request, locationID pgtype.UUID) bool {
	s, ok := a.requireScope(w, r)
	if !ok {
		return false
	}
	if !s.AllowsNullUUID(locationID) {
		httpx.Error(w, http.StatusForbidden, "location out of scope")
		return false
	}
	return true
}

func (a *API) requireSubnetAccess(w http.ResponseWriter, r *http.Request, subnetID uuid.UUID) bool {
	subnet, err := a.Store.Queries.GetSubnet(r.Context(), subnetID)
	if err != nil || subnet.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return false
	}
	return a.requireLocationAccess(w, r, subnet.LocationID)
}
