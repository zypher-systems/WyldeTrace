package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/wyldetechnologies/wyldetrace/internal/handlers"
	"github.com/wyldetechnologies/wyldetrace/internal/server"
	"gopkg.in/yaml.v3"
)

// Paths registered on the chi mux but intentionally omitted from
// docs/openapi.yaml (non-/api/v1 ops surface or SPA fallback).
var openAPIIgnoredPaths = map[string]bool{
	"/healthz": true, // also mounted at /api/v1/healthz (documented)
	"/metrics": true, // Prometheus scrape (chi Handle registers all methods)
	"/*":       true, // SPA catch-all
}

var openAPIMethods = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true,
	"options": true, "head": true, "trace": true,
}

type openAPIDoc struct {
	Servers []struct {
		URL string `yaml:"url"`
	} `yaml:"servers"`
	Paths map[string]map[string]any `yaml:"paths"`
}

func TestOpenAPIRouteDrift(t *testing.T) {
	t.Parallel()

	mux, ok := server.New(server.Options{API: &handlers.API{}}).(*chi.Mux)
	if !ok {
		t.Fatal("server.New did not return *chi.Mux")
	}

	routerRoutes := map[string]bool{}
	err := chi.Walk(mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = normalizeRoute(route)
		if openAPIIgnoredPaths[route] {
			return nil
		}
		routerRoutes[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}

	doc := loadOpenAPI(t)
	base := "/api/v1"
	if len(doc.Servers) > 0 && strings.TrimSpace(doc.Servers[0].URL) != "" {
		base = strings.TrimRight(doc.Servers[0].URL, "/")
	}

	specRoutes := map[string]bool{}
	for path, ops := range doc.Paths {
		full := base + "/" + strings.TrimPrefix(path, "/")
		if path == "/" {
			full = base
		}
		full = normalizeRoute(full)
		for method := range ops {
			m := strings.ToLower(method)
			if !openAPIMethods[m] {
				continue
			}
			specRoutes[strings.ToUpper(m)+" "+full] = true
		}
	}

	var missingInSpec, missingInRouter []string
	for r := range routerRoutes {
		if !specRoutes[r] {
			missingInSpec = append(missingInSpec, r)
		}
	}
	for r := range specRoutes {
		if !routerRoutes[r] {
			missingInRouter = append(missingInRouter, r)
		}
	}
	sort.Strings(missingInSpec)
	sort.Strings(missingInRouter)

	if len(missingInSpec) > 0 || len(missingInRouter) > 0 {
		var b strings.Builder
		b.WriteString("OpenAPI drift vs chi router (docs/openapi.yaml)\n")
		if len(missingInSpec) > 0 {
			b.WriteString("\nIn router but missing from OpenAPI:\n")
			for _, r := range missingInSpec {
				b.WriteString("  - " + r + "\n")
			}
		}
		if len(missingInRouter) > 0 {
			b.WriteString("\nIn OpenAPI but missing from router:\n")
			for _, r := range missingInRouter {
				b.WriteString("  - " + r + "\n")
			}
		}
		t.Fatal(b.String())
	}
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return "/"
	}
	// chi sometimes appends a trailing slash for mount points.
	if len(route) > 1 && strings.HasSuffix(route, "/") {
		route = strings.TrimSuffix(route, "/")
	}
	return route
}

func loadOpenAPI(t *testing.T) openAPIDoc {
	t.Helper()
	path := findOpenAPIPath(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("openapi paths empty")
	}
	return doc
}

func findOpenAPIPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"docs/openapi.yaml",
		filepath.Join("..", "..", "docs", "openapi.yaml"),
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(file), "..", "..", "docs", "openapi.yaml"))
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(dir, "docs", "openapi.yaml"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	t.Fatalf("docs/openapi.yaml not found; tried: %v", candidates)
	return ""
}
