package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type oidcCodeEntry struct {
	Token     string
	ExpiresAt time.Time
}

var oidcCodes = struct {
	mu sync.Mutex
	m  map[string]oidcCodeEntry
}{m: map[string]oidcCodeEntry{}}

func mintOIDCLoginCode(token string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	code := hex.EncodeToString(b[:])
	oidcCodes.mu.Lock()
	defer oidcCodes.mu.Unlock()
	// prune expired
	now := time.Now()
	for k, v := range oidcCodes.m {
		if now.After(v.ExpiresAt) {
			delete(oidcCodes.m, k)
		}
	}
	oidcCodes.m[code] = oidcCodeEntry{Token: token, ExpiresAt: now.Add(2 * time.Minute)}
	return code, nil
}

func consumeOIDCLoginCode(code string) (string, bool) {
	oidcCodes.mu.Lock()
	defer oidcCodes.mu.Unlock()
	ent, ok := oidcCodes.m[code]
	if !ok {
		return "", false
	}
	delete(oidcCodes.m, code)
	if time.Now().After(ent.ExpiresAt) {
		return "", false
	}
	return ent.Token, true
}

func (a *API) OIDCExchange(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.Code == "" {
		httpx.Error(w, http.StatusBadRequest, "code required")
		return
	}
	token, ok := consumeOIDCLoginCode(req.Code)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid or expired code")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"token": token})
}
