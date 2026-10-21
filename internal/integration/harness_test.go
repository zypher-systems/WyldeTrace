//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/handlers"
	"github.com/wyldetechnologies/wyldetrace/internal/server"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

type harness struct {
	t      *testing.T
	store  *db.Store
	tokens *auth.TokenService
	srv    *httptest.Server
	api    *handlers.API
}

func openHarness(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	mig := migrationsPath()
	if err := db.RunMigrations(dsn, mig); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("db unavailable: %v", err)
	}
	t.Cleanup(store.Close)

	tokens := auth.NewTokenService("integration-test-secret", time.Hour)
	hooks := webhook.New(store)
	api := handlers.New(store, tokens, nil, hooks, nil, "changeme")
	api.AllowPlainSecrets = true
	h := server.New(server.Options{
		API: api, Tokens: tokens, Store: store, CORSOrigins: []string{"*"},
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &harness{t: t, store: store, tokens: tokens, srv: srv, api: api}
}

func migrationsPath() string {
	for _, p := range []string{"db/migrations", "/app/db/migrations"} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	// Fall back relative to this file when tests run from package dir.
	root, _ := filepath.Abs("../..")
	return filepath.Join(root, "db/migrations")
}

func (h *harness) createUser(role auth.Role, email string) (uuid.UUID, string) {
	h.t.Helper()
	hash, err := auth.HashPassword("test-pass-1")
	if err != nil {
		h.t.Fatal(err)
	}
	u, err := h.store.Queries.CreateUser(context.Background(), sqlcdb.CreateUserParams{
		Email: email, PasswordHash: hash, Role: string(role),
	})
	if err != nil {
		h.t.Fatalf("create user: %v", err)
	}
	tok, err := h.tokens.Issue(u.ID, u.Email, role)
	if err != nil {
		h.t.Fatal(err)
	}
	return u.ID, tok
}

func (h *harness) setUserLocations(userID uuid.UUID, locs []uuid.UUID) {
	h.t.Helper()
	ctx := context.Background()
	if err := h.store.Queries.ClearUserLocations(ctx, userID); err != nil {
		h.t.Fatal(err)
	}
	for _, lid := range locs {
		if err := h.store.Queries.InsertUserLocation(ctx, sqlcdb.InsertUserLocationParams{
			UserID: userID, LocationID: lid,
		}); err != nil {
			h.t.Fatal(err)
		}
	}
}

func (h *harness) createLocation(name string) uuid.UUID {
	h.t.Helper()
	loc, err := h.store.Queries.CreateLocation(context.Background(), sqlcdb.CreateLocationParams{
		Name: name, Description: "integration",
	})
	if err != nil {
		h.t.Fatalf("create location: %v", err)
	}
	return loc.ID
}

func (h *harness) createSubnet(cidr, name string, locID uuid.UUID) uuid.UUID {
	h.t.Helper()
	row, err := h.store.Queries.CreateSubnet(context.Background(), sqlcdb.CreateSubnetParams{
		Cidr: cidr, Name: name, Description: "integration",
		LocationID: db.UUIDToNull(locID), VrfID: db.DefaultVRFID,
		VlanID: pgtype.UUID{}, ParentID: pgtype.UUID{},
		Tags: []string{"integration"}, CustomFields: []byte(`{}`),
	})
	if err != nil {
		h.t.Fatalf("create subnet %s: %v", cidr, err)
	}
	return row.ID
}

func (h *harness) do(method, path, token string, body any) (int, []byte) {
	h.t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, rdr)
	if err != nil {
		h.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.NewString()[:8])
}

// uniqueCIDR returns a /24 unlikely to collide with other integration tests.
func uniqueCIDR() string {
	id := uuid.New()
	// Avoid 0.x and keep host bits free for .10 style addresses in tests.
	a := 1 + int(id[0])%254
	b := int(id[1])
	return fmt.Sprintf("10.%d.%d.0/24", a, b)
}
