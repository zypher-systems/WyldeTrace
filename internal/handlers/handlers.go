package handlers

import (
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	"github.com/wyldetechnologies/wyldetrace/internal/mail"
	oidcauth "github.com/wyldetechnologies/wyldetrace/internal/oidc"
	"github.com/wyldetechnologies/wyldetrace/internal/scheduler"
	"github.com/wyldetechnologies/wyldetrace/internal/secretbox"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

type API struct {
	Store             *db.Store
	Tokens            *auth.TokenService
	Scheduler         *scheduler.Scheduler
	Hooks             *webhook.Notifier
	Mail              *mail.Sender
	OIDC              *oidcauth.Service
	Secrets           *secretbox.Box
	AllowPlainSecrets bool // lab / WYLDTRACE_DEV only
	BootstrapPassword string
}

func New(store *db.Store, tokens *auth.TokenService, sched *scheduler.Scheduler, hooks *webhook.Notifier, oidcSvc *oidcauth.Service, bootstrapPassword string) *API {
	mailer := mail.New(store)
	a := &API{
		Store: store, Tokens: tokens, Scheduler: sched, Hooks: hooks, Mail: mailer, OIDC: oidcSvc,
		BootstrapPassword: bootstrapPassword,
	}
	mailer.OpenFn = a.openSecret
	if sched != nil {
		sched.BuildReportsSummary = a.unrestrictedReportsPayload
		sched.Mail = mailer
	}
	return a
}

func (a *API) sealSecret(plain string) (string, error) {
	return secretbox.SealRequired(a.Secrets, plain, a.AllowPlainSecrets)
}

func (a *API) openSecret(value string) (string, error) {
	return secretbox.OpenOptional(a.Secrets, value)
}
