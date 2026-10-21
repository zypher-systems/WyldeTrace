package server

import (
	"crypto/subtle"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	wyldetrace "github.com/wyldetechnologies/wyldetrace"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	"github.com/wyldetechnologies/wyldetrace/internal/handlers"
	"github.com/wyldetechnologies/wyldetrace/internal/metrics"
	appmw "github.com/wyldetechnologies/wyldetrace/internal/middleware"
)

type Options struct {
	API          *handlers.API
	Tokens       *auth.TokenService
	Store        *db.Store
	CORSOrigins  []string
	MetricsToken string
}

func New(opts Options) http.Handler {
	origins := opts.CORSOrigins
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	// Heavy mutating endpoints: ~30 req/min per user or API token.
	heavyLimit := appmw.RateLimit(30)
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(metrics.Middleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", opts.API.Healthz)
	r.Get("/api/v1/healthz", opts.API.Healthz)
	metricsHandler := promhttp.Handler()
	r.Handle("/metrics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if opts.MetricsToken != "" {
			authz := r.Header.Get("Authorization")
			token := strings.TrimPrefix(authz, "Bearer ")
			if token == authz {
				token = r.Header.Get("X-Metrics-Token")
			}
			if subtleConstantTimeEq(token, opts.MetricsToken) {
				metricsHandler.ServeHTTP(w, r)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// No METRICS_TOKEN configured: allow scrape (lab / private network).
		metricsHandler.ServeHTTP(w, r)
	}))

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", opts.API.Login)
		r.Post("/auth/mfa/verify", opts.API.MFAVerify)
		r.Get("/auth/oidc/status", opts.API.OIDCStatus)
		r.Get("/auth/oidc/login", opts.API.OIDCLogin)
		r.Get("/auth/oidc/callback", opts.API.OIDCCallback)
		r.Post("/auth/oidc/exchange", opts.API.OIDCExchange)

		r.Group(func(r chi.Router) {
			r.Use(appmw.RequireAuth(opts.Tokens, opts.Store))

			r.Get("/auth/me", opts.API.Me)
			r.Post("/auth/password", opts.API.ChangePassword)
			r.Get("/auth/mfa", opts.API.MFAStatus)
			r.Post("/auth/mfa/enroll/begin", opts.API.MFAEnrollBegin)
			r.Post("/auth/mfa/enroll/confirm", opts.API.MFAEnrollConfirm)
			r.Post("/auth/mfa/disable", opts.API.MFADisable)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/auth/oidc/preview", opts.API.OIDCPreview)
			r.With(appmw.RequireReports).Get("/reports/summary", opts.API.ReportsSummary)
			r.With(appmw.RequireReports).Get("/reports/summary.csv", opts.API.ExportReportsSummaryCSV)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/reports/summary/dispatch", opts.API.DispatchReportsSummary)

			r.With(appmw.RequireRole(auth.RoleAdmin)).Route("/users", func(r chi.Router) {
				r.Get("/", opts.API.ListUsers)
				r.Post("/", opts.API.CreateUser)
				r.Put("/{id}", opts.API.UpdateUser)
				r.Delete("/{id}", opts.API.DeleteUser)
				r.Get("/{id}/locations", opts.API.GetUserLocations)
				r.Put("/{id}/locations", opts.API.SetUserLocations)
				r.Post("/{id}/mfa/disable", opts.API.AdminDisableMFA)
			})

			r.Route("/ip-requests", func(r chi.Router) {
				r.With(appmw.RequireReports).Get("/", opts.API.ListIPRequests)
				r.With(appmw.RequireReports).Post("/", opts.API.CreateIPRequest)
				r.With(appmw.RequireRole(auth.RoleEditor)).Get("/pending-count", opts.API.CountPendingIPRequests)
				r.With(appmw.RequireReports).Get("/{id}", opts.API.GetIPRequest)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/approve", opts.API.ApproveIPRequest)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/deny", opts.API.DenyIPRequest)
				r.With(appmw.RequireReports).Delete("/{id}", opts.API.DeleteIPRequest)
			})

			r.With(appmw.RequireRole(auth.RoleAdmin)).Get("/audit-logs", opts.API.ListAuditLogs)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Get("/audit-logs/export.csv", opts.API.ExportAuditLogsCSV)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/audit-logs/purge", opts.API.PurgeAuditLogs)

			r.Route("/api-tokens", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleAdmin)).Get("/", opts.API.ListAPITokens)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/", opts.API.CreateAPIToken)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Put("/{id}/locations", opts.API.SetAPITokenLocations)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/{id}/revoke", opts.API.RevokeAPIToken)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Delete("/{id}", opts.API.DeleteAPIToken)
			})

			r.Route("/discovery-jobs", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListDiscoveryJobs)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/templates", opts.API.ListDiscoveryJobTemplates)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/templates", opts.API.CreateDiscoveryJobTemplate)
				r.With(appmw.RequireRole(auth.RoleEditor)).Put("/templates/{id}", opts.API.UpdateDiscoveryJobTemplate)
				r.With(appmw.RequireRole(auth.RoleEditor)).Delete("/templates/{id}", opts.API.DeleteDiscoveryJobTemplate)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/from-template", opts.API.CreateDiscoveryJobFromTemplate)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/", opts.API.CreateDiscoveryJob)
				r.With(appmw.RequireRole(auth.RoleEditor)).Put("/{id}", opts.API.UpdateDiscoveryJob)
				r.With(appmw.RequireRole(auth.RoleEditor)).Delete("/{id}", opts.API.DeleteDiscoveryJob)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/run", opts.API.RunDiscoveryJob)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}/runs", opts.API.ListDiscoveryJobRuns)
			})

			r.Route("/locations", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListLocations)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/", opts.API.CreateLocation)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}", opts.API.GetLocation)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Put("/{id}", opts.API.UpdateLocation)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Delete("/{id}", opts.API.DeleteLocation)
			})

			r.Route("/vrfs", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListVRFs)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/", opts.API.CreateVRF)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}", opts.API.GetVRF)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Put("/{id}", opts.API.UpdateVRF)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Delete("/{id}", opts.API.DeleteVRF)
			})

			r.Route("/vlans", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListVLANs)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/", opts.API.CreateVLAN)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}", opts.API.GetVLAN)
				r.With(appmw.RequireRole(auth.RoleEditor)).Put("/{id}", opts.API.UpdateVLAN)
				r.With(appmw.RequireRole(auth.RoleEditor)).Delete("/{id}", opts.API.DeleteVLAN)
			})

			r.Route("/device-types", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListDeviceTypes)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/", opts.API.CreateDeviceType)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Put("/{id}", opts.API.UpdateDeviceType)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Delete("/{id}", opts.API.DeleteDeviceType)
			})

			r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/search", opts.API.Search)

			r.Route("/export", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/subnets.csv", opts.API.ExportSubnetsCSV)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/ips.csv", opts.API.ExportIPsCSV)
			})
			r.Route("/import", func(r chi.Router) {
				r.With(heavyLimit, appmw.RequireRole(auth.RoleEditor)).Post("/subnets.csv", opts.API.ImportSubnetsCSV)
				r.With(heavyLimit, appmw.RequireRole(auth.RoleEditor)).Post("/ips.csv", opts.API.ImportIPsCSV)
				r.With(heavyLimit, appmw.RequireRole(auth.RoleEditor)).Post("/dhcp-leases.csv", opts.API.ImportDHCPLeasesCSV)
				r.With(heavyLimit, appmw.RequireRole(auth.RoleEditor)).Post("/dns-hosts", opts.API.ImportDNSHosts)
			})

			r.Route("/inbox", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListInbox)
				r.With(heavyLimit, appmw.RequireRole(auth.RoleEditor)).Post("/bulk", opts.API.BulkInbox)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/dismiss", opts.API.DismissFinding)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/claim", opts.API.ClaimFinding)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/mark-available", opts.API.MarkFindingAvailable)
			})

			r.Route("/custom-fields", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListCustomFieldDefs)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/", opts.API.CreateCustomFieldDef)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Put("/{id}", opts.API.UpdateCustomFieldDef)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Delete("/{id}", opts.API.DeleteCustomFieldDef)
			})

			r.Route("/subnets", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListSubnets)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/tags", opts.API.ListSubnetTags)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/tree", opts.API.SubnetTree)
				r.With(appmw.RequireRole(auth.RoleEditor)).Get("/deleted", opts.API.ListDeletedSubnets)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/purge-deleted", opts.API.PurgeDeletedSubnets)
				r.With(heavyLimit, appmw.RequireRole(auth.RoleEditor)).Post("/bulk", opts.API.BulkUpdateSubnets)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/", opts.API.CreateSubnet)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}", opts.API.GetSubnet)
				r.With(appmw.RequireRole(auth.RoleEditor)).Put("/{id}", opts.API.UpdateSubnet)
				r.With(appmw.RequireRole(auth.RoleEditor)).Delete("/{id}", opts.API.DeleteSubnet)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/restore", opts.API.RestoreDeletedSubnet)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}/utilization", opts.API.GetSubnetUtilization)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}/ip-map", opts.API.GetSubnetIPMap)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}/settings", opts.API.GetSubnetSettings)
				r.With(appmw.RequireRole(auth.RoleEditor)).Put("/{id}/settings", opts.API.UpdateSubnetSettings)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}/children", opts.API.ListSubnetChildren)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}/ips", opts.API.ListIPsBySubnet)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/ips", opts.API.CreateIP)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/ips/next", opts.API.AllocateNextIP)
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/{id}/ranges", opts.API.ListIPRanges)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/ranges", opts.API.CreateIPRange)
				r.With(appmw.RequireRole(auth.RoleEditor)).Put("/{id}/ranges/{rangeID}", opts.API.UpdateIPRange)
				r.With(appmw.RequireRole(auth.RoleEditor)).Delete("/{id}/ranges/{rangeID}", opts.API.DeleteIPRange)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/allocate-child", opts.API.AllocateChild)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/split", opts.API.SplitSubnet)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/merge", opts.API.MergeSubnets)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/resize", opts.API.ResizeSubnet)
			})

			r.Route("/ips", func(r chi.Router) {
				r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/", opts.API.ListIPs)
				r.With(appmw.RequireRole(auth.RoleEditor)).Get("/deleted", opts.API.ListDeletedIPs)
				r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/purge-deleted", opts.API.PurgeDeletedIPs)
				r.With(heavyLimit, appmw.RequireRole(auth.RoleEditor)).Post("/bulk", opts.API.BulkUpdateIPs)
				r.With(appmw.RequireRole(auth.RoleEditor)).Put("/{id}", opts.API.UpdateIP)
				r.With(appmw.RequireRole(auth.RoleEditor)).Delete("/{id}", opts.API.DeleteIP)
				r.With(appmw.RequireRole(auth.RoleEditor)).Post("/{id}/restore", opts.API.RestoreDeletedIP)
			})

			r.With(appmw.RequireRole(auth.RoleAdmin)).Route("/webhooks", func(r chi.Router) {
				r.Get("/", opts.API.ListWebhooks)
				r.Post("/", opts.API.CreateWebhook)
				r.Post("/preview", opts.API.PreviewWebhook)
				r.Get("/deliveries", opts.API.ListWebhookDeliveries)
				r.Post("/deliveries/{deliveryID}/retry", opts.API.RetryWebhookDelivery)
				r.Post("/deliveries/purge", opts.API.PurgeWebhookDeliveries)
				r.Put("/{id}", opts.API.UpdateWebhook)
				r.Delete("/{id}", opts.API.DeleteWebhook)
				r.Post("/{id}/preview", opts.API.PreviewWebhook)
				r.Post("/{id}/test", opts.API.TestWebhook)
				r.Get("/{id}/deliveries", opts.API.ListWebhookDeliveries)
			})

			r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/ip-history", opts.API.ListIPHistory)
			r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/enrich/suggest", opts.API.SuggestDevice)

			r.With(appmw.RequireRole(auth.RoleAdmin)).Get("/backup", opts.API.ExportBackup)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/backup/restore/preview", opts.API.PreviewRestoreBackup)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Post("/backup/restore", opts.API.RestoreBackup)

			r.With(appmw.RequireRole(auth.RoleAdmin)).Get("/settings", opts.API.GetSettings)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Put("/settings", opts.API.UpdateSettings)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Get("/instance", opts.API.InstanceInfo)

			r.With(appmw.RequireRole(auth.RoleReadOnly)).Get("/notifications", opts.API.ListNotifications)
			r.With(appmw.RequireRole(auth.RoleReadOnly)).Post("/notifications/read-all", opts.API.MarkAllNotificationsRead)
			r.With(appmw.RequireRole(auth.RoleReadOnly)).Post("/notifications/{id}/read", opts.API.MarkNotificationRead)

			r.With(heavyLimit, appmw.RequireRole(auth.RoleEditor)).Post("/scanner/sweep", opts.API.Sweep)
			r.With(appmw.RequireRole(auth.RoleEditor)).Post("/scanner/claim", opts.API.ClaimDiscovered)
			r.With(appmw.RequireRole(auth.RoleEditor)).Post("/scanner/discover-subnets", opts.API.DiscoverSubnets)
			r.With(appmw.RequireRole(auth.RoleEditor)).Post("/scanner/discover-subnets/commit", opts.API.CommitDiscoveredSubnets)
			r.With(appmw.RequireRole(auth.RoleEditor)).Post("/scanner/snmp-enrich", opts.API.SNMPEnrich)

			r.With(appmw.RequireRole(auth.RoleEditor)).Post("/integrations/kea/sync", opts.API.SyncKea)
			r.With(appmw.RequireRole(auth.RoleAdmin)).Get("/integrations/deliveries", opts.API.ListIntegrationDeliveries)
		})
	})

	spa, err := fs.Sub(wyldetrace.Dist, "web/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(spa))
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/")
		if path != "" {
			if f, err := spa.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, req)
				return
			}
		}
		req.URL.Path = "/"
		fileServer.ServeHTTP(w, req)
	})

	return r
}

func subtleConstantTimeEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
