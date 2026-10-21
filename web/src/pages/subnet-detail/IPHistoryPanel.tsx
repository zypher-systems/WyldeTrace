import { Sheet } from "../../components/Sheet";
import { StatusBadge } from "../../components/ui";

export type IPHistoryRow = {
  id: string;
  ip_id: string | null;
  address: string;
  hostname: string;
  status: string;
  mac_address: string;
  open_ports: string;
  action: string;
  actor_email: string;
  details: Record<string, unknown>;
  created_at: string;
};

/** Slide-over timeline of changes for one address. Closed when `historyAddr` is empty. */
export function IPHistoryPanel({
  historyAddr,
  history,
  onClose,
}: {
  historyAddr: string;
  history: IPHistoryRow[];
  onClose: () => void;
}) {
  return (
    <Sheet
      open={!!historyAddr}
      onClose={onClose}
      title={<span className="font-mono">{historyAddr}</span>}
      subtitle="Change history, newest first"
    >
      {history.length === 0 ? (
        <p className="py-10 text-center text-sm text-zinc-500">No history for this address yet</p>
      ) : (
        <ol className="relative space-y-5 border-l border-[var(--color-border-strong)] pl-5">
          {history.map((h) => (
            <li key={h.id} className="relative">
              <span className="absolute -left-[25px] top-1.5 h-2.5 w-2.5 rounded-full border-2 border-[var(--color-surface-raised)] bg-brand" />
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-mono text-xs font-medium text-brand">{h.action}</span>
                {h.status && <StatusBadge status={h.status} />}
              </div>
              <div className="mt-1 text-xs text-zinc-500">
                {new Date(h.created_at).toLocaleString()} · {h.actor_email || "system"}
              </div>
              <dl className="mt-2 grid grid-cols-[6rem_1fr] gap-x-3 gap-y-1 text-xs">
                <dt className="text-zinc-500">Hostname</dt>
                <dd className="text-zinc-300">{h.hostname || "—"}</dd>
                <dt className="text-zinc-500">MAC</dt>
                <dd className="font-mono text-zinc-300">{h.mac_address || "—"}</dd>
                {h.open_ports && (
                  <>
                    <dt className="text-zinc-500">Ports</dt>
                    <dd className="font-mono text-zinc-300">{h.open_ports}</dd>
                  </>
                )}
              </dl>
            </li>
          ))}
        </ol>
      )}
    </Sheet>
  );
}
