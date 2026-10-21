import { CheckCircle2, Info, XCircle } from "lucide-react";
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type ToastTone = "info" | "success" | "error";

export type ToastAction = {
  label: string;
  onClick: () => void;
};

export type ToastOptions = {
  tone?: ToastTone;
  action?: ToastAction;
  /** Defaults to 4000ms, or 7000ms when an action is present. */
  durationMs?: number;
};

type ToastItem = {
  id: number;
  message: string;
  tone: ToastTone;
  action?: ToastAction;
};

export type ToastFn = {
  (message: string, tone?: ToastTone): void;
  (message: string, options: ToastOptions): void;
};

const ToastContext = createContext<ToastFn | null>(null);

function normalizeOpts(toneOrOpts?: ToastTone | ToastOptions): ToastOptions {
  if (toneOrOpts == null) return { tone: "info" };
  if (typeof toneOrOpts === "string") return { tone: toneOrOpts };
  return toneOrOpts;
}

export function useToast(): ToastFn {
  const fn = useContext(ToastContext);
  if (!fn) {
    return ((message: string) => {
      console.info(message);
    }) as ToastFn;
  }
  return fn;
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const toast = useCallback<ToastFn>((message, toneOrOpts) => {
    const opts = normalizeOpts(toneOrOpts);
    const tone = opts.tone ?? "info";
    const id = Date.now() + Math.floor(Math.random() * 1000);
    const duration =
      opts.durationMs ?? (opts.action ? 7000 : 4000);
    setItems((prev) => [...prev.slice(-4), { id, message, tone, action: opts.action }]);
    window.setTimeout(() => {
      setItems((prev) => prev.filter((t) => t.id !== id));
    }, duration);
  }, []);

  const value = useMemo(() => toast, [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        className="pointer-events-none fixed bottom-4 right-4 z-[120] flex w-full max-w-sm flex-col gap-2 px-4"
        aria-live="polite"
        aria-relevant="additions"
      >
        {items.map((t) => (
          <div
            key={t.id}
            role="status"
            className="pointer-events-auto flex items-start gap-3 rounded-xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)]/95 px-3.5 py-3 text-sm text-zinc-100 shadow-[var(--shadow-pop)] backdrop-blur wt-animate-pop"
          >
            {t.tone === "error" ? (
              <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-400" />
            ) : t.tone === "success" ? (
              <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-brand" />
            ) : (
              <Info className="mt-0.5 h-4 w-4 shrink-0 text-sky-300" />
            )}
            <p className="min-w-0 flex-1 leading-snug">{t.message}</p>
            {t.action && (
              <button
                type="button"
                className="shrink-0 text-xs font-medium text-brand hover:underline"
                onClick={() => {
                  t.action?.onClick();
                  setItems((prev) => prev.filter((x) => x.id !== t.id));
                }}
              >
                {t.action.label}
              </button>
            )}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
