/** Lightweight SVG charts — no extra dependency. */

export function DonutChart({
  segments,
  size = 160,
}: {
  segments: { label: string; value: number; color: string }[];
  size?: number;
}) {
  const total = segments.reduce((s, x) => s + x.value, 0) || 1;
  const r = size / 2 - 8;
  const c = size / 2;
  const circ = 2 * Math.PI * r;
  let offset = 0;
  return (
    <div className="flex flex-wrap items-center gap-4">
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="shrink-0">
        <circle cx={c} cy={c} r={r} fill="none" stroke="#1a1f2a" strokeWidth={14} />
        {segments.map((seg) => {
          const len = (seg.value / total) * circ;
          const el = (
            <circle
              key={seg.label}
              cx={c}
              cy={c}
              r={r}
              fill="none"
              stroke={seg.color}
              strokeWidth={14}
              strokeDasharray={`${len} ${circ - len}`}
              strokeDashoffset={-offset}
              transform={`rotate(-90 ${c} ${c})`}
            />
          );
          offset += len;
          return el;
        })}
        <text
          x={c}
          y={c}
          textAnchor="middle"
          dominantBaseline="middle"
          className="fill-zinc-200"
          style={{ fontSize: 18, fontFamily: "IBM Plex Mono, monospace" }}
        >
          {total}
        </text>
      </svg>
      <ul className="space-y-1.5 text-sm">
        {segments.map((seg) => (
          <li key={seg.label} className="flex items-center gap-2 text-zinc-300">
            <span className="inline-block h-2.5 w-2.5 rounded-sm" style={{ background: seg.color }} />
            <span className="font-mono text-xs text-zinc-400">{seg.label}</span>
            <span className="ml-auto font-mono text-xs">{seg.value}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function BarChart({
  rows,
  valueKey = "value",
  labelKey = "label",
  maxBars = 8,
}: {
  rows: Record<string, unknown>[];
  valueKey?: string;
  labelKey?: string;
  maxBars?: number;
}) {
  const sliced = rows.slice(0, maxBars);
  const max = Math.max(1, ...sliced.map((r) => Number(r[valueKey]) || 0));
  return (
    <div className="space-y-2">
      {sliced.map((r) => {
        const label = String(r[labelKey] ?? "");
        const value = Number(r[valueKey]) || 0;
        const pct = (value / max) * 100;
        return (
          <div key={label} className="space-y-1">
            <div className="flex justify-between gap-2 text-xs">
              <span className="truncate font-mono text-zinc-300">{label}</span>
              <span className="shrink-0 font-mono text-zinc-500">{value.toFixed?.(1) ?? value}%</span>
            </div>
            <div className="h-2 overflow-hidden rounded bg-zinc-900">
              <div className="h-full rounded bg-brand transition-all" style={{ width: `${pct}%` }} />
            </div>
          </div>
        );
      })}
      {sliced.length === 0 && <p className="text-sm text-zinc-500">No data</p>}
    </div>
  );
}
