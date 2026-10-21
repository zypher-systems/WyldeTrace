import { X } from "lucide-react";
import { useEffect, useId, useRef, type ReactNode } from "react";
import { useFocusTrap } from "../hooks/useFocusTrap";

/** Centered modal shell for form dialogs (subnet/IP edit, etc.). */
export function FormDialog({
  open,
  title,
  onClose,
  children,
  wide,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  /** Wider panel for dense field grids. */
  wide?: boolean;
}) {
  const titleId = useId();
  const panelRef = useRef<HTMLDivElement | null>(null);
  useFocusTrap(open, panelRef);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
      <button
        type="button"
        className="absolute inset-0 bg-black/60 backdrop-blur-sm wt-animate-fade"
        aria-label="Dismiss"
        onClick={onClose}
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className={[
          "relative z-10 max-h-[min(90vh,48rem)] w-full overflow-y-auto rounded-2xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)] p-6 shadow-[var(--shadow-pop)] wt-animate-pop",
          wide ? "max-w-3xl" : "max-w-xl",
        ].join(" ")}
      >
        <div className="mb-4 flex items-start justify-between gap-3">
          <h2 id={titleId} className="text-base font-semibold tracking-tight text-zinc-50">
            {title}
          </h2>
          <button
            type="button"
            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm -mr-2 -mt-1"
            aria-label="Close"
            onClick={onClose}
          >
            <X />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
