package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/apitoken"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/middleware"
)

type apiTokenDTO struct {
	ID          uuid.UUID   `json:"id"`
	Name        string      `json:"name"`
	Prefix      string      `json:"prefix"`
	Role        string      `json:"role"`
	LocationIDs []uuid.UUID `json:"location_ids"`
	SiteScoped  bool        `json:"site_scoped"`
	CreatedBy   *uuid.UUID  `json:"created_by"`
	LastUsedAt  *time.Time  `json:"last_used_at"`
	ExpiresAt   *time.Time  `json:"expires_at"`
	RevokedAt   *time.Time  `json:"revoked_at"`
	CreatedAt   time.Time   `json:"created_at"`
	Token       string      `json:"token,omitempty"`
}

func (a *API) toAPITokenDTO(r *http.Request, t sqlcdb.ApiToken, plaintext string) apiTokenDTO {
	ids, _ := a.Store.Queries.ListLocationIDsByAPIToken(r.Context(), t.ID)
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return apiTokenDTO{
		ID: t.ID, Name: t.Name, Prefix: t.Prefix, Role: t.Role,
		LocationIDs: ids, SiteScoped: len(ids) > 0,
		CreatedBy: db.NullUUID(t.CreatedBy),
		LastUsedAt: timestamptzPtr(t.LastUsedAt),
		ExpiresAt:  timestamptzPtr(t.ExpiresAt),
		RevokedAt:  timestamptzPtr(t.RevokedAt),
		CreatedAt:  t.CreatedAt.Time,
		Token:      plaintext,
	}
}

type createAPITokenRequest struct {
	Name        string      `json:"name"`
	Role        string      `json:"role"`
	ExpiresAt   *time.Time  `json:"expires_at"`
	LocationIDs []uuid.UUID `json:"location_ids"`
}

func (a *API) ListAPITokens(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.Queries.ListAPITokens(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]apiTokenDTO, 0, len(items))
	for _, item := range items {
		out = append(out, a.toAPITokenDTO(r, item, ""))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	var req createAPITokenRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	role, err := auth.ParseRole(req.Role)
	if err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid token payload")
		return
	}
	// Admin tokens cannot be site-scoped (admins are always unrestricted).
	if role == auth.RoleAdmin && len(req.LocationIDs) > 0 {
		httpx.Error(w, http.StatusBadRequest, "admin tokens cannot be site-scoped; use editor/readonly with locations")
		return
	}
	for _, lid := range req.LocationIDs {
		if _, err := a.Store.Queries.GetLocation(r.Context(), lid); err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid location_id: "+lid.String())
			return
		}
	}
	plaintext, prefix, hash, err := apitoken.Generate()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	createdBy := pgtype.UUID{}
	if claims, ok := middleware.ClaimsFromContext(r.Context()); ok && !claims.IsAPIToken {
		createdBy = db.UUIDToNull(claims.UserID)
	}
	item, err := a.Store.Queries.CreateAPIToken(r.Context(), sqlcdb.CreateAPITokenParams{
		Name:      req.Name,
		Prefix:    prefix,
		TokenHash: hash,
		Role:      string(role),
		CreatedBy: createdBy,
		ExpiresAt: ptrToTimestamptz(req.ExpiresAt),
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, lid := range req.LocationIDs {
		_ = a.Store.Queries.InsertAPITokenLocation(r.Context(), sqlcdb.InsertAPITokenLocationParams{
			TokenID: item.ID, LocationID: lid,
		})
	}
	a.audit(r.Context(), "api_token.create", "api_token", item.ID.String(), map[string]any{
		"name": item.Name, "role": item.Role, "prefix": item.Prefix,
		"location_ids": req.LocationIDs, "site_scoped": len(req.LocationIDs) > 0,
	})
	httpx.JSON(w, http.StatusCreated, a.toAPITokenDTO(r, item, plaintext))
}

func (a *API) SetAPITokenLocations(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	tok, err := a.Store.Queries.GetAPITokenByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "token not found")
		return
	}
	if tok.Role == string(auth.RoleAdmin) {
		httpx.Error(w, http.StatusBadRequest, "admin tokens cannot be site-scoped")
		return
	}
	var req struct {
		LocationIDs []uuid.UUID `json:"location_ids"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	for _, lid := range req.LocationIDs {
		if _, err := a.Store.Queries.GetLocation(r.Context(), lid); err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid location_id")
			return
		}
	}
	if err := a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		if err := q.ClearAPITokenLocations(r.Context(), id); err != nil {
			return err
		}
		for _, lid := range req.LocationIDs {
			if err := q.InsertAPITokenLocation(r.Context(), sqlcdb.InsertAPITokenLocationParams{
				TokenID: id, LocationID: lid,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "api_token.locations", "api_token", id.String(), map[string]any{
		"location_ids": req.LocationIDs, "site_scoped": len(req.LocationIDs) > 0,
	})
	httpx.JSON(w, http.StatusOK, a.toAPITokenDTO(r, tok, ""))
}

func (a *API) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.RevokeAPIToken(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "token not found or already revoked")
		return
	}
	a.audit(r.Context(), "api_token.revoke", "api_token", item.ID.String(), map[string]any{
		"name": item.Name, "prefix": item.Prefix,
	})
	httpx.JSON(w, http.StatusOK, a.toAPITokenDTO(r, item, ""))
}

func (a *API) DeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.Queries.DeleteAPIToken(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "api_token.delete", "api_token", id.String(), map[string]any{})
	w.WriteHeader(http.StatusNoContent)
}
