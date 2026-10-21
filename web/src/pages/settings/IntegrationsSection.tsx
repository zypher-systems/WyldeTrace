import type { ReactNode } from "react";
import { Globe, Network, RefreshCw, Router, Save, Sparkles } from "lucide-react";
import { api } from "../../api";
import { useToast } from "../../components/Toast";
import { Badge, Button, Field, Switch, cx } from "../../components/ui";

function IntegrationBlock({
  icon,
  title,
  description,
  enabled,
  onEnabledChange,
  badges,
  children,
  footer,
}: {
  icon: ReactNode;
  title: string;
  description: ReactNode;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  badges?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <section className="overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border)] px-4 py-3">
        <div className="flex min-w-0 items-center gap-3">
          <span
            className={cx(
              "grid h-8 w-8 shrink-0 place-items-center rounded-lg",
              enabled ? "bg-brand/15 text-brand" : "bg-[var(--color-surface-overlay)] text-zinc-500",
            )}
          >
            {icon}
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="text-sm font-semibold text-zinc-100">{title}</h3>
              <Badge tone={enabled ? "brand" : "neutral"}>{enabled ? "Enabled" : "Off"}</Badge>
              {badges}
            </div>
            <p className="mt-0.5 text-xs text-zinc-500">{description}</p>
          </div>
        </div>
        <Switch checked={enabled} onChange={onEnabledChange} label={`Enable ${title}`} />
      </header>
      <div className={cx("space-y-4 px-4 py-4 transition", !enabled && "opacity-70")}>{children}</div>
      {footer ? (
        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-[var(--color-border)] px-4 py-3">
          {footer}
        </div>
      ) : null}
    </section>
  );
}

function DryRunToggle({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm text-zinc-300">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      {label}
    </label>
  );
}

export function IntegrationsSection({
  pdnsEnabled,
  setPdnsEnabled,
  pdnsURL,
  setPdnsURL,
  pdnsAPIKey,
  setPdnsAPIKey,
  pdnsAPIKeySet,
  pdnsDryRun,
  setPdnsDryRun,
  pdnsServer,
  setPdnsServer,
  keaEnabled,
  setKeaEnabled,
  keaURL,
  setKeaURL,
  keaDryRun,
  setKeaDryRun,
  snmpEnabled,
  setSnmpEnabled,
  snmpCommunity,
  setSnmpCommunity,
  snmpVersion,
  setSnmpVersion,
  snmpTimeout,
  setSnmpTimeout,
  snmpTarget,
  setSnmpTarget,
  markSettingsSaved,
}: {
  pdnsEnabled: boolean;
  pdnsURL: string;
  pdnsAPIKey: string;
  pdnsAPIKeySet: boolean;
  pdnsDryRun: boolean;
  pdnsServer: string;
  keaEnabled: boolean;
  keaURL: string;
  keaDryRun: boolean;
  snmpEnabled: boolean;
  snmpCommunity: string;
  snmpVersion: string;
  snmpTimeout: number;
  snmpTarget: string;
  setPdnsEnabled: (v: boolean) => void;
  setPdnsURL: (v: string) => void;
  setPdnsAPIKey: (v: string) => void;
  setPdnsDryRun: (v: boolean) => void;
  setPdnsServer: (v: string) => void;
  setKeaEnabled: (v: boolean) => void;
  setKeaURL: (v: string) => void;
  setKeaDryRun: (v: boolean) => void;
  setSnmpEnabled: (v: boolean) => void;
  setSnmpCommunity: (v: string) => void;
  setSnmpVersion: (v: string) => void;
  setSnmpTimeout: (v: number) => void;
  setSnmpTarget: (v: string) => void;
  markSettingsSaved: (overrides?: Record<string, unknown>) => void;
}) {
  const toast = useToast();
  return (
    <div className="space-y-4">
      <p className="text-xs text-zinc-500">
        Delivery attempts are logged under{" "}
        <span className="font-mono text-zinc-400">/api/v1/integrations/deliveries</span>. Changes
        apply after <span className="text-zinc-300">Save integrations</span>.
      </p>

      <IntegrationBlock
        icon={<Globe className="h-4 w-4" />}
        title="PowerDNS"
        description="Write A/PTR records back to PowerDNS when IPs change."
        enabled={pdnsEnabled}
        onEnabledChange={setPdnsEnabled}
        badges={pdnsDryRun ? <Badge tone="warn">Dry run</Badge> : null}
      >
        <div className="grid gap-4 sm:grid-cols-6">
          <Field label="API URL" className="sm:col-span-4">
            <input
              className="wt-input w-full font-mono text-xs"
              value={pdnsURL}
              onChange={(e) => setPdnsURL(e.target.value)}
              placeholder="https://pdns.example.com:8081"
            />
          </Field>
          <Field label="Server ID" className="sm:col-span-2">
            <input
              className="wt-input w-full font-mono text-xs"
              value={pdnsServer}
              onChange={(e) => setPdnsServer(e.target.value)}
            />
          </Field>
          <Field
            label="API key"
            hint={pdnsAPIKeySet ? "Set — leave blank to keep" : undefined}
            className="sm:col-span-4"
          >
            <input
              type="password"
              className="wt-input w-full font-mono text-xs"
              value={pdnsAPIKey}
              onChange={(e) => setPdnsAPIKey(e.target.value)}
              placeholder={pdnsAPIKeySet ? "••••••••" : "optional"}
              autoComplete="new-password"
            />
          </Field>
          <div className="flex items-end pb-2 sm:col-span-2">
            <DryRunToggle checked={pdnsDryRun} onChange={setPdnsDryRun} label="Dry run (log only)" />
          </div>
        </div>
      </IntegrationBlock>

      <IntegrationBlock
        icon={<Network className="h-4 w-4" />}
        title="Kea DHCP"
        description="Sync pools and reservations from the Kea Control Agent."
        enabled={keaEnabled}
        onEnabledChange={setKeaEnabled}
        badges={keaDryRun ? <Badge tone="warn">Dry run</Badge> : null}
        footer={
          <>
            <span className="text-xs text-zinc-500">Pull pools and reservations now.</span>
            <Button
              size="sm"
              icon={<RefreshCw />}
              onClick={() =>
                void (async () => {
                  try {
                    const res = await api<{ status: string; pools: number; reservations: number }>(
                      "/api/v1/integrations/kea/sync",
                      { method: "POST" },
                    );
                    toast(`Kea sync ${res.status}: ${res.pools} pools, ${res.reservations} reservations`, "success");
                  } catch (err) {
                    toast(err instanceof Error ? err.message : "Kea sync failed", "error");
                  }
                })()
              }
            >
              Sync now
            </Button>
          </>
        }
      >
        <div className="grid gap-4 sm:grid-cols-6">
          <Field label="Control Agent URL" className="sm:col-span-4">
            <input
              className="wt-input w-full font-mono text-xs"
              value={keaURL}
              onChange={(e) => setKeaURL(e.target.value)}
              placeholder="http://kea:8000"
            />
          </Field>
          <div className="flex items-end pb-2 sm:col-span-2">
            <DryRunToggle checked={keaDryRun} onChange={setKeaDryRun} label="Dry run" />
          </div>
        </div>
      </IntegrationBlock>

      <IntegrationBlock
        icon={<Router className="h-4 w-4" />}
        title="SNMP"
        description="Enrich IPs with switch port and MAC data from ARP/bridge tables."
        enabled={snmpEnabled}
        onEnabledChange={setSnmpEnabled}
        badges={<Badge className="font-mono">v{snmpVersion}</Badge>}
        footer={
          <div className="flex w-full flex-wrap items-end gap-2">
            <Field label="Enrich target" className="min-w-[14rem] flex-1">
              <input
                className="wt-input w-full font-mono text-xs"
                value={snmpTarget}
                onChange={(e) => setSnmpTarget(e.target.value)}
                placeholder="switch.example.com"
              />
            </Field>
            <Button
              icon={<Sparkles />}
              onClick={() =>
                void (async () => {
                  try {
                    const res = await api<{ switch_id: string; ips_updated: number }>(
                      "/api/v1/scanner/snmp-enrich",
                      { method: "POST", body: JSON.stringify({ target: snmpTarget }) },
                    );
                    toast(`SNMP enrich ${res.switch_id}: ${res.ips_updated} IP(s) updated`, "success");
                  } catch (err) {
                    toast(err instanceof Error ? err.message : "SNMP enrich failed", "error");
                  }
                })()
              }
            >
              Enrich now
            </Button>
          </div>
        }
      >
        <div className="grid gap-4 sm:grid-cols-6">
          <Field
            label="Community"
            hint={snmpCommunity === "" ? "Leave blank to keep the current value" : undefined}
            className="sm:col-span-3"
          >
            <input
              className="wt-input w-full font-mono text-xs"
              value={snmpCommunity}
              placeholder="••••••••"
              onChange={(e) => setSnmpCommunity(e.target.value)}
            />
          </Field>
          <Field label="Version" className="sm:col-span-1">
            <select
              className="wt-input w-full"
              value={snmpVersion}
              onChange={(e) => setSnmpVersion(e.target.value)}
            >
              <option value="2c">2c</option>
              <option value="3">3</option>
            </select>
          </Field>
          <Field label="Timeout (s)" className="sm:col-span-2">
            <input
              type="number"
              min={1}
              max={120}
              className="wt-input w-full font-mono"
              value={snmpTimeout}
              onChange={(e) => setSnmpTimeout(Number(e.target.value))}
            />
          </Field>
        </div>
      </IntegrationBlock>

      <div className="flex justify-end border-t border-[var(--color-border)] pt-4">
        <Button
          variant="primary"
          icon={<Save />}
          onClick={() =>
            void (async () => {
              try {
                const body: Record<string, unknown> = {
                  powerdns_enabled: pdnsEnabled,
                  powerdns_url: pdnsURL,
                  powerdns_dry_run: pdnsDryRun,
                  powerdns_server: pdnsServer,
                  kea_enabled: keaEnabled,
                  kea_url: keaURL,
                  kea_dry_run: keaDryRun,
                  snmp_enabled: snmpEnabled,
                  snmp_version: snmpVersion,
                  snmp_timeout_seconds: snmpTimeout,
                };
                if (pdnsAPIKey) body.powerdns_api_key = pdnsAPIKey;
                if (snmpCommunity) body.snmp_community = snmpCommunity;
                await api("/api/v1/settings", { method: "PUT", body: JSON.stringify(body) });
                setPdnsAPIKey("");
                toast("Integration settings saved", "success");
                markSettingsSaved();
              } catch (err) {
                toast(err instanceof Error ? err.message : "Settings update failed", "error");
              }
            })()
          }
        >
          Save integrations
        </Button>
      </div>
    </div>
  );
}
