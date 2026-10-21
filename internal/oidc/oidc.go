package oidcauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"golang.org/x/oauth2"
)

type Config struct {
	Enabled       bool
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	DefaultRole   auth.Role
	AutoProvision bool
	FrontendURL   string
	GroupsClaim   string
	RoleClaim     string
	RoleMap       map[auth.Role][]string
	SyncRole      bool
}

type Identity struct {
	Email  string
	Role   auth.Role
	Groups []string
}

type pendingAuth struct {
	Expiry   time.Time
	Verifier string
	Nonce    string
}

type Service struct {
	cfg      Config
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    oauth2.Config
	states   sync.Map // state -> pendingAuth
}

func New(ctx context.Context, cfg Config) (*Service, error) {
	if !cfg.Enabled {
		return &Service{cfg: cfg}, nil
	}
	if cfg.Issuer == "" || cfg.ClientID == "" || cfg.RedirectURL == "" {
		return nil, fmt.Errorf("OIDC enabled but issuer/client_id/redirect_url incomplete")
	}
	if cfg.DefaultRole == "" {
		cfg.DefaultRole = auth.RoleReadOnly
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "http://localhost:5173"
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	if cfg.RoleClaim == "" {
		cfg.RoleClaim = "role"
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	s := &Service{
		cfg:      cfg,
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       cfg.Scopes,
		},
	}
	go s.gcStates()
	return s, nil
}

func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled && s.provider != nil
}

func (s *Service) Status() map[string]any {
	if s == nil || !s.cfg.Enabled {
		return map[string]any{"enabled": false}
	}
	mapped := map[string][]string{}
	for role, groups := range s.cfg.RoleMap {
		mapped[string(role)] = groups
	}
	return map[string]any{
		"enabled":        true,
		"issuer":         s.cfg.Issuer,
		"auto_provision": s.cfg.AutoProvision,
		"default_role":   string(s.cfg.DefaultRole),
		"groups_claim":   s.cfg.GroupsClaim,
		"role_claim":     s.cfg.RoleClaim,
		"role_map":       mapped,
		"sync_role":      s.cfg.SyncRole,
		"pkce":           true,
	}
}

func (s *Service) AuthCodeURL() (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("oidc disabled")
	}
	state, err := randomState()
	if err != nil {
		return "", err
	}
	verifier, err := randomPKCEVerifier()
	if err != nil {
		return "", err
	}
	nonce, err := randomState()
	if err != nil {
		return "", err
	}
	challenge := pkceChallengeS256(verifier)
	s.states.Store(state, pendingAuth{
		Expiry:   time.Now().Add(10 * time.Minute),
		Verifier: verifier,
		Nonce:    nonce,
	})
	return s.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oidc.Nonce(nonce),
	), nil
}

func (s *Service) Exchange(ctx context.Context, r *http.Request) (Identity, error) {
	if !s.Enabled() {
		return Identity{}, fmt.Errorf("oidc disabled")
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		return Identity{}, fmt.Errorf("missing state")
	}
	pendingRaw, ok := s.states.LoadAndDelete(state)
	if !ok {
		return Identity{}, fmt.Errorf("invalid state")
	}
	pending, _ := pendingRaw.(pendingAuth)
	if time.Now().After(pending.Expiry) {
		return Identity{}, fmt.Errorf("expired state")
	}
	if pending.Verifier == "" || pending.Nonce == "" {
		return Identity{}, fmt.Errorf("invalid state")
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		return Identity{}, fmt.Errorf("oidc error: %s", errMsg)
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		return Identity{}, fmt.Errorf("missing code")
	}
	tok, err := s.oauth.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", pending.Verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("token exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return Identity{}, fmt.Errorf("id_token missing")
	}
	idTok, err := s.verifier.Verify(ctx, rawID)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id_token: %w", err)
	}
	if idTok.Nonce != pending.Nonce {
		return Identity{}, fmt.Errorf("nonce mismatch")
	}
	var claims map[string]any
	if err := idTok.Claims(&claims); err != nil {
		return Identity{}, err
	}
	email := claimString(claims, "email")
	if email == "" {
		pref := claimString(claims, "preferred_username")
		if strings.Contains(pref, "@") {
			email = pref
		}
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return Identity{}, fmt.Errorf("email claim missing")
	}
	groups := claimStringSlice(claims, s.cfg.GroupsClaim)
	direct := claimString(claims, s.cfg.RoleClaim)
	role := ResolveRole(s.cfg.DefaultRole, s.cfg.RoleMap, direct, groups)
	return Identity{Email: email, Role: role, Groups: groups}, nil
}

func (s *Service) DefaultRole() auth.Role {
	if s == nil || s.cfg.DefaultRole == "" {
		return auth.RoleReadOnly
	}
	return s.cfg.DefaultRole
}

func (s *Service) AutoProvision() bool {
	return s != nil && s.cfg.AutoProvision
}

func (s *Service) SyncRole() bool {
	return s != nil && s.cfg.SyncRole
}

// PreviewClaims resolves a role from sample claim values using live OIDC config.
func (s *Service) PreviewClaims(directRole string, groups []string) RolePreview {
	if s == nil {
		return PreviewRole(auth.RoleReadOnly, nil, directRole, groups)
	}
	return PreviewRole(s.cfg.DefaultRole, s.cfg.RoleMap, directRole, groups)
}

func (s *Service) FrontendURL() string {
	if s == nil || s.cfg.FrontendURL == "" {
		return "http://localhost:5173"
	}
	return strings.TrimRight(s.cfg.FrontendURL, "/")
}

func (s *Service) gcStates() {
	t := time.NewTicker(5 * time.Minute)
	for range t.C {
		now := time.Now()
		s.states.Range(func(k, v any) bool {
			if p, ok := v.(pendingAuth); ok && now.After(p.Expiry) {
				s.states.Delete(k)
			}
			return true
		})
	}
}

func claimString(claims map[string]any, key string) string {
	if key == "" {
		return ""
	}
	v, ok := claims[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func claimStringSlice(claims map[string]any, key string) []string {
	if key == "" {
		return nil
	}
	v, ok := claims[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s := strings.TrimSpace(fmt.Sprint(item))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	case string:
		// comma-separated fallback
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	default:
		return nil
	}
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randomPKCEVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func RandomPassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
