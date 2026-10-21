import { useEffect, useMemo, useState, type FormEvent } from "react";
import { ArrowLeft, CheckCircle2, Info, Network, Plus, ScanSearch } from "lucide-react";
import { Link, Navigate } from "react-router-dom";
import { api, type Location } from "../api";
import { useAuth } from "../auth";
import { PageHeader } from "../components/PageHeader";
import { Badge, Button, Field, cx } from "../components/ui";

type Candidate = {
  cidr: string;
  alive: number;
  sampled: number;
  sample_ips: string[];
  alive_ips?: string[];
  likely_used: boolean;
  exists_in_db: boolean;
  existing_id?: string;
  suggested: boolean;
};

type DiscoverResponse = {
  parent_cidr: string;
  child_prefix: number;
  total: number;
  likely_used: number;
  suggested: number;
  candidates: Candidate[];
};

const MAX_CHILDREN = 256;

function estimateChildren(cidr: string, childPrefix: number): number | null {
  try {
    const slash = cidr.indexOf("/");
    if (slash < 0) return null;
    const parentBits = Number(cidr.slice(slash + 1));
    if (!Number.isFinite(parentBits) || childPrefix <= parentBits) return null;
    const delta = childPrefix - parentBits;
    if (delta > 16) return Infinity;
    return 2 ** delta;
  } catch {
    return null;
  }
}

export function DiscoverSubnetsPage() {
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [cidr, setCidr] = useState("10.0.0.0/22");
  const [childPrefix, setChildPrefix] = useState(24);
  const [sampleHosts, setSampleHosts] = useState(3);
  const [locationID, setLocationID] = useState("");
  const [locations, setLocations] = useState<Location[]>([]);
  const [result, setResult] = useState<DiscoverResponse | null>(null);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [showQuiet, setShowQuiet] = useState(false);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    if (!canEdit) return;
    void api<Location[]>("/api/v1/locations")
      .then(setLocations)
      .catch(() => setLocations([]));
  }, [canEdit]);

  useEffect(() => {
    if (!busy) {
      setElapsed(0);
      return;
    }
    const started = Date.now();
    const t = window.setInterval(() => setElapsed(Math.floor((Date.now() - started) / 1000)), 250);
    return () => window.clearInterval(t);
  }, [busy]);

  const estimated = useMemo(() => estimateChildren(cidr.trim(), childPrefix), [cidr, childPrefix]);
  const estimateOk = estimated != null && Number.isFinite(estimated) && estimated <= MAX_CHILDREN && estimated > 0;
  const estimateLabel =
    estimated == null
      ? "Enter a valid supernet CIDR"
      : !Number.isFinite(estimated) || estimated > MAX_CHILDREN
        ? `Too many children (max ${MAX_CHILDREN}) — tighten the supernet or raise child prefix`
        : estimated <= 0
          ? "Child prefix must be longer than the supernet"
          : `Will probe ~${estimated} /${childPrefix} children`;

  if (user?.role === "reportview") return <Navigate to="/" replace />;
  if (!canEdit) return <Navigate to="/subnets" replace />;

  async function probe() {
    if (!estimateOk) {
      setError(estimateLabel);
      return;
    }
    setBusy(true);
    setError("");
    setMsg("");
    try {
      const res = await api<DiscoverResponse>("/api/v1/scanner/discover-subnets", {
        method: "POST",
        body: JSON.stringify({
          cidr,
          child_prefix: childPrefix,
          sample_hosts: sampleHosts,
          location_id: locationID || undefined,
        }),
      });
      setResult(res);
      const pick: Record<string, boolean> = {};
      for (const c of res.candidates) {
        if (c.suggested) pick[c.cidr] = true;
      }
      setSelected(pick);
      setMsg(
        `Probed ${res.total} /${res.child_prefix} children · ${res.likely_used} with replies · ${res.suggested} new suggestions`,
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Probe failed");
    } finally {
      setBusy(false);
    }
  }

  async function onProbe(e: FormEvent) {
    e.preventDefault();
    setResult(null);
    await probe();
  }

  async function onCommit() {
    if (!result) return;
    const cidrs = result.candidates
      .filter((c) => selected[c.cidr] && !c.exists_in_db)
      .map((c) => ({ cidr: c.cidr, name: c.cidr }));
    if (cidrs.length === 0) {
      setError("Select at least one new candidate");
      return;
    }
    setBusy(true);
    try {
      const res = await api<{ created: number; skipped: number; errors: string[] }>(
        "/api/v1/scanner/discover-subnets/commit",
        {
          method: "POST",
          body: JSON.stringify({
            parent_cidr: result.parent_cidr,
            location_id: locationID || undefined,
            cidrs,
          }),
        },
      );
      setMsg(`Created ${res.created}, skipped ${res.skipped}`);
      if (res.errors?.length) setError(res.errors.slice(0, 3).join("; "));
      await probe();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Commit failed");
      setBusy(false);
    }
  }

  const visible = result
    ? result.candidates.filter((c) => showQuiet || c.likely_used || c.exists_in_db || selected[c.cidr])
    : [];

  function selectSuggested() {
    if (!result) return;
    const pick: Record<string, boolean> = {};
    for (const c of result.candidates) {
      if (c.suggested) pick[c.cidr] = true;
    }
    setSelected(pick);
  }

  const newSelected = result
    ? result.candidates.filter((c) => selected[c.cidr] && !c.exists_in_db).length
    : 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Find subnets"
        subtitle="Probe child prefixes under a supernet and add the networks that reply"
        actions={
          <Link to="/discovery" className="wt-btn wt-btn-ghost">
            <ArrowLeft />
            Scan jobs
          </Link>
        }
      />

      <form onSubmit={onProbe} className="wt-card p-5">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-[minmax(0,1.4fr)_repeat(3,minmax(0,1fr))]">
          <Field label="Supernet CIDR" hint="e.g. 10.0.0.0/16 → /24s">
            <input
              className="wt-input w-full font-mono"
              value={cidr}
              onChange={(e) => setCidr(e.target.value)}
              placeholder="10.0.0.0/22"
              required
            />
          </Field>
          <Field label="Child prefix">
            <input
              type="number"
              min={1}
              max={32}
              className="wt-input w-full"
              value={childPrefix}
              onChange={(e) => setChildPrefix(Number(e.target.value))}
            />
          </Field>
          <Field label="Sample hosts" hint="Pinged per child (1–8)">
            <input
              type="number"
              min={1}
              max={8}
              className="wt-input w-full"
              value={sampleHosts}
              onChange={(e) => setSampleHosts(Number(e.target.value))}
            />
          </Field>
          {locations.length > 0 ? (
            <Field label="Assign site">
              <select
                className="wt-input w-full"
                value={locationID}
                onChange={(e) => setLocationID(e.target.value)}
              >
                <option value="">None</option>
                {locations.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name}
                  </option>
                ))}
              </select>
            </Field>
          ) : (
            <div />
          )}
        </div>
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-[var(--color-border)] pt-4">
          <p
            className={cx(
              "inline-flex items-center gap-1.5 text-xs",
              estimateOk ? "text-zinc-400" : "text-amber-300",
            )}
            title={`Max ${MAX_CHILDREN} children per probe. Quiet children are hidden until you show them.`}
          >
            <Info className="h-3.5 w-3.5 shrink-0" />
            {estimateLabel}
            {estimateOk && (
              <span className="text-zinc-500">
                · {sampleHosts} sample host{sampleHosts === 1 ? "" : "s"} each
              </span>
            )}
          </p>
          <Button type="submit" variant="primary" icon={<ScanSearch />} disabled={busy || !estimateOk}>
            {busy ? `Probing… ${elapsed}s` : "Probe children"}
          </Button>
        </div>
        {error && <p className="mt-3 text-sm text-red-300">{error}</p>}
      </form>

      {result ? (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-sm text-zinc-100">{result.parent_cidr}</span>
            <Badge>
              {result.total} × /{result.child_prefix}
            </Badge>
            <Badge tone="info">{result.likely_used} replying</Badge>
            <Badge tone="brand">{result.suggested} new</Badge>
            {msg && (
              <span className="inline-flex items-center gap-1 text-xs text-zinc-400">
                <CheckCircle2 className="h-3.5 w-3.5 text-brand" />
                {msg}
              </span>
            )}
            <div className="ml-auto flex flex-wrap items-center gap-2">
              <label className="flex items-center gap-2 text-sm text-zinc-400">
                <input
                  type="checkbox"
                  checked={showQuiet}
                  onChange={(e) => setShowQuiet(e.target.checked)}
                />
                Show quiet ({visible.length}/{result.candidates.length})
              </label>
              <Button size="sm" variant="ghost" onClick={selectSuggested}>
                Select suggested
              </Button>
              <Button
                size="sm"
                variant="primary"
                icon={<Plus />}
                disabled={busy}
                onClick={() => void onCommit()}
              >
                Create {newSelected || ""} selected
              </Button>
            </div>
          </div>

          <div className="wt-table-wrap">
            <table className="min-w-full text-left text-sm">
              <thead>
                <tr className="border-b border-[var(--color-border)]">
                  <th className="h-10 w-10 px-3">
                    <span className="sr-only">Add</span>
                  </th>
                  <th className="h-10 px-3">CIDR</th>
                  <th className="h-10 px-3">Alive</th>
                  <th className="h-10 px-3">Replying hosts</th>
                  <th className="h-10 px-3">State</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {visible.map((c) => (
                  <tr key={c.cidr}>
                    <td className="h-11 px-3">
                      {c.exists_in_db ? (
                        <span className="text-xs text-zinc-600">—</span>
                      ) : (
                        <input
                          type="checkbox"
                          aria-label={`Add ${c.cidr}`}
                          checked={!!selected[c.cidr]}
                          onChange={(e) =>
                            setSelected((prev) => ({ ...prev, [c.cidr]: e.target.checked }))
                          }
                        />
                      )}
                    </td>
                    <td className="h-11 px-3 font-mono">
                      {c.exists_in_db && c.existing_id ? (
                        <Link className="text-brand hover:underline" to={`/subnets/${c.existing_id}`}>
                          {c.cidr}
                        </Link>
                      ) : (
                        <span className="text-zinc-100">{c.cidr}</span>
                      )}
                    </td>
                    <td className="h-11 px-3 font-mono text-xs tabular-nums text-zinc-300">
                      {c.alive}/{c.sampled}
                    </td>
                    <td className="h-11 px-3 font-mono text-xs text-zinc-500">
                      {(c.alive_ips ?? []).join(", ") || "—"}
                    </td>
                    <td className="h-11 px-3">
                      {c.exists_in_db ? (
                        <Badge>in IPAM</Badge>
                      ) : c.likely_used ? (
                        <Badge tone="brand">suggested</Badge>
                      ) : (
                        <span className="text-xs text-zinc-600">quiet</span>
                      )}
                    </td>
                  </tr>
                ))}
                {visible.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-3 py-12 text-center text-sm text-zinc-500">
                      No children replied. Tick “Show quiet” to see every probed prefix.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        !busy && (
          <div className="wt-card flex flex-col items-center px-6 py-12 text-center">
            <Network className="h-8 w-8 text-zinc-600" />
            <p className="mt-3 text-sm font-medium text-zinc-200">No probe yet</p>
            <p className="mt-1 max-w-sm text-xs text-zinc-500">
              Enter a supernet and child prefix, then probe. Replying networks are pre-selected so you can
              add them in one click.
            </p>
          </div>
        )
      )}
    </div>
  );
}
