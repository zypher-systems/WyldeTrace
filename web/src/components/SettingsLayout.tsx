import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../auth";
import { permsFor, settingsTabs } from "../navigation";
import { cx } from "./ui";

export function SettingsLayout() {
  const { user } = useAuth();
  const tabs = settingsTabs(permsFor(user?.role));
  const groups = [...new Set(tabs.map((t) => t.group))];

  return (
    <div className="grid grid-cols-1 gap-8 lg:grid-cols-[220px_minmax(0,1fr)]">
      <nav className="min-w-0 lg:sticky lg:top-24 lg:self-start">
        <div className="flex gap-1 overflow-x-auto pb-2 lg:block lg:space-y-5 lg:overflow-visible lg:pb-0">
          {groups.map((g) => (
            <div key={g} className="contents lg:block">
              <p className="wt-eyebrow mb-1.5 hidden px-2.5 lg:block">{g}</p>
              <div className="contents lg:block lg:space-y-0.5">
                {tabs
                  .filter((t) => t.group === g)
                  .map((t) => (
                    <NavLink
                      key={t.to}
                      to={t.to}
                      className={({ isActive }) =>
                        cx(
                          "flex h-8 shrink-0 items-center gap-2.5 rounded-lg px-2.5 text-[13px] font-medium transition",
                          isActive
                            ? "bg-[var(--color-surface-overlay)] text-zinc-50"
                            : "text-zinc-400 hover:bg-[var(--color-surface-overlay)]/60 hover:text-zinc-100",
                        )
                      }
                    >
                      {({ isActive }) => (
                        <>
                          <t.icon className={cx("h-4 w-4", isActive ? "text-brand" : "text-zinc-500")} />
                          {t.label}
                        </>
                      )}
                    </NavLink>
                  ))}
              </div>
            </div>
          ))}
        </div>
      </nav>
      <div className="min-w-0">
        <Outlet />
      </div>
    </div>
  );
}
