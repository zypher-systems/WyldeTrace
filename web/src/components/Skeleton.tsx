export function Skeleton({ className = "" }: { className?: string }) {
  return (
    <div
      className={`animate-pulse rounded-md bg-[var(--color-surface-overlay)] ${className}`}
      aria-hidden="true"
    />
  );
}

export function TableSkeleton({ rows = 6, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="space-y-3" role="status" aria-label="Loading">
      <Skeleton className="h-9 w-64" />
      <div className="wt-table-wrap">
        <div className="space-y-0">
          {Array.from({ length: rows }).map((_, i) => (
            <div
              key={i}
              className="flex gap-3 border-t border-[var(--color-border)] px-3 py-3 first:border-t-0"
            >
              {Array.from({ length: cols }).map((_, j) => (
                <Skeleton key={j} className="h-4 flex-1" />
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
