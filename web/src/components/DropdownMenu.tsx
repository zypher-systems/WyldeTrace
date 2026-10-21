import { ChevronDown, MoreHorizontal } from "lucide-react";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { cx } from "./ui";

export type DropdownItem = {
  label: string;
  onClick: () => void;
  icon?: ReactNode;
  danger?: boolean;
  /** Draw a separator above this item. */
  separated?: boolean;
  disabled?: boolean;
};

type Props = {
  label: ReactNode;
  items: DropdownItem[];
  accent?: boolean;
  align?: "left" | "right";
  /** Icon-only trigger ("⋯"); `label` becomes the accessible name. */
  iconOnly?: boolean;
  icon?: ReactNode;
};

export function DropdownMenu({ label, items, accent = false, align = "right", iconOnly, icon }: Props) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const menuId = useId();

  useEffect(() => {
    if (!open) return;
    function onDoc(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const btns = [...(menuRef.current?.querySelectorAll<HTMLButtonElement>("button:not(:disabled)") ?? [])];
        const i = btns.indexOf(document.activeElement as HTMLButtonElement);
        const next = e.key === "ArrowDown" ? (i + 1) % btns.length : (i - 1 + btns.length) % btns.length;
        btns[next]?.focus();
      }
    }
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div className="relative" ref={rootRef}>
      <button
        type="button"
        className={cx(
          "wt-btn",
          accent ? "wt-btn-secondary text-brand" : "wt-btn-secondary",
          iconOnly && "wt-btn-icon",
        )}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        aria-label={iconOnly && typeof label === "string" ? label : undefined}
        title={iconOnly && typeof label === "string" ? label : undefined}
        onClick={() => setOpen((v) => !v)}
      >
        {iconOnly ? (
          icon ?? <MoreHorizontal />
        ) : (
          <>
            {icon}
            {label}
            <ChevronDown className="!h-3.5 !w-3.5 text-zinc-500" />
          </>
        )}
      </button>
      {open && (
        <div
          id={menuId}
          ref={menuRef}
          role="menu"
          className={cx(
            "absolute z-40 mt-1.5 min-w-[13rem] overflow-hidden rounded-xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)] p-1 shadow-[var(--shadow-pop)] wt-animate-pop",
            align === "right" ? "right-0" : "left-0",
          )}
        >
          {items.map((item) => (
            <div key={item.label}>
              {item.separated && <div className="my-1 h-px bg-[var(--color-border)]" />}
              <button
                type="button"
                role="menuitem"
                disabled={item.disabled}
                className={cx(
                  "flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-[13px] outline-none transition disabled:opacity-40 [&_svg]:h-4 [&_svg]:w-4",
                  item.danger
                    ? "text-red-300 hover:bg-red-500/10 focus:bg-red-500/10"
                    : "text-zinc-300 hover:bg-[var(--color-surface-overlay)] hover:text-zinc-50 focus:bg-[var(--color-surface-overlay)] focus:text-zinc-50",
                )}
                onClick={() => {
                  setOpen(false);
                  item.onClick();
                }}
              >
                {item.icon && <span className="text-zinc-500">{item.icon}</span>}
                {item.label}
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
