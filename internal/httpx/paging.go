package httpx

import (
	"net/http"
	"strconv"
	"strings"
)

type PageQuery struct {
	Page     int32
	PageSize int32
	Offset   int32
	Q        string
	Status   string
}

type SortQuery struct {
	By  string
	Dir string // "asc" or "desc"
}

type PageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int32 `json:"page"`
	PageSize int32 `json:"page_size"`
}

func ParsePage(r *http.Request) PageQuery {
	page := int32(parseIntDefault(r.URL.Query().Get("page"), 1))
	if page < 1 {
		page = 1
	}
	size := int32(parseIntDefault(r.URL.Query().Get("page_size"), 25))
	if size < 1 {
		size = 25
	}
	if size > 200 {
		size = 200
	}
	return PageQuery{
		Page:     page,
		PageSize: size,
		Offset:   (page - 1) * size,
		Q:        strings.TrimSpace(r.URL.Query().Get("q")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
	}
}

// ParseSort reads sort/order query params, whitelisting columns.
// allowed maps client-facing sort keys (e.g. "cidr") to themselves.
func ParseSort(r *http.Request, allowed map[string]bool, defaultBy, defaultDir string) SortQuery {
	by := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if !allowed[by] {
		by = defaultBy
	}
	dir := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	if dir != "asc" && dir != "desc" {
		dir = strings.ToLower(defaultDir)
	}
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}
	return SortQuery{By: by, Dir: dir}
}

func parseIntDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
