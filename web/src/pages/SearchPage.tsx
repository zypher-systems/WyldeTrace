import { GitBranch, History, MapPin, Search, SearchX, Server } from "lucide-react";
import { Children, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { Link, useNavigate, useNavigationType, useSearchParams } from "react-router-dom";
import { api, type SearchResponse } from "../api";
import { PageHeader } from "../components/PageHeader";
import { Button, StatusBadge } from "../components/ui";

const RECENT_KEY = "wyldetrace_search_recent";

function loadRecent(): string[] {
  try {
    const raw = localStorage.getItem(RECENT_KEY);
    const parsed = raw ? (JSON.parse(raw) as string[]) : [];
    return Array.isArray(parsed) ? parsed.filter((x) => typeof x === "string").slice(0, 8) : [];
  } catch {
    return [];
  }
}

function pushRecent(q: string) {
  const next = [q, ...loadRecent().filter((x) => x.toLowerCase() !== q.toLowerCase())].slice(0, 8);
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(next));
  } catch {
    /* storage unavailable */
  }
  return next;
}

export function SearchPage() {
  const navigate = useNavigate();
  const navigationType = useNavigationType();
  const [searchParams, setSearchParams] = useSearchParams();
  const initialQ = searchParams.get("q") || "";
  const [q, setQ] = useState(initialQ);
  const [result, setResult] = useState<SearchResponse | null>(null);
  const [recent, setRecent] = useState<string[]>(() => loadRecent());
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const lastRun = useRef<string | null>(null);

  async function runSearch(query: string, opts: { fromUrl?: boolean } = {}) {
    const trimmed = query.trim();
    if (!trimmed) return;
    lastRun.current = trimmed;
    setBusy(true);
    setError("");
    if (!opts.fromUrl) {
      const sp = new URLSearchParams(searchParams);
      sp.set("q", trimmed);
      setSearchParams(sp, { replace: true });
    }
    try {
      const res = await api<SearchResponse>(`/api/v1/search?q=${encodeURIComponent(trimmed)}`);
      setResult(res);
      setRecent(pushRecent(trimmed));
      // Jump when there's a single unambiguous hit. When arriving via a ?q= link, replace the
      // search entry so Back doesn't bounce; skip the jump when returning via Back/Forward.
      if (opts.fromUrl && navigationType === "POP") return;
      const navOpts = opts.fromUrl ? { replace: true } : undefined;
      if (res.ips.length === 1 && res.subnets.length === 0) {
        navigate(`/subnets/${res.ips[0].subnet_id}?q=${encodeURIComponent(res.ips[0].address)}`, navOpts);
        return;
      }
      if (res.subnets.length === 1 && res.ips.length === 0) {
        navigate(`/subnets/${res.subnets[0].id}`, navOpts);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Search failed");
    } finally {
      setBusy(false);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    await runSearch(q);
  }

  useEffect(() => {
    setRecent(loadRecent());
  }, []);

  // Run the ?q= query on mount, and again if a link (e.g. the ⌘K palette) changes it.
  useEffect(() => {
    const t = initialQ.trim();
    if (!t || t === lastRun.current) return;
    setQ(initialQ);
    void runSearch(initialQ, { fromUrl: true });
  }, [initialQ]);

  const totalHits = result ? result.subnets.length + result.ips.length + result.locations.length : 0;

  return (
    <div className="space-y-6">
      <PageHeader title="Search" subtitle="Find subnets, IP addresses, hostnames, and sites" />

      <form onSubmit={onSubmit} className="flex flex-wrap gap-2">
        <div className="relative min-w-[240px] flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
          <input
            className="wt-input h-11 w-full pl-10 text-[15px]"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="10.0.0, router, HQ…"
            aria-label="Search query"
            autoFocus
          />
        </div>
        <Button type="submit" variant="primary" disabled={busy} className="h-11 disabled:opacity-60">
          {busy ? "Searching…" : "Search"}
        </Button>
      </form>

      {recent.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 text-sm">
          <span className="mr-1 inline-flex items-center gap-1.5 text-xs text-zinc-500">
            <History className="h-3.5 w-3.5" />
            Recent:
          </span>
          {recent.map((r) => (
            <button
              key={r}
              type="button"
              className="wt-badge cursor-pointer font-mono hover:border-brand/50 hover:text-brand"
              onClick={() => {
                setQ(r);
                void runSearch(r);
              }}
            >
              {r}
            </button>
          ))}
        </div>
      )}

      {error && <p className="text-sm text-red-400">{error}</p>}

      {result && totalHits === 0 && (
        <div className="wt-card px-6 py-12 text-center">
          <SearchX className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">No matches for “{result.query}”</p>
          <p className="mt-1 text-xs text-zinc-500">
            Try part of an address (10.0.0), a hostname, or a site name.
          </p>
        </div>
      )}

      {result && totalHits > 0 && (
        <div className="grid items-start gap-6 lg:grid-cols-3">
          <ResultCard icon={<GitBranch className="h-4 w-4" />} title="Subnets" count={result.subnets.length}>
            {result.subnets.map((s) => (
              <ResultRow key={s.id} to={`/subnets/${s.id}`}>
                <span className="font-mono text-zinc-50 group-hover:text-brand">{s.cidr}</span>
                <span className="truncate text-xs text-zinc-500">{s.name || "Unnamed"}</span>
              </ResultRow>
            ))}
          </ResultCard>
          <ResultCard icon={<Server className="h-4 w-4" />} title="IP addresses" count={result.ips.length}>
            {result.ips.map((ip) => (
              <ResultRow
                key={ip.id}
                to={`/subnets/${ip.subnet_id}?q=${encodeURIComponent(ip.address)}`}
                trailing={<StatusBadge status={ip.status} />}
              >
                <span className="font-mono text-zinc-50 group-hover:text-brand">{ip.address}</span>
                <span className="truncate text-xs text-zinc-500">{ip.hostname || "—"}</span>
              </ResultRow>
            ))}
          </ResultCard>
          <ResultCard icon={<MapPin className="h-4 w-4" />} title="Sites" count={result.locations.length}>
            {result.locations.map((l) => (
              <ResultRow key={l.id} to={`/locations/${l.id}`}>
                <span className="text-zinc-50 group-hover:text-brand">{l.name}</span>
                <span className="truncate text-xs text-zinc-500">{l.description || "—"}</span>
              </ResultRow>
            ))}
          </ResultCard>
        </div>
      )}

      {!result && !busy && !error && (
        <p className="text-xs text-zinc-500">
          Tip: a query that matches exactly one subnet or address opens it directly.
        </p>
      )}
    </div>
  );
}

function ResultCard({
  icon,
  title,
  count,
  children,
}: {
  icon: ReactNode;
  title: string;
  count: number;
  children: ReactNode;
}) {
  const [expanded, setExpanded] = useState(false);
  const rows = Children.toArray(children);
  const limit = 12;
  const shown = expanded ? rows : rows.slice(0, limit);
  return (
    <section className="wt-card flex flex-col">
      <header className="flex items-center justify-between gap-3 border-b border-[var(--color-border)] px-5 py-3.5">
        <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-100">
          <span className="text-zinc-500">{icon}</span>
          {title}
        </h2>
        <span className="wt-badge h-5 px-1.5 tabular-nums">{count}</span>
      </header>
      {count === 0 ? (
        <p className="px-5 py-6 text-sm text-zinc-500">No matches</p>
      ) : (
        <>
          <ul className="divide-y divide-[var(--color-border)]">{shown}</ul>
          {rows.length > limit && (
            <button
              type="button"
              className="border-t border-[var(--color-border)] px-5 py-2.5 text-left text-xs text-zinc-400 hover:text-brand"
              onClick={() => setExpanded((v) => !v)}
            >
              {expanded ? "Show fewer" : `Show all ${rows.length}`}
            </button>
          )}
        </>
      )}
    </section>
  );
}

function ResultRow({ to, children, trailing }: { to: string; children: ReactNode; trailing?: ReactNode }) {
  return (
    <li>
      <Link
        to={to}
        className="group flex items-center justify-between gap-3 px-5 py-2.5 text-sm transition hover:bg-[var(--color-surface-hover)]"
      >
        <span className="flex min-w-0 flex-col">{children}</span>
        {trailing}
      </Link>
    </li>
  );
}
