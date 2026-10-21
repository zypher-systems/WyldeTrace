import { RotateCcw, Search, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Navigate } from "react-router-dom";
import { api, pageQuery, type PageResult, type Subnet } from "../api";
import { useAuth } from "../auth";
import { PageHeader } from "../components/PageHeader";
import { Pagination } from "../components/Pagination";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Button } from "../components/ui";
import { relativeTime } from "../lib/time";

type DeletedSubnet = Subnet & { deleted_at: string };

export function DeletedSubnetsPage() {
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [items, setItems] = useState<DeletedSubnet[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState("");
  const toast = useToast();
  const [loaded, setLoaded] = useState(false);
  const pageSize = 50;

  async function load(nextPage = page, q = filter) {
    try {
      const res = await api<PageResult<DeletedSubnet>>(
        `/api/v1/subnets/deleted${pageQuery({ page: nextPage, pageSize, q })}`,
      );
      setItems(res.items);
      setTotal(res.total);
      setPage(res.page);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load deleted subnets");
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
      await api(`/api/v1/subnets/${id}/restore`, { method: "POST" });
      toast("Subnet restored", "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Restore failed", "error");
    }
  }

  const pageCount = Math.max(1, Math.ceil(total / pageSize) || 1);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Deleted subnets"
        subtitle={`${total} soft-deleted prefix${total === 1 ? "" : "es"} · restore to bring them back`}
      />

      <div className="relative w-full max-w-xs">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
        <input
          value={filter}
          onChange={(e) => {
            setPage(1);
            setFilter(e.target.value);
          }}
          placeholder="Filter by CIDR or name…"
          aria-label="Filter by CIDR or name"
          className="wt-input w-full pl-9"
        />
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      {!loaded ? (
        <TableSkeleton rows={6} cols={4} />
      ) : items.length === 0 && !error ? (
        <div className="wt-card px-6 py-12 text-center">
          <Trash2 className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">{filter ? "No deleted subnets match" : "Trash is empty"}</p>
          <p className="mt-1 text-xs text-zinc-500">
            {filter ? "Try a different CIDR or name." : "Soft-deleted subnets show up here until restored."}
          </p>
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                <th className="h-10 px-3">CIDR</th>
                <th className="h-10 px-3">Name</th>
                <th className="h-10 px-3">Deleted</th>
                <th className="h-10 px-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {items.map((s) => (
                <tr key={s.id}>
                  <td className="h-11 px-3 font-mono text-zinc-100">{s.cidr}</td>
                  <td className="h-11 px-3 text-zinc-300">{s.name || <span className="text-zinc-600">—</span>}</td>
                  <td
                    className="h-11 whitespace-nowrap px-3 text-zinc-400"
                    title={s.deleted_at ? new Date(s.deleted_at).toLocaleString() : undefined}
                  >
                    {s.deleted_at ? relativeTime(s.deleted_at) : "—"}
                  </td>
                  <td className="h-11 px-3">
                    <div className="flex justify-end">
                      <Button
                        size="sm"
                        icon={<RotateCcw />}
                        aria-label={`Restore ${s.cidr}`}
                        onClick={() => void restore(s.id)}
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

