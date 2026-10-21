import { Inbox } from "lucide-react";
import type { ReactNode } from "react";
import { Link } from "react-router-dom";

export function EmptyState({
  title,
  detail,
  actionLabel,
  actionTo,
  onAction,
  children,
  icon,
}: {
  icon?: ReactNode;
  title: string;
  detail?: string;
  actionLabel?: string;
  actionTo?: string;
  onAction?: () => void;
  children?: ReactNode;
}) {
  return (
    <div className="wt-card px-6 py-12 text-center">
      <div className="mx-auto mb-3 grid h-10 w-10 place-items-center rounded-xl border border-[var(--color-border-strong)] bg-[var(--color-surface)] text-zinc-500">
        {icon ?? <Inbox className="h-5 w-5" />}
      </div>
      <p className="text-sm font-medium text-zinc-200">{title}</p>
      {detail ? <p className="mx-auto mt-1 max-w-md text-xs text-zinc-500">{detail}</p> : null}
      {children}
      {actionLabel && actionTo && (
        <Link to={actionTo} className="wt-btn wt-btn-primary mt-4 inline-flex">
          {actionLabel}
        </Link>
      )}
      {actionLabel && onAction && !actionTo && (
        <button type="button" className="wt-btn wt-btn-primary mt-4" onClick={onAction}>
          {actionLabel}
        </button>
      )}
    </div>
  );
}
