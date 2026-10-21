package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/config"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	"github.com/wyldetechnologies/wyldetrace/internal/handlers"
	oidcauth "github.com/wyldetechnologies/wyldetrace/internal/oidc"
	"github.com/wyldetechnologies/wyldetrace/internal/scheduler"
	"github.com/wyldetechnologies/wyldetrace/internal/secretbox"
	"github.com/wyldetechnologies/wyldetrace/internal/server"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	if err := db.RunMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer store.Close()

	if err := db.Seed(ctx, store, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("seed: %v", err)
	}

	tokens := auth.NewTokenService(cfg.JWTSecret, cfg.JWTTTL)

	hooks := webhook.New(store)
	sched := &scheduler.Scheduler{Store: store, Hooks: hooks, Interval: time.Minute}

	defaultRole, _ := auth.ParseRole(cfg.OIDCDefaultRole)
	if defaultRole == "" {
		defaultRole = auth.RoleReadOnly
	}
	oidcSvc, err := oidcauth.New(ctx, oidcauth.Config{
		Enabled:       cfg.OIDCEnabled,
		Issuer:        cfg.OIDCIssuer,
		ClientID:      cfg.OIDCClientID,
		ClientSecret:  cfg.OIDCClientSecret,
		RedirectURL:   cfg.OIDCRedirectURL,
		Scopes:        cfg.OIDCScopes,
		DefaultRole:   defaultRole,
		AutoProvision: cfg.OIDCAutoProvision,
		FrontendURL:   cfg.PublicAppURL,
		GroupsClaim:   cfg.OIDCGroupsClaim,
		RoleClaim:     cfg.OIDCRoleClaim,
		RoleMap:       oidcauth.ParseRoleMap(cfg.OIDCRoleMap),
		SyncRole:      cfg.OIDCSyncRole,
	})
	if err != nil {
		log.Fatalf("oidc: %v", err)
	}
	if oidcSvc.Enabled() {
		log.Printf("OIDC enabled (issuer=%s)", cfg.OIDCIssuer)
	}

	var secrets *secretbox.Box
	if cfg.SettingsEncryptionKey != "" {
		key, kerr := secretbox.ParseKey(cfg.SettingsEncryptionKey)
		if kerr != nil {
			log.Fatalf("SETTINGS_ENCRYPTION_KEY: %v", kerr)
		}
		secrets, err = secretbox.New(key)
		if err != nil {
			log.Fatalf("secretbox: %v", err)
		}
		if err := secretbox.MigratePlaintext(ctx, store.Pool, secrets); err != nil {
			log.Fatalf("secretbox migrate: %v", err)
		}
	}

	api := handlers.New(store, tokens, sched, hooks, oidcSvc, cfg.AdminPassword)
	api.Secrets = secrets
	api.AllowPlainSecrets = cfg.DevMode && secrets == nil
	handler := server.New(server.Options{
		API:          api,
		Tokens:       tokens,
		Store:        store,
		CORSOrigins:  cfg.CORSOrigins,
		MetricsToken: cfg.MetricsToken,
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	sched.Start(runCtx)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("WyldeTrace listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	cancelRun()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
