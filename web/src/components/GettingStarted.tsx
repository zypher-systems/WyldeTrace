import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth";

const KEY = "wyldetrace_getting_started_v1";

type StepID = "password" | "site" | "subnet" | "discover" | "inbox";

type Persisted = {
  dismissed?: boolean;
  done: Partial<Record<StepID, boolean>>;
};

function load(): Persisted {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return { done: {} };
    const parsed = JSON.parse(raw) as Persisted;
    return { dismissed: !!parsed.dismissed, done: parsed.done || {} };
  } catch {
    return { done: {} };
  }
}

function save(next: Persisted) {
  localStorage.setItem(KEY, JSON.stringify(next));
}

export function GettingStarted({
  hasSubnets,
  hasDiscoveryRuns,
  inboxOpen,
  usingDefaultPassword,
}: {
  hasSubnets: boolean;
  hasDiscoveryRuns: boolean;
  inboxOpen: number;
  usingDefaultPassword?: boolean;
}) {
  const { user } = useAuth();
  const canGuide = user?.role === "admin" || user?.role === "editor";
  const [state, setState] = useState<Persisted>(() => load());

  useEffect(() => {
    setState((prev) => {
      const done = { ...prev.done };
      if (!usingDefaultPassword) done.password = true;
      if (hasSubnets) done.subnet = true;
      if (hasDiscoveryRuns) done.discover = true;
      if (inboxOpen === 0 && hasDiscoveryRuns) done.inbox = true;
      const next = { ...prev, done };
      save(next);
      return next;
    });
  }, [usingDefaultPassword, hasSubnets, hasDiscoveryRuns, inboxOpen]);

  const isAdmin = user?.role === "admin";
  const steps = useMemo(
    () =>
      [
        {
          id: "password" as const,
          label: "Change the bootstrap password",
          to: "/settings/account",
          hint: "Settings → Account & security",
        },
        {
          id: "site" as const,
          label: isAdmin ? "Confirm your primary site" : "Confirm your assigned site",
          to: "/locations",
          hint: isAdmin
            ? "Rename HQ or add a location"
            : "Check your site membership (ask an admin if empty)",
        },
        {
          id: "subnet" as const,
          label: "Open or create a /24",
          to: "/subnets",
          hint: "Start with Lab LAN or add your CIDR",
        },
        {
          id: "discover" as const,
          label: "Run one discovery",
          to: "/discovery",
          hint: "Create a job from a template, then Run now",
        },
        {
          id: "inbox" as const,
          label: "Review the inbox",
          to: "/inbox",
          hint: "Claim new hosts or dismiss noise",
        },
      ] as const,
    [isAdmin],
  );

  if (!canGuide || state.dismissed) return null;

  const completed = steps.filter((s) => state.done[s.id]).length;
  if (completed >= steps.length) return null;

  function toggle(id: StepID) {
    setState((prev) => {
      const next = {
        ...prev,
        done: { ...prev.done, [id]: !prev.done[id] },
      };
      save(next);
      return next;
    });
  }

  function dismiss() {
    const next = { ...state, dismissed: true };
    save(next);
    setState(next);
  }

  const pct = Math.round((completed / steps.length) * 100);
  return (
    <div className="wt-card relative overflow-hidden p-5">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(600px_200px_at_0%_0%,rgb(46_230_195/0.08),transparent)]" />
      <div className="relative flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-zinc-100">Set up WyldeTrace</h2>
          <p className="mt-0.5 text-xs text-zinc-500">
            {completed} of {steps.length} done — a useful inventory is a few minutes away
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div className="wt-meter w-32">
            <span style={{ width: `${pct}%` }} />
          </div>
          <button type="button" className="wt-btn-pill" onClick={dismiss}>
            Dismiss
          </button>
        </div>
      </div>
      <ol className="relative mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
        {steps.map((s, i) => {
          const done = !!state.done[s.id];
          return (
            <li
              key={s.id}
              className={`group flex items-start gap-2.5 rounded-xl border p-3 transition ${
                done
                  ? "border-[var(--color-border)] bg-transparent"
                  : "border-[var(--color-border-strong)] bg-[var(--color-surface)] hover:border-brand/40"
              }`}
            >
              <button
                type="button"
                className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-[10px] font-semibold ${
                  done
                    ? "border-brand bg-brand text-[var(--color-brand-ink)]"
                    : "border-zinc-600 text-zinc-400 hover:border-brand"
                }`}
                title={done ? "Mark incomplete" : "Mark done"}
                aria-label={done ? `Mark “${s.label}” incomplete` : `Mark “${s.label}” done`}
                onClick={() => toggle(s.id)}
              >
                {done ? "✓" : i + 1}
              </button>
              <Link to={s.to} className="min-w-0 flex-1">
                <div
                  className={`text-[13px] font-medium ${done ? "text-zinc-500 line-through" : "text-zinc-100 group-hover:text-brand"}`}
                >
                  {s.label}
                </div>
                <div className="mt-0.5 text-[11px] leading-snug text-zinc-500">{s.hint}</div>
              </Link>
            </li>
          );
        })}
      </ol>
    </div>
  );
}
