import { Activity, ArrowUpRight, Filter, Search, User } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { Link, Navigate, useSearchParams } from "react-router-dom";
import { api, pageQuery, type PageResult } from "../api";
import { useAuth } from "../auth";
import { PageHeader } from "../components/PageHeader";
import { Pagination } from "../components/Pagination";
import { TableSkeleton } from "../components/Skeleton";
import { Badge, Button, StatusBadge } from "../components/ui";
import { relativeTime } from "../lib/time";

type IPHistoryRow = {
  id: string;
  ip_id: string | null;
  address: string;
  subnet_id: string | null;
  hostname: string;
  status: string;
  mac_address: string;
  open_ports: string;
  action: string;
  actor_email: string;
  details: Record<string, unknown> | null;
  created_at: string;
};

function historyQuery(params: {
  page?: number;
  pageSize?: number;
  address?: string;
  action?: string;
  actor?: string;
  from?: string;
  to?: string;
  sort?: string;
  order?: "asc" | "desc";
}): string {
  const base = pageQuery({
    page: params.page,
    pageSize: params.pageSize,
    sort: params.sort,
    order: params.order,
  });
  const sp = new URLSearchParams(base.startsWith("?") ? base.slice(1) : base);
  if (params.address) sp.set("address", params.address);
  if (params.action) sp.set("action", params.action);
  if (params.actor) sp.set("actor", params.actor);
  if (params.from) sp.set("from", params.from);
  if (params.to) sp.set("to", params.to);
  const qs = sp.toString();
  return qs ? `?${qs}` : "";
}

export function IPHistoryPage() {
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const page = Math.max(1, Number(searchParams.get("page") || "1") || 1);
  const address = searchParams.get("address") || "";
  const action = searchParams.get("action") || "";
  const actor = searchParams.get("actor") || "";
  const from = searchParams.get("from") || "";
  const to = searchParams.get("to") || "";
  const sort = searchParams.get("sort") || "created_at";
  const order = (searchParams.get("order") === "asc" ? "asc" : "desc") as "asc" | "desc";

  const [draftAddress, setDraftAddress] = useState(address);
  const [draftAction, setDraftAction] = useState(action);
  const [draftActor, setDraftActor] = useState(actor);
  const [draftFrom, setDraftFrom] = useState(from);
  const [draftTo, setDraftTo] = useState(to);
  const [rows, setRows] = useState<IPHistoryRow[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const pageSize = 50;

  useEffect(() => {
    setDraftAddress(address);
    setDraftAction(action);
    setDraftActor(actor);
    setDraftFrom(from);
    setDraftTo(to);
  }, [address, action, actor, from, to]);

  useEffect(() => {
    if (user?.role === "reportview") return;
    void (async () => {
      setLoading(true);
      try {
        const res = await api<PageResult<IPHistoryRow>>(
          `/api/v1/ip-history${historyQuery({
            page,
            pageSize,
            address,
            action,
            actor,
            from,
            to,
            sort,
            order,
          })}`,
        );
        setRows(res.items);
        setTotal(res.total);
        setError("");
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load IP history");
      } finally {
        setLoading(false);
      }
    })();
  }, [user?.role, page, address, action, actor, from, to, sort, order]);

  if (user?.role === "reportview") return <Navigate to="/" replace />;

  const pageCount = Math.max(1, Math.ceil(total / pageSize) || 1);

  function patchParams(next: Record<string, string | null>, resetPage = false) {
    const sp = new URLSearchParams(searchParams);
    for (const [k, v] of Object.entries(next)) {
      if (v == null || v === "") sp.delete(k);
      else sp.set(k, v);
    }
    if (resetPage) sp.set("page", "1");
    setSearchParams(sp, { replace: true });
  }

  const hasFilters = !!(address || action || actor || from || to);

  return (
    <div className="space-y-6">
      <PageHeader
        title="IP history"
        subtitle={`${total} event${total === 1 ? "" : "s"} · address create, update, delete and discovery claims`}
      />

      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e: FormEvent) => {
          e.preventDefault();
          patchParams(
            {
              address: draftAddress.trim() || null,
              action: draftAction.trim() || null,
              actor: draftActor.trim() || null,
              from: draftFrom || null,
              to: draftTo || null,
            },
            true,
          );
        }}
      >
        <div className="relative w-full sm:w-48">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
          <input
            className="wt-input w-full pl-9 font-mono"
            value={draftAddress}
            onChange={(e) => setDraftAddress(e.target.value)}
            placeholder="10.0.0.12"
            aria-label="Address"
            title="Address"
          />
        </div>
        <div className="relative w-full sm:w-44">
          <Activity className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
          <input
            className="wt-input w-full pl-9"
            value={draftAction}
            onChange={(e) => setDraftAction(e.target.value)}
            placeholder="update, delete…"
            aria-label="Action"
            title="Action"
          />
        </div>
        <div className="relative w-full sm:w-52">
          <User className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
          <input
            className="wt-input w-full pl-9"
            value={draftActor}
            onChange={(e) => setDraftActor(e.target.value)}
            placeholder="Actor email"
            aria-label="Actor"
            title="Actor"
          />
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-zinc-500">From</span>
          <input
            type="date"
            className="wt-input"
            value={draftFrom}
            onChange={(e) => setDraftFrom(e.target.value)}
            aria-label="From date"
          />
          <span className="text-xs text-zinc-500">to</span>
          <input
            type="date"
            className="wt-input"
            value={draftTo}
            onChange={(e) => setDraftTo(e.target.value)}
            aria-label="To date"
          />
        </div>
        <Button type="submit" icon={<Filter />}>
          Apply
        </Button>
      </form>

      {error && <p className="text-sm text-red-400">{error}</p>}

      {loading ? (
        <TableSkeleton rows={8} cols={6} />
      ) : (
        <>
          <div className="wt-table-wrap">
            <table className="min-w-full text-left text-sm">
              <thead>
                <tr className="border-b border-[var(--color-border)]">
                  <th className="h-10 px-3">When</th>
                  <th className="h-10 px-3">Address</th>
                  <th className="h-10 px-3">Action</th>
                  <th className="h-10 px-3">Status</th>
                  <th className="h-10 px-3">Hostname</th>
                  <th className="h-10 px-3">Actor</th>
                  <th className="h-10 px-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {rows.map((row) => (
                  <tr key={row.id}>
                    <td
                      className="h-11 whitespace-nowrap px-3 text-zinc-400"
                      title={new Date(row.created_at).toLocaleString()}
                    >
                      {relativeTime(row.created_at)}
                    </td>
                    <td className="h-11 px-3 font-mono text-zinc-100">{row.address}</td>
                    <td className="h-11 px-3">
                      <Badge tone={actionTone(row.action)} className="font-mono">
                        {row.action}
                      </Badge>
                    </td>
                    <td className="h-11 px-3">
                      {row.status ? <StatusBadge status={row.status} /> : <span className="text-zinc-600">—</span>}
                    </td>
                    <td className="h-11 px-3 text-zinc-300">
                      {row.hostname || <span className="text-zinc-600">—</span>}
                    </td>
                    <td className="h-11 px-3 text-zinc-400">
                      {row.actor_email || <span className="text-zinc-600">system</span>}
                    </td>
                    <td className="h-11 px-3 text-right">
                      {row.subnet_id ? (
                        <Link
                          className="inline-flex items-center gap-1 whitespace-nowrap text-xs text-zinc-400 hover:text-brand"
                          to={`/subnets/${row.subnet_id}`}
                        >
                          Open subnet <ArrowUpRight className="h-3 w-3" />
                        </Link>
                      ) : null}
                    </td>
                  </tr>
                ))}
                {rows.length === 0 && (
                  <tr>
                    <td colSpan={7} className="px-3 py-12 text-center text-sm text-zinc-500">
                      {hasFilters ? "No history rows match these filters" : "No IP history yet"}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <Pagination
            page={page}
            pageCount={pageCount}
            total={total}
            pageSize={pageSize}
            onPageChange={(p) => patchParams({ page: String(p) })}
          />
        </>
      )}
    </div>
  );
}

function actionTone(action: string): "brand" | "info" | "danger" | "neutral" {
  const a = action.toLowerCase();
  if (/(delete|remove|release)/.test(a)) return "danger";
  if (/(create|add|restore|claim)/.test(a)) return "brand";
  if (/(update|edit|move)/.test(a)) return "info";
  return "neutral";
}

