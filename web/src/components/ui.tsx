import type { ComponentPropsWithoutRef, ReactNode } from "react";
import { NavLink } from "react-router-dom";

export function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

export function Button({
  variant = "secondary",
  size,
  icon,
  className,
  children,
  type = "button",
  ...rest
}: ComponentPropsWithoutRef<"button"> & {
  variant?: ButtonVariant;
  size?: "sm";
  icon?: ReactNode;
}) {
  return (
    <button
      type={type}
      className={cx(
        "wt-btn",
        `wt-btn-${variant}`,
        size === "sm" && "wt-btn-sm",
        !!icon && !children && "wt-btn-icon",
        className,
      )}
      {...rest}
    >
      {icon}
      {children}
    </button>
  );
}

export function Card({
  className,
  children,
  interactive,
  ...rest
}: ComponentPropsWithoutRef<"div"> & { interactive?: boolean }) {
  return (
    <div className={cx("wt-card", interactive && "wt-card-interactive", className)} {...rest}>
      {children}
    </div>
  );
}

/** Card with a title row and optional actions; body gets consistent padding. */
export function Section({
  title,
  description,
  actions,
  children,
  className,
  bodyClassName,
  id,
}: {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  className?: string;
  bodyClassName?: string;
  id?: string;
}) {
  return (
    <section id={id} className={cx("wt-card", className)}>
      <header className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--color-border)] px-5 py-4">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-zinc-100">{title}</h2>
          {description ? <p className="mt-0.5 text-xs text-zinc-500">{description}</p> : null}
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </header>
      <div className={cx("px-5 py-4", bodyClassName)}>{children}</div>
    </section>
  );
}

type BadgeTone = "neutral" | "brand" | "info" | "warn" | "danger" | "accent";

export function Badge({
  tone = "neutral",
  className,
  children,
}: {
  tone?: BadgeTone;
  className?: string;
  children: ReactNode;
}) {
  return (
    <span className={cx("wt-badge", tone !== "neutral" && `wt-badge-${tone}`, className)}>
      {children}
    </span>
  );
}

const statusTone: Record<string, BadgeTone> = {
  assigned: "brand",
  dhcp: "info",
  reserved: "warn",
  available: "neutral",
  deleted: "danger",
  ok: "brand",
  success: "brand",
  running: "info",
  pending: "warn",
  approved: "brand",
  denied: "danger",
  failed: "danger",
  error: "danger",
};

const statusDot: Record<string, string> = {
  assigned: "bg-[var(--color-status-assigned)]",
  dhcp: "bg-[var(--color-status-dhcp)]",
  reserved: "bg-[var(--color-status-reserved)]",
  available: "bg-zinc-500",
};

export function StatusBadge({ status }: { status: string }) {
  return (
    <Badge tone={statusTone[status] ?? "neutral"}>
      {statusDot[status] ? <span className={cx("wt-status-dot", statusDot[status])} /> : null}
      {status}
    </Badge>
  );
}

export function Meter({ value, className }: { value: number; className?: string }) {
  const pct = Math.max(0, Math.min(100, value));
  return (
    <div className={cx("wt-meter", pct >= 90 ? "crit" : pct >= 75 && "warn", className)}>
      <span style={{ width: `${pct}%` }} />
    </div>
  );
}

export function Stat({
  label,
  value,
  hint,
  icon,
}: {
  label: ReactNode;
  value: ReactNode;
  hint?: ReactNode;
  icon?: ReactNode;
}) {
  return (
    <div className="min-w-0">
      <div className="flex items-center gap-1.5 text-zinc-500">
        {icon}
        <span className="wt-eyebrow">{label}</span>
      </div>
      <div className="mt-1.5 text-2xl font-semibold tracking-tight text-zinc-50 tabular-nums">
        {value}
      </div>
      {hint ? <div className="mt-0.5 truncate text-xs text-zinc-500">{hint}</div> : null}
    </div>
  );
}

export function Field({
  label,
  hint,
  children,
  className,
}: {
  label: ReactNode;
  hint?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <label className={cx("block space-y-1.5", className)}>
      <span className="wt-label">{label}</span>
      {children}
      {hint ? <span className="wt-hint block">{hint}</span> : null}
    </label>
  );
}

export type TabItem = {
  to: string;
  label: ReactNode;
  icon?: ReactNode;
  end?: boolean;
  count?: number;
};

/** Route-driven tab strip. */
export function RouteTabs({ items, className }: { items: TabItem[]; className?: string }) {
  return (
    <nav className={cx("wt-tabs", className)}>
      {items.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          end={t.end}
          className={({ isActive }) => cx("wt-tab", isActive && "active")}
        >
          {t.icon}
          {t.label}
          {t.count ? <span className="wt-badge h-5 px-1.5">{t.count}</span> : null}
        </NavLink>
      ))}
    </nav>
  );
}

/** State-driven tab strip. */
export function Tabs<T extends string>({
  value,
  onChange,
  items,
  className,
}: {
  value: T;
  onChange: (v: T) => void;
  items: Array<{ value: T; label: ReactNode; icon?: ReactNode; count?: number }>;
  className?: string;
}) {
  return (
    <div role="tablist" className={cx("wt-tabs", className)}>
      {items.map((t) => (
        <button
          key={t.value}
          type="button"
          role="tab"
          aria-selected={value === t.value}
          className="wt-tab"
          onClick={() => onChange(t.value)}
        >
          {t.icon}
          {t.label}
          {t.count != null && t.count > 0 ? (
            <span className="wt-badge h-5 px-1.5">{t.count}</span>
          ) : null}
        </button>
      ))}
    </div>
  );
}

export function Segmented<T extends string>({
  value,
  onChange,
  items,
  label,
}: {
  value: T;
  onChange: (v: T) => void;
  items: Array<{ value: T; label: ReactNode; icon?: ReactNode }>;
  label?: string;
}) {
  return (
    <div className="wt-segment" role="group" aria-label={label}>
      {items.map((it) => (
        <button
          key={it.value}
          type="button"
          aria-pressed={value === it.value}
          onClick={() => onChange(it.value)}
        >
          {it.icon}
          {it.label}
        </button>
      ))}
    </div>
  );
}

export function Kbd({ children }: { children: ReactNode }) {
  return <kbd className="wt-kbd">{children}</kbd>;
}

/** Accessible on/off toggle; `onChange` receives the next value. */
export function Switch({
  checked,
  onChange,
  label,
  disabled,
}: {
  checked: boolean;
  onChange?: (next: boolean) => void;
  label: string;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={() => onChange?.(!checked)}
      className={cx(
        "relative inline-flex h-5 w-9 shrink-0 items-center rounded-full border transition-colors disabled:cursor-default disabled:opacity-60",
        checked
          ? "border-brand/40 bg-brand/80"
          : "border-[var(--color-border-strong)] bg-[var(--color-surface-overlay)]",
      )}
    >
      <span
        className={cx(
          "inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform",
          checked ? "translate-x-[18px]" : "translate-x-[2px]",
        )}
      />
    </button>
  );
}
