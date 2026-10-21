package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

const (
	settingIPDeletedRetentionDays         = "ip_deleted_retention_days"
	settingSubnetDeletedRetentionDays     = "subnet_deleted_retention_days"
	settingAuditRetentionDays             = "audit_retention_days"
	settingWebhookDeliveryRetentionDays   = "webhook_delivery_retention_days"
	settingReportsSummaryIntervalHours    = "reports_summary_interval_hours"
	settingSubnetUtilizationAlertPercent  = "subnet_utilization_alert_percent"
	settingWebhookMuteHours               = "webhook_mute_hours"
	settingSMTPEnabled                    = "smtp_enabled"
	settingSMTPHost                       = "smtp_host"
	settingSMTPPort                       = "smtp_port"
	settingSMTPUser                       = "smtp_user"
	settingSMTPPass                       = "smtp_pass"
	settingSMTPFrom                       = "smtp_from"
	settingSMTPTo                         = "smtp_to"
	settingSMTPSubjectTemplate            = "smtp_subject_template"
	settingSMTPBodyTemplate               = "smtp_body_template"
	defaultRetentionDays                  = 90
	defaultAuditRetentionDays             = 365
	defaultWebhookDeliveryRetentionDays   = 30
	defaultReportsSummaryIntervalHours    = 24
	defaultSubnetUtilizationAlertPercent  = 90
	defaultWebhookMuteHours               = 6
)

type settingsDTO struct {
	IPDeletedRetentionDays         int        `json:"ip_deleted_retention_days"`
	SubnetDeletedRetentionDays     int        `json:"subnet_deleted_retention_days"`
	AuditRetentionDays             int        `json:"audit_retention_days"`
	WebhookDeliveryRetentionDays   int        `json:"webhook_delivery_retention_days"`
	ReportsSummaryIntervalHours    int        `json:"reports_summary_interval_hours"`
	SubnetUtilizationAlertPercent  int        `json:"subnet_utilization_alert_percent"`
	WebhookMuteHours               int        `json:"webhook_mute_hours"`
	PowerDNSEnabled                bool       `json:"powerdns_enabled"`
	PowerDNSURL                    string     `json:"powerdns_url"`
	PowerDNSAPIKeySet              bool       `json:"powerdns_api_key_set"`
	PowerDNSDryRun                 bool       `json:"powerdns_dry_run"`
	PowerDNSServer                 string     `json:"powerdns_server"`
	KeaEnabled                     bool       `json:"kea_enabled"`
	KeaURL                         string     `json:"kea_url"`
	KeaDryRun                      bool       `json:"kea_dry_run"`
	SNMPEnabled                    bool       `json:"snmp_enabled"`
	SNMPCommunitySet               bool       `json:"snmp_community_set"`
	SNMPVersion                    string     `json:"snmp_version"`
	SNMPTimeoutSeconds             int        `json:"snmp_timeout_seconds"`
	SMTPEnabled                    bool       `json:"smtp_enabled"`
	SMTPHost                       string     `json:"smtp_host"`
	SMTPPort                       int        `json:"smtp_port"`
	SMTPUser                       string     `json:"smtp_user"`
	SMTPPassSet                    bool       `json:"smtp_pass_set"`
	SMTPFrom                       string     `json:"smtp_from"`
	SMTPTo                         string     `json:"smtp_to"`
	SMTPSubjectTemplate            string     `json:"smtp_subject_template"`
	SMTPBodyTemplate               string     `json:"smtp_body_template"`
	UpdatedAt                      *time.Time `json:"updated_at,omitempty"`
}

type settingsRequest struct {
	IPDeletedRetentionDays         *int    `json:"ip_deleted_retention_days"`
	SubnetDeletedRetentionDays     *int    `json:"subnet_deleted_retention_days"`
	AuditRetentionDays             *int    `json:"audit_retention_days"`
	WebhookDeliveryRetentionDays   *int    `json:"webhook_delivery_retention_days"`
	ReportsSummaryIntervalHours    *int    `json:"reports_summary_interval_hours"`
	SubnetUtilizationAlertPercent  *int    `json:"subnet_utilization_alert_percent"`
	WebhookMuteHours               *int    `json:"webhook_mute_hours"`
	PowerDNSEnabled                *bool   `json:"powerdns_enabled"`
	PowerDNSURL                    *string `json:"powerdns_url"`
	PowerDNSAPIKey                 *string `json:"powerdns_api_key"`
	PowerDNSDryRun                 *bool   `json:"powerdns_dry_run"`
	PowerDNSServer                 *string `json:"powerdns_server"`
	KeaEnabled                     *bool   `json:"kea_enabled"`
	KeaURL                         *string `json:"kea_url"`
	KeaDryRun                      *bool   `json:"kea_dry_run"`
	SNMPEnabled                    *bool   `json:"snmp_enabled"`
	SNMPCommunity                  *string `json:"snmp_community"`
	SNMPVersion                    *string `json:"snmp_version"`
	SNMPTimeoutSeconds             *int    `json:"snmp_timeout_seconds"`
	SMTPEnabled                    *bool   `json:"smtp_enabled"`
	SMTPHost                       *string `json:"smtp_host"`
	SMTPPort                       *int    `json:"smtp_port"`
	SMTPUser                       *string `json:"smtp_user"`
	SMTPPass                       *string `json:"smtp_pass"`
	SMTPFrom                       *string `json:"smtp_from"`
	SMTPTo                         *string `json:"smtp_to"`
	SMTPSubjectTemplate            *string `json:"smtp_subject_template"`
	SMTPBodyTemplate               *string `json:"smtp_body_template"`
}

func (a *API) GetSettings(w http.ResponseWriter, r *http.Request) {
	ipDays, ipUpdated := a.retentionDays(r.Context())
	subnetDays, subnetUpdated := a.subnetRetentionDays(r.Context())
	auditDays, auditUpdated := a.auditRetentionDays(r.Context())
	whDays, whUpdated := a.webhookDeliveryRetentionDays(r.Context())
	repHours, repUpdated := a.reportsSummaryIntervalHours(r.Context())
	utilPct, utilUpdated := a.subnetUtilizationAlertPercent(r.Context())
	muteHours, muteUpdated := a.webhookMuteHours(r.Context())
	var updated *time.Time
	for _, t := range []*time.Time{ipUpdated, subnetUpdated, auditUpdated, whUpdated, repUpdated, utilUpdated, muteUpdated} {
		if t != nil && (updated == nil || t.After(*updated)) {
			updated = t
		}
	}
	snmpTimeout, _ := strconv.Atoi(strings.TrimSpace(a.settingString(r.Context(), settingSNMPTimeout, "5")))
	if snmpTimeout <= 0 {
		snmpTimeout = 5
	}
	httpx.JSON(w, http.StatusOK, a.settingsResponse(r.Context(), ipDays, subnetDays, auditDays, whDays, repHours, utilPct, muteHours, snmpTimeout, updated))
}

func (a *API) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	hasIntegration := req.PowerDNSEnabled != nil || req.PowerDNSURL != nil || req.PowerDNSAPIKey != nil ||
		req.PowerDNSDryRun != nil || req.PowerDNSServer != nil ||
		req.KeaEnabled != nil || req.KeaURL != nil || req.KeaDryRun != nil ||
		req.SNMPEnabled != nil || req.SNMPCommunity != nil || req.SNMPVersion != nil || req.SNMPTimeoutSeconds != nil
	hasSMTP := req.SMTPEnabled != nil || req.SMTPHost != nil || req.SMTPPort != nil || req.SMTPUser != nil ||
		req.SMTPPass != nil || req.SMTPFrom != nil || req.SMTPTo != nil ||
		req.SMTPSubjectTemplate != nil || req.SMTPBodyTemplate != nil
	if req.IPDeletedRetentionDays == nil && req.SubnetDeletedRetentionDays == nil &&
		req.AuditRetentionDays == nil && req.WebhookDeliveryRetentionDays == nil &&
		req.ReportsSummaryIntervalHours == nil && req.SubnetUtilizationAlertPercent == nil &&
		req.WebhookMuteHours == nil && !hasIntegration && !hasSMTP {
		httpx.Error(w, http.StatusBadRequest, "at least one setting required")
		return
	}
	ipDays, _ := a.retentionDays(r.Context())
	subnetDays, _ := a.subnetRetentionDays(r.Context())
	auditDays, _ := a.auditRetentionDays(r.Context())
	whDays, _ := a.webhookDeliveryRetentionDays(r.Context())
	repHours, _ := a.reportsSummaryIntervalHours(r.Context())
	utilPct, _ := a.subnetUtilizationAlertPercent(r.Context())
	muteHours, _ := a.webhookMuteHours(r.Context())
	var lastUpdated *time.Time

	save := func(key, field string, value int, max int) bool {
		if value < 0 || value > max {
			httpx.Error(w, http.StatusBadRequest, field+" must be 0–"+strconv.Itoa(max)+" (0 disables)")
			return false
		}
		row, err := a.Store.Queries.UpsertSetting(r.Context(), sqlcdb.UpsertSettingParams{
			Key: key, Value: strconv.Itoa(value),
		})
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return false
		}
		t := row.UpdatedAt.Time
		lastUpdated = &t
		a.audit(r.Context(), "settings.update", "app_settings", key, map[string]any{field: value})
		return true
	}
	saveStr := func(key, field, value string) bool {
		row, err := a.Store.Queries.UpsertSetting(r.Context(), sqlcdb.UpsertSettingParams{
			Key: key, Value: value,
		})
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return false
		}
		t := row.UpdatedAt.Time
		lastUpdated = &t
		a.audit(r.Context(), "settings.update", "app_settings", key, map[string]any{field: value})
		return true
	}
	saveBool := func(key, field string, value bool) bool {
		v := "false"
		if value {
			v = "true"
		}
		return saveStr(key, field, v)
	}

	if req.IPDeletedRetentionDays != nil {
		ipDays = *req.IPDeletedRetentionDays
		if !save(settingIPDeletedRetentionDays, "ip_deleted_retention_days", ipDays, 3650) {
			return
		}
	}
	if req.SubnetDeletedRetentionDays != nil {
		subnetDays = *req.SubnetDeletedRetentionDays
		if !save(settingSubnetDeletedRetentionDays, "subnet_deleted_retention_days", subnetDays, 3650) {
			return
		}
	}
	if req.AuditRetentionDays != nil {
		auditDays = *req.AuditRetentionDays
		if !save(settingAuditRetentionDays, "audit_retention_days", auditDays, 3650) {
			return
		}
	}
	if req.WebhookDeliveryRetentionDays != nil {
		whDays = *req.WebhookDeliveryRetentionDays
		if !save(settingWebhookDeliveryRetentionDays, "webhook_delivery_retention_days", whDays, 3650) {
			return
		}
	}
	if req.ReportsSummaryIntervalHours != nil {
		repHours = *req.ReportsSummaryIntervalHours
		if !save(settingReportsSummaryIntervalHours, "reports_summary_interval_hours", repHours, 24*90) {
			return
		}
	}
	if req.SubnetUtilizationAlertPercent != nil {
		utilPct = *req.SubnetUtilizationAlertPercent
		if utilPct < 0 || utilPct > 100 {
			httpx.Error(w, http.StatusBadRequest, "subnet_utilization_alert_percent must be 0–100 (0 disables)")
			return
		}
		if !save(settingSubnetUtilizationAlertPercent, "subnet_utilization_alert_percent", utilPct, 100) {
			return
		}
	}
	if req.WebhookMuteHours != nil {
		muteHours = *req.WebhookMuteHours
		if !save(settingWebhookMuteHours, "webhook_mute_hours", muteHours, 24*30) {
			return
		}
	}
	if req.PowerDNSEnabled != nil && !saveBool(settingPowerDNSEnabled, "powerdns_enabled", *req.PowerDNSEnabled) {
		return
	}
	if req.PowerDNSURL != nil {
		url := strings.TrimSpace(*req.PowerDNSURL)
		if url != "" {
			if err := a.validateIntegrationURL(url); err != nil {
				httpx.Error(w, http.StatusBadRequest, "powerdns_url: "+err.Error())
				return
			}
		}
		if !saveStr(settingPowerDNSURL, "powerdns_url", url) {
			return
		}
	}
	if req.PowerDNSAPIKey != nil {
		sealed, err := a.sealSecret(*req.PowerDNSAPIKey)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		if !saveStr(settingPowerDNSAPIKey, "powerdns_api_key", sealed) {
			return
		}
	}
	if req.PowerDNSDryRun != nil && !saveBool(settingPowerDNSDryRun, "powerdns_dry_run", *req.PowerDNSDryRun) {
		return
	}
	if req.PowerDNSServer != nil && !saveStr(settingPowerDNSServer, "powerdns_server", strings.TrimSpace(*req.PowerDNSServer)) {
		return
	}
	if req.KeaEnabled != nil && !saveBool(settingKeaEnabled, "kea_enabled", *req.KeaEnabled) {
		return
	}
	if req.KeaURL != nil {
		url := strings.TrimSpace(*req.KeaURL)
		if url != "" {
			if err := a.validateIntegrationURL(url); err != nil {
				httpx.Error(w, http.StatusBadRequest, "kea_url: "+err.Error())
				return
			}
		}
		if !saveStr(settingKeaURL, "kea_url", url) {
			return
		}
	}
	if req.KeaDryRun != nil && !saveBool(settingKeaDryRun, "kea_dry_run", *req.KeaDryRun) {
		return
	}
	if req.SNMPEnabled != nil && !saveBool(settingSNMPEnabled, "snmp_enabled", *req.SNMPEnabled) {
		return
	}
	if req.SNMPCommunity != nil {
		sealed, err := a.sealSecret(*req.SNMPCommunity)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		if !saveStr(settingSNMPCommunity, "snmp_community", sealed) {
			return
		}
	}
	if req.SNMPVersion != nil {
		v := strings.TrimSpace(*req.SNMPVersion)
		if v != "2c" && v != "3" {
			httpx.Error(w, http.StatusBadRequest, "snmp_version must be 2c or 3")
			return
		}
		if !saveStr(settingSNMPVersion, "snmp_version", v) {
			return
		}
	}
	if req.SNMPTimeoutSeconds != nil && !save(settingSNMPTimeout, "snmp_timeout_seconds", *req.SNMPTimeoutSeconds, 120) {
		return
	}
	if req.SMTPEnabled != nil && !saveBool(settingSMTPEnabled, "smtp_enabled", *req.SMTPEnabled) {
		return
	}
	if req.SMTPHost != nil && !saveStr(settingSMTPHost, "smtp_host", strings.TrimSpace(*req.SMTPHost)) {
		return
	}
	if req.SMTPPort != nil {
		if *req.SMTPPort < 1 || *req.SMTPPort > 65535 {
			httpx.Error(w, http.StatusBadRequest, "smtp_port must be 1–65535")
			return
		}
		if !save(settingSMTPPort, "smtp_port", *req.SMTPPort, 65535) {
			return
		}
	}
	if req.SMTPUser != nil && !saveStr(settingSMTPUser, "smtp_user", strings.TrimSpace(*req.SMTPUser)) {
		return
	}
	if req.SMTPPass != nil {
		sealed, err := a.sealSecret(*req.SMTPPass)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		if !saveStr(settingSMTPPass, "smtp_pass", sealed) {
			return
		}
	}
	if req.SMTPFrom != nil && !saveStr(settingSMTPFrom, "smtp_from", strings.TrimSpace(*req.SMTPFrom)) {
		return
	}
	if req.SMTPTo != nil && !saveStr(settingSMTPTo, "smtp_to", strings.TrimSpace(*req.SMTPTo)) {
		return
	}
	if req.SMTPSubjectTemplate != nil && !saveStr(settingSMTPSubjectTemplate, "smtp_subject_template", *req.SMTPSubjectTemplate) {
		return
	}
	if req.SMTPBodyTemplate != nil && !saveStr(settingSMTPBodyTemplate, "smtp_body_template", *req.SMTPBodyTemplate) {
		return
	}

	snmpTimeout, _ := strconv.Atoi(strings.TrimSpace(a.settingString(r.Context(), settingSNMPTimeout, "5")))
	if snmpTimeout <= 0 {
		snmpTimeout = 5
	}
	httpx.JSON(w, http.StatusOK, a.settingsResponse(r.Context(), ipDays, subnetDays, auditDays, whDays, repHours, utilPct, muteHours, snmpTimeout, lastUpdated))
}

func (a *API) settingsResponse(
	ctx context.Context,
	ipDays, subnetDays, auditDays, whDays, repHours, utilPct, muteHours, snmpTimeout int,
	updated *time.Time,
) settingsDTO {
	smtpPort, _ := strconv.Atoi(strings.TrimSpace(a.settingString(ctx, settingSMTPPort, "587")))
	if smtpPort <= 0 {
		smtpPort = 587
	}
	return settingsDTO{
		IPDeletedRetentionDays:        ipDays,
		SubnetDeletedRetentionDays:    subnetDays,
		AuditRetentionDays:            auditDays,
		WebhookDeliveryRetentionDays:  whDays,
		ReportsSummaryIntervalHours:   repHours,
		SubnetUtilizationAlertPercent: utilPct,
		WebhookMuteHours:              muteHours,
		PowerDNSEnabled:               a.settingBool(ctx, settingPowerDNSEnabled, false),
		PowerDNSURL:                   a.settingString(ctx, settingPowerDNSURL, ""),
		PowerDNSAPIKeySet:             strings.TrimSpace(a.settingString(ctx, settingPowerDNSAPIKey, "")) != "",
		PowerDNSDryRun:                a.settingBool(ctx, settingPowerDNSDryRun, true),
		PowerDNSServer:                a.settingString(ctx, settingPowerDNSServer, "localhost"),
		KeaEnabled:                    a.settingBool(ctx, settingKeaEnabled, false),
		KeaURL:                        a.settingString(ctx, settingKeaURL, ""),
		KeaDryRun:                     a.settingBool(ctx, settingKeaDryRun, true),
		SNMPEnabled:                   a.settingBool(ctx, settingSNMPEnabled, false),
		SNMPCommunitySet:              strings.TrimSpace(a.settingString(ctx, settingSNMPCommunity, "")) != "",
		SNMPVersion:                   a.settingString(ctx, settingSNMPVersion, "2c"),
		SNMPTimeoutSeconds:            snmpTimeout,
		SMTPEnabled:                   a.settingBool(ctx, settingSMTPEnabled, false),
		SMTPHost:                      a.settingString(ctx, settingSMTPHost, ""),
		SMTPPort:                      smtpPort,
		SMTPUser:                      a.settingString(ctx, settingSMTPUser, ""),
		SMTPPassSet:                   strings.TrimSpace(a.settingString(ctx, settingSMTPPass, "")) != "",
		SMTPFrom:                      a.settingString(ctx, settingSMTPFrom, ""),
		SMTPTo:                        a.settingString(ctx, settingSMTPTo, ""),
		SMTPSubjectTemplate:           a.settingString(ctx, settingSMTPSubjectTemplate, "[WyldeTrace] {{.Title}}"),
		SMTPBodyTemplate:              a.settingString(ctx, settingSMTPBodyTemplate, "{{.Title}}\n\n{{.Body}}\n\nEvent: {{.Event}}\nEntity: {{.EntityKey}}\n"),
		UpdatedAt:                     updated,
	}
}

type purgeResponse struct {
	Purged        int64 `json:"purged"`
	RetentionDays int   `json:"retention_days"`
}

func (a *API) PurgeDeletedIPs(w http.ResponseWriter, r *http.Request) {
	a.purgeWithConfirm(w, r, a.retentionDays, "ip_deleted_retention_days", "ip.purge", "ip_address",
		func(before time.Time) (int64, error) {
			return a.Store.Queries.PurgeDeletedIPsBefore(r.Context(), pgtype.Timestamptz{Time: before, Valid: true})
		})
}

func (a *API) PurgeDeletedSubnets(w http.ResponseWriter, r *http.Request) {
	a.purgeWithConfirm(w, r, a.subnetRetentionDays, "subnet_deleted_retention_days", "subnet.purge", "subnet",
		func(before time.Time) (int64, error) {
			return a.Store.Queries.PurgeDeletedSubnetsBefore(r.Context(), pgtype.Timestamptz{Time: before, Valid: true})
		})
}

func (a *API) PurgeAuditLogs(w http.ResponseWriter, r *http.Request) {
	a.purgeWithConfirm(w, r, a.auditRetentionDays, "audit_retention_days", "audit.purge", "audit_log",
		func(before time.Time) (int64, error) {
			return a.Store.Queries.PurgeAuditLogsBefore(r.Context(), pgtype.Timestamptz{Time: before, Valid: true})
		})
}

func (a *API) PurgeWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	a.purgeWithConfirm(w, r, a.webhookDeliveryRetentionDays, "webhook_delivery_retention_days", "webhook.purge", "webhook_delivery",
		func(before time.Time) (int64, error) {
			return a.Store.Queries.PurgeWebhookDeliveriesBefore(r.Context(), pgtype.Timestamptz{Time: before, Valid: true})
		})
}

func (a *API) purgeWithConfirm(
	w http.ResponseWriter,
	r *http.Request,
	daysFn func(context.Context) (int, *time.Time),
	settingKey, auditAction, entityType string,
	purgeFn func(time.Time) (int64, error),
) {
	days, _ := daysFn(r.Context())
	if days <= 0 {
		httpx.Error(w, http.StatusBadRequest, "purge disabled (set "+settingKey+" > 0)")
		return
	}
	if !(strings.EqualFold(r.URL.Query().Get("confirm"), "true") || r.Header.Get("X-Confirm-Purge") == "true") {
		httpx.Error(w, http.StatusBadRequest, "confirm required (query confirm=true or X-Confirm-Purge: true)")
		return
	}
	before := time.Now().UTC().AddDate(0, 0, -days)
	n, err := purgeFn(before)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), auditAction, entityType, "", map[string]any{
		"purged": n, "retention_days": days, "before": before,
	})
	httpx.JSON(w, http.StatusOK, purgeResponse{Purged: n, RetentionDays: days})
}

func (a *API) retentionDays(ctx context.Context) (int, *time.Time) {
	return a.settingDays(ctx, settingIPDeletedRetentionDays, defaultRetentionDays)
}

func (a *API) subnetRetentionDays(ctx context.Context) (int, *time.Time) {
	return a.settingDays(ctx, settingSubnetDeletedRetentionDays, defaultRetentionDays)
}

func (a *API) auditRetentionDays(ctx context.Context) (int, *time.Time) {
	return a.settingDays(ctx, settingAuditRetentionDays, defaultAuditRetentionDays)
}

func (a *API) webhookDeliveryRetentionDays(ctx context.Context) (int, *time.Time) {
	return a.settingDays(ctx, settingWebhookDeliveryRetentionDays, defaultWebhookDeliveryRetentionDays)
}

func (a *API) reportsSummaryIntervalHours(ctx context.Context) (int, *time.Time) {
	return a.settingDays(ctx, settingReportsSummaryIntervalHours, defaultReportsSummaryIntervalHours)
}

func (a *API) subnetUtilizationAlertPercent(ctx context.Context) (int, *time.Time) {
	return a.settingDays(ctx, settingSubnetUtilizationAlertPercent, defaultSubnetUtilizationAlertPercent)
}

func (a *API) webhookMuteHours(ctx context.Context) (int, *time.Time) {
	return a.settingDays(ctx, settingWebhookMuteHours, defaultWebhookMuteHours)
}

func (a *API) settingDays(ctx context.Context, key string, fallback int) (int, *time.Time) {
	row, err := a.Store.Queries.GetSetting(ctx, key)
	if err != nil {
		return fallback, nil
	}
	days, err := strconv.Atoi(strings.TrimSpace(row.Value))
	if err != nil || days < 0 {
		return fallback, nil
	}
	t := row.UpdatedAt.Time
	return days, &t
}
