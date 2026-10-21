import { Command } from "cmdk";
import { ArrowRight, Building2, CornerDownLeft, GitBranch, Search, Server } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type SearchResponse } from "../api";
import { useAuth } from "../auth";
import { areasFor, permsFor, settingsTabs } from "../navigation";
import { Kbd } from "./ui";

type Props = { open: boolean; onOpenChange: (open: boolean) => void };

const itemClass =
  "flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 text-sm text-zinc-300 data-[selected=true]:bg-[var(--color-surface-overlay)] data-[selected=true]:text-zinc-50";
const groupClass =
  "[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:pb-1.5 [&_[cmdk-group-heading]]:pt-3 [&_[cmdk-group-heading]]:text-[11px] [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-wider [&_[cmdk-group-heading]]:text-zinc-500";

export function CommandPalette({ open, onOpenChange }: Props) {
  const navigate = useNavigate();
  const { user } = useAuth();
  const perms = permsFor(user?.role);
  const [q, setQ] = useState("");
  const [result, setResult] = useState<SearchResponse | null>(null);
  const [busy, setBusy] = useState(false);

  const destinations = useMemo(() => {
    const out: Array<{ to: string; label: string; group: string; icon: typeof Search }> = [];
    for (const a of areasFor(perms)) {
      if (a.key === "settings") continue;
      if (a.tabs) {
        for (const t of a.tabs) out.push({ to: t.to, label: t.label, group: a.label, icon: t.icon });
      } else {
        out.push({ to: a.to, label: a.label, group: a.label, icon: a.icon });
      }
    }
    for (const t of settingsTabs(perms)) {
      out.push({ to: t.to, label: t.label, group: "Settings", icon: t.icon });
    }
    return out;
  }, [perms.role]);

  useEffect(() => {
    if (!open) {
      setQ("");
      setResult(null);
    }
  }, [open]);

  useEffect(() => {
    const term = q.trim();
    if (perms.reportOnly || term.length < 2) {
      setResult(null);
      return;
    }
    setBusy(true);
    const t = window.setTimeout(() => {
      api<SearchResponse>(`/api/v1/search?q=${encodeURIComponent(term)}`)
        .then(setResult)
        .catch(() => setResult(null))
        .finally(() => setBusy(false));
    }, 180);
    return () => window.clearTimeout(t);
  }, [q, perms.reportOnly]);

  function go(to: string) {
    onOpenChange(false);
    navigate(to);
  }

  const hasHits =
    !!result && result.subnets.length + result.ips.length + result.locations.length > 0;

  return (
    <Command.Dialog
      open={open}
      onOpenChange={onOpenChange}
      label="Command palette"
      shouldFilter={!hasHits}
      overlayClassName="fixed inset-0 z-[110] bg-black/60 backdrop-blur-sm wt-animate-fade"
      contentClassName="fixed left-1/2 top-[12vh] z-[111] w-[min(640px,calc(100vw-2rem))] -translate-x-1/2 overflow-hidden rounded-2xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)] shadow-[var(--shadow-pop)] wt-animate-pop"
    >
      <div className="flex items-center gap-3 border-b border-[var(--color-border)] px-4">
        <Search className="h-4 w-4 shrink-0 text-zinc-500" />
        <Command.Input
          value={q}
          onValueChange={setQ}
          placeholder={
            perms.reportOnly ? "Jump to…" : "Search IPs, hostnames, subnets, sites — or jump to a page"
          }
          className="wt-bare h-14 w-full bg-transparent focus-visible:outline-none text-[15px] text-zinc-100 outline-none placeholder:text-zinc-500"
        />
        {busy && (
          <span className="h-3.5 w-3.5 shrink-0 animate-spin rounded-full border-2 border-zinc-600 border-t-brand" />
        )}
        <Kbd>esc</Kbd>
      </div>
      <Command.List className="max-h-[min(60vh,440px)] overflow-y-auto p-2">
        <Command.Empty className="px-3 py-8 text-center text-sm text-zinc-500">
          {q.trim().length >= 2 && busy ? "Searching…" : "No matches"}
        </Command.Empty>

        {hasHits && (
          <>
            {result!.ips.length > 0 && (
              <Command.Group heading="IP addresses" className={groupClass}>
                {result!.ips.slice(0, 8).map((ip) => (
                  <Command.Item
                    key={ip.id}
                    value={`ip-${ip.id}`}
                    className={itemClass}
                    onSelect={() =>
                      go(`/subnets/${ip.subnet_id}?q=${encodeURIComponent(ip.address)}`)
                    }
                  >
                    <Server className="h-4 w-4 text-zinc-500" />
                    <span className="font-mono text-zinc-100">{ip.address}</span>
                    <span className="truncate text-zinc-500">{ip.hostname}</span>
                    <span className="ml-auto text-xs text-zinc-600">{ip.status}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            )}
            {result!.subnets.length > 0 && (
              <Command.Group heading="Subnets" className={groupClass}>
                {result!.subnets.slice(0, 8).map((s) => (
                  <Command.Item
                    key={s.id}
                    value={`subnet-${s.id}`}
                    className={itemClass}
                    onSelect={() => go(`/subnets/${s.id}`)}
                  >
                    <GitBranch className="h-4 w-4 text-zinc-500" />
                    <span className="font-mono text-zinc-100">{s.cidr}</span>
                    <span className="truncate text-zinc-500">{s.name}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            )}
            {result!.locations.length > 0 && (
              <Command.Group heading="Sites" className={groupClass}>
                {result!.locations.slice(0, 5).map((l) => (
                  <Command.Item
                    key={l.id}
                    value={`loc-${l.id}`}
                    className={itemClass}
                    onSelect={() => go(`/locations/${l.id}`)}
                  >
                    <Building2 className="h-4 w-4 text-zinc-500" />
                    <span className="text-zinc-100">{l.name}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            )}
            <Command.Group className={groupClass}>
              <Command.Item
                value="all-results"
                className={itemClass}
                onSelect={() => go(`/search?q=${encodeURIComponent(q.trim())}`)}
              >
                <ArrowRight className="h-4 w-4 text-zinc-500" />
                See all results for “{q.trim()}”
              </Command.Item>
            </Command.Group>
          </>
        )}

        {!hasHits && (
          <Command.Group heading="Go to" className={groupClass}>
            {destinations.map((d) => (
              <Command.Item
                key={d.to}
                value={`${d.group} ${d.label}`}
                className={itemClass}
                onSelect={() => go(d.to)}
              >
                <d.icon className="h-4 w-4 text-zinc-500" />
                <span>{d.label}</span>
                {d.group !== d.label && (
                  <span className="ml-auto text-xs text-zinc-600">{d.group}</span>
                )}
              </Command.Item>
            ))}
          </Command.Group>
        )}
      </Command.List>
      <div className="flex items-center gap-4 border-t border-[var(--color-border)] px-4 py-2.5 text-[11px] text-zinc-500">
        <span className="inline-flex items-center gap-1.5">
          <Kbd>↑</Kbd>
          <Kbd>↓</Kbd> navigate
        </span>
        <span className="inline-flex items-center gap-1.5">
          <Kbd>
            <CornerDownLeft className="h-3 w-3" />
          </Kbd>{" "}
          open
        </span>
      </div>
    </Command.Dialog>
  );
}
