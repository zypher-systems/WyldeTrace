package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}
