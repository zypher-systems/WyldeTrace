package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type userDTO struct {
	ID                   uuid.UUID   `json:"id"`
	Email                string      `json:"email"`
	Role                 string      `json:"role"`
	LocationIDs          []uuid.UUID `json:"location_ids"`
	SiteScoped           bool        `json:"site_scoped"`
	TOTPEnabled          bool        `json:"totp_enabled"`
	UsingDefaultPassword bool        `json:"using_default_password,omitempty"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
}

func toUserDTO(u sqlcdb.User, locationIDs []uuid.UUID) userDTO {
	if locationIDs == nil {
		locationIDs = []uuid.UUID{}
	}
	role := u.Role
	scoped := len(locationIDs) > 0 && role != "admin"
	return userDTO{
		ID:          u.ID,
		Email:       u.Email,
		Role:        role,
		LocationIDs: locationIDs,
		SiteScoped:  scoped,
		TOTPEnabled: u.TotpEnabled,
		CreatedAt:   u.CreatedAt.Time,
		UpdatedAt:   u.UpdatedAt.Time,
	}
}

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateUserRequest struct {
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	Password *string `json:"password"`
}

func (a *API) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.Store.Queries.ListUsers(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		locs, _ := a.Store.Queries.ListLocationIDsByUser(r.Context(), u.ID)
		out = append(out, toUserDTO(u, locs))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	role, err := auth.ParseRole(req.Role)
	if err != nil || req.Email == "" || len(req.Password) < 8 {
		httpx.Error(w, http.StatusBadRequest, "invalid user payload")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "hash failed")
		return
	}
	user, err := a.Store.Queries.CreateUser(r.Context(), sqlcdb.CreateUserParams{
		Email:        req.Email,
		PasswordHash: hash,
		Role:         string(role),
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "user.create", "user", user.ID.String(), map[string]any{"email": user.Email, "role": user.Role})
	httpx.JSON(w, http.StatusCreated, toUserDTO(user, nil))
}

func (a *API) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateUserRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	role, err := auth.ParseRole(req.Role)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid role")
		return
	}
	params := sqlcdb.UpdateUserParams{
		ID:    id,
		Email: strings.TrimSpace(strings.ToLower(req.Email)),
		Role:  string(role),
	}
	if req.Password != nil && *req.Password != "" {
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "hash failed")
			return
		}
		params.PasswordHash = pgtype.Text{String: hash, Valid: true}
	}
	user, err := a.Store.Queries.UpdateUser(r.Context(), params)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	locs, _ := a.Store.Queries.ListLocationIDsByUser(r.Context(), user.ID)
	a.audit(r.Context(), "user.update", "user", user.ID.String(), map[string]any{"email": user.Email, "role": user.Role})
	httpx.JSON(w, http.StatusOK, toUserDTO(user, locs))
}

type setUserLocationsRequest struct {
	LocationIDs []uuid.UUID `json:"location_ids"`
}

func (a *API) GetUserLocations(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	locs, err := a.Store.Queries.ListLocationIDsByUser(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if locs == nil {
		locs = []uuid.UUID{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"location_ids": locs})
}

func (a *API) SetUserLocations(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req setUserLocationsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := a.Store.Queries.GetUserByID(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	for _, locID := range req.LocationIDs {
		if _, err := a.Store.Queries.GetLocation(r.Context(), locID); err != nil {
			httpx.Error(w, http.StatusBadRequest, "unknown location: "+locID.String())
			return
		}
	}
	if err := a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		if err := q.ClearUserLocations(r.Context(), id); err != nil {
			return err
		}
		for _, locID := range req.LocationIDs {
			if err := q.InsertUserLocation(r.Context(), sqlcdb.InsertUserLocationParams{
				UserID: id, LocationID: locID,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	locs, _ := a.Store.Queries.ListLocationIDsByUser(r.Context(), id)
	a.audit(r.Context(), "user.locations", "user", id.String(), map[string]any{"location_ids": locs})
	httpx.JSON(w, http.StatusOK, map[string]any{"location_ids": locs})
}

func (a *API) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.Queries.DeleteUser(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "user.delete", "user", id.String(), map[string]any{})
	w.WriteHeader(http.StatusNoContent)
}
