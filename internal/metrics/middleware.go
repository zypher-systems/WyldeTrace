package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Middleware records request counts and latency. Path labels use the chi route
// pattern when available so cardinality stays bounded.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		path := routeLabel(r)
		status := strconv.Itoa(ww.Status())
		if status == "0" {
			status = "200"
		}
		class := status[:1] + "xx"
		HTTPRequests.WithLabelValues(r.Method, path, class).Inc()
		HTTPDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

func routeLabel(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	p := r.URL.Path
	if strings.HasPrefix(p, "/api/") {
		return "/api/*"
	}
	if p == "/metrics" || p == "/healthz" {
		return p
	}
	return "/*"
}
