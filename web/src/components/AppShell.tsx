import {
  Bell,
  ChevronsLeft,
  ChevronsRight,
  LogOut,
  Menu,
  Search,
  ShieldAlert,
  X,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { useFocusTrap } from "../hooks/useFocusTrap";
import { activeArea, areasFor, permsFor, type Area, type Counts } from "../navigation";
import { CommandPalette } from "./CommandPalette";
import { Kbd, RouteTabs, cx } from "./ui";
import { relativeTime } from "../lib/time";

type Notification = {
  id: string;
  kind: string;
  title: string;
  body: string;
  entity_type: string;
  entity_id: string;
  created_at: string;
  read_at: string | null;
};

const COLLAPSE_KEY = "wyldetrace_nav_collapsed";

function readCollapsed() {
  try {
    return localStorage.getItem(COLLAPSE_KEY) === "1";
  } catch {
    return false;
  }
}

function useShellData(reportOnly: boolean, canEdit: boolean) {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unread, setUnread] = useState(0);
  const [counts, setCounts] = useState<Counts>({ inbox: 0, requests: 0 });

  async function load() {
    if (reportOnly) {
      const reqs = await api<{ total: number }>(
        "/api/v1/ip-requests?page=1&page_size=1&status=pending",
      ).catch(() => ({ total: 0 }));
      setCounts({ inbox: 0, requests: reqs.total || 0 });
      return;
    }
    try {
      const [res, inbox, reqs] = await Promise.all([
        api<{ items: Notification[]; unread: number }>("/api/v1/notifications?page=1&page_size=12"),
        api<{ total: number }>("/api/v1/inbox?page=1&page_size=1").catch(() => ({ total: 0 })),
        canEdit
          ? api<{ pending: number }>("/api/v1/ip-requests/pending-count").catch(() => ({
              pending: 0,
            }))
          : api<{ total: number }>("/api/v1/ip-requests?page=1&page_size=1&status=pending").catch(
              () => ({ total: 0 }),
            ),
      ]);
      setNotifications(res.items);
      setUnread(res.unread);
      setCounts({
        inbox: inbox.total || 0,
        requests: "pending" in reqs ? reqs.pending || 0 : (reqs as { total?: number }).total || 0,
      });
    } catch {
      /* ignore */
    }
  }

  useEffect(() => {
    void load();
    const t = window.setInterval(() => void load(), 60000);
    return () => window.clearInterval(t);
  }, [reportOnly, canEdit]);

  return { notifications, unread, counts, reload: load };
}

export function AppShell() {
  const { user, logout } = useAuth();
  const location = useLocation();
  const perms = permsFor(user?.role);
  const areas = areasFor(perms);
  const area = activeArea(areas, location.pathname);
  const { notifications, unread, counts, reload } = useShellData(perms.reportOnly, perms.canEdit);
  const [navOpen, setNavOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(readCollapsed);
  const [paletteOpen, setPaletteOpen] = useState(false);

  useEffect(() => {
    setNavOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    try {
      localStorage.setItem(COLLAPSE_KEY, collapsed ? "1" : "0");
    } catch {
      /* ignore */
    }
  }, [collapsed]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((v) => !v);
      } else if (
        e.key === "/" &&
        !(e.target instanceof HTMLInputElement) &&
        !(e.target instanceof HTMLTextAreaElement) &&
        !(e.target instanceof HTMLSelectElement)
      ) {
        e.preventDefault();
        setPaletteOpen(true);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const sidebar = (mobile: boolean) => (
    <Sidebar
      areas={areas}
      active={area}
      counts={counts}
      collapsed={!mobile && collapsed}
      onToggleCollapsed={mobile ? undefined : () => setCollapsed((v) => !v)}
      email={user?.email ?? ""}
      role={user?.role ?? ""}
      onLogout={logout}
    />
  );

  return (
    <div className="flex min-h-screen">
      <aside
        className={cx(
          "sticky top-0 hidden h-screen shrink-0 border-r border-[var(--color-border)] bg-[color-mix(in_srgb,var(--color-surface)_80%,transparent)] backdrop-blur-xl transition-[width] duration-200 md:block",
          collapsed ? "w-[68px]" : "w-60",
        )}
      >
        {sidebar(false)}
      </aside>

      {navOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <button
            type="button"
            className="absolute inset-0 bg-black/60 wt-animate-fade"
            aria-label="Close navigation"
            onClick={() => setNavOpen(false)}
          />
          <aside className="relative z-10 h-full w-64 max-w-[85vw] border-r border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl">
            {sidebar(true)}
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 border-b border-[var(--color-border)] bg-[color-mix(in_srgb,var(--color-canvas)_78%,transparent)] backdrop-blur-xl">
          <div className="flex h-14 items-center gap-3 px-4 md:px-6">
            <button
              type="button"
              className="wt-btn wt-btn-ghost wt-btn-icon md:hidden"
              aria-label="Open navigation"
              aria-expanded={navOpen}
              onClick={() => setNavOpen(true)}
            >
              <Menu />
            </button>
            <div className="flex min-w-0 items-center gap-2">
              {area && <area.icon className="h-4 w-4 shrink-0 text-brand" />}
              <span className="truncate text-sm font-semibold text-zinc-100">
                {area?.label ?? "WyldeTrace"}
              </span>
              {user?.site_scoped && (
                <span
                  className="wt-badge wt-badge-info ml-1 hidden sm:inline-flex"
                  title="Unassigned subnets are hidden"
                >
                  {user.location_ids?.length ?? 0} site
                  {(user.location_ids?.length ?? 0) === 1 ? "" : "s"} in scope
                </span>
              )}
            </div>
            <div className="ml-auto flex items-center gap-2">
              <button
                type="button"
                onClick={() => setPaletteOpen(true)}
                className="group hidden h-9 w-72 items-center gap-2.5 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-sm text-zinc-500 transition hover:border-[var(--color-border-strong)] hover:text-zinc-300 lg:flex"
              >
                <Search className="h-4 w-4" />
                <span className="flex-1 text-left">
                  {perms.reportOnly ? "Jump to…" : "Search or jump to…"}
                </span>
                <Kbd>⌘K</Kbd>
              </button>
              <button
                type="button"
                className="wt-btn wt-btn-ghost wt-btn-icon lg:hidden"
                aria-label="Search"
                onClick={() => setPaletteOpen(true)}
              >
                <Search />
              </button>
              {!perms.reportOnly && (
                <NotificationsButton
                  notifications={notifications}
                  unread={unread}
                  reload={reload}
                />
              )}
            </div>
          </div>
          {area?.tabs && area.tabs.length > 1 && (
            <div className="px-4 md:px-6">
              <RouteTabs
                className="border-b-0"
                items={area.tabs.map((t) => ({
                  to: t.to,
                  label: t.label,
                  end: t.end,
                  icon: <t.icon />,
                  count: t.count ? counts[t.count] : undefined,
                }))}
              />
            </div>
          )}
        </header>

        {user?.using_default_password && (
          <Notice tone="warn">
            You’re still using the bootstrap password.{" "}
            <Link to="/settings/account" className="font-medium text-amber-100 underline">
              Change it
            </Link>{" "}
            before using this instance in production.
          </Notice>
        )}

        <main className="mx-auto w-full max-w-[1400px] flex-1 px-4 py-6 md:px-8 md:py-8">
          <Outlet />
        </main>
      </div>
      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
    </div>
  );
}

function Notice({ tone, children }: { tone: "warn" | "danger"; children: React.ReactNode }) {
  return (
    <div
      className={cx(
        "flex items-center gap-2 px-4 py-2 text-[13px] md:px-8",
        tone === "warn"
          ? "bg-amber-400/[0.07] text-amber-200/90"
          : "bg-red-500/[0.08] text-red-200/90",
      )}
    >
      <ShieldAlert className="h-4 w-4 shrink-0" />
      <span>{children}</span>
    </div>
  );
}

function Sidebar({
  areas,
  active,
  counts,
  collapsed,
  onToggleCollapsed,
  email,
  role,
  onLogout,
}: {
  areas: Area[];
  active: Area | undefined;
  counts: Counts;
  collapsed: boolean;
  onToggleCollapsed?: () => void;
  email: string;
  role: string;
  onLogout: () => void;
}) {
  const main = areas.filter((a) => a.key !== "settings");
  const settings = areas.find((a) => a.key === "settings");

  const item = (a: Area) => {
    const count = a.count ? counts[a.count] : 0;
    const isActive = active?.key === a.key;
    return (
      <NavLink
        key={a.key}
        to={a.to}
        title={collapsed ? a.label : undefined}
        className={cx(
          "group relative flex h-9 items-center gap-3 rounded-lg px-2.5 text-[13px] font-medium transition",
          isActive
            ? "bg-[var(--color-surface-overlay)] text-zinc-50 shadow-[inset_0_0_0_1px_var(--color-border-strong)]"
            : "text-zinc-400 hover:bg-[var(--color-surface-overlay)]/60 hover:text-zinc-100",
        )}
      >
        {isActive && (
          <span className="absolute -left-3 top-1/2 h-5 w-[3px] -translate-y-1/2 rounded-r bg-brand shadow-[0_0_10px_var(--color-brand)]" />
        )}
        <a.icon
          className={cx("h-[18px] w-[18px] shrink-0", isActive ? "text-brand" : "text-zinc-500 group-hover:text-zinc-300")}
        />
        {!collapsed && <span className="flex-1 truncate">{a.label}</span>}
        {count > 0 &&
          (collapsed ? (
            <span className="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-brand" />
          ) : (
            <span className="wt-count">{count > 99 ? "99+" : count}</span>
          ))}
      </NavLink>
    );
  };

  return (
    <div className="flex h-full flex-col">
      <div className={cx("flex h-14 items-center gap-2.5 px-4", collapsed && "justify-center px-0")}>
        <Link to="/" className="flex min-w-0 items-center gap-2.5">
          <img src="/logos/logo.svg" alt="WyldeTrace" className="h-7 w-7 shrink-0" />
          {!collapsed && (
            <span className="truncate text-[15px] font-semibold tracking-tight text-zinc-50">
              Wylde<span className="font-normal text-brand">Trace</span>
            </span>
          )}
        </Link>
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-3">{main.map(item)}</nav>

      <div className="space-y-1 border-t border-[var(--color-border)] px-3 py-3">
        {settings && item(settings)}
        {onToggleCollapsed && (
          <button
            type="button"
            onClick={onToggleCollapsed}
            title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            className="flex h-9 w-full items-center gap-3 rounded-lg px-2.5 text-[13px] text-zinc-500 transition hover:bg-[var(--color-surface-overlay)]/60 hover:text-zinc-200"
          >
            {collapsed ? (
              <ChevronsRight className="h-[18px] w-[18px]" />
            ) : (
              <ChevronsLeft className="h-[18px] w-[18px]" />
            )}
            {!collapsed && <span>Collapse</span>}
          </button>
        )}
        <div
          className={cx(
            "mt-2 flex items-center gap-2.5 rounded-lg p-1.5",
            collapsed && "flex-col",
          )}
        >
          <div
            className="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-gradient-to-br from-brand/80 to-[var(--color-accent)]/80 text-xs font-semibold uppercase text-[var(--color-brand-ink)]"
            title={email}
          >
            {email.slice(0, 1)}
          </div>
          {!collapsed && (
            <div className="min-w-0 flex-1">
              <div className="truncate text-xs font-medium text-zinc-200">{email}</div>
              <div className="font-mono text-[11px] text-zinc-500">{role}</div>
            </div>
          )}
          <button
            type="button"
            onClick={onLogout}
            title="Sign out"
            aria-label="Sign out"
            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
          >
            <LogOut />
          </button>
        </div>
      </div>
    </div>
  );
}

function NotificationsButton({
  notifications,
  unread,
  reload,
}: {
  notifications: Notification[];
  unread: number;
  reload: () => Promise<void>;
}) {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const rootRef = useRef<HTMLDivElement | null>(null);
  useFocusTrap(open, panelRef);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    function onDoc(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    }
    window.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onDoc);
    return () => {
      window.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onDoc);
    };
  }, [open]);

  function openNotification(n: Notification) {
    void api(`/api/v1/notifications/${n.id}/read`, { method: "POST" }).then(() => {
      void reload();
      setOpen(false);
      if (n.entity_id && n.entity_type === "subnet") navigate(`/subnets/${n.entity_id}`);
      else if (n.kind.startsWith("discovery.")) navigate("/inbox");
      else if (n.entity_id) navigate(`/subnets/${n.entity_id}`);
    });
  }

  return (
    <div className="relative" ref={rootRef}>
      <button
        type="button"
        className="wt-btn wt-btn-ghost wt-btn-icon relative"
        aria-label={unread > 0 ? `Notifications (${unread} unread)` : "Notifications"}
        aria-expanded={open}
        onClick={() => {
          setOpen((v) => !v);
          void reload();
        }}
      >
        <Bell />
        {unread > 0 && (
          <span className="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-brand shadow-[0_0_0_2px_var(--color-canvas)]" />
        )}
      </button>
      {open && (
        <div
          ref={panelRef}
          role="dialog"
          aria-label="Notifications"
          className="absolute right-0 z-50 mt-2 w-[22rem] overflow-hidden rounded-xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)] shadow-[var(--shadow-pop)] wt-animate-pop"
        >
          <div className="flex items-center justify-between border-b border-[var(--color-border)] px-4 py-3">
            <span className="text-sm font-semibold text-zinc-100">Notifications</span>
            <div className="flex items-center gap-1">
              {unread > 0 && (
                <button
                  type="button"
                  className="wt-btn-pill"
                  onClick={() =>
                    void api("/api/v1/notifications/read-all", { method: "POST" }).then(() => reload())
                  }
                >
                  Mark all read
                </button>
              )}
              <button
                type="button"
                className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                aria-label="Close"
                onClick={() => setOpen(false)}
              >
                <X />
              </button>
            </div>
          </div>
          <ul className="max-h-96 overflow-y-auto p-1.5">
            {notifications.map((n) => (
              <li key={n.id}>
                <button
                  type="button"
                  className="flex w-full gap-3 rounded-lg px-2.5 py-2.5 text-left hover:bg-[var(--color-surface-overlay)]"
                  onClick={() => openNotification(n)}
                >
                  <span
                    className={cx(
                      "mt-1.5 h-2 w-2 shrink-0 rounded-full",
                      n.read_at ? "bg-transparent" : "bg-brand",
                    )}
                  />
                  <span className="min-w-0 flex-1">
                    <span
                      className={cx(
                        "block text-[13px] font-medium",
                        n.read_at ? "text-zinc-400" : "text-zinc-100",
                      )}
                    >
                      {n.title}
                    </span>
                    <span className="mt-0.5 block text-xs text-zinc-500">{n.body}</span>
                    <span className="mt-1 block text-[11px] text-zinc-600">
                      {relativeTime(n.created_at)}
                    </span>
                  </span>
                </button>
              </li>
            ))}
            {notifications.length === 0 && (
              <li className="px-3 py-10 text-center text-sm text-zinc-500">You’re all caught up</li>
            )}
          </ul>
          <Link
            to="/inbox"
            className="block border-t border-[var(--color-border)] px-4 py-2.5 text-xs font-medium text-brand hover:bg-[var(--color-surface-overlay)]"
            onClick={() => setOpen(false)}
          >
            Open discovery inbox →
          </Link>
        </div>
      )}
    </div>
  );
}
