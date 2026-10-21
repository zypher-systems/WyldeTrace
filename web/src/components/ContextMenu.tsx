import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
} from "react";

export type ContextMenuItem = {
  id: string;
  label: string;
  danger?: boolean;
  disabled?: boolean;
  onSelect: () => void;
};

type Pos = { x: number; y: number };

export function useContextMenu() {
  const [state, setState] = useState<{ pos: Pos; items: ContextMenuItem[] } | null>(null);

  function open(e: ReactMouseEvent, items: ContextMenuItem[]) {
    e.preventDefault();
    e.stopPropagation();
    const enabled = items.filter(Boolean);
    if (enabled.length === 0) return;
    setState({ pos: { x: e.clientX, y: e.clientY }, items: enabled });
  }

  function close() {
    setState(null);
  }

  return {
    open,
    close,
    menu: state ? (
      <ContextMenu pos={state.pos} items={state.items} onClose={close} />
    ) : null,
  };
}

function ContextMenu({
  pos,
  items,
  onClose,
}: {
  pos: Pos;
  items: ContextMenuItem[];
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  const [fixed, setFixed] = useState(pos);
  const [activeIdx, setActiveIdx] = useState(() => {
    const first = items.findIndex((i) => !i.disabled);
    return first >= 0 ? first : 0;
  });

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const pad = 8;
    let x = pos.x;
    let y = pos.y;
    if (x + rect.width > window.innerWidth - pad) x = Math.max(pad, window.innerWidth - rect.width - pad);
    if (y + rect.height > window.innerHeight - pad) y = Math.max(pad, window.innerHeight - rect.height - pad);
    setFixed({ x, y });
  }, [pos, items]);

  useEffect(() => {
    const buttons = ref.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]');
    buttons?.[activeIdx]?.focus();
  }, [activeIdx]);

  useEffect(() => {
    function move(delta: number) {
      setActiveIdx((cur) => {
        let next = cur;
        for (let i = 0; i < items.length; i++) {
          next = (next + delta + items.length) % items.length;
          if (!items[next]?.disabled) return next;
        }
        return cur;
      });
    }

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
        return;
      }
      if (e.key === "ArrowDown") {
        e.preventDefault();
        move(1);
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        move(-1);
        return;
      }
      if (e.key === "Home") {
        e.preventDefault();
        const first = items.findIndex((i) => !i.disabled);
        if (first >= 0) setActiveIdx(first);
        return;
      }
      if (e.key === "End") {
        e.preventDefault();
        for (let i = items.length - 1; i >= 0; i--) {
          if (!items[i]?.disabled) {
            setActiveIdx(i);
            break;
          }
        }
        return;
      }
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        const item = items[activeIdx];
        if (item && !item.disabled) {
          onClose();
          item.onSelect();
        }
      }
    }
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    window.addEventListener("keydown", onKey);
    window.addEventListener("mousedown", onDown);
    window.addEventListener("scroll", onClose, true);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("scroll", onClose, true);
    };
  }, [onClose, items, activeIdx]);

  return (
    <div
      ref={ref}
      role="menu"
      className="fixed z-[90] min-w-[12rem] rounded-xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)] p-1 shadow-[var(--shadow-pop)] wt-animate-pop"
      style={{ left: fixed.x, top: fixed.y }}
    >
      {items.map((item, idx) => (
        <button
          key={item.id}
          type="button"
          role="menuitem"
          disabled={item.disabled}
          tabIndex={idx === activeIdx ? 0 : -1}
          className={[
            "block w-full rounded-lg px-2.5 py-2 text-left text-[13px] outline-none disabled:opacity-40",
            idx === activeIdx ? (item.danger ? "bg-red-500/10" : "bg-[var(--color-surface-overlay)]") : "",
            item.danger ? "text-red-300" : "text-zinc-200",
          ].join(" ")}
          onMouseEnter={() => {
            if (!item.disabled) setActiveIdx(idx);
          }}
          onClick={() => {
            if (item.disabled) return;
            onClose();
            item.onSelect();
          }}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}

/** Convenience wrapper for a row that opens a context menu. */
export function ContextMenuTarget({
  children,
  items,
  className,
}: {
  children: ReactNode;
  items: ContextMenuItem[];
  className?: string;
}) {
  const { open, menu } = useContextMenu();
  return (
    <>
      <div className={className} onContextMenu={(e) => open(e, items)}>
        {children}
      </div>
      {menu}
    </>
  );
}
