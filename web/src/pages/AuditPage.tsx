import {
  Activity,
  ArrowDown,
  ArrowUp,
  ChevronRight,
  Download,
  Filter,
  User,
  X,
} from "lucide-react";
import { useEffect, useState } from "react";
import { Navigate, useSearchParams } from "react-router-dom";
import { api, downloadCSV, pageQuery, type AuditLog, type PageResult } from "../api";
import { useAuth } from "../auth";
import { PageHeader } from "../components/PageHeader";
import { Pagination } from "../components/Pagination";
import { Sheet } from "../components/Sheet";
import { TableSkeleton } from "../components/Skeleton";
import { Badge, Button, cx } from "../components/ui";
import { relativeTime } from "../lib/time";

function auditQuery(params: {
  page?: number;
  pageSize?: number;
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
  if (params.action) sp.set("action", params.action);
  if (params.actor) sp.set("actor", params.actor);
  if (params.from) sp.set("from", params.from);
  if (params.to) sp.set("to", params.to);
  const qs = sp.toString();
  return qs ? `?${qs}` : "";
}

export function AuditPage() {
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const page = Math.max(1, Number(searchParams.get("page") || "1") || 1);
  const action = searchParams.get("action") || "";
  const actor = searchParams.get("actor") || "";
  const from = searchParams.get("from") || "";
  const to = searchParams.get("to") || "";
  const sort = searchParams.get("sort") || "created_at";
  const order = (searchParams.get("order") === "asc" ? "asc" : "desc") as "asc" | "desc";

  const [draftAction, setDraftAction] = useState(action);
  const [draftActor, setDraftActor] = useState(actor);
  const [draftFrom, setDraftFrom] = useState(from);
  const [draftTo, setDraftTo] = useState(to);
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [openLog, setOpenLog] = useState<AuditLog | null>(null);
  const pageSize = 50;

  useEffect(() => {
    setDraftAction(action);
    setDraftActor(actor);
    setDraftFrom(from);
    setDraftTo(to);
  }, [action, actor, from, to]);

  useEffect(() => {
    if (user?.role !== "admin") return;
    void (async () => {
      setLoading(true);
      try {
        const res = await api<PageResult<AuditLog>>(
          `/api/v1/audit-logs${auditQuery({
            page,
            pageSize,
            action,
            actor,
            from,
            to,
            sort,
            order,
          })}`,
        );
        setLogs(res.items);
        setTotal(res.total);
        setError("");
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load audit log");
      } finally {
        setLoading(false);
      }
    })();
  }, [user?.role, page, action, actor, from, to, sort, order]);

  if (user?.role !== "admin") return <Navigate to="/" replace />;

  const pageCount = Math.max(1, Math.ceil(total / pageSize) || 1);
  const exportPath = `/api/v1/audit-logs/export.csv${auditQuery({
    action,
    actor,
    from,
    to,
  })}`;

  function patchParams(next: Record<string, string | null>, resetPage = false) {
    const sp = new URLSearchParams(searchParams);
    for (const [k, v] of Object.entries(next)) {
      if (v == null || v === "") sp.delete(k);
      else sp.set(k, v);
    }
    if (resetPage) sp.set("page", "1");
    setSearchParams(sp, { replace: true });
  }

  function toggleSort(col: string) {
    if (sort === col) {
      patchParams({ order: order === "asc" ? "desc" : "asc" });
    } else {
      patchParams({ sort: col, order: col === "created_at" ? "desc" : "asc" }, true);
    }
  }

  const hasFilters = !!(action || actor || from || to);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Audit log"
        subtitle={`${total} event${total === 1 ? "" : "s"} · mutating actions across the instance`}
        actions={
          <Button
            icon={<Download />}
            onClick={() =>
              void downloadCSV(exportPath, "audit-logs.csv").catch((e) =>
                setError(e instanceof Error ? e.message : "Export failed"),
              )
            }
          >
            Export CSV
          </Button>
        }
      />

      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          patchParams(
            {
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
          <Activity className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
          <input
            className="wt-input w-full pl-9 font-mono"
            value={draftAction}
            onChange={(e) => setDraftAction(e.target.value)}
            placeholder="ip.create"
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
        {(hasFilters || draftAction || draftActor || draftFrom || draftTo) && (
          <Button
            variant="ghost"
            icon={<X />}
            onClick={() => {
              setDraftAction("");
              setDraftActor("");
              setDraftFrom("");
              setDraftTo("");
              setSearchParams({}, { replace: true });
            }}
          >
            Clear
          </Button>
        )}
      </form>

      {error && <p className="text-sm text-red-400">{error}</p>}
      {loading ? (
        <TableSkeleton rows={8} cols={5} />
      ) : (
        <>
          <div className="wt-table-wrap">
            <table className="min-w-full text-left text-sm">
              <thead>
                <tr className="border-b border-[var(--color-border)]">
                  {(
                    [
                      ["created_at", "When"],
                      ["action", "Action"],
                      ["actor_email", "Actor"],
                      ["entity_type", "Entity"],
                    ] as const
                  ).map(([col, label]) => (
                    <th
                      key={col}
                      className={cx(
                        "h-10 cursor-pointer select-none whitespace-nowrap px-3 hover:text-zinc-200",
                        sort === col && "text-zinc-200",
                      )}
                      aria-sort={
                        sort === col ? (order === "asc" ? "ascending" : "descending") : "none"
                      }
                      tabIndex={0}
                      onClick={() => toggleSort(col)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          toggleSort(col);
                        }
                      }}
                    >
                      <span className="inline-flex items-center gap-1">
                        {label}
                        {sort === col &&
                          (order === "asc" ? (
                            <ArrowUp className="h-3 w-3" />
                          ) : (
                            <ArrowDown className="h-3 w-3" />
                          ))}
                      </span>
                    </th>
                  ))}
                  <th className="h-10 px-3">Details</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {logs.map((l) => {
                  const detailText = l.details && Object.keys(l.details).length ? JSON.stringify(l.details) : "";
                  return (
                    <tr key={l.id}>
                      <td
                        className="h-11 whitespace-nowrap px-3 text-zinc-400"
                        title={new Date(l.created_at).toLocaleString()}
                      >
                        {relativeTime(l.created_at)}
                      </td>
                      <td className="h-11 px-3">
                        <Badge tone={actionTone(l.action)} className="font-mono">
                          {l.action}
                        </Badge>
                      </td>
                      <td className="h-11 px-3 text-zinc-200">
                        {l.actor_email || <span className="text-zinc-600">system</span>}
                      </td>
                      <td className="h-11 px-3">
                        <span className="text-zinc-300">{l.entity_type}</span>
                        {l.entity_id ? (
                          <span className="ml-1.5 font-mono text-xs text-zinc-500" title={l.entity_id}>
                            {l.entity_id.length > 12 ? `${l.entity_id.slice(0, 8)}…` : l.entity_id}
                          </span>
                        ) : null}
                      </td>
                      <td className="h-11 max-w-md px-3">
                        {detailText ? (
                          <button
                            type="button"
                            className="group flex w-full min-w-0 items-center gap-2 text-left"
                            onClick={() => setOpenLog(l)}
                            aria-label={`View details for ${l.action}`}
                          >
                            <span className="min-w-0 flex-1 truncate font-mono text-xs text-zinc-500 group-hover:text-zinc-300">
                              {detailText}
                            </span>
                            <ChevronRight className="h-4 w-4 shrink-0 text-zinc-600 group-hover:text-brand" />
                          </button>
                        ) : (
                          <span className="text-zinc-600">—</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
                {logs.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-3 py-12 text-center text-sm text-zinc-500">
                      {hasFilters ? "No audit events match these filters" : "No audit events yet"}
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

      <Sheet
        open={!!openLog}
        title={openLog ? <span className="font-mono">{openLog.action}</span> : ""}
        subtitle={openLog ? new Date(openLog.created_at).toLocaleString() : undefined}
        onClose={() => setOpenLog(null)}
      >
        {openLog && (
          <div className="space-y-5">
            <dl className="divide-y divide-[var(--color-border)] text-sm">
              <div className="flex justify-between gap-4 py-2">
                <dt className="text-zinc-500">Actor</dt>
                <dd className="text-zinc-200">{openLog.actor_email || "system"}</dd>
              </div>
              <div className="flex justify-between gap-4 py-2">
                <dt className="text-zinc-500">Entity</dt>
                <dd className="text-zinc-200">{openLog.entity_type}</dd>
              </div>
              <div className="flex justify-between gap-4 py-2">
                <dt className="text-zinc-500">Entity ID</dt>
                <dd className="break-all font-mono text-xs text-zinc-300">{openLog.entity_id || "—"}</dd>
              </div>
            </dl>
            <div>
              <p className="wt-label mb-1.5">Details</p>
              <pre className="overflow-auto rounded-lg border border-[var(--color-border)] bg-[var(--color-canvas)] p-3 font-mono text-xs leading-relaxed text-zinc-300">
                {JSON.stringify(openLog.details, null, 2)}
              </pre>
            </div>
          </div>
        )}
      </Sheet>
    </div>
  );
}

function actionTone(action: string): "brand" | "info" | "danger" | "warn" | "neutral" {
  const a = action.toLowerCase();
  if (/(delete|remove|revoke|purge)/.test(a)) return "danger";
  if (/(create|add|restore|claim)/.test(a)) return "brand";
  if (/(update|edit|rename|move|resize|merge|split)/.test(a)) return "info";
  if (/(login|auth|password|token)/.test(a)) return "warn";
  return "neutral";
}

