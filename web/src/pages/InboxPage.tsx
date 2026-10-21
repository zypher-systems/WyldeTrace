import { useEffect, useMemo, useState, type MouseEvent as ReactMouseEvent } from "react";
import { Link, Navigate, useSearchParams } from "react-router-dom";
import { api, pageQuery, type PageResult, type Subnet } from "../api";
import { useAuth } from "../auth";
import { Check, CheckCheck, Inbox, Radar, X } from "lucide-react";
import { useContextMenu, type ContextMenuItem } from "../components/ContextMenu";
import { PageHeader } from "../components/PageHeader";
import { Pagination } from "../components/Pagination";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Badge, Button, Kbd, Segmented, cx } from "../components/ui";
import { relativeTime } from "../lib/time";

type Finding = {
  id: string;
  subnet_id: string;
  address: string;
  kind: "new_alive" | "stale" | string;
  hostname: string;
  mac_address: string;
  db_status: string;
  subnet_cidr?: string;
  subnet_name?: string;
  location_id?: string;
  location_name?: string;
  last_seen_at: string;
};

export function InboxPage() {
  const { user } = useAuth();
  const toast = useToast();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const { open: openMenu, menu } = useContextMenu();
  const [searchParams, setSearchParams] = useSearchParams();
  const page = Math.max(1, Number(searchParams.get("page") || "1") || 1);
  const kind = searchParams.get("kind") || "";
  const subnetId = searchParams.get("subnet_id") || "";
  const siteId = searchParams.get("location_id") || "";
  const [items, setItems] = useState<Finding[]>([]);
  const [subnets, setSubnets] = useState<Subnet[]>([]);
  const [total, setTotal] = useState(0);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [focusIdx, setFocusIdx] = useState(0);
  const [loading, setLoading] = useState(true);
  const pageSize = 50;

  function patchParams(next: Record<string, string | null>, resetPage = false) {
    const sp = new URLSearchParams(searchParams);
    for (const [k, v] of Object.entries(next)) {
      if (v == null || v === "") sp.delete(k);
      else sp.set(k, v);
    }
    if (resetPage) sp.set("page", "1");
    setSearchParams(sp, { replace: true });
  }

  async function load(nextPage = page, k = kind, sid = subnetId, loc = siteId) {
    setLoading(true);
    try {
      const q =
        `/api/v1/inbox${pageQuery({ page: nextPage, pageSize })}` +
        (k ? `&kind=${encodeURIComponent(k)}` : "") +
        (sid ? `&subnet_id=${encodeURIComponent(sid)}` : "") +
        (loc ? `&location_id=${encodeURIComponent(loc)}` : "");
      const res = await api<PageResult<Finding>>(q);
      setItems(res.items);
      setTotal(res.total);
      setSelected({});
      setFocusIdx(0);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load inbox", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (user?.role !== "reportview") void load(page, kind, subnetId, siteId);
  }, [user?.role, page, kind, subnetId, siteId]);

  useEffect(() => {
    if (user?.role === "reportview") return;
    void api<PageResult<Subnet>>(`/api/v1/subnets${pageQuery({ page: 1, pageSize: 200 })}`)
      .then((r) => setSubnets(r.items))
      .catch(() => setSubnets([]));
  }, [user?.role]);

  async function act(id: string, action: "claim" | "dismiss" | "mark-available") {
    try {
      await api(`/api/v1/inbox/${id}/${action}`, { method: "POST" });
      toast(`${action} ok`, "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Action failed", "error");
    }
  }

  async function bulk(action: "claim" | "dismiss" | "mark-available") {
    const ids = Object.entries(selected)
      .filter(([, v]) => v)
      .map(([id]) => id);
    if (ids.length === 0) {
      toast("Select at least one finding", "error");
      return;
    }
    try {
      const res = await api<{ ok: number; skipped: number }>("/api/v1/inbox/bulk", {
        method: "POST",
        body: JSON.stringify({ ids, action }),
      });
      toast(`Bulk ${action}: ${res.ok} ok, ${res.skipped} skipped`, "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Bulk action failed", "error");
    }
  }

  const subnetOptions = useMemo(() => {
    const fromFindings = items.map((f) => ({
      id: f.subnet_id,
      label: f.subnet_cidr || f.subnet_id.slice(0, 8),
    }));
    const fromList = subnets.map((s) => ({ id: s.id, label: `${s.cidr} — ${s.name}` }));
    const map = new Map<string, string>();
    for (const s of [...fromList, ...fromFindings]) map.set(s.id, s.label);
    return [...map.entries()].map(([id, label]) => ({ id, label }));
  }, [items, subnets]);

  const siteOptions = useMemo(() => {
    const map = new Map<string, string>();
    for (const f of items) {
      if (f.location_id) map.set(f.location_id, f.location_name || f.location_id.slice(0, 8));
    }
    return [...map.entries()]
      .map(([id, label]) => ({ id, label }))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [items]);

  const visibleItems = items;

  const selectedCount = Object.values(selected).filter(Boolean).length;
  const pageCount = Math.max(1, Math.ceil(total / pageSize) || 1);
  const allSelected = visibleItems.length > 0 && visibleItems.every((f) => selected[f.id]);
  const focused = visibleItems[focusIdx];

  useEffect(() => {
    setFocusIdx(0);
  }, [siteId]);

  useEffect(() => {
    if (!canEdit || user?.role === "reportview") return;
    function onKey(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement | null)?.tagName;
      if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") return;
      if (!visibleItems.length) return;
      if (e.key === "j" || e.key === "ArrowDown") {
        e.preventDefault();
        setFocusIdx((i) => Math.min(visibleItems.length - 1, i + 1));
      } else if (e.key === "k" || e.key === "ArrowUp") {
        e.preventDefault();
        setFocusIdx((i) => Math.max(0, i - 1));
      } else if (e.key === " " || e.key === "x") {
        e.preventDefault();
        const f = visibleItems[focusIdx];
        if (!f) return;
        setSelected((prev) => ({ ...prev, [f.id]: !prev[f.id] }));
      } else if (e.key === "c") {
        e.preventDefault();
        const f = visibleItems[focusIdx];
        if (f?.kind === "new_alive") void act(f.id, "claim");
        else if (selectedCount) void bulk("claim");
      } else if (e.key === "d") {
        e.preventDefault();
        const f = visibleItems[focusIdx];
        if (selectedCount) void bulk("dismiss");
        else if (f) void act(f.id, "dismiss");
      } else if (e.key === "a") {
        e.preventDefault();
        const f = visibleItems[focusIdx];
        if (f?.kind === "stale") void act(f.id, "mark-available");
        else if (selectedCount) void bulk("mark-available");
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [canEdit, visibleItems, focusIdx, selectedCount, user?.role]);

  function openFindingMenu(e: ReactMouseEvent, f: Finding) {
    if (!canEdit) return;
    const items: ContextMenuItem[] = [];
    if (f.kind === "new_alive") {
      items.push({
        id: "claim",
        label: "Claim",
        onSelect: () => void act(f.id, "claim"),
      });
    }
    if (f.kind === "stale") {
      items.push({
        id: "available",
        label: "Mark available",
        onSelect: () => void act(f.id, "mark-available"),
      });
    }
    items.push({
      id: "dismiss",
      label: "Dismiss",
      onSelect: () => void act(f.id, "dismiss"),
    });
    openMenu(e, items);
  }

  if (user?.role === "reportview") return <Navigate to="/" replace />;

  const filtered = !!(kind || subnetId || siteId);

  return (
    <div className="space-y-6">
      {menu}
      <PageHeader
        title="Inbox"
        subtitle={`${total} open finding${total === 1 ? "" : "s"} · new alive hosts and stale records from scan jobs`}
      />

      <div className="flex flex-wrap items-center gap-2">
        <Segmented
          label="Finding kind"
          value={kind || "all"}
          onChange={(v) => patchParams({ kind: v === "all" ? null : v }, true)}
          items={[
            { value: "all", label: "All" },
            { value: "new_alive", label: "New alive" },
            { value: "stale", label: "Stale" },
          ]}
        />
        <select
          className="wt-input min-w-[200px]"
          aria-label="Subnet filter"
          value={subnetId}
          onChange={(e) => patchParams({ subnet_id: e.target.value || null }, true)}
        >
          <option value="">All subnets</option>
          {subnetOptions.map((s) => (
            <option key={s.id} value={s.id}>
              {s.label}
            </option>
          ))}
        </select>
        <select
          className="wt-input min-w-[160px]"
          aria-label="Site filter"
          value={siteId}
          onChange={(e) => patchParams({ location_id: e.target.value || null }, true)}
        >
          <option value="">All sites</option>
          {siteOptions.map((s) => (
            <option key={s.id} value={s.id}>
              {s.label}
            </option>
          ))}
        </select>
        {canEdit && (
          <div className="ml-auto hidden flex-wrap items-center gap-x-3 gap-y-1 text-xs text-zinc-500 md:flex" aria-label="Keyboard shortcuts">
            <span className="inline-flex items-center gap-1"><Kbd>j</Kbd><Kbd>k</Kbd> move</span>
            <span className="inline-flex items-center gap-1"><Kbd>space</Kbd> select</span>
            <span className="inline-flex items-center gap-1"><Kbd>c</Kbd> claim</span>
            <span className="inline-flex items-center gap-1"><Kbd>a</Kbd> available</span>
            <span className="inline-flex items-center gap-1"><Kbd>d</Kbd> dismiss</span>
          </div>
        )}
      </div>

      {canEdit && selectedCount > 0 && (
        <div className="flex flex-wrap items-center gap-2 rounded-xl border border-brand/25 bg-brand/[0.06] px-3 py-2 wt-animate-pop">
          <span className="mr-1 text-sm text-zinc-200">
            <span className="font-semibold text-brand tabular-nums">{selectedCount}</span> selected
          </span>
          <span className="h-5 w-px bg-[var(--color-border-strong)]" />
          <Button size="sm" icon={<Check />} onClick={() => void bulk("claim")}>
            Claim
          </Button>
          <Button size="sm" icon={<CheckCheck />} onClick={() => void bulk("mark-available")}>
            Mark available
          </Button>
          <div className="ml-auto flex items-center gap-1">
            <Button size="sm" variant="danger" icon={<X />} onClick={() => void bulk("dismiss")}>
              Dismiss
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setSelected({})}>
              Clear
            </Button>
          </div>
        </div>
      )}

      {loading ? (
        <TableSkeleton rows={8} cols={7} />
      ) : visibleItems.length === 0 ? (
        <div className="wt-card flex flex-col items-center px-6 py-14 text-center">
          <Inbox className="h-8 w-8 text-zinc-600" />
          <p className="mt-3 text-sm font-medium text-zinc-200">
            {filtered ? "Nothing matches these filters" : "Inbox zero"}
          </p>
          <p className="mt-1 max-w-sm text-xs text-zinc-500">
            {filtered
              ? "Try another kind, subnet or site."
              : "New alive hosts and stale records show up here after a scan job runs."}
          </p>
          {filtered ? (
            <Button
              size="sm"
              variant="secondary"
              className="mt-4"
              onClick={() => patchParams({ kind: null, subnet_id: null, location_id: null }, true)}
            >
              Clear filters
            </Button>
          ) : (
            <Link to="/discovery" className="wt-btn wt-btn-secondary wt-btn-sm mt-4">
              <Radar />
              Go to scan jobs
            </Link>
          )}
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                {canEdit && (
                  <th className="h-10 w-10 px-3">
                    <input
                      type="checkbox"
                      aria-label="Select all on page"
                      checked={allSelected}
                      onChange={(e) => {
                        const next: Record<string, boolean> = {};
                        if (e.target.checked) {
                          for (const f of visibleItems) next[f.id] = true;
                        }
                        setSelected(next);
                      }}
                    />
                  </th>
                )}
                <th className="h-10 px-3">Kind</th>
                <th className="h-10 px-3">Address</th>
                <th className="h-10 px-3">Hostname</th>
                <th className="h-10 px-3">Subnet</th>
                <th className="h-10 px-3">Site</th>
                <th className="h-10 px-3">Last seen</th>
                <th className="h-10 px-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {visibleItems.map((f, idx) => (
                <tr
                  key={f.id}
                  className={cx(
                    "transition-colors",
                    focused?.id === f.id && canEdit && "bg-[var(--color-surface-overlay)] shadow-[inset_2px_0_0_var(--color-brand)]",
                  )}
                  onClick={() => setFocusIdx(idx)}
                  onContextMenu={(e) => openFindingMenu(e, f)}
                >
                  {canEdit && (
                    <td className="h-11 px-3">
                      <input
                        type="checkbox"
                        aria-label={`Select ${f.address}`}
                        checked={!!selected[f.id]}
                        onChange={(e) =>
                          setSelected((prev) => ({ ...prev, [f.id]: e.target.checked }))
                        }
                      />
                    </td>
                  )}
                  <td className="h-11 px-3">
                    {f.kind === "new_alive" ? (
                      <Badge tone="brand">new alive</Badge>
                    ) : f.kind === "stale" ? (
                      <Badge tone="warn">stale</Badge>
                    ) : (
                      <Badge>{f.kind}</Badge>
                    )}
                  </td>
                  <td className="h-11 px-3 font-mono text-zinc-100">{f.address}</td>
                  <td className="h-11 px-3 text-zinc-300">{f.hostname || <span className="text-zinc-600">—</span>}</td>
                  <td className="h-11 px-3">
                    <Link
                      className="font-mono text-brand hover:underline"
                      to={`/subnets/${f.subnet_id}`}
                    >
                      {f.subnet_cidr || f.subnet_id.slice(0, 8)}
                    </Link>
                  </td>
                  <td className="h-11 px-3 text-zinc-400">
                    {f.location_name || <span className="text-zinc-600">—</span>}
                  </td>
                  <td
                    className="h-11 whitespace-nowrap px-3 text-xs text-zinc-400"
                    title={new Date(f.last_seen_at).toLocaleString()}
                  >
                    {relativeTime(f.last_seen_at)}
                  </td>
                  <td className="h-11 px-3">
                    {canEdit && (
                      <div className="flex justify-end gap-1">
                        {f.kind === "new_alive" && (
                          <button
                            type="button"
                            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                            aria-label={`Claim ${f.address}`}
                            title="Claim (c)"
                            onClick={() => void act(f.id, "claim")}
                          >
                            <Check />
                          </button>
                        )}
                        {f.kind === "stale" && (
                          <button
                            type="button"
                            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                            aria-label={`Mark ${f.address} available`}
                            title="Mark available (a)"
                            onClick={() => void act(f.id, "mark-available")}
                          >
                            <CheckCheck />
                          </button>
                        )}
                        <button
                          type="button"
                          className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                          aria-label={`Dismiss ${f.address}`}
                          title="Dismiss (d)"
                          onClick={() => void act(f.id, "dismiss")}
                        >
                          <X />
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Pagination
        page={page}
        pageCount={pageCount}
        total={total}
        onPageChange={(p) => patchParams({ page: String(p) })}
      />
    </div>
  );
}
