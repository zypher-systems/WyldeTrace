package handlers

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	oidcauth "github.com/wyldetechnologies/wyldetrace/internal/oidc"
)

func (a *API) OIDCStatus(w http.ResponseWriter, r *http.Request) {
	if a.OIDC == nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	httpx.JSON(w, http.StatusOK, a.OIDC.Status())
}

type oidcPreviewRequest struct {
	Groups      []string `json:"groups"`
	Role        string   `json:"role"`
	CurrentRole string   `json:"current_role"`
}

func (a *API) OIDCPreview(w http.ResponseWriter, r *http.Request) {
	if a.OIDC == nil || !a.OIDC.Enabled() {
		httpx.Error(w, http.StatusBadRequest, "oidc disabled")
		return
	}
	var req oidcPreviewRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	preview := a.OIDC.PreviewClaims(req.Role, req.Groups)
	if req.CurrentRole != "" {
		preview.CurrentRole = req.CurrentRole
		preview.WouldSync = a.OIDC.SyncRole() && string(preview.Role) != req.CurrentRole
	}
	httpx.JSON(w, http.StatusOK, preview)
}

func (a *API) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	if a.OIDC == nil || !a.OIDC.Enabled() {
		httpx.Error(w, http.StatusNotFound, "oidc disabled")
		return
	}
	u, err := a.OIDC.AuthCodeURL()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, u, http.StatusFound)
}

func (a *API) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if a.OIDC == nil || !a.OIDC.Enabled() {
		httpx.Error(w, http.StatusNotFound, "oidc disabled")
		return
	}
	front := a.OIDC.FrontendURL()
	ident, err := a.OIDC.Exchange(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, front+"/login?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	user, err := a.Store.Queries.GetUserByEmail(r.Context(), ident.Email)
	roleSynced := false
	prevRole := ""
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) || !a.OIDC.AutoProvision() {
			http.Redirect(w, r, front+"/login?error="+url.QueryEscape("user not provisioned"), http.StatusFound)
			return
		}
		pw, perr := oidcauth.RandomPassword()
		if perr != nil {
			http.Redirect(w, r, front+"/login?error=provision_failed", http.StatusFound)
			return
		}
		hash, herr := auth.HashPassword(pw)
		if herr != nil {
			http.Redirect(w, r, front+"/login?error=provision_failed", http.StatusFound)
			return
		}
		role := string(ident.Role)
		user, err = a.Store.Queries.CreateUser(r.Context(), sqlcdb.CreateUserParams{
			Email: ident.Email, PasswordHash: hash, Role: role,
		})
		if err != nil {
			http.Redirect(w, r, front+"/login?error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
		a.audit(r.Context(), "auth.oidc.provision", "user", user.ID.String(), map[string]any{
			"email": ident.Email, "role": role, "groups": ident.Groups,
		})
	} else if a.OIDC.SyncRole() && string(ident.Role) != user.Role {
		prevRole = user.Role
		updated, uerr := a.Store.Queries.UpdateUser(r.Context(), sqlcdb.UpdateUserParams{
			ID: user.ID, Email: user.Email, Role: string(ident.Role), PasswordHash: pgtype.Text{},
		})
		if uerr == nil {
			a.audit(r.Context(), "auth.oidc.role_sync", "user", user.ID.String(), map[string]any{
				"from": user.Role, "to": ident.Role, "groups": ident.Groups,
			})
			user = updated
			roleSynced = true
		}
	}
	parsed, err := auth.ParseRole(user.Role)
	if err != nil {
		http.Redirect(w, r, front+"/login?error=invalid_role", http.StatusFound)
		return
	}
	token, err := a.Tokens.Issue(user.ID, user.Email, parsed)
	if err != nil {
		http.Redirect(w, r, front+"/login?error=token_issue", http.StatusFound)
		return
	}
	a.audit(r.Context(), "auth.oidc.login", "user", user.ID.String(), map[string]any{
		"email": ident.Email, "role": parsed, "groups": ident.Groups,
	})
	code, err := mintOIDCLoginCode(token)
	if err != nil {
		http.Redirect(w, r, front+"/login?error=token_issue", http.StatusFound)
		return
	}
	redir := front + "/login?code=" + url.QueryEscape(code)
	if roleSynced {
		redir += "&role_synced=1&from=" + url.QueryEscape(prevRole) + "&to=" + url.QueryEscape(string(ident.Role))
	}
	http.Redirect(w, r, redir, http.StatusFound)
}
