package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	JWTSecret      string
	JWTTTL         time.Duration
	AdminEmail     string
	AdminPassword  string
	MigrationsPath string
	// DevMode relaxes lab-only checks: optional secrets encryption, wildcard CORS, and http outbound URLs.
	DevMode bool

	OIDCEnabled       bool
	OIDCIssuer        string
	OIDCClientID      string
	OIDCClientSecret  string
	OIDCRedirectURL   string
	OIDCScopes        []string
	OIDCDefaultRole   string
	OIDCAutoProvision bool
	OIDCGroupsClaim   string
	OIDCRoleClaim     string
	OIDCRoleMap       string
	OIDCSyncRole      bool
	PublicAppURL      string
	CORSOrigins       []string
	// SettingsEncryptionKey is 32 bytes (64 hex or base64). Required in prod to store MFA/integration secrets.
	SettingsEncryptionKey string
	MetricsToken          string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:       getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		JWTTTL:         24 * time.Hour,
		AdminEmail:     getenv("ADMIN_EMAIL", "admin@wyldetech.local"),
		AdminPassword:  getenv("ADMIN_PASSWORD", "changeme"),
		MigrationsPath: getenv("MIGRATIONS_PATH", "db/migrations"),
		DevMode:        parseBool(os.Getenv("WYLDTRACE_DEV"), false),

		OIDCEnabled:           parseBool(os.Getenv("OIDC_ENABLED"), false),
		OIDCIssuer:            strings.TrimSpace(os.Getenv("OIDC_ISSUER")),
		OIDCClientID:          strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID")),
		OIDCClientSecret:      os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:       strings.TrimSpace(os.Getenv("OIDC_REDIRECT_URL")),
		OIDCScopes:            splitCSV(getenv("OIDC_SCOPES", "openid,profile,email")),
		OIDCDefaultRole:       getenv("OIDC_DEFAULT_ROLE", "readonly"),
		OIDCAutoProvision:     parseBool(os.Getenv("OIDC_AUTO_PROVISION"), true),
		OIDCGroupsClaim:       getenv("OIDC_GROUPS_CLAIM", "groups"),
		OIDCRoleClaim:         getenv("OIDC_ROLE_CLAIM", "role"),
		OIDCRoleMap:           os.Getenv("OIDC_ROLE_MAP"),
		OIDCSyncRole:          parseBool(os.Getenv("OIDC_SYNC_ROLE"), false),
		PublicAppURL:          getenv("PUBLIC_APP_URL", "http://localhost:5173"),
		CORSOrigins:           splitCSV(os.Getenv("CORS_ORIGINS")),
		SettingsEncryptionKey: firstNonEmpty(os.Getenv("SETTINGS_ENCRYPTION_KEY"), os.Getenv("WYLDTRACE_SECRETS_KEY")),
		MetricsToken:          strings.TrimSpace(os.Getenv("METRICS_TOKEN")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	if !cfg.DevMode && cfg.SettingsEncryptionKey == "" {
		return Config{}, fmt.Errorf("SETTINGS_ENCRYPTION_KEY is required unless WYLDTRACE_DEV=true")
	}
	if !cfg.DevMode && (len(cfg.CORSOrigins) == 0 || (len(cfg.CORSOrigins) == 1 && cfg.CORSOrigins[0] == "*")) {
		return Config{}, fmt.Errorf("CORS_ORIGINS must list explicit origins unless WYLDTRACE_DEV=true (wildcard not allowed)")
	}
	if cfg.DevMode && len(cfg.CORSOrigins) == 0 {
		cfg.CORSOrigins = []string{"*"}
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseBool(v string, fallback bool) bool {
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
