import { ChevronLeft, ChevronRight } from "lucide-react";

type Props = {
  page: number;
  pageCount: number;
  total?: number;
  /** When set with `total`, shows an "x–y of n" range instead of "page x of y". */
  pageSize?: number;
  onPageChange: (page: number) => void;
};

export function Pagination({ page, pageCount, total, pageSize, onPageChange }: Props) {
  if (pageCount <= 1 && (total == null || total <= (pageSize ?? Infinity))) {
    return typeof total === "number" && total > 0 ? (
      <p className="text-xs text-zinc-500 tabular-nums">
        {total} {total === 1 ? "row" : "rows"}
      </p>
    ) : null;
  }
  const label =
    typeof total === "number" && pageSize
      ? `${Math.min(total, (page - 1) * pageSize + 1)}–${Math.min(total, page * pageSize)} of ${total}`
      : `Page ${page} of ${pageCount}${typeof total === "number" ? ` · ${total} total` : ""}`;
  return (
    <div className="flex items-center justify-between gap-3 text-xs text-zinc-500">
      <span className="tabular-nums">{label}</span>
      <div className="flex items-center gap-1">
        <button
          type="button"
          className="wt-btn wt-btn-secondary wt-btn-sm wt-btn-icon"
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
          aria-label="Previous page"
        >
          <ChevronLeft />
        </button>
        <span className="px-2 tabular-nums">
          {page} / {pageCount}
        </span>
        <button
          type="button"
          className="wt-btn wt-btn-secondary wt-btn-sm wt-btn-icon"
          onClick={() => onPageChange(page + 1)}
          disabled={page >= pageCount}
          aria-label="Next page"
        >
          <ChevronRight />
        </button>
      </div>
    </div>
  );
}
