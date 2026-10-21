import { X } from "lucide-react";
import { useEffect, useId, useRef, type ReactNode } from "react";
import { useFocusTrap } from "../hooks/useFocusTrap";

/** Right-hand slide-over panel for contextual detail (history, previews). */
export function Sheet({
  open,
  title,
  subtitle,
  onClose,
  children,
  width = "max-w-xl",
}: {
  open: boolean;
  title: ReactNode;
  subtitle?: ReactNode;
  onClose: () => void;
  children: ReactNode;
  width?: string;
}) {
  const titleId = useId();
  const panelRef = useRef<HTMLDivElement | null>(null);
  useFocusTrap(open, panelRef);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-[90]">
      <button
        type="button"
        className="absolute inset-0 bg-black/50 backdrop-blur-[2px] wt-animate-fade"
        aria-label="Close panel"
        onClick={onClose}
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className={`absolute inset-y-0 right-0 flex w-full ${width} flex-col border-l border-[var(--color-border-strong)] bg-[var(--color-surface-raised)] shadow-[var(--shadow-pop)] wt-animate-slide`}
      >
        <header className="flex items-start justify-between gap-3 border-b border-[var(--color-border)] px-5 py-4">
          <div className="min-w-0">
            <h2 id={titleId} className="truncate text-base font-semibold text-zinc-50">
              {title}
            </h2>
            {subtitle ? <p className="mt-0.5 text-xs text-zinc-500">{subtitle}</p> : null}
          </div>
          <button
            type="button"
            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
            aria-label="Close"
            onClick={onClose}
          >
            <X />
          </button>
        </header>
        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
      </div>
    </div>
  );
}
