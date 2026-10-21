package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/middleware"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token       string  `json:"token,omitempty"`
	User        userDTO `json:"user,omitempty"`
	MFARequired bool    `json:"mfa_required,omitempty"`
	MFAToken    string  `json:"mfa_token,omitempty"`
}

func (a *API) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !loginAllowed(ip) {
		httpx.Error(w, http.StatusTooManyRequests, "too many failed logins; try again later")
		return
	}
	var req loginRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "email and password required")
		return
	}

	user, err := a.Store.Queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		loginFail(ip)
		httpx.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	role, err := auth.ParseRole(user.Role)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "invalid user role")
		return
	}
	if user.TotpEnabled {
		mfaToken, err := a.Tokens.IssueMFAPending(user.ID, user.Email, role)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "token issue failed")
			return
		}
		httpx.JSON(w, http.StatusOK, loginResponse{
			MFARequired: true,
			MFAToken:    mfaToken,
		})
		return
	}
	loginSuccess(ip)
	token, err := a.Tokens.Issue(user.ID, user.Email, role)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	locs, _ := a.Store.Queries.ListLocationIDsByUser(r.Context(), user.ID)
	dto := toUserDTO(user, locs)
	dto.UsingDefaultPassword = a.usingBootstrapPassword(user.PasswordHash)
	httpx.JSON(w, http.StatusOK, loginResponse{
		Token: token,
		User:  dto,
	})
}

func (a *API) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if claims.IsAPIToken {
		locs, _ := a.Store.Queries.ListLocationIDsByAPIToken(r.Context(), claims.TokenID)
		if locs == nil {
			locs = []uuid.UUID{}
		}
		httpx.JSON(w, http.StatusOK, userDTO{
			ID: claims.UserID, Email: claims.Email, Role: string(claims.Role),
			LocationIDs: locs, SiteScoped: len(locs) > 0,
		})
		return
	}
	user, err := a.Store.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	locs, _ := a.Store.Queries.ListLocationIDsByUser(r.Context(), user.ID)
	dto := toUserDTO(user, locs)
	dto.UsingDefaultPassword = a.usingBootstrapPassword(user.PasswordHash)
	httpx.JSON(w, http.StatusOK, dto)
}

func (a *API) usingBootstrapPassword(hash string) bool {
	if a == nil || a.BootstrapPassword == "" {
		return false
	}
	return auth.CheckPassword(hash, a.BootstrapPassword)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (a *API) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req changePasswordRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.NewPassword) < 8 {
		httpx.Error(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}
	user, err := a.Store.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		httpx.Error(w, http.StatusUnauthorized, "current password incorrect")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "hash failed")
		return
	}
	_, err = a.Store.Queries.UpdateUser(r.Context(), sqlcdb.UpdateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		Role:         user.Role,
		PasswordHash: pgtype.Text{String: hash, Valid: true},
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "user.password_change", "user", user.ID.String(), map[string]any{})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
