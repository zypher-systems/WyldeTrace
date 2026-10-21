package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
	"github.com/wyldetechnologies/wyldetrace/internal/mail"
	"github.com/wyldetechnologies/wyldetrace/internal/metrics"
	"github.com/wyldetechnologies/wyldetrace/internal/scanner"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

const (
	settingIPDeletedRetentionDays        = "ip_deleted_retention_days"
	settingAuditRetentionDays            = "audit_retention_days"
	settingWebhookDeliveryRetentionDays  = "webhook_delivery_retention_days"
	settingSubnetDeletedRetentionDays    = "subnet_deleted_retention_days"
	settingReportsSummaryIntervalHours   = "reports_summary_interval_hours"
	settingReportsSummaryLastSentAt      = "reports_summary_last_sent_at"
	settingSubnetUtilizationAlertPercent = "subnet_utilization_alert_percent"
	settingWebhookMuteHours              = "webhook_mute_hours"
	defaultUtilHighThreshold             = 90.0
	defaultWebhookMuteHours              = 6
	// Session-scoped advisory lock key for scheduler ticks (must lock/unlock on same conn).
	schedulerTickLockKey int64 = 87231405
	// Namespace for per-job locks (pg_try_advisory_lock(classid, objid)).
	schedulerJobLockClass int32 = 87231406
)

// ErrBusy is returned when an advisory lock is held by another replica/run.
var ErrBusy = errors.New("scheduler busy")

type Scheduler struct {
	Store               *db.Store
	Hooks               *webhook.Notifier
	Mail                *mail.Sender
	Interval            time.Duration
	BuildReportsSummary func(ctx context.Context) (map[string]any, error)
}

func (s *Scheduler) Start(ctx context.Context) {
	if s.Interval <= 0 {
		s.Interval = time.Minute
	}
	t := time.NewTicker(s.Interval)
	go func() {
		defer t.Stop()
		s.tick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.tick(ctx)
			}
		}
	}()
}

// withSessionLock acquires a dedicated pool connection, tries pg_try_advisory_lock
// on that connection only, runs fn, then unlocks on the same connection.
func (s *Scheduler) withSessionLock(ctx context.Context, lockSQL string, unlockSQL string, args []any, fn func(context.Context) error) error {
	conn, err := s.Store.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	var got bool
	if err := conn.QueryRow(ctx, lockSQL, args...).Scan(&got); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	if !got {
		return ErrBusy
	}
	defer unlockSession(conn, unlockSQL, args)

	return fn(ctx)
}

func unlockSession(conn *pgxpool.Conn, unlockSQL string, args []any) {
	// Use Background so unlock still runs if the request ctx was cancelled.
	_, _ = conn.Exec(context.Background(), unlockSQL, args...)
}

func (s *Scheduler) tick(ctx context.Context) {
	err := s.withSessionLock(ctx,
		`SELECT pg_try_advisory_lock($1)`,
		`SELECT pg_advisory_unlock($1)`,
		[]any{schedulerTickLockKey},
		func(ctx context.Context) error {
			metrics.SchedulerTicks.Inc()

			jobs, err := s.Store.Queries.ListDueDiscoveryJobs(ctx)
			if err != nil {
				return fmt.Errorf("list due jobs: %w", err)
			}
			for _, job := range jobs {
				if err := s.runJob(ctx, job); err != nil && !errors.Is(err, ErrBusy) {
					log.Printf("scheduler: job %s: %v", job.ID, err)
				}
			}
			s.purgeExpiredDeletedIPs(ctx)
			s.purgeExpiredDeletedSubnets(ctx)
			s.purgeExpiredAuditLogs(ctx)
			s.purgeExpiredWebhookDeliveries(ctx)
			s.maybeDispatchReportsSummary(ctx)
			return nil
		},
	)
	if errors.Is(err, ErrBusy) {
		return
	}
	if err != nil {
		log.Printf("scheduler: tick: %v", err)
	}
}

func jobLockObjID(jobID uuid.UUID) int32 {
	h := fnv.New32a()
	_, _ = h.Write(jobID[:])
	return int32(h.Sum32())
}

func (s *Scheduler) runJob(ctx context.Context, job sqlcdb.DiscoveryJob) error {
	objID := jobLockObjID(job.ID)
	return s.withSessionLock(ctx,
		`SELECT pg_try_advisory_lock($1, $2)`,
		`SELECT pg_advisory_unlock($1, $2)`,
		[]any{schedulerJobLockClass, objID},
		func(ctx context.Context) error {
			s.executeJob(ctx, job)
			return nil
		},
	)
}

func (s *Scheduler) executeJob(ctx context.Context, job sqlcdb.DiscoveryJob) {
	subnet, err := s.Store.Queries.GetSubnet(ctx, job.SubnetID)
	if err != nil {
		s.mark(ctx, job.ID, "error", map[string]any{"error": err.Error()})
		s.notifyError(ctx, job, "", err.Error())
		return
	}
	if subnet.DeletedAt.Valid {
		s.mark(ctx, job.ID, "error", map[string]any{"error": "subnet is deleted"})
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	report, err := scanner.Sweep(runCtx, subnet.Cidr, scanner.SweepOptions{
		Workers: 32,
		Enrich:  job.Enrich,
	})
	if err != nil && report == nil {
		s.mark(ctx, job.ID, "error", map[string]any{"error": err.Error()})
		s.notifyError(ctx, job, subnet.Cidr, err.Error())
		return
	}
	ips, _ := s.Store.Queries.ListIPsBySubnet(ctx, job.SubnetID)
	known := map[string]string{} // address -> status
	for _, ip := range ips {
		known[ip.Address] = ip.Status
	}
	aliveSet := map[string]bool{}
	var newAlive []string
	for _, r := range report.Results {
		if !r.Alive {
			continue
		}
		aliveSet[r.IP] = true
		if _, ok := known[r.IP]; !ok {
			newAlive = append(newAlive, r.IP)
		}
	}
	var stale []string
	for addr, status := range known {
		if status == "available" {
			continue
		}
		if !aliveSet[addr] {
			stale = append(stale, addr)
		}
	}
	summary := map[string]any{
		"cidr":      subnet.Cidr,
		"alive":     report.Alive,
		"total":     report.Total,
		"new_alive": len(newAlive),
		"stale":     len(stale),
	}
	s.mark(ctx, job.ID, "ok", summary)
	log.Printf("scheduler: job %s (%s) alive=%d new=%d stale=%d", job.Name, subnet.Cidr, report.Alive, len(newAlive), len(stale))

	base := map[string]any{
		"job_id":    job.ID.String(),
		"job_name":  job.Name,
		"subnet_id": job.SubnetID.String(),
		"cidr":      subnet.Cidr,
	}
	jobUUID := pgtype.UUID{Bytes: job.ID, Valid: true}
	for _, addr := range newAlive {
		s.upsertFinding(ctx, job.SubnetID, addr, "new_alive", "", "", "", pgtype.UUID{}, "", jobUUID)
	}
	for _, addr := range stale {
		var ipID pgtype.UUID
		var status string
		for _, ip := range ips {
			if ip.Address == addr {
				ipID = pgtype.UUID{Bytes: ip.ID, Valid: true}
				status = ip.Status
				break
			}
		}
		s.upsertFinding(ctx, job.SubnetID, addr, "stale", "", "", "", ipID, status, jobUUID)
	}
	if len(newAlive) > 0 {
		payload := map[string]any{}
		for k, v := range base {
			payload[k] = v
		}
		payload["count"] = len(newAlive)
		payload["hosts"] = newAlive
		s.dispatchMuted(ctx, webhook.EventDiscoveryNewAlive, job.SubnetID.String(), payload,
			"New alive hosts", fmt.Sprintf("%d new host(s) in %s", len(newAlive), subnet.Cidr))
	}
	if len(stale) > 0 {
		payload := map[string]any{}
		for k, v := range base {
			payload[k] = v
		}
		payload["count"] = len(stale)
		payload["hosts"] = stale
		s.dispatchMuted(ctx, webhook.EventDiscoveryStale, job.SubnetID.String(), payload,
			"Stale hosts", fmt.Sprintf("%d stale host(s) in %s", len(stale), subnet.Cidr))
	}
	s.checkUtilization(ctx, subnet)
}

func (s *Scheduler) upsertFinding(ctx context.Context, subnetID uuid.UUID, address, kind, hostname, mac, ports string, dbIPID pgtype.UUID, dbStatus string, jobID pgtype.UUID) {
	existing, err := s.Store.Queries.GetOpenFinding(ctx, sqlcdb.GetOpenFindingParams{
		SubnetID: subnetID, Address: address, Kind: kind,
	})
	if err == nil {
		_, _ = s.Store.Queries.TouchFinding(ctx, sqlcdb.TouchFindingParams{
			ID: existing.ID, Hostname: hostname, MacAddress: mac, OpenPorts: ports,
			InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
			DbIpID: dbIPID, DbStatus: dbStatus, JobID: jobID,
		})
		return
	}
	_, _ = s.Store.Queries.InsertFinding(ctx, sqlcdb.InsertFindingParams{
		SubnetID: subnetID, Address: address, Kind: kind,
		Hostname: hostname, MacAddress: mac, OpenPorts: ports,
		DbIpID: dbIPID, DbStatus: dbStatus, JobID: jobID,
	})
}

func (s *Scheduler) checkUtilization(ctx context.Context, subnet sqlcdb.GetSubnetRow) {
	threshold := s.utilAlertThreshold(ctx, subnet.ID)
	if threshold <= 0 {
		return
	}
	prefix, err := ipam.ParsePrefix(subnet.Cidr)
	if err != nil {
		return
	}
	if ipam.IsSparsePrefix(prefix) {
		// No meaningful utilization % for sparse (e.g. IPv6 /64) prefixes.
		return
	}
	usedAddrs, err := s.Store.Queries.ListUsedAddressesBySubnet(ctx, subnet.ID)
	if err != nil {
		return
	}
	used := len(ipam.UsedSet(usedAddrs))
	total, err := ipam.TotalUsable(prefix)
	if err != nil || total <= 0 {
		return
	}
	pct := float64(used) / float64(total) * 100
	if pct < threshold {
		return
	}
	s.dispatchMuted(ctx, webhook.EventSubnetUtilHigh, subnet.ID.String(), map[string]any{
		"subnet_id": subnet.ID.String(),
		"cidr":      subnet.Cidr,
		"name":      subnet.Name,
		"used":      used,
		"total":     total,
		"percent":   pct,
		"threshold": threshold,
	}, "High utilization", fmt.Sprintf("%s at %.0f%% (threshold %.0f%%)", subnet.Cidr, pct, threshold))
}

func (s *Scheduler) utilAlertThreshold(ctx context.Context, subnetID uuid.UUID) float64 {
	if row, err := s.Store.Queries.GetSubnetSettings(ctx, subnetID); err == nil && row.UtilAlertPercent.Valid {
		return float64(row.UtilAlertPercent.Int32)
	}
	global := defaultUtilHighThreshold
	if row, err := s.Store.Queries.GetSetting(ctx, settingSubnetUtilizationAlertPercent); err == nil {
		if v, perr := strconv.Atoi(strings.TrimSpace(row.Value)); perr == nil {
			if v <= 0 {
				return 0
			}
			if v > 100 {
				v = 100
			}
			global = float64(v)
		}
	}
	return global
}

func (s *Scheduler) purgeExpiredDeletedIPs(ctx context.Context) {
	row, err := s.Store.Queries.GetSetting(ctx, settingIPDeletedRetentionDays)
	days := 90
	if err == nil {
		if d, perr := strconv.Atoi(strings.TrimSpace(row.Value)); perr == nil {
			days = d
		}
	}
	if days <= 0 {
		return
	}
	before := time.Now().UTC().AddDate(0, 0, -days)
	n, err := s.Store.Queries.PurgeDeletedIPsBefore(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		log.Printf("scheduler: purge deleted ips: %v", err)
		return
	}
	if n > 0 {
		log.Printf("scheduler: purged %d soft-deleted IPs older than %d days", n, days)
	}
}

func (s *Scheduler) purgeExpiredDeletedSubnets(ctx context.Context) {
	row, err := s.Store.Queries.GetSetting(ctx, settingSubnetDeletedRetentionDays)
	days := 90
	if err == nil {
		if d, perr := strconv.Atoi(strings.TrimSpace(row.Value)); perr == nil {
			days = d
		}
	}
	if days <= 0 {
		return
	}
	before := time.Now().UTC().AddDate(0, 0, -days)
	n, err := s.Store.Queries.PurgeDeletedSubnetsBefore(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		log.Printf("scheduler: purge deleted subnets: %v", err)
		return
	}
	if n > 0 {
		log.Printf("scheduler: purged %d soft-deleted subnets older than %d days", n, days)
	}
}

func (s *Scheduler) maybeDispatchReportsSummary(ctx context.Context) {
	if s.BuildReportsSummary == nil {
		return
	}
	if s.Hooks == nil && (s.Mail == nil || !s.Mail.Enabled(ctx)) {
		return
	}
	row, err := s.Store.Queries.GetSetting(ctx, settingReportsSummaryIntervalHours)
	hours := 24
	if err == nil {
		if h, perr := strconv.Atoi(strings.TrimSpace(row.Value)); perr == nil {
			hours = h
		}
	}
	if hours <= 0 {
		return
	}
	lastRow, err := s.Store.Queries.GetSetting(ctx, settingReportsSummaryLastSentAt)
	if err == nil {
		if last, perr := time.Parse(time.RFC3339, strings.TrimSpace(lastRow.Value)); perr == nil {
			if time.Since(last) < time.Duration(hours)*time.Hour {
				return
			}
		}
	}
	payload, err := s.BuildReportsSummary(ctx)
	if err != nil {
		log.Printf("scheduler: build reports summary: %v", err)
		return
	}
	if s.Hooks != nil {
		s.Hooks.Dispatch(ctx, webhook.EventReportsSummary, payload)
	}
	if s.Mail != nil {
		s.Mail.SendAlert(ctx, mail.AlertData{
			Event:     webhook.EventReportsSummary,
			Title:     "Reports summary",
			Body:      "Scheduled IPAM reports summary is ready.",
			EntityKey: "reports",
		})
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = s.Store.Queries.UpsertSetting(ctx, sqlcdb.UpsertSettingParams{
		Key: settingReportsSummaryLastSentAt, Value: now,
	})
	log.Printf("scheduler: dispatched reports.summary")
}

func (s *Scheduler) purgeExpiredAuditLogs(ctx context.Context) {
	row, err := s.Store.Queries.GetSetting(ctx, settingAuditRetentionDays)
	days := 365
	if err == nil {
		if d, perr := strconv.Atoi(strings.TrimSpace(row.Value)); perr == nil {
			days = d
		}
	}
	if days <= 0 {
		return
	}
	before := time.Now().UTC().AddDate(0, 0, -days)
	n, err := s.Store.Queries.PurgeAuditLogsBefore(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		log.Printf("scheduler: purge audit logs: %v", err)
		return
	}
	if n > 0 {
		log.Printf("scheduler: purged %d audit logs older than %d days", n, days)
	}
}

func (s *Scheduler) purgeExpiredWebhookDeliveries(ctx context.Context) {
	row, err := s.Store.Queries.GetSetting(ctx, settingWebhookDeliveryRetentionDays)
	days := 30
	if err == nil {
		if d, perr := strconv.Atoi(strings.TrimSpace(row.Value)); perr == nil {
			days = d
		}
	}
	if days <= 0 {
		return
	}
	before := time.Now().UTC().AddDate(0, 0, -days)
	n, err := s.Store.Queries.PurgeWebhookDeliveriesBefore(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		log.Printf("scheduler: purge webhook deliveries: %v", err)
		return
	}
	if n > 0 {
		log.Printf("scheduler: purged %d webhook deliveries older than %d days", n, days)
	}
}

func (s *Scheduler) notifyError(ctx context.Context, job sqlcdb.DiscoveryJob, cidr, msg string) {
	s.dispatchMuted(ctx, webhook.EventDiscoveryError, job.ID.String(), map[string]any{
		"job_id": job.ID.String(), "job_name": job.Name,
		"subnet_id": job.SubnetID.String(), "cidr": cidr, "error": msg,
	}, "Discovery error", msg)
}

func (s *Scheduler) mark(ctx context.Context, jobID uuid.UUID, status string, summary map[string]any) {
	raw, _ := json.Marshal(summary)
	now := time.Now().UTC()
	_, err := s.Store.Queries.MarkDiscoveryJobRun(ctx, sqlcdb.MarkDiscoveryJobRunParams{
		ID:          jobID,
		LastStatus:  status,
		LastSummary: raw,
	})
	if err != nil {
		log.Printf("scheduler: mark job %s: %v", jobID, err)
	}
	_, _ = s.Store.Queries.CreateDiscoveryJobRun(ctx, sqlcdb.CreateDiscoveryJobRunParams{
		JobID: jobID, Status: status, Summary: raw,
		StartedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		FinishedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
}

func (s *Scheduler) muteHours(ctx context.Context) int {
	row, err := s.Store.Queries.GetSetting(ctx, settingWebhookMuteHours)
	if err != nil {
		return defaultWebhookMuteHours
	}
	v, err := strconv.Atoi(strings.TrimSpace(row.Value))
	if err != nil || v < 0 {
		return defaultWebhookMuteHours
	}
	return v
}

func (s *Scheduler) dispatchMuted(ctx context.Context, event, entityKey string, payload map[string]any, title, body string) {
	hours := s.muteHours(ctx)
	if hours > 0 {
		if row, err := s.Store.Queries.GetWebhookMute(ctx, sqlcdb.GetWebhookMuteParams{
			Event: event, EntityKey: entityKey,
		}); err == nil && row.MutedUntil.Valid && row.MutedUntil.Time.After(time.Now().UTC()) {
			return
		}
	}
	if s.Hooks != nil {
		s.Hooks.Dispatch(ctx, event, payload)
	}
	if s.Mail != nil {
		s.Mail.SendAlert(ctx, mail.AlertData{
			Event: event, Title: title, Body: body, EntityKey: entityKey,
		})
	}
	_, _ = s.Store.Queries.CreateNotification(ctx, sqlcdb.CreateNotificationParams{
		Kind: event, Title: title, Body: body, EntityType: "subnet", EntityID: entityKey,
	})
	if hours > 0 {
		_ = s.Store.Queries.UpsertWebhookMute(ctx, sqlcdb.UpsertWebhookMuteParams{
			Event: event, EntityKey: entityKey,
			MutedUntil: pgtype.Timestamptz{Time: time.Now().UTC().Add(time.Duration(hours) * time.Hour), Valid: true},
		})
	}
}

// RunNow executes a job immediately (used by API). Gated by a per-job session advisory lock.
func (s *Scheduler) RunNow(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.Store.Queries.GetDiscoveryJob(ctx, jobID)
	if err != nil {
		return err
	}
	if err := s.runJob(ctx, job); err != nil {
		return err
	}
	return nil
}
