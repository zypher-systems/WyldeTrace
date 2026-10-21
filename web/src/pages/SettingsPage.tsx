import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { api, getToken } from "../api";
import { useAuth } from "../auth";
import { useConfirm } from "../components/ConfirmDialog";
import { PageHeader } from "../components/PageHeader";
import { Button, Card, Field, Section } from "../components/ui";
import { Skeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";
import { AccountSummary } from "./settings/AccountSummary";
import { SSOSection } from "./settings/SSOSection";
import { IntegrationsSection } from "./settings/IntegrationsSection";
import { MFASection } from "./settings/MFASection";
import { PasswordSection } from "./settings/PasswordSection";

export type SettingsSection = "account" | "general" | "notifications" | "integrations" | "data";

const SECTION_META: Record<SettingsSection, { title: string; subtitle: string }> = {
  account: { title: "Account & security", subtitle: "Your sign-in, password, and two-factor setup" },
  general: { title: "General", subtitle: "Instance health, alert thresholds, and SSO" },
  notifications: {
    title: "Notifications",
    subtitle: "Email alerts, scheduled report dispatch, and duplicate suppression",
  },
  integrations: {
    title: "DNS, DHCP & SNMP",
    subtitle: "PowerDNS write-back, Kea DHCP sync, and SNMP switch enrichment",
  },
  data: { title: "Retention & backup", subtitle: "Automatic purge windows and JSON backup/restore" },
};

export function SettingsPage({ section = "account" }: { section?: SettingsSection }) {
  const { user, refresh } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const [settingsLoaded, setSettingsLoaded] = useState(false);
  const settingsBaseline = useRef("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [retentionDays, setRetentionDays] = useState(90);
  const [subnetRetentionDays, setSubnetRetentionDays] = useState(90);
  const [auditRetentionDays, setAuditRetentionDays] = useState(365);
  const [webhookRetentionDays, setWebhookRetentionDays] = useState(30);
  const [reportsIntervalHours, setReportsIntervalHours] = useState(24);
  const [utilAlertPercent, setUtilAlertPercent] = useState(90);
  const [muteHours, setMuteHours] = useState(6);
  const [pdnsEnabled, setPdnsEnabled] = useState(false);
  const [pdnsURL, setPdnsURL] = useState("");
  const [pdnsAPIKey, setPdnsAPIKey] = useState("");
  const [pdnsAPIKeySet, setPdnsAPIKeySet] = useState(false);
  const [pdnsDryRun, setPdnsDryRun] = useState(true);
  const [pdnsServer, setPdnsServer] = useState("localhost");
  const [keaEnabled, setKeaEnabled] = useState(false);
  const [keaURL, setKeaURL] = useState("");
  const [keaDryRun, setKeaDryRun] = useState(true);
  const [snmpEnabled, setSnmpEnabled] = useState(false);
  const [snmpCommunity, setSnmpCommunity] = useState("public");
  const [snmpVersion, setSnmpVersion] = useState("2c");
  const [snmpTimeout, setSnmpTimeout] = useState(5);
  const [snmpTarget, setSnmpTarget] = useState("");
  const [smtpEnabled, setSmtpEnabled] = useState(false);
  const [smtpHost, setSmtpHost] = useState("");
  const [smtpPort, setSmtpPort] = useState(587);
  const [smtpUser, setSmtpUser] = useState("");
  const [smtpPass, setSmtpPass] = useState("");
  const [smtpPassSet, setSmtpPassSet] = useState(false);
  const [smtpFrom, setSmtpFrom] = useState("");
  const [smtpTo, setSmtpTo] = useState("");
  const [smtpSubject, setSmtpSubject] = useState("[WyldeTrace] {{.Title}}");
  const [smtpBody, setSmtpBody] = useState(
    "{{.Title}}\n\n{{.Body}}\n\nEvent: {{.Event}}\nEntity: {{.EntityKey}}\n",
  );
  const [mfaEnabled, setMfaEnabled] = useState(false);
  const [mfaSecret, setMfaSecret] = useState("");
  const [mfaURL, setMfaURL] = useState("");
  const [mfaCode, setMfaCode] = useState("");
  const [mfaRecovery, setMfaRecovery] = useState<string[] | null>(null);
  const [mfaDisablePassword, setMfaDisablePassword] = useState("");
  const [mfaDisableCode, setMfaDisableCode] = useState("");
  const [instance, setInstance] = useState<{
    version?: string;
    migration_version?: string;
    db_ok?: boolean;
    login_rate_limit?: string;
  } | null>(null);
  const [restorePreview, setRestorePreview] = useState<{
    would_create: Record<string, number>;
    would_skip: Record<string, number>;
    in_backup: Record<string, number>;
    notes?: string[];
  } | null>(null);
  const [pendingRestore, setPendingRestore] = useState("");
  const [oidc, setOidc] = useState<{
    enabled: boolean;
    issuer?: string;
    auto_provision?: boolean;
    default_role?: string;
    groups_claim?: string;
    role_claim?: string;
    role_map?: Record<string, string[]>;
    sync_role?: boolean;
  } | null>(null);
  const [previewGroups, setPreviewGroups] = useState("");
  const [previewRole, setPreviewRole] = useState("");
  const [previewResult, setPreviewResult] = useState<{
    role: string;
    source: string;
    matched_groups: string[];
    would_sync?: boolean;
  } | null>(null);
  const restoreRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    void (async () => {
      try {
        setOidc(
          await api<{
            enabled: boolean;
            issuer?: string;
            auto_provision?: boolean;
            default_role?: string;
            groups_claim?: string;
            role_claim?: string;
            role_map?: Record<string, string[]>;
            sync_role?: boolean;
          }>("/api/v1/auth/oidc/status"),
        );
      } catch {
        setOidc({ enabled: false });
      }
      if (user?.role === "admin") {
        try {
          const s = await api<{
            ip_deleted_retention_days: number;
            subnet_deleted_retention_days: number;
            audit_retention_days: number;
            webhook_delivery_retention_days: number;
            reports_summary_interval_hours: number;
            subnet_utilization_alert_percent: number;
            webhook_mute_hours: number;
            powerdns_enabled: boolean;
            powerdns_url: string;
            powerdns_api_key_set: boolean;
            powerdns_dry_run: boolean;
            powerdns_server: string;
            kea_enabled: boolean;
            kea_url: string;
            kea_dry_run: boolean;
            snmp_enabled: boolean;
            snmp_community_set: boolean;
            snmp_version: string;
            snmp_timeout_seconds: number;
            smtp_enabled: boolean;
            smtp_host: string;
            smtp_port: number;
            smtp_user: string;
            smtp_pass_set: boolean;
            smtp_from: string;
            smtp_to: string;
            smtp_subject_template: string;
            smtp_body_template: string;
          }>("/api/v1/settings");
          setRetentionDays(s.ip_deleted_retention_days);
          setSubnetRetentionDays(s.subnet_deleted_retention_days);
          setAuditRetentionDays(s.audit_retention_days);
          setWebhookRetentionDays(s.webhook_delivery_retention_days);
          setReportsIntervalHours(s.reports_summary_interval_hours);
          setUtilAlertPercent(s.subnet_utilization_alert_percent);
          setMuteHours(s.webhook_mute_hours);
          setPdnsEnabled(s.powerdns_enabled);
          setPdnsURL(s.powerdns_url || "");
          setPdnsAPIKeySet(s.powerdns_api_key_set);
          setPdnsDryRun(s.powerdns_dry_run);
          setPdnsServer(s.powerdns_server || "localhost");
          setKeaEnabled(s.kea_enabled);
          setKeaURL(s.kea_url || "");
          setKeaDryRun(s.kea_dry_run);
          setSnmpEnabled(s.snmp_enabled);
          setSnmpCommunity("");
          setSnmpVersion(s.snmp_version || "2c");
          setSnmpTimeout(s.snmp_timeout_seconds || 5);
          setSmtpEnabled(s.smtp_enabled);
          setSmtpHost(s.smtp_host || "");
          setSmtpPort(s.smtp_port || 587);
          setSmtpUser(s.smtp_user || "");
          setSmtpPassSet(s.smtp_pass_set);
          setSmtpFrom(s.smtp_from || "");
          setSmtpTo(s.smtp_to || "");
          setSmtpSubject(s.smtp_subject_template || "[WyldeTrace] {{.Title}}");
          setSmtpBody(
            s.smtp_body_template ||
              "{{.Title}}\n\n{{.Body}}\n\nEvent: {{.Event}}\nEntity: {{.EntityKey}}\n",
          );
          setInstance(await api("/api/v1/instance"));
          settingsBaseline.current = JSON.stringify({
            retentionDays: s.ip_deleted_retention_days,
            subnetRetentionDays: s.subnet_deleted_retention_days,
            auditRetentionDays: s.audit_retention_days,
            webhookRetentionDays: s.webhook_delivery_retention_days,
            reportsIntervalHours: s.reports_summary_interval_hours,
            utilAlertPercent: s.subnet_utilization_alert_percent,
            muteHours: s.webhook_mute_hours,
            pdnsEnabled: s.powerdns_enabled,
            pdnsURL: s.powerdns_url || "",
            pdnsAPIKey: "",
            pdnsDryRun: s.powerdns_dry_run,
            pdnsServer: s.powerdns_server || "localhost",
            keaEnabled: s.kea_enabled,
            keaURL: s.kea_url || "",
            keaDryRun: s.kea_dry_run,
            snmpEnabled: s.snmp_enabled,
            snmpCommunity: "",
            snmpVersion: s.snmp_version || "2c",
            snmpTimeout: s.snmp_timeout_seconds || 5,
            smtpEnabled: s.smtp_enabled,
            smtpHost: s.smtp_host || "",
            smtpPort: s.smtp_port || 587,
            smtpUser: s.smtp_user || "",
            smtpPass: "",
            smtpFrom: s.smtp_from || "",
            smtpTo: s.smtp_to || "",
            smtpSubject: s.smtp_subject_template || "[WyldeTrace] {{.Title}}",
            smtpBody:
              s.smtp_body_template ||
              "{{.Title}}\n\n{{.Body}}\n\nEvent: {{.Event}}\nEntity: {{.EntityKey}}\n",
          });
          setSettingsLoaded(true);
        } catch {
          /* ignore */
        }
      }
      try {
        const mfa = await api<{ totp_enabled: boolean }>("/api/v1/auth/mfa");
        setMfaEnabled(mfa.totp_enabled);
      } catch {
        setMfaEnabled(!!user?.totp_enabled);
      }
    })();
  }, [user?.role, user?.totp_enabled]);

  const settingsSnapshot = useMemo(
    () =>
      JSON.stringify({
        retentionDays,
        subnetRetentionDays,
        auditRetentionDays,
        webhookRetentionDays,
        reportsIntervalHours,
        utilAlertPercent,
        muteHours,
        pdnsEnabled,
        pdnsURL,
        pdnsAPIKey,
        pdnsDryRun,
        pdnsServer,
        keaEnabled,
        keaURL,
        keaDryRun,
        snmpEnabled,
        snmpCommunity,
        snmpVersion,
        snmpTimeout,
        smtpEnabled,
        smtpHost,
        smtpPort,
        smtpUser,
        smtpPass,
        smtpFrom,
        smtpTo,
        smtpSubject,
        smtpBody,
      }),
    [
      retentionDays,
      subnetRetentionDays,
      auditRetentionDays,
      webhookRetentionDays,
      reportsIntervalHours,
      utilAlertPercent,
      muteHours,
      pdnsEnabled,
      pdnsURL,
      pdnsAPIKey,
      pdnsDryRun,
      pdnsServer,
      keaEnabled,
      keaURL,
      keaDryRun,
      snmpEnabled,
      snmpCommunity,
      snmpVersion,
      snmpTimeout,
      smtpEnabled,
      smtpHost,
      smtpPort,
      smtpUser,
      smtpPass,
      smtpFrom,
      smtpTo,
      smtpSubject,
      smtpBody,
    ],
  );
  useUnsavedChanges(settingsLoaded && settingsSnapshot !== settingsBaseline.current);

  function markSettingsSaved(overrides: Record<string, unknown> = {}) {
    const base = JSON.parse(settingsSnapshot) as Record<string, unknown>;
    settingsBaseline.current = JSON.stringify({
      ...base,
      pdnsAPIKey: "",
      smtpPass: "",
      ...overrides,
    });
  }

  async function downloadBackup() {
    try {
      const token = getToken();
      const res = await fetch("/api/v1/backup", {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "wyldetrace-backup.json";
      a.click();
      URL.revokeObjectURL(url);
      toast("Backup downloaded", "success");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Backup failed", "error");
    }
  }

  async function onRestoreFile(file: File | null) {
    if (!file) return;
    try {
      const body = await file.text();
      const token = getToken();
      const res = await fetch("/api/v1/backup/restore/preview", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body,
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || res.statusText);
      setPendingRestore(body);
      setRestorePreview(data);
      toast("Review merge preview, then confirm restore", "success");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Preview failed", "error");
      setRestorePreview(null);
      setPendingRestore("");
    }
  }

  async function confirmRestore() {
    if (!pendingRestore) return;
    if (
      !(await confirm({
        title: "Merge this backup into the current database?",
        danger: true,
        confirmLabel: "Merge",
      }))
    )
      return;
    try {
      const token = getToken();
      const res = await fetch("/api/v1/backup/restore", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Confirm-Restore": "true",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: pendingRestore,
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || res.statusText);
      toast(`Restore ok: ${JSON.stringify(data.created)}`, "success");
      setRestorePreview(null);
      setPendingRestore("");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Restore failed", "error");
    }
  }

  async function onChangePassword(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/api/v1/auth/password", {
        method: "POST",
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      });
      setCurrentPassword("");
      setNewPassword("");
      toast("Password updated", "success");
      await refresh();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Password change failed", "error");
    }
  }

  const isAdmin = user?.role === "admin";
  const meta = SECTION_META[section];

  async function saveSetting(body: Record<string, unknown>, message: string) {
    try {
      await api("/api/v1/settings", { method: "PUT", body: JSON.stringify(body) });
      toast(message, "success");
      markSettingsSaved();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Settings update failed", "error");
    }
  }

  async function purge(title: string, path: string, noun: string) {
    if (!(await confirm({ title, danger: true, confirmLabel: "Purge" }))) return;
    try {
      const res = await api<{ purged: number }>(path, {
        method: "POST",
        headers: { "X-Confirm-Purge": "true" },
      });
      toast(`Purged ${res.purged} ${noun}`, "success");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Purge failed", "error");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader title={meta.title} subtitle={meta.subtitle} />

      {section !== "account" && isAdmin && !settingsLoaded && (
        <div className="max-w-lg space-y-3" role="status" aria-label="Loading settings">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      )}

      {section === "account" && (
        <>
          <Section title="Profile" description="Your identity and role">
            <AccountSummary user={user} />
          </Section>
          {user?.role === "reportview" && (
            <Card className="p-5 text-sm text-sky-100">
              <p className="font-medium">Report-only account</p>
              <p className="mt-1 text-sky-200/80">
                You can view the overview (site-scoped when assigned), submit IP requests, and
                change your password. Inventory, discovery, and admin tools are hidden.
              </p>
            </Card>
          )}
          <div className="grid gap-6 lg:grid-cols-2">
            <Card className="p-5">
              <PasswordSection
                currentPassword={currentPassword}
                setCurrentPassword={setCurrentPassword}
                newPassword={newPassword}
                setNewPassword={setNewPassword}
                onSubmit={onChangePassword}
              />
            </Card>
            <Card className="p-5">
              <MFASection
                mfaEnabled={mfaEnabled}
                setMfaEnabled={setMfaEnabled}
                mfaSecret={mfaSecret}
                setMfaSecret={setMfaSecret}
                mfaURL={mfaURL}
                setMfaURL={setMfaURL}
                mfaCode={mfaCode}
                setMfaCode={setMfaCode}
                mfaRecovery={mfaRecovery}
                setMfaRecovery={setMfaRecovery}
                mfaDisablePassword={mfaDisablePassword}
                setMfaDisablePassword={setMfaDisablePassword}
                mfaDisableCode={mfaDisableCode}
                setMfaDisableCode={setMfaDisableCode}
                refresh={refresh}
              />
            </Card>
          </div>
          {oidc && !isAdmin && (
            <Card className="p-5">
              <SSOSection
                oidc={oidc}
                isAdmin={false}
                currentRole={user?.role}
                previewGroups={previewGroups}
                setPreviewGroups={setPreviewGroups}
                previewRole={previewRole}
                setPreviewRole={setPreviewRole}
                previewResult={previewResult}
                setPreviewResult={setPreviewResult}
              />
            </Card>
          )}
        </>
      )}

      {section === "general" && isAdmin && (
        <>
          <Section title="Instance" description="Build and runtime health">
            {instance ? (
              <dl className="divide-y divide-[var(--color-border)] text-sm">
                <InfoRow label="Version" value={<span className="font-mono">{instance.version || "—"}</span>} />
                <InfoRow
                  label="Migration"
                  value={<span className="font-mono">{instance.migration_version || "—"}</span>}
                />
                <InfoRow
                  label="Database"
                  value={
                    <span className={instance.db_ok ? "text-brand" : "text-red-400"}>
                      {instance.db_ok ? "Healthy" : "Down"}
                    </span>
                  }
                />
                <InfoRow label="Login limit" value={<span className="text-xs">{instance.login_rate_limit}</span>} />
              </dl>
            ) : (
              <Skeleton className="h-24 w-full" />
            )}
          </Section>

          <Section
            title="Utilization alerts"
            description={
              <>
                Fire <span className="font-mono">subnet.utilization_high</span> when a subnet
                reaches this percent used (0 disables). Subnets can override on their detail page.
              </>
            }
          >
            <SettingNumber
              label="Threshold (%)"
              min={0}
              max={100}
              value={utilAlertPercent}
              onChange={setUtilAlertPercent}
              onSave={() =>
                void saveSetting(
                  { subnet_utilization_alert_percent: utilAlertPercent },
                  `Utilization alert threshold set to ${utilAlertPercent}%`,
                )
              }
              saveLabel="Save threshold"
            />
          </Section>

          {oidc && (
            <Card className="p-5">
              <SSOSection
                oidc={oidc}
                isAdmin
                currentRole={user?.role}
                previewGroups={previewGroups}
                setPreviewGroups={setPreviewGroups}
                previewRole={previewRole}
                setPreviewRole={setPreviewRole}
                previewResult={previewResult}
                setPreviewResult={setPreviewResult}
              />
            </Card>
          )}
        </>
      )}

      {section === "notifications" && isAdmin && (
        <>
          <Section
            title="Email alerts (SMTP)"
            description={
              <>
                Email utilization and discovery alerts alongside webhooks. Templates use{" "}
                <code className="text-[11px]">{"{{.Title}} {{.Body}} {{.Event}} {{.EntityKey}}"}</code>.
              </>
            }
            actions={
              <label className="flex items-center gap-2 text-sm text-zinc-300">
                <input
                  type="checkbox"
                  checked={smtpEnabled}
                  onChange={(e) => setSmtpEnabled(e.target.checked)}
                />
                Enabled
              </label>
            }
          >
            <div className="grid gap-4 sm:grid-cols-6">
              <Field label="Host" className="sm:col-span-4">
                <input
                  className="wt-input w-full font-mono text-xs"
                  value={smtpHost}
                  onChange={(e) => setSmtpHost(e.target.value)}
                  placeholder="smtp.example.com"
                />
              </Field>
              <Field label="Port" className="sm:col-span-2">
                <input
                  type="number"
                  className="wt-input w-full font-mono"
                  value={smtpPort}
                  onChange={(e) => setSmtpPort(Number(e.target.value))}
                />
              </Field>
              <Field label="Username" className="sm:col-span-3">
                <input
                  className="wt-input w-full font-mono text-xs"
                  value={smtpUser}
                  onChange={(e) => setSmtpUser(e.target.value)}
                />
              </Field>
              <Field
                label="Password"
                hint={smtpPassSet ? "Set — leave blank to keep" : undefined}
                className="sm:col-span-3"
              >
                <input
                  type="password"
                  className="wt-input w-full font-mono text-xs"
                  value={smtpPass}
                  onChange={(e) => setSmtpPass(e.target.value)}
                  autoComplete="new-password"
                />
              </Field>
              <Field label="From" className="sm:col-span-3">
                <input
                  className="wt-input w-full font-mono text-xs"
                  value={smtpFrom}
                  onChange={(e) => setSmtpFrom(e.target.value)}
                  placeholder="wyldetrace@example.com"
                />
              </Field>
              <Field label="To" hint="Comma-separated" className="sm:col-span-3">
                <input
                  className="wt-input w-full font-mono text-xs"
                  value={smtpTo}
                  onChange={(e) => setSmtpTo(e.target.value)}
                  placeholder="ops@example.com"
                />
              </Field>
              <Field label="Subject template" className="sm:col-span-6">
                <input
                  className="wt-input w-full font-mono text-xs"
                  value={smtpSubject}
                  onChange={(e) => setSmtpSubject(e.target.value)}
                />
              </Field>
              <Field label="Body template" className="sm:col-span-6">
                <textarea
                  className="wt-input w-full font-mono text-xs"
                  rows={4}
                  value={smtpBody}
                  onChange={(e) => setSmtpBody(e.target.value)}
                />
              </Field>
            </div>
            <div className="mt-4 flex justify-end">
              <Button
                variant="primary"
                onClick={() =>
                  void (async () => {
                    try {
                      const body: Record<string, unknown> = {
                        smtp_enabled: smtpEnabled,
                        smtp_host: smtpHost,
                        smtp_port: smtpPort,
                        smtp_user: smtpUser,
                        smtp_from: smtpFrom,
                        smtp_to: smtpTo,
                        smtp_subject_template: smtpSubject,
                        smtp_body_template: smtpBody,
                      };
                      if (smtpPass) body.smtp_pass = smtpPass;
                      await api("/api/v1/settings", { method: "PUT", body: JSON.stringify(body) });
                      setSmtpPass("");
                      if (smtpPass) setSmtpPassSet(true);
                      toast("SMTP settings saved", "success");
                      markSettingsSaved();
                    } catch (err) {
                      toast(err instanceof Error ? err.message : "SMTP settings failed", "error");
                    }
                  })()
                }
              >
                Save SMTP
              </Button>
            </div>
          </Section>

          <div className="grid gap-6 lg:grid-cols-2">
            <Section
              title="Scheduled report summary"
              description={
                <>
                  Dispatch <span className="font-mono">reports.summary</span> to matching webhooks
                  on this interval. 0 disables scheduled dispatch.
                </>
              }
            >
              <SettingNumber
                label="Interval (hours)"
                min={0}
                max={2160}
                value={reportsIntervalHours}
                onChange={setReportsIntervalHours}
                onSave={() =>
                  void saveSetting(
                    { reports_summary_interval_hours: reportsIntervalHours },
                    `Reports webhook interval set to ${reportsIntervalHours} hour(s)`,
                  )
                }
                saveLabel="Save interval"
              />
            </Section>
            <Section
              title="Duplicate suppression"
              description="Mute repeat discovery/utilization webhooks for the same entity (0 = no mute)."
            >
              <SettingNumber
                label="Mute window (hours)"
                min={0}
                max={720}
                value={muteHours}
                onChange={setMuteHours}
                onSave={() =>
                  void saveSetting(
                    { webhook_mute_hours: muteHours },
                    `Webhook mute set to ${muteHours} hour(s)`,
                  )
                }
                saveLabel="Save mute"
              />
            </Section>
          </div>
        </>
      )}

      {section === "integrations" && isAdmin && (
        <Card className="p-5">
          <IntegrationsSection
            pdnsEnabled={pdnsEnabled}
            setPdnsEnabled={setPdnsEnabled}
            pdnsURL={pdnsURL}
            setPdnsURL={setPdnsURL}
            pdnsAPIKey={pdnsAPIKey}
            setPdnsAPIKey={setPdnsAPIKey}
            pdnsAPIKeySet={pdnsAPIKeySet}
            pdnsDryRun={pdnsDryRun}
            setPdnsDryRun={setPdnsDryRun}
            pdnsServer={pdnsServer}
            setPdnsServer={setPdnsServer}
            keaEnabled={keaEnabled}
            setKeaEnabled={setKeaEnabled}
            keaURL={keaURL}
            setKeaURL={setKeaURL}
            keaDryRun={keaDryRun}
            setKeaDryRun={setKeaDryRun}
            snmpEnabled={snmpEnabled}
            setSnmpEnabled={setSnmpEnabled}
            snmpCommunity={snmpCommunity}
            setSnmpCommunity={setSnmpCommunity}
            snmpVersion={snmpVersion}
            setSnmpVersion={setSnmpVersion}
            snmpTimeout={snmpTimeout}
            setSnmpTimeout={setSnmpTimeout}
            snmpTarget={snmpTarget}
            setSnmpTarget={setSnmpTarget}
            markSettingsSaved={markSettingsSaved}
          />
        </Card>
      )}

      {section === "data" && isAdmin && (
        <>
          <Section
            title="Retention"
            description="Records older than these windows are purged automatically. 0 disables automatic purge."
            bodyClassName="divide-y divide-[var(--color-border)] py-0"
          >
            <RetentionRow
              title="Deleted IP retention"
              detail="Soft-deleted IPs in the trash."
              value={retentionDays}
              onChange={setRetentionDays}
              onSave={() =>
                void saveSetting(
                  { ip_deleted_retention_days: retentionDays },
                  `Retention set to ${retentionDays} day(s)`,
                )
              }
              onPurge={() =>
                void purge(
                  `Permanently purge soft-deleted IPs older than ${retentionDays} days?`,
                  "/api/v1/ips/purge-deleted?confirm=true",
                  "deleted IP(s)",
                )
              }
            />
            <RetentionRow
              title="Deleted subnet retention"
              detail="Soft-deleted subnets; purge also removes cascaded IP records."
              value={subnetRetentionDays}
              onChange={setSubnetRetentionDays}
              onSave={() =>
                void saveSetting(
                  { subnet_deleted_retention_days: subnetRetentionDays },
                  `Subnet retention set to ${subnetRetentionDays} day(s)`,
                )
              }
              onPurge={() =>
                void purge(
                  `Permanently purge soft-deleted subnets older than ${subnetRetentionDays} days?`,
                  "/api/v1/subnets/purge-deleted?confirm=true",
                  "deleted subnet(s)",
                )
              }
            />
            <RetentionRow
              title="Audit log retention"
              detail="Audit events."
              value={auditRetentionDays}
              onChange={setAuditRetentionDays}
              onSave={() =>
                void saveSetting(
                  { audit_retention_days: auditRetentionDays },
                  `Audit retention set to ${auditRetentionDays} day(s)`,
                )
              }
              onPurge={() =>
                void purge(
                  `Permanently purge audit logs older than ${auditRetentionDays} days?`,
                  "/api/v1/audit-logs/purge?confirm=true",
                  "audit log(s)",
                )
              }
            />
            <RetentionRow
              title="Webhook delivery retention"
              detail="Webhook delivery log rows."
              value={webhookRetentionDays}
              onChange={setWebhookRetentionDays}
              onSave={() =>
                void saveSetting(
                  { webhook_delivery_retention_days: webhookRetentionDays },
                  `Webhook delivery retention set to ${webhookRetentionDays} day(s)`,
                )
              }
              onPurge={() =>
                void purge(
                  `Permanently purge webhook deliveries older than ${webhookRetentionDays} days?`,
                  "/api/v1/webhooks/deliveries/purge?confirm=true",
                  "webhook delivery(ies)",
                )
              }
            />
          </Section>

          <Section
            title="Backup & restore"
            description="JSON export of locations, device types, subnets, IPs, and users. Restore merges inventory (does not delete or overwrite users)."
            actions={
              <>
                <Button onClick={() => void downloadBackup()}>Download backup</Button>
                <Button onClick={() => restoreRef.current?.click()}>Preview restore…</Button>
                <input
                  ref={restoreRef}
                  type="file"
                  accept="application/json,.json"
                  className="hidden"
                  onChange={(e) => {
                    void onRestoreFile(e.target.files?.[0] ?? null);
                    e.target.value = "";
                  }}
                />
              </>
            }
          >
            {restorePreview ? (
              <div className="space-y-3 text-sm">
                <p className="font-medium text-zinc-200">Merge preview</p>
                <div className="grid gap-2 sm:grid-cols-4">
                  {(["locations", "device_types", "subnets", "ips"] as const).map((k) => (
                    <div key={k} className="rounded-lg border border-[var(--color-border)] p-3">
                      <div className="wt-eyebrow">{k.replace("_", " ")}</div>
                      <div className="mt-1 font-mono text-xs text-zinc-300">
                        +{restorePreview.would_create[k] ?? 0} create ·{" "}
                        {restorePreview.would_skip[k] ?? 0} skip
                      </div>
                      <div className="font-mono text-[11px] text-zinc-500">
                        {restorePreview.in_backup[k] ?? 0} in backup
                      </div>
                    </div>
                  ))}
                </div>
                {restorePreview.notes?.map((n) => (
                  <p key={n} className="text-xs text-zinc-500">
                    {n}
                  </p>
                ))}
                <div className="flex flex-wrap gap-2 pt-1">
                  <Button variant="primary" onClick={() => void confirmRestore()}>
                    Confirm restore
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setRestorePreview(null);
                      setPendingRestore("");
                    }}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            ) : (
              <p className="text-sm text-zinc-500">
                Choose a backup file to see what would be created before anything is merged.
              </p>
            )}
          </Section>
        </>
      )}
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3 py-2.5">
      <dt className="text-zinc-400">{label}</dt>
      <dd className="text-zinc-200">{value}</dd>
    </div>
  );
}

function SettingNumber({
  label,
  value,
  onChange,
  onSave,
  saveLabel,
  min,
  max,
}: {
  label: string;
  value: number;
  onChange: (n: number) => void;
  onSave: () => void;
  saveLabel: string;
  min: number;
  max: number;
}) {
  return (
    <div className="flex flex-wrap items-end gap-2">
      <Field label={label}>
        <input
          type="number"
          min={min}
          max={max}
          className="wt-input w-32"
          value={value}
          onChange={(e) => onChange(Number(e.target.value))}
        />
      </Field>
      <Button onClick={onSave}>{saveLabel}</Button>
    </div>
  );
}

function RetentionRow({
  title,
  detail,
  value,
  onChange,
  onSave,
  onPurge,
}: {
  title: string;
  detail: string;
  value: number;
  onChange: (n: number) => void;
  onSave: () => void;
  onPurge: () => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-4 py-4">
      <div className="min-w-[14rem] flex-1">
        <h3 className="text-sm font-medium text-zinc-100">{title}</h3>
        <p className="text-xs text-zinc-500">{detail}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="number"
          min={0}
          max={3650}
          aria-label={`${title} (days)`}
          className="wt-input w-24 text-right tabular-nums"
          value={value}
          onChange={(e) => onChange(Number(e.target.value))}
        />
        <span className="text-xs text-zinc-500">days</span>
        <Button size="sm" onClick={onSave}>
          Save retention
        </Button>
        <Button size="sm" variant="danger" onClick={onPurge}>
          Purge now
        </Button>
      </div>
    </div>
  );
}
