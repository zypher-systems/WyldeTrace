import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Meter, Segmented, cx } from "./ui";

export type IPMapCell = {
  address: string;
  status: string;
  hostname?: string;
  id?: string | null;
  special?: boolean;
};

export type IPMapData = {
  cidr: string;
  prefix_len: number;
  supported: boolean;
  message?: string;
  columns: number;
  cells: IPMapCell[];
  children?: { id: string; cidr: string; name: string }[];
  percent?: number;
  used?: number;
  total?: number;
  sparse?: boolean;
};

/**
 * Tinted, low-saturation cell styles: translucent fill + coloured border/text.
 * Free addresses stay nearly invisible so used space reads at a glance.
 */
const STATUS_STYLE: Record<string, string> = {
  available: "border-white/[0.07] bg-white/[0.025] text-zinc-500",
  assigned:
    "border-[var(--color-status-assigned)]/25 bg-[var(--color-status-assigned)]/12 text-[var(--color-status-assigned)]",
  reserved:
    "border-[var(--color-status-reserved)]/25 bg-[var(--color-status-reserved)]/12 text-[var(--color-status-reserved)]",
  dhcp: "border-[var(--color-status-dhcp)]/25 bg-[var(--color-status-dhcp)]/12 text-[var(--color-status-dhcp)]",
  deleted: "border-[var(--color-danger)]/30 bg-[var(--color-danger)]/10 text-[var(--color-danger)]/80 line-through",
  empty: "border-dashed border-white/[0.06] bg-transparent text-zinc-600",
  special: "border-transparent bg-white/[0.015] text-zinc-700",
};

const LEGEND = [
  ["available", "Available"],
  ["assigned", "Assigned"],
  ["reserved", "Reserved"],
  ["dhcp", "DHCP"],
  ["deleted", "Deleted"],
  ["empty", "Open"],
  ["special", "Net / bcast"],
] as const;

type FilterMode = "all" | "used" | "unused";

function isUsed(cell: IPMapCell): boolean {
  return (
    cell.status === "assigned" ||
    cell.status === "reserved" ||
    cell.status === "dhcp" ||
    cell.status === "deleted"
  );
}

function isUnused(cell: IPMapCell): boolean {
  return cell.status === "available" || cell.status === "empty";
}

function lastOctet(address: string): string {
  const i = address.lastIndexOf(".");
  if (i >= 0) return address.slice(i + 1);
  const j = address.lastIndexOf(":");
  return j >= 0 ? address.slice(j + 1) : address;
}

function cellTitle(cell: IPMapCell): string {
  const client = cell.hostname?.trim();
  if (client) return `${cell.address}\n${client}\n${cell.status}`;
  if (cell.special) return `${cell.address}\nnetwork / broadcast`;
  if (cell.status === "empty") return `${cell.address}\nopen — click to add`;
  return `${cell.address}\n${cell.status}`;
}

export function IPStatusGrid({
  data,
  selectedAddress,
  fullView = false,
  childLinkSuffix = "",
  onSelect,
}: {
  data: IPMapData;
  selectedAddress?: string;
  fullView?: boolean;
  /** Appended to child subnet links (e.g. `/map`). */
  childLinkSuffix?: string;
  onSelect?: (cell: IPMapCell, anchor: DOMRect) => void;
}) {
  const [filter, setFilter] = useState<FilterMode>("all");
  const [hover, setHover] = useState<IPMapCell | null>(null);

  const visible = useMemo(() => {
    if (filter === "used") return data.cells.filter(isUsed);
    if (filter === "unused") return data.cells.filter(isUnused);
    return data.cells;
  }, [data.cells, filter]);

  if (!data.supported) {
    return (
      <div className="wt-card space-y-4 p-5">
        <div className="flex flex-wrap items-end justify-between gap-2">
          <div>
            <h2 className="text-sm font-semibold text-zinc-100">Address map</h2>
            <p className="mt-0.5 text-xs text-zinc-500">{data.message || "Prefix too large for grid"}</p>
          </div>
          {data.sparse || data.total == null ? (
            <p className="font-mono text-xs text-zinc-400">{data.used ?? 0} registered</p>
          ) : (
            <p className="font-mono text-xs tabular-nums text-zinc-400">
              {data.used}/{data.total} · {(data.percent ?? 0).toFixed(1)}%
            </p>
          )}
        </div>
        {data.total != null && !data.sparse && <Meter value={data.percent ?? 0} />}
        {!!data.children?.length && (
          <div className="flex flex-wrap items-center gap-1.5 text-xs">
            <span className="mr-1 text-zinc-500">Child prefixes</span>
            {data.children.map((c) => (
              <Link
                key={c.id}
                to={`/subnets/${c.id}${childLinkSuffix}`}
                className="wt-badge font-mono hover:border-brand/50 hover:text-brand"
              >
                {c.cidr}
              </Link>
            ))}
          </div>
        )}
      </div>
    );
  }

  const cols = filter === "all" ? data.columns || 20 : Math.min(20, Math.max(10, data.columns || 20));

  return (
    <div className="wt-card space-y-3 p-4 wt-animate-in">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-zinc-100">Address map</h2>
          <p className="mt-0.5 truncate text-xs text-zinc-500">
            <span className="tabular-nums">
              {visible.length}/{data.cells.length} shown
            </span>
            {hover ? (
              <span className="ml-2 font-mono text-zinc-300">
                · {hover.address}
                {hover.hostname ? ` · ${hover.hostname}` : ""}
              </span>
            ) : (
              <span className="ml-2">· hover for client · click to add/edit</span>
            )}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          {data.sparse || data.total == null ? (
            <p className="font-mono text-xs text-zinc-400">{data.used ?? 0} registered</p>
          ) : (
            <div className="flex items-center gap-2">
              <Meter value={data.percent ?? 0} className="w-24" />
              <p className="font-mono text-xs tabular-nums text-zinc-400">
                {data.used}/{data.total} · {(data.percent ?? 0).toFixed(1)}%
              </p>
            </div>
          )}
          <Segmented
            label="Filter addresses"
            value={filter}
            onChange={setFilter}
            items={[
              { value: "all", label: "All" },
              { value: "used", label: "Hide unused" },
              { value: "unused", label: "Hide used" },
            ]}
          />
        </div>
      </div>

      <div
        className={cx(
          "relative overflow-auto rounded-lg border border-[var(--color-border)] bg-[var(--color-canvas)]/60 p-2.5",
          fullView ? "max-h-[calc(100vh-12rem)]" : "max-h-[44vh]",
        )}
      >
        <div
          className="grid justify-start"
          style={{
            gridTemplateColumns: `repeat(${cols}, 40px)`,
            gap: "4px",
          }}
        >
          {visible.map((cell) => {
            const selected = selectedAddress === cell.address;
            const interactive = cell.status !== "special";
            return (
              <button
                key={cell.address}
                type="button"
                disabled={!interactive}
                title={cellTitle(cell)}
                aria-label={cellTitle(cell).replace(/\n/g, ", ")}
                aria-pressed={selected || undefined}
                className={cx(
                  "flex h-[40px] w-[40px] items-center justify-center rounded-md border font-mono text-[11px] font-medium leading-none transition-colors",
                  STATUS_STYLE[cell.status] || STATUS_STYLE.empty,
                  interactive
                    ? "hover:border-white/40 hover:text-zinc-50"
                    : "cursor-default",
                  selected &&
                    "border-brand text-zinc-50 ring-2 ring-brand/60 ring-offset-1 ring-offset-[var(--color-canvas)]",
                )}
                onMouseEnter={() => setHover(cell)}
                onMouseLeave={() => setHover(null)}
                onClick={(e) => {
                  if (!interactive) return;
                  onSelect?.(cell, e.currentTarget.getBoundingClientRect());
                }}
              >
                {lastOctet(cell.address)}
              </button>
            );
          })}
        </div>
        {visible.length === 0 && (
          <p className="py-6 text-center text-xs text-zinc-500">No addresses match this filter</p>
        )}
      </div>

      <div className="flex flex-wrap gap-x-4 gap-y-1.5 text-xs text-zinc-400">
        {LEGEND.map(([key, label]) => (
          <span key={key} className="inline-flex items-center gap-1.5">
            <span className={cx("h-3 w-3 rounded-[3px] border", STATUS_STYLE[key])} />
            {label}
          </span>
        ))}
      </div>
    </div>
  );
}
