import {
  AlertTriangle,
  ArrowRight,
  Bookmark,
  Building2,
  CheckCircle2,
  Download,
  GitBranch,
  Link2,
  Radar,
  Send,
  Server,
  Trash2,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api, downloadCSV, type Location } from "../api";
import { useAuth } from "../auth";
import { DropdownMenu, type DropdownItem } from "../components/DropdownMenu";
import { FormDialog } from "../components/FormDialog";
import { GettingStarted } from "../components/GettingStarted";
import { PageHeader } from "../components/PageHeader";
import { useToast } from "../components/Toast";
import { Badge, Button, Card, Meter, Stat, cx } from "../components/ui";
import { relativeTime } from "../lib/time";

const SAVED_VIEWS_KEY = "wyldetrace_report_views";
const LAST_EXPORT_KEY = "wyldetrace_reports_last_export";

type SavedView = { name: string; location_id: string };

type ReportsSummary = {
  generated_at: string;
  site_scoped: boolean;
  site_count: number;
  subnets: { total: number; with_location: number };
  ips: { total: number; by_status: Record<string, number>; with_mac: number };
  utilization_top: {
    id: string;
    cidr: string;
    name: string;
    used: number;
    total: number;
    free: number;
    percent: number;
  }[];
  by_site: {
    location_id: string | null;
    name: string;
    subnets: number;
    used: number;
    total: number;
    free: number;
    percent: number;
  }[];
  discovery: {
    jobs: number;
    enabled: number;
    recent_ok: number;
    recent_error: number;
    last_run_at?: string | null;
    recent: {
      id: string;
      name: string;
      enabled: boolean;
      last_run_at: string | null;
      last_status: string;
      alive?: number;
      new_alive?: number;
    }[];
  };
  inbox: {
    open_total: number;
    open_new_alive: number;
    open_stale: number;
    last_seen_at?: string | null;
  };
};

function loadSavedViews(): SavedView[] {
  try {
    const raw = localStorage.getItem(SAVED_VIEWS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as SavedView[];
    return Array.isArray(parsed) ? parsed.filter((v) => v?.name) : [];
  } catch {
    return [];
  }
}

export function OpsDashboardPage() {
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const canOpenSubnets = user?.role !== "reportview";
  const isAdmin = user?.role === "admin";
  const locationFilter = searchParams.get("location_id") || "";
  const [data, setData] = useState<ReportsSummary | null>(null);
  const [locations, setLocations] = useState<Location[]>([]);
  const [error, setError] = useState("");
  const toast = useToast();
  const [viewsOpen, setViewsOpen] = useState(false);
  const [savedViews, setSavedViews] = useState<SavedView[]>(() => loadSavedViews());
  const [viewName, setViewName] = useState("");
  const [lastExportedAt, setLastExportedAt] = useState(() => {
    try {
      return localStorage.getItem(LAST_EXPORT_KEY) || "";
    } catch {
      return "";
    }
  });

  const summaryPath = useMemo(() => {
    const q = locationFilter ? `?location_id=${encodeURIComponent(locationFilter)}` : "";
    return `/api/v1/reports/summary${q}`;
  }, [locationFilter]);

  const csvPath = useMemo(() => {
    const q = locationFilter ? `?location_id=${encodeURIComponent(locationFilter)}` : "";
    return `/api/v1/reports/summary.csv${q}`;
  }, [locationFilter]);

  useEffect(() => {
    void (async () => {
      try {
        setData(await api<ReportsSummary>(summaryPath));
        setError("");
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load dashboard");
      }
    })();
  }, [summaryPath]);

  useEffect(() => {
    if (user?.role === "reportview") return;
    void api<Location[]>("/api/v1/locations")
      .then(setLocations)
      .catch(() => setLocations([]));
  }, [user?.role]);

  const [siteOptions, setSiteOptions] = useState<{ id: string; name: string }[]>([]);

  useEffect(() => {
    if (locations.length) {
      setSiteOptions(locations.map((l) => ({ id: l.id, name: l.name })));
      return;
    }
    if (!data?.by_site?.length) return;
    setSiteOptions((prev) => {
      const map = new Map(prev.map((s) => [s.id, s.name]));
      for (const s of data.by_site) {
        if (s.location_id) map.set(s.location_id, s.name);
      }
      return [...map.entries()].map(([id, name]) => ({ id, name }));
    });
  }, [locations, data]);

  function setLocationFilter(id: string) {
    if (id) setSearchParams({ location_id: id }, { replace: true });
    else setSearchParams({}, { replace: true });
  }

  function saveCurrentView() {
    const name = viewName.trim();
    if (!name) return;
    const next = [
      { name, location_id: locationFilter },
      ...savedViews.filter((v) => v.name.toLowerCase() !== name.toLowerCase()),
    ].slice(0, 12);
    localStorage.setItem(SAVED_VIEWS_KEY, JSON.stringify(next));
    setSavedViews(next);
    setViewName("");
    toast(`Saved view “${name}”`, "success");
  }

  function deleteView(name: string) {
    const next = savedViews.filter((v) => v.name !== name);
    localStorage.setItem(SAVED_VIEWS_KEY, JSON.stringify(next));
    setSavedViews(next);
  }

  function viewShareURL(locationId: string) {
    const url = new URL(window.location.href);
    url.search = "";
    if (locationId) url.searchParams.set("location_id", locationId);
    return url.toString();
  }

  async function copyShareLink(locationId = locationFilter) {
    try {
      await navigator.clipboard.writeText(viewShareURL(locationId));
      toast("Share link copied", "success");
    } catch {
      toast("Could not copy link — copy from the address bar instead", "error");
    }
  }

  async function exportCSV() {
    try {
      await downloadCSV(
        csvPath,
        locationFilter || data?.site_scoped
          ? "wyldetrace-reports-scoped.csv"
          : "wyldetrace-reports.csv",
      );
      const when = new Date().toISOString();
      localStorage.setItem(LAST_EXPORT_KEY, when);
      setLastExportedAt(when);
      toast("CSV exported", "success");
    } catch (e) {
      toast(e instanceof Error ? e.message : "Export failed", "error");
    }
  }

  async function dispatchReport() {
    try {
      await api("/api/v1/reports/summary/dispatch", { method: "POST" });
      toast("Dispatched reports.summary to matching webhooks", "success");
    } catch (e) {
      toast(e instanceof Error ? e.message : "Dispatch failed", "error");
    }
  }

  if (error && !data) {
    return (
      <Card className="flex items-center gap-3 p-5 text-sm text-red-300">
        <AlertTriangle className="h-4 w-4" /> {error}
      </Card>
    );
  }
  if (!data) {
    return (
      <div className="space-y-6" role="status" aria-label="Loading overview">
        <div className="h-8 w-48 animate-pulse rounded bg-[var(--color-surface-overlay)]" />
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="wt-card h-28 animate-pulse" />
          ))}
        </div>
        <div className="wt-card h-72 animate-pulse" />
      </div>
    );
  }

  const byStatus = data.ips.by_status || {};
  const statusOrder = ["assigned", "dhcp", "reserved", "available"];
  const statusColor: Record<string, string> = {
    assigned: "var(--color-status-assigned)",
    dhcp: "var(--color-status-dhcp)",
    reserved: "var(--color-status-reserved)",
    available: "var(--color-zinc-500)",
  };
  const statusEntries = [
    ...statusOrder.filter((k) => byStatus[k]).map((k) => [k, byStatus[k]] as const),
    ...Object.entries(byStatus).filter(([k]) => !statusOrder.includes(k)),
  ];
  const statusTotal = statusEntries.reduce((n, [, v]) => n + v, 0) || 1;
  const filteredLabel = locationFilter
    ? siteOptions.find((s) => s.id === locationFilter)?.name || "selected site"
    : "";
  const hot = data.utilization_top.filter((u) => u.percent >= 75);
  const attention = [
    data.inbox.open_new_alive > 0 && {
      key: "new",
      icon: <Radar className="h-4 w-4 text-brand" />,
      title: `${data.inbox.open_new_alive} new host${data.inbox.open_new_alive === 1 ? "" : "s"} discovered`,
      detail: "Claim them into inventory or dismiss",
      to: "/inbox",
    },
    data.inbox.open_stale > 0 && {
      key: "stale",
      icon: <AlertTriangle className="h-4 w-4 text-amber-300" />,
      title: `${data.inbox.open_stale} stale assignment${data.inbox.open_stale === 1 ? "" : "s"}`,
      detail: "Recorded as assigned but not responding",
      to: "/inbox",
    },
    data.discovery.recent_error > 0 && {
      key: "err",
      icon: <AlertTriangle className="h-4 w-4 text-red-300" />,
      title: `${data.discovery.recent_error} discovery job${data.discovery.recent_error === 1 ? "" : "s"} failing`,
      detail: "Check scan job configuration",
      to: "/discovery",
    },
    ...hot
      .filter((u) => u.percent >= 90)
      .slice(0, 3)
      .map((u) => ({
        key: `hot-${u.id}`,
        icon: <GitBranch className="h-4 w-4 text-red-300" />,
        title: `${u.cidr} is ${Math.round(u.percent)}% full`,
        detail: `${u.free} addresses left${u.name ? ` · ${u.name}` : ""}`,
        to: `/subnets/${u.id}`,
      })),
  ].filter(Boolean) as Array<{ key: string; icon: React.ReactNode; title: string; detail: string; to: string }>;

  const viewItems: DropdownItem[] = [
    ...savedViews.map((v) => ({
      label: v.name,
      icon: <Bookmark />,
      onClick: () => setLocationFilter(v.location_id),
    })),
    {
      label: savedViews.length ? "Manage views…" : "Save this view…",
      separated: savedViews.length > 0,
      onClick: () => setViewsOpen(true),
    },
  ];
  const moreItems: DropdownItem[] = [
    {
      label: `Export CSV${locationFilter || data.site_scoped ? " (scoped)" : ""}`,
      icon: <Download />,
      onClick: () => void exportCSV(),
    },
    { label: "Copy share link", icon: <Link2 />, onClick: () => void copyShareLink() },
    ...(isAdmin
      ? [{ label: "Send summary to webhooks", icon: <Send />, onClick: () => void dispatchReport() }]
      : []),
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Overview"
        subtitle={`Updated ${relativeTime(data.generated_at)}${
          data.site_scoped
            ? ` · scoped to ${data.site_count} site${data.site_count === 1 ? "" : "s"}`
            : ""
        }${filteredLabel ? ` · ${filteredLabel}` : ""}${
          lastExportedAt ? ` · last export ${relativeTime(lastExportedAt)}` : ""
        }`}
        actions={
          <>
            <div className="relative">
              <Building2 className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
              <select
                className="wt-input min-w-[11rem] pl-9"
                aria-label="Site filter"
                value={locationFilter}
                onChange={(e) => setLocationFilter(e.target.value)}
              >
                <option value="">All sites</option>
                {siteOptions.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </div>
            <DropdownMenu label="Views" icon={<Bookmark />} items={viewItems} />
            <DropdownMenu label="More actions" iconOnly items={moreItems} />
          </>
        }
      />

      {user?.role === "reportview" && (
        <Card className="px-4 py-3 text-sm text-zinc-400">
          Report-only view — inventory editing and discovery are not available for this account.
        </Card>
      )}

      {canOpenSubnets && (
        <GettingStarted
          hasSubnets={(data.subnets?.total ?? 0) > 0}
          hasDiscoveryRuns={!!data.discovery?.last_run_at || (data.discovery?.recent_ok ?? 0) > 0}
          inboxOpen={data.inbox?.open_total ?? 0}
          usingDefaultPassword={!!user?.using_default_password}
        />
      )}

      {/* KPIs */}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard to={canOpenSubnets ? "/subnets" : undefined}>
          <Stat
            icon={<GitBranch className="h-3.5 w-3.5" />}
            label="Subnets"
            value={data.subnets.total}
            hint={`${data.subnets.with_location} assigned to a site`}
          />
        </KpiCard>
        <KpiCard>
          <Stat
            icon={<Server className="h-3.5 w-3.5" />}
            label="IP records"
            value={data.ips.total}
            hint={`${data.ips.with_mac} with a MAC address`}
          />
          <div className="mt-3 flex h-1.5 overflow-hidden rounded-full bg-[var(--color-surface-overlay)]">
            {statusEntries.map(([k, v]) => (
              <span
                key={k}
                title={`${k}: ${v}`}
                style={{ width: `${(v / statusTotal) * 100}%`, background: statusColor[k] ?? "#6b7280" }}
              />
            ))}
          </div>
        </KpiCard>
        <KpiCard to={canOpenSubnets ? "/inbox" : undefined}>
          <Stat
            icon={<Radar className="h-3.5 w-3.5" />}
            label="Discovery inbox"
            value={data.inbox?.open_total ?? 0}
            hint={
              (data.inbox?.open_total ?? 0) === 0
                ? "All findings reviewed"
                : `${data.inbox.open_new_alive} new · ${data.inbox.open_stale} stale`
            }
          />
        </KpiCard>
        <KpiCard to={canOpenSubnets ? "/discovery" : undefined}>
          <Stat
            icon={<CheckCircle2 className="h-3.5 w-3.5" />}
            label="Last scan"
            value={
              data.discovery.last_run_at
                ? relativeTime(data.discovery.last_run_at)
                : data.inbox?.last_seen_at
                  ? relativeTime(data.inbox.last_seen_at)
                  : "Never"
            }
            hint={`${data.discovery.enabled} of ${data.discovery.jobs} jobs enabled · ${data.discovery.recent_error} failing`}
          />
        </KpiCard>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Utilization pressure */}
        <Card className="lg:col-span-2">
          <CardHeader
            title="Busiest subnets"
            subtitle="Highest utilization first"
            link={canOpenSubnets ? { to: "/subnets", label: "All subnets" } : undefined}
          />
          <ul className="divide-y divide-[var(--color-border)]">
            {data.utilization_top.slice(0, 8).map((u) => {
              const row = (
                <div className="grid grid-cols-[minmax(0,1fr)_7rem] items-center gap-4 px-5 py-3 sm:grid-cols-[10rem_minmax(0,1fr)_12rem]">
                  <div className="min-w-0">
                    <div className="font-mono text-sm text-zinc-100">{u.cidr}</div>
                    <div className="truncate text-xs text-zinc-500 sm:hidden">{u.name}</div>
                  </div>
                  <div className="hidden truncate text-sm text-zinc-400 sm:block">{u.name}</div>
                  <div className="flex items-center gap-3">
                    <Meter value={u.percent} className="flex-1" />
                    <span className="w-10 text-right text-xs tabular-nums text-zinc-300">
                      {Math.round(u.percent)}%
                    </span>
                  </div>
                </div>
              );
              return (
                <li key={u.id}>
                  {canOpenSubnets ? (
                    <Link to={`/subnets/${u.id}`} className="block transition hover:bg-white/[0.02]">
                      {row}
                    </Link>
                  ) : (
                    row
                  )}
                </li>
              );
            })}
            {data.utilization_top.length === 0 && (
              <li className="px-5 py-10 text-center text-sm text-zinc-500">No subnets yet</li>
            )}
          </ul>
        </Card>

        {/* Needs attention */}
        <Card>
          <CardHeader title="Needs attention" subtitle={attention.length ? `${attention.length} item${attention.length === 1 ? "" : "s"}` : undefined} />
          {attention.length === 0 ? (
            <div className="px-5 pb-8 pt-4 text-center">
              <CheckCircle2 className="mx-auto h-7 w-7 text-brand/70" />
              <p className="mt-2 text-sm text-zinc-300">Nothing needs you right now</p>
              <p className="mt-1 text-xs text-zinc-500">
                {data.discovery.last_run_at || !canOpenSubnets
                  ? "Inbox is clear and no subnet is over 90%."
                  : "Schedule a scan job to start finding hosts."}
              </p>
              {!data.discovery.last_run_at && canOpenSubnets && (
                <Link to="/discovery" className="wt-btn wt-btn-secondary mt-4">
                  Set up discovery
                </Link>
              )}
            </div>
          ) : (
            <ul className="space-y-1 px-2 pb-2">
              {attention.map((a) => (
                <li key={a.key}>
                  <Link
                    to={canOpenSubnets ? a.to : "#"}
                    className="group flex items-start gap-3 rounded-lg px-3 py-2.5 transition hover:bg-[var(--color-surface-overlay)]"
                  >
                    <span className="mt-0.5">{a.icon}</span>
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm text-zinc-100">{a.title}</span>
                      <span className="block text-xs text-zinc-500">{a.detail}</span>
                    </span>
                    <ArrowRight className="mt-1 h-3.5 w-3.5 text-zinc-600 transition group-hover:translate-x-0.5 group-hover:text-zinc-300" />
                  </Link>
                </li>
              ))}
            </ul>
          )}
          <div className="border-t border-[var(--color-border)] px-5 py-4">
            <div className="wt-eyebrow mb-2.5">Address mix</div>
            <ul className="space-y-1.5">
              {statusEntries.map(([k, v]) => (
                <li key={k} className="flex items-center gap-2 text-xs">
                  <span className="wt-status-dot" style={{ background: statusColor[k] ?? "#6b7280" }} />
                  <span className="flex-1 text-zinc-400">{k}</span>
                  <span className="tabular-nums text-zinc-200">{v}</span>
                  <span className="w-10 text-right tabular-nums text-zinc-500">
                    {Math.round((v / statusTotal) * 100)}%
                  </span>
                </li>
              ))}
              {statusEntries.length === 0 && <li className="text-xs text-zinc-500">No addresses yet</li>}
            </ul>
          </div>
        </Card>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader
            title="Sites"
            subtitle="Utilization across each site's subnets"
            link={canOpenSubnets ? { to: "/locations", label: "All sites" } : undefined}
          />
          <ul className="divide-y divide-[var(--color-border)]">
            {(data.by_site ?? []).map((s) => (
              <li key={s.location_id || "unassigned"} className="flex items-center gap-4 px-5 py-3">
                <div className="min-w-0 flex-1">
                  {canOpenSubnets && s.location_id ? (
                    <Link className="text-sm text-zinc-100 hover:text-brand" to={`/locations/${s.location_id}`}>
                      {s.name}
                    </Link>
                  ) : (
                    <span className="text-sm text-zinc-300">{s.name}</span>
                  )}
                  <div className="text-xs text-zinc-500 tabular-nums">
                    {s.subnets} subnet{s.subnets === 1 ? "" : "s"} · {s.used.toLocaleString()} /{" "}
                    {s.total.toLocaleString()} used
                  </div>
                </div>
                <Meter value={s.percent} className="w-28" />
                <span className="w-10 text-right text-xs tabular-nums text-zinc-300">
                  {Math.round(s.percent)}%
                </span>
              </li>
            ))}
            {(data.by_site ?? []).length === 0 && (
              <li className="px-5 py-10 text-center text-sm text-zinc-500">No site data</li>
            )}
          </ul>
        </Card>

        <Card>
          <CardHeader
            title="Recent scans"
            subtitle="Latest discovery job runs"
            link={canOpenSubnets ? { to: "/discovery", label: "Scan jobs" } : undefined}
          />
          <ul className="divide-y divide-[var(--color-border)]">
            {data.discovery.recent.map((j) => (
              <li key={j.id} className="flex items-center gap-3 px-5 py-3">
                <span
                  className={cx(
                    "h-2 w-2 shrink-0 rounded-full",
                    !j.last_status
                      ? "bg-zinc-600"
                      : /err|fail/i.test(j.last_status)
                        ? "bg-red-400"
                        : "bg-brand",
                  )}
                />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm text-zinc-100">{j.name}</div>
                  <div className="text-xs text-zinc-500">
                    {j.last_run_at ? relativeTime(j.last_run_at) : "Never run"}
                    {!j.enabled && " · disabled"}
                  </div>
                </div>
                {j.alive != null && (
                  <span className="text-xs tabular-nums text-zinc-400">
                    {j.alive} alive
                    {j.new_alive ? <Badge tone="brand" className="ml-2">{j.new_alive} new</Badge> : null}
                  </span>
                )}
              </li>
            ))}
            {data.discovery.recent.length === 0 && (
              <li className="px-5 py-10 text-center text-sm text-zinc-500">
                No scan jobs yet.{" "}
                {canOpenSubnets && (
                  <Link to="/discovery" className="text-brand hover:underline">
                    Create one
                  </Link>
                )}
              </li>
            )}
          </ul>
        </Card>
      </div>

      <FormDialog open={viewsOpen} title="Saved views" onClose={() => setViewsOpen(false)}>
        <div className="space-y-5">
          <form
            className="flex items-end gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              saveCurrentView();
            }}
          >
            <label className="flex-1 space-y-1.5">
              <span className="wt-label">Save current filter as</span>
              <input
                className="wt-input w-full"
                value={viewName}
                onChange={(e) => setViewName(e.target.value)}
                placeholder={filteredLabel ? `${filteredLabel} overview` : "e.g. HQ utilization"}
                autoFocus
              />
            </label>
            <Button type="submit" variant="primary" disabled={!viewName.trim()}>
              Save
            </Button>
          </form>
          {savedViews.length > 0 && (
            <ul className="divide-y divide-[var(--color-border)] rounded-xl border border-[var(--color-border)]">
              {savedViews.map((v) => (
                <li key={v.name} className="flex items-center gap-2 px-3 py-2">
                  <button
                    type="button"
                    className="flex-1 truncate text-left text-sm text-zinc-200 hover:text-brand"
                    onClick={() => {
                      setLocationFilter(v.location_id);
                      setViewsOpen(false);
                    }}
                  >
                    {v.name}
                  </button>
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                    title="Copy share link"
                    aria-label={`Copy share link for ${v.name}`}
                    onClick={() => void copyShareLink(v.location_id)}
                  >
                    <Link2 />
                  </button>
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                    title="Remove"
                    aria-label={`Remove ${v.name}`}
                    onClick={() => deleteView(v.name)}
                  >
                    <Trash2 />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </FormDialog>
    </div>
  );
}

function KpiCard({ to, children }: { to?: string; children: React.ReactNode }) {
  const cls = "wt-card block p-5";
  return to ? (
    <Link to={to} className={cx(cls, "wt-card-interactive")}>
      {children}
    </Link>
  ) : (
    <div className={cls}>{children}</div>
  );
}

function CardHeader({
  title,
  subtitle,
  link,
}: {
  title: string;
  subtitle?: string;
  link?: { to: string; label: string };
}) {
  return (
    <div className="flex items-start justify-between gap-3 px-5 pb-3 pt-4">
      <div>
        <h2 className="text-sm font-semibold text-zinc-100">{title}</h2>
        {subtitle && <p className="mt-0.5 text-xs text-zinc-500">{subtitle}</p>}
      </div>
      {link && (
        <Link to={link.to} className="inline-flex items-center gap-1 text-xs text-zinc-400 hover:text-brand">
          {link.label} <ArrowRight className="h-3 w-3" />
        </Link>
      )}
    </div>
  );
}

