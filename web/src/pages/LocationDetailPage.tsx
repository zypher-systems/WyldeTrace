import { ChevronRight, GitBranch, Layers, MapPin, Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, type Location, type Subnet } from "../api";
import { Skeleton } from "../components/Skeleton";
import { Badge, Meter } from "../components/ui";

type LocationDetail = {
  location: Location;
  subnets: Subnet[];
  utilization: {
    used: number;
    total: number;
    free: number;
    percent: number;
    subnet_count: number;
  };
};

export function LocationDetailPage() {
  const { id } = useParams();
  const [data, setData] = useState<LocationDetail | null>(null);
  const [error, setError] = useState("");
  const [filter, setFilter] = useState("");

  useEffect(() => {
    if (!id) return;
    void (async () => {
      try {
        setData(await api<LocationDetail>(`/api/v1/locations/${id}`));
        setError("");
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load site");
      }
    })();
  }, [id]);

  const vlanCount = useMemo(
    () => new Set((data?.subnets ?? []).map((s) => s.vlan_vid).filter((v) => v != null)).size,
    [data],
  );

  const visible = useMemo(() => {
    const list = data?.subnets ?? [];
    const q = filter.trim().toLowerCase();
    if (!q) return list;
    return list.filter(
      (s) =>
        s.cidr.toLowerCase().includes(q) ||
        (s.name || "").toLowerCase().includes(q) ||
        (s.tags ?? []).some((t) => t.toLowerCase().includes(q)) ||
        (s.vlan_vid != null && String(s.vlan_vid).includes(q)),
    );
  }, [data, filter]);

  const breadcrumb = (
    <nav className="flex items-center gap-1.5 text-xs text-zinc-500" aria-label="Breadcrumb">
      <Link to="/locations" className="hover:text-zinc-200">
        Sites
      </Link>
      <ChevronRight className="h-3 w-3" />
      <span className="text-zinc-300">{data?.location.name ?? "…"}</span>
    </nav>
  );

  if (error) {
    return (
      <div className="space-y-5">
        {breadcrumb}
        <div className="wt-card px-6 py-10 text-center">
          <MapPin className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">Could not load this site</p>
          <p className="mt-1 text-xs text-red-400">{error}</p>
          <Link to="/locations" className="wt-btn wt-btn-secondary mt-4 inline-flex">
            Back to sites
          </Link>
        </div>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="space-y-5">
        {breadcrumb}
        <Skeleton className="h-10 w-72" />
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  const util = data.utilization;
  const pct = Math.min(100, Math.round(util.percent));

  return (
    <div className="space-y-6">
      {/* ---- Header ---- */}
      <div className="space-y-5">
        {breadcrumb}

        <div className="min-w-0 wt-animate-in">
          <h1 className="text-3xl font-semibold tracking-tight text-zinc-50 md:text-[2.25rem]">
            {data.location.name}
          </h1>
          <p className="mt-1 text-[15px] text-zinc-400">
            {data.location.description || "Site inventory and utilization"}
          </p>
          <div className="mt-3 flex flex-wrap items-center gap-1.5">
            <Badge>
              <MapPin className="h-3 w-3" />
              Site
            </Badge>
            <Badge tone="brand">
              <GitBranch className="h-3 w-3" />
              {util.subnet_count} subnet{util.subnet_count === 1 ? "" : "s"}
            </Badge>
            {vlanCount > 0 && (
              <Badge tone="info">
                <Layers className="h-3 w-3" />
                {vlanCount} VLAN{vlanCount === 1 ? "" : "s"}
              </Badge>
            )}
          </div>
        </div>

        {/* ---- Stats strip ---- */}
        <div className="wt-card grid gap-px overflow-hidden bg-[var(--color-border)] sm:grid-cols-2 lg:grid-cols-4">
          <div className="bg-[var(--color-surface-raised)] p-5 lg:col-span-2">
            <span className="wt-eyebrow">Utilization</span>
            <div className="mt-2 flex items-baseline gap-2">
              <span className="text-3xl font-semibold tabular-nums tracking-tight text-zinc-50">
                {pct}%
              </span>
              <span className="text-sm tabular-nums text-zinc-500">
                {util.used} of {util.total} used across sized subnets
              </span>
            </div>
            <Meter value={pct} className="mt-3 h-2" />
          </div>
          <div className="bg-[var(--color-surface-raised)] p-5">
            <span className="wt-eyebrow">Free addresses</span>
            <div className="mt-2 text-lg font-semibold tabular-nums text-zinc-50">{util.free}</div>
            <p className="mt-1 text-xs text-zinc-500">Unassigned in this site's subnets</p>
          </div>
          <div className="bg-[var(--color-surface-raised)] p-5">
            <span className="wt-eyebrow">Subnets</span>
            <div className="mt-2 text-lg font-semibold tabular-nums text-zinc-50">
              {util.subnet_count}
            </div>
            <p className="mt-1 text-xs text-zinc-500">
              {vlanCount} VLAN{vlanCount === 1 ? "" : "s"} in use
            </p>
          </div>
        </div>
      </div>

      {/* ---- Subnets ---- */}
      <section className="wt-card">
        <header className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border)] px-5 py-4">
          <div>
            <h2 className="text-sm font-semibold text-zinc-100">Subnets</h2>
            <p className="mt-0.5 text-xs text-zinc-500">Prefixes assigned to this site</p>
          </div>
          {data.subnets.length > 0 && (
            <div className="relative w-full max-w-xs">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
              <input
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder="Filter subnets…"
                aria-label="Filter subnets"
                className="wt-input w-full pl-9"
              />
            </div>
          )}
        </header>
        {data.subnets.length === 0 ? (
          <div className="px-6 py-12 text-center">
            <GitBranch className="mx-auto h-6 w-6 text-zinc-600" />
            <p className="mt-2 text-sm text-zinc-300">No subnets at this site</p>
            <p className="mt-1 text-xs text-zinc-500">
              Assign a site when creating or editing a subnet.
            </p>
            <Link to="/subnets" className="wt-btn wt-btn-secondary mt-4 inline-flex">
              Go to subnets
            </Link>
          </div>
        ) : visible.length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-zinc-500">No subnets match “{filter}”.</p>
        ) : (
          <ul className="divide-y divide-[var(--color-border)]">
            {visible.map((s) => (
              <SubnetRow key={s.id} subnet={s} />
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function SubnetRow({ subnet: s }: { subnet: Subnet }) {
  const sized = s.total != null && !s.sparse;
  const pct = sized ? Math.min(100, Math.round(s.percent ?? 0)) : 0;
  return (
    <li className="group relative flex flex-wrap items-center gap-x-6 gap-y-2 px-5 py-3 transition hover:bg-[var(--color-surface-hover)]">
      <div className="min-w-0 flex-1">
        <Link
          to={`/subnets/${s.id}`}
          className="font-mono text-[15px] text-zinc-50 after:absolute after:inset-0 hover:text-brand"
        >
          {s.cidr}
        </Link>
        <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-zinc-500">
          <span className="truncate">{s.name || "Unnamed"}</span>
          {s.vlan_vid != null && (
            <Badge tone="info" className="h-5 text-[10px]">
              VLAN {s.vlan_vid}
            </Badge>
          )}
          {(s.tags ?? []).map((t) => (
            <Link
              key={t}
              to={`/subnets?tags=${encodeURIComponent(t)}`}
              className="relative z-10"
            >
              <Badge className="h-5 font-mono text-[10px] hover:border-brand/50 hover:text-brand">
                #{t}
              </Badge>
            </Link>
          ))}
        </div>
      </div>
      <div className="flex w-full items-center gap-3 sm:w-64" title={sized ? `${s.used}/${s.total} used` : undefined}>
        {s.used == null ? (
          <span className="text-xs text-zinc-600">—</span>
        ) : sized ? (
          <>
            <Meter value={pct} className="flex-1" />
            <span className="w-9 text-right text-xs tabular-nums text-zinc-300">{pct}%</span>
          </>
        ) : (
          <span className="text-xs text-zinc-400">
            {s.used} used <span className="text-zinc-600">(sparse)</span>
          </span>
        )}
      </div>
      <ChevronRight className="hidden h-4 w-4 text-zinc-600 transition group-hover:translate-x-0.5 group-hover:text-brand sm:block" />
    </li>
  );
}
