import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useFocusTrap } from "../hooks/useFocusTrap";

export type ConfirmOptions = {
  title: string;
  body?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Destructive actions require an explicit click (Enter does not confirm). */
  danger?: boolean;
};

type ConfirmFn = (opts: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

export function useConfirm(): ConfirmFn {
  const fn = useContext(ConfirmContext);
  if (!fn) {
    return async (opts) => window.confirm(opts.body || opts.title);
  }
  return fn;
}

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const resolver = useRef<((v: boolean) => void) | null>(null);
  const titleId = useId();
  const panelRef = useRef<HTMLDivElement | null>(null);
  const cancelRef = useRef<HTMLButtonElement | null>(null);

  const confirm = useCallback<ConfirmFn>((next) => {
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
      setOpts(next);
    });
  }, []);

  function close(result: boolean) {
    resolver.current?.(result);
    resolver.current = null;
    setOpts(null);
  }

  useFocusTrap(!!opts, panelRef, cancelRef);

  useEffect(() => {
    if (!opts) return;
    const current = opts;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        close(false);
      } else if (e.key === "Enter" && !current.danger) {
        e.preventDefault();
        close(true);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [opts]);

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {opts && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
          <button
            type="button"
            className="absolute inset-0 bg-black/60 backdrop-blur-sm wt-animate-fade"
            aria-label="Dismiss"
            onClick={() => close(false)}
          />
          <div
            ref={panelRef}
            role="alertdialog"
            aria-modal="true"
            aria-labelledby={titleId}
            className="relative z-10 w-full max-w-md rounded-2xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)] p-6 shadow-[var(--shadow-pop)] wt-animate-pop"
          >
            <h2 id={titleId} className="text-base font-semibold tracking-tight text-zinc-50">
              {opts.title}
            </h2>
            {opts.body && <p className="mt-2 text-sm leading-relaxed text-zinc-400">{opts.body}</p>}
            <div className="mt-5 flex flex-wrap justify-end gap-2">
              <button
                ref={cancelRef}
                type="button"
                className="wt-btn wt-btn-secondary"
                onClick={() => close(false)}
              >
                {opts.cancelLabel || "Cancel"}
              </button>
              <button
                type="button"
                className={opts.danger ? "wt-btn wt-btn-danger" : "wt-btn wt-btn-primary"}
                onClick={() => close(true)}
              >
                {opts.confirmLabel || (opts.danger ? "Delete" : "Confirm")}
              </button>
            </div>
          </div>
        </div>
      )}
    </ConfirmContext.Provider>
  );
}
