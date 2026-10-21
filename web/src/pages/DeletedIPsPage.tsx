import { ArrowUpRight, RotateCcw, Search, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { api, pageQuery, type IPAddress, type PageResult } from "../api";
import { useAuth } from "../auth";
import { PageHeader } from "../components/PageHeader";
import { Pagination } from "../components/Pagination";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Button, StatusBadge } from "../components/ui";
import { relativeTime } from "../lib/time";

export function DeletedIPsPage() {
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [items, setItems] = useState<IPAddress[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState("");
  const toast = useToast();
  const [loaded, setLoaded] = useState(false);
  const pageSize = 50;

  async function load(nextPage = page, q = filter) {
    try {
      const res = await api<PageResult<IPAddress>>(
        `/api/v1/ips/deleted${pageQuery({ page: nextPage, pageSize, q })}`,
      );
      setItems(res.items);
      setTotal(res.total);
      setPage(res.page);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load deleted IPs");
    } finally {
      setLoaded(true);
    }
  }

  useEffect(() => {
    if (canEdit) void load(page, filter);
  }, [canEdit, page, filter]);

  if (!canEdit) return <Navigate to="/" replace />;

  async function restore(id: string) {
    try {
      await api(`/api/v1/ips/${id}/restore`, { method: "POST" });
      toast("IP restored", "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Restore failed", "error");
    }
  }

  const pageCount = Math.max(1, Math.ceil(total / pageSize) || 1);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Deleted IPs"
        subtitle={`${total} soft-deleted address${total === 1 ? "" : "es"} · restore to bring them back`}
      />

      <div className="relative w-full max-w-xs">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
        <input
          value={filter}
          onChange={(e) => {
            setPage(1);
            setFilter(e.target.value);
          }}
          placeholder="Filter by address or hostname…"
          aria-label="Filter by address or hostname"
          className="wt-input w-full pl-9"
        />
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      {!loaded ? (
        <TableSkeleton rows={6} cols={4} />
      ) : items.length === 0 && !error ? (
        <div className="wt-card px-6 py-12 text-center">
          <Trash2 className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">{filter ? "No deleted IPs match" : "Trash is empty"}</p>
          <p className="mt-1 text-xs text-zinc-500">
            {filter ? "Try a different address or hostname." : "Soft-deleted addresses show up here until restored."}
          </p>
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                <th className="h-10 px-3">Address</th>
                <th className="h-10 px-3">Hostname</th>
                <th className="h-10 px-3">Status</th>
                <th className="h-10 px-3">Deleted</th>
                <th className="h-10 px-3">Subnet</th>
                <th className="h-10 px-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {items.map((ip) => (
                <tr key={ip.id}>
                  <td className="h-11 px-3 font-mono text-zinc-100">{ip.address}</td>
                  <td className="h-11 px-3 text-zinc-300">{ip.hostname || <span className="text-zinc-600">—</span>}</td>
                  <td className="h-11 px-3">
                    <StatusBadge status={ip.status} />
                  </td>
                  <td
                    className="h-11 whitespace-nowrap px-3 text-zinc-400"
                    title={ip.deleted_at ? new Date(ip.deleted_at).toLocaleString() : undefined}
                  >
                    {ip.deleted_at ? relativeTime(ip.deleted_at) : "—"}
                  </td>
                  <td className="h-11 px-3">
                    <Link
                      className="inline-flex items-center gap-1 text-xs text-zinc-400 hover:text-brand"
                      to={`/subnets/${ip.subnet_id}`}
                    >
                      Open subnet <ArrowUpRight className="h-3 w-3" />
                    </Link>
                  </td>
                  <td className="h-11 px-3">
                    <div className="flex justify-end">
                      <Button
                        size="sm"
                        icon={<RotateCcw />}
                        aria-label={`Restore ${ip.address}`}
                        onClick={() => void restore(ip.id)}
                      >
                        Restore
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Pagination page={page} pageCount={pageCount} total={total} pageSize={pageSize} onPageChange={setPage} />
    </div>
  );
}

