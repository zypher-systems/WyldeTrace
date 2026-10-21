import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type Updater,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, Search } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { Pagination } from "./Pagination";

type ServerPaging = {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  filter: string;
  onFilterChange: (q: string) => void;
  status?: string;
  onStatusChange?: (status: string) => void;
  statusOptions?: { value: string; label: string }[];
  sorting?: SortingState;
  onSortingChange?: (sorting: SortingState) => void;
};

type Props<T> = {
  data: T[];
  columns: ColumnDef<T, unknown>[];
  filterPlaceholder?: string;
  server?: ServerPaging;
  loading?: boolean;
  /** Extra controls rendered at the right of the filter row. */
  toolbar?: ReactNode;
  /** Rendered between the filter row and the table (e.g. a bulk-action bar). */
  banner?: ReactNode;
  emptyMessage?: ReactNode;
};

export function DataTable<T>({
  data,
  columns,
  filterPlaceholder = "Filter…",
  server,
  loading,
  toolbar,
  banner,
  emptyMessage = "No rows",
}: Props<T>) {
  const [sorting, setSorting] = useState<SortingState>(server?.sorting ?? []);
  const [globalFilter, setGlobalFilter] = useState("");
  const [draftFilter, setDraftFilter] = useState(server?.filter ?? "");

  useEffect(() => {
    if (!server) return;
    setDraftFilter(server.filter);
  }, [server?.filter]);

  useEffect(() => {
    if (server?.sorting) setSorting(server.sorting);
  }, [server?.sorting]);

  useEffect(() => {
    if (!server) return;
    const t = window.setTimeout(() => {
      if (draftFilter !== server.filter) server.onFilterChange(draftFilter);
    }, 300);
    return () => window.clearTimeout(t);
  }, [draftFilter, server]);

  function handleSortingChange(updater: Updater<SortingState>) {
    setSorting((prev) => {
      const next = typeof updater === "function" ? updater(prev) : updater;
      server?.onSortingChange?.(next);
      return next;
    });
  }

  const table = useReactTable({
    data,
    columns,
    state: { sorting, globalFilter },
    onSortingChange: handleSortingChange,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: server ? undefined : getSortedRowModel(),
    getFilteredRowModel: server ? undefined : getFilteredRowModel(),
    getPaginationRowModel: server ? undefined : getPaginationRowModel(),
    manualPagination: !!server,
    manualSorting: !!server,
    // Server sort only applies to columns with a known accessor/id whitelist on the API.
    enableSortingRemoval: true,
    pageCount: server ? Math.max(1, Math.ceil(server.total / server.pageSize)) : undefined,
  });

  const pageCount = server
    ? Math.max(1, Math.ceil(server.total / server.pageSize) || 1)
    : table.getPageCount() || 1;
  const page = server ? server.page : table.getState().pagination.pageIndex + 1;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative w-full max-w-xs">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
          <input
            value={server ? draftFilter : globalFilter}
            onChange={(e) =>
              server ? setDraftFilter(e.target.value) : setGlobalFilter(e.target.value)
            }
            placeholder={filterPlaceholder}
            aria-label={filterPlaceholder}
            className="wt-input w-full pl-9"
          />
        </div>
        {server?.onStatusChange && (
          <select
            value={server.status ?? ""}
            onChange={(e) => server.onStatusChange?.(e.target.value)}
            className="wt-input"
            aria-label="Status filter"
          >
            <option value="">All statuses</option>
            {(server.statusOptions ?? []).map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        )}
        {toolbar ? <div className="ml-auto flex flex-wrap items-center gap-2">{toolbar}</div> : null}
      </div>
      {banner}
      <div className="wt-table-wrap">
        <table className="min-w-full text-left text-sm">
          <thead>
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id} className="border-b border-[var(--color-border)]">
                {hg.headers.map((header) => {
                  const sorted = header.column.getIsSorted();
                  const canSort = header.column.getCanSort();
                  const ariaSort =
                    sorted === "asc" ? "ascending" : sorted === "desc" ? "descending" : "none";
                  return (
                    <th
                      key={header.id}
                      className={[
                        "h-10 select-none whitespace-nowrap px-3",
                        canSort ? "cursor-pointer hover:text-zinc-200" : "",
                        sorted ? "text-zinc-200" : "",
                      ].join(" ")}
                      aria-sort={canSort ? ariaSort : undefined}
                      tabIndex={canSort ? 0 : undefined}
                      role={canSort ? "columnheader" : undefined}
                      onClick={header.column.getToggleSortingHandler()}
                      onKeyDown={(e) => {
                        if (!canSort) return;
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          header.column.toggleSorting();
                        }
                      }}
                    >
                      <span className="inline-flex items-center gap-1">
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {sorted === "asc" && <ArrowUp className="h-3 w-3" />}
                        {sorted === "desc" && <ArrowDown className="h-3 w-3" />}
                      </span>
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody className="divide-y divide-[var(--color-border)]">
            {loading && data.length === 0 ? (
              Array.from({ length: 6 }, (_, i) => (
                <tr key={i}>
                  <td colSpan={columns.length} className="px-3 py-3">
                    <div className="h-4 animate-pulse rounded bg-[var(--color-surface-overlay)]" style={{ width: `${60 + ((i * 13) % 35)}%` }} />
                  </td>
                </tr>
              ))
            ) : (
              <>
                {table.getRowModel().rows.map((row) => (
                  <tr key={row.id} className={loading ? "opacity-60" : undefined}>
                    {row.getVisibleCells().map((cell) => (
                      <td key={cell.id} className="h-11 px-3 py-1.5 text-zinc-200">
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                ))}
                {table.getRowModel().rows.length === 0 && (
                  <tr>
                    <td colSpan={columns.length} className="px-3 py-12 text-center text-sm text-zinc-500">
                      {emptyMessage}
                    </td>
                  </tr>
                )}
              </>
            )}
          </tbody>
        </table>
      </div>
      <Pagination
        page={page}
        pageCount={pageCount}
        total={server?.total}
        pageSize={server?.pageSize}
        onPageChange={(p) => (server ? server.onPageChange(p) : table.setPageIndex(p - 1))}
      />
    </div>
  );
}
