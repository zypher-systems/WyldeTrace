package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/apitoken"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type ctxKey string

const claimsKey ctxKey = "claims"

func WithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*auth.Claims)
	return c, ok
}

func RequireAuth(tokens *auth.TokenService, store *db.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httpx.Error(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			raw := strings.TrimPrefix(header, "Bearer ")
			claims, err := tokens.ParseSession(raw)
			if err != nil {
				claims, err = authenticateAPIToken(r.Context(), store, raw)
				if err != nil {
					httpx.Error(w, http.StatusUnauthorized, "invalid token")
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}

func authenticateAPIToken(ctx context.Context, store *db.Store, raw string) (*auth.Claims, error) {
	prefix, ok := apitoken.Parse(raw)
	if !ok {
		return nil, auth.ErrInvalidToken
	}
	row, err := store.Queries.GetAPITokenByPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}
	want := apitoken.Hash(raw)
	if subtle.ConstantTimeCompare([]byte(want), []byte(row.TokenHash)) != 1 {
		return nil, auth.ErrInvalidToken
	}
	role, err := auth.ParseRole(row.Role)
	if err != nil {
		return nil, err
	}
	actor := uuid.Nil
	if row.CreatedBy.Valid {
		actor = uuid.UUID(row.CreatedBy.Bytes)
	}
	_ = store.Queries.TouchAPIToken(ctx, row.ID)
	return &auth.Claims{
		UserID:     actor,
		Email:      "api:" + row.Name,
		Role:       role,
		IsAPIToken: true,
		TokenID:    row.ID,
		TokenName:  row.Name,
	}, nil
}

func RequireRole(min auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if claims.Role.Rank() < min.Rank() {
				httpx.Error(w, http.StatusForbidden, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireReports allows reportview plus any role that can read IPAM (readonly+).
func RequireReports(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !claims.Role.CanViewReports() {
			httpx.Error(w, http.StatusForbidden, "insufficient role")
			return
		}
		next.ServeHTTP(w, r)
	})
}
