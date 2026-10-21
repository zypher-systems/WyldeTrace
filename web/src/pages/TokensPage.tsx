import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { Ban, Check, Copy, KeyRound, MapPin, Plus, Trash2, X } from "lucide-react";
import { api, type APIToken, type Location, type Role } from "../api";
import { useAuth } from "../auth";
import { useConfirm } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Badge, Button, Field, cx } from "../components/ui";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";
import { relativeTime } from "../lib/time";

const ROLE_TONE: Record<Role, "accent" | "brand" | "info" | "neutral"> = {
  admin: "accent",
  editor: "brand",
  reportview: "info",
  readonly: "neutral",
};

function SiteChips({
  locations,
  selected,
  onToggle,
}: {
  locations: Location[];
  selected: string[];
  onToggle: (id: string) => void;
}) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {locations.map((l) => {
        const on = selected.includes(l.id);
        return (
          <button
            key={l.id}
            type="button"
            aria-pressed={on}
            className={cx("wt-badge cursor-pointer", on && "wt-badge-brand")}
            onClick={() => onToggle(l.id)}
          >
            {on && <Check className="h-3 w-3" />}
            {l.name}
          </button>
        );
      })}
    </div>
  );
}

export function TokensPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [locations, setLocations] = useState<Location[]>([]);
  const [name, setName] = useState("");
  const [role, setRole] = useState<Role>("readonly");
  const [locationIDs, setLocationIDs] = useState<string[]>([]);
  const [expiresDays, setExpiresDays] = useState("");
  const [created, setCreated] = useState("");
  const [copied, setCopied] = useState(false);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [sitesToken, setSitesToken] = useState<APIToken | null>(null);
  const [sitesDraft, setSitesDraft] = useState<string[]>([]);
  useUnsavedChanges(!!(name || expiresDays || locationIDs.length));

  async function load() {
    setLoading(true);
    try {
      const [toks, locs] = await Promise.all([
        api<APIToken[]>("/api/v1/api-tokens"),
        api<Location[]>("/api/v1/locations"),
      ]);
      setTokens(toks);
      setLocations(locs);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load tokens", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (user?.role === "admin") void load();
  }, [user?.role]);

  const sortedTokens = useMemo(
    () =>
      [...tokens].sort((a, b) => {
        const at = a.last_used_at ? new Date(a.last_used_at).getTime() : 0;
        const bt = b.last_used_at ? new Date(b.last_used_at).getTime() : 0;
        return bt - at;
      }),
    [tokens],
  );

  if (user?.role !== "admin") return <Navigate to="/" replace />;

  function toggleLoc(id: string) {
    setLocationIDs((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    setCreated("");
    setCopied(false);
    try {
      const body: Record<string, unknown> = {
        name,
        role,
        location_ids: role === "admin" ? [] : locationIDs,
      };
      if (expiresDays.trim()) {
        const days = Number(expiresDays);
        if (!Number.isFinite(days) || days <= 0) {
          toast("Expires days must be a positive number", "error");
          return;
        }
        body.expires_at = new Date(Date.now() + days * 86400000).toISOString();
      }
      const tok = await api<APIToken>("/api/v1/api-tokens", {
        method: "POST",
        body: JSON.stringify(body),
      });
      setName("");
      setRole("readonly");
      setLocationIDs([]);
      setExpiresDays("");
      setCreated(tok.token || "");
      setCreateOpen(false);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Create failed", "error");
    }
  }

  async function copyCreated() {
    try {
      await navigator.clipboard.writeText(created);
      setCopied(true);
      toast("Token copied to clipboard", "success");
    } catch {
      toast("Copy failed — select the token and copy manually", "error");
    }
  }

  async function onRevoke(id: string) {
    if (!(await confirm({ title: "Revoke this token?", danger: true, confirmLabel: "Revoke" })))
      return;
    try {
      await api(`/api/v1/api-tokens/${id}/revoke`, { method: "POST" });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Revoke failed", "error");
    }
  }

  function openSites(t: APIToken) {
    setSitesToken(t);
    setSitesDraft(t.location_ids ?? []);
  }

  async function onSaveSites(e: FormEvent) {
    e.preventDefault();
    if (!sitesToken) return;
    try {
      await api(`/api/v1/api-tokens/${sitesToken.id}/locations`, {
        method: "PUT",
        body: JSON.stringify({ location_ids: sitesDraft }),
      });
      setSitesToken(null);
      toast("Token scope updated", "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Site update failed", "error");
    }
  }

  async function onDelete(id: string) {
    if (
      !(await confirm({
        title: "Delete this token permanently?",
        danger: true,
        confirmLabel: "Delete",
      }))
    )
      return;
    try {
      await api(`/api/v1/api-tokens/${id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  const locName = Object.fromEntries(locations.map((l) => [l.id, l.name]));

  return (
    <div className="space-y-6">
      <PageHeader
        title="API tokens"
        subtitle="Service accounts for automation. Role controls read vs write; sites optionally limit scope."
        actions={
          <Button variant="primary" icon={<Plus />} onClick={() => setCreateOpen(true)}>
            New token
          </Button>
        }
      />

      {created && (
        <div
          role="status"
          className="wt-card relative overflow-hidden border-brand/40 p-5"
          style={{
            background:
              "linear-gradient(135deg, color-mix(in srgb, var(--color-brand) 9%, transparent), transparent 60%), var(--color-surface-raised)",
          }}
        >
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-start gap-3">
              <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-brand/15 text-brand">
                <KeyRound className="h-4 w-4" />
              </span>
              <div>
                <p className="text-sm font-semibold text-zinc-50">Token created</p>
                <p className="mt-0.5 text-xs text-zinc-400">
                  Copy this token now — it won’t be shown again.
                </p>
              </div>
            </div>
            <button
              type="button"
              className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
              aria-label="Dismiss token"
              title="Dismiss"
              onClick={() => setCreated("")}
            >
              <X />
            </button>
          </div>
          <div className="mt-4 flex flex-wrap items-stretch gap-2">
            <code className="min-w-0 flex-1 select-all break-all rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-canvas)] px-3 py-2.5 font-mono text-sm text-brand">
              {created}
            </code>
            <Button
              variant="primary"
              icon={copied ? <Check /> : <Copy />}
              onClick={() => void copyCreated()}
            >
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>
        </div>
      )}

      {loading ? (
        <TableSkeleton rows={6} cols={8} />
      ) : tokens.length === 0 ? (
        <div className="wt-card flex flex-col items-center gap-2 px-6 py-12 text-center">
          <KeyRound className="h-6 w-6 text-zinc-500" />
          <p className="text-sm font-medium text-zinc-200">No API tokens</p>
          <p className="text-xs text-zinc-500">
            Create a token for scripts, CI, or integrations that call the WyldeTrace API.
          </p>
          <Button variant="primary" icon={<Plus />} className="mt-2" onClick={() => setCreateOpen(true)}>
            New token
          </Button>
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                <th className="h-10 px-3">Name</th>
                <th className="h-10 px-3">Prefix</th>
                <th className="h-10 px-3">Role</th>
                <th className="h-10 px-3">Scope</th>
                <th className="h-10 px-3">Expires</th>
                <th className="h-10 px-3">Last used</th>
                <th className="h-10 px-3">Status</th>
                <th className="h-10 px-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {sortedTokens.map((t) => {
                const expired = !!t.expires_at && new Date(t.expires_at).getTime() < Date.now();
                return (
                  <tr key={t.id} className={t.revoked_at ? "opacity-60" : undefined}>
                    <td className="h-11 px-3 font-medium text-zinc-100">{t.name}</td>
                    <td className="h-11 px-3 font-mono text-xs text-zinc-400">{t.prefix}</td>
                    <td className="h-11 px-3">
                      <Badge tone={ROLE_TONE[t.role] ?? "neutral"}>{t.role}</Badge>
                    </td>
                    <td className="h-11 px-3">
                      {t.site_scoped ? (
                        <div className="flex flex-wrap gap-1">
                          {(t.location_ids ?? []).map((id) => (
                            <Badge key={id}>{locName[id] || id.slice(0, 8)}</Badge>
                          ))}
                        </div>
                      ) : (
                        <span className="text-xs text-zinc-500">All sites</span>
                      )}
                    </td>
                    <td
                      className={cx("h-11 px-3 text-xs", expired ? "text-red-300" : "text-zinc-400")}
                      title={t.expires_at ? new Date(t.expires_at).toLocaleString() : undefined}
                    >
                      {t.expires_at ? new Date(t.expires_at).toLocaleDateString() : "Never"}
                    </td>
                    <td
                      className="h-11 px-3 text-xs text-zinc-400"
                      title={t.last_used_at ? new Date(t.last_used_at).toLocaleString() : undefined}
                    >
                      {t.last_used_at ? relativeTime(t.last_used_at) : "never"}
                    </td>
                    <td className="h-11 px-3">
                      {t.revoked_at ? (
                        <Badge tone="warn">revoked</Badge>
                      ) : expired ? (
                        <Badge tone="danger">expired</Badge>
                      ) : (
                        <Badge tone="brand">active</Badge>
                      )}
                    </td>
                    <td className="h-11 px-3">
                      <div className="flex justify-end gap-0.5">
                        {!t.revoked_at && (
                          <button
                            type="button"
                            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                            aria-label={`Edit sites for ${t.name}`}
                            title="Edit site scope"
                            onClick={() => openSites(t)}
                          >
                            <MapPin />
                          </button>
                        )}
                        {!t.revoked_at && (
                          <button
                            type="button"
                            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-amber-300"
                            aria-label={`Revoke ${t.name}`}
                            title="Revoke"
                            onClick={() => void onRevoke(t.id)}
                          >
                            <Ban />
                          </button>
                        )}
                        <button
                          type="button"
                          className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                          aria-label={`Delete ${t.name}`}
                          title="Delete"
                          onClick={() => void onDelete(t.id)}
                        >
                          <Trash2 />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <FormDialog open={createOpen} title="New API token" onClose={() => setCreateOpen(false)}>
        <form onSubmit={onCreate} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Name" className="sm:col-span-2">
              <input
                className="wt-input w-full"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="ci-deploy"
                autoFocus
                required
              />
            </Field>
            <Field label="Role">
              <select
                className="wt-input w-full"
                value={role}
                onChange={(e) => {
                  const next = e.target.value as Role;
                  setRole(next);
                  if (next === "admin") setLocationIDs([]);
                }}
              >
                <option value="readonly">readonly (read)</option>
                <option value="reportview">reportview</option>
                <option value="editor">editor (write)</option>
                <option value="admin">admin (global write)</option>
              </select>
            </Field>
            <Field label="Expires after (days)" hint="Leave blank to never expire">
              <input
                type="number"
                min={1}
                className="wt-input w-full"
                value={expiresDays}
                onChange={(e) => setExpiresDays(e.target.value)}
                placeholder="never"
              />
            </Field>
          </div>
          {role !== "admin" && locations.length > 0 && (
            <div className="space-y-2">
              <div>
                <span className="wt-label">Sites</span>
                <span className="wt-hint">Optional — leave empty for all sites</span>
              </div>
              <SiteChips locations={locations} selected={locationIDs} onToggle={toggleLoc} />
            </div>
          )}
          <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="primary">
              Create token
            </Button>
          </div>
        </form>
      </FormDialog>

      <FormDialog
        open={!!sitesToken}
        title={sitesToken ? `Site scope — ${sitesToken.name}` : "Site scope"}
        onClose={() => setSitesToken(null)}
      >
        <form onSubmit={onSaveSites} className="space-y-4">
          <p className="wt-hint">Select the sites this token may access. None selected = all sites.</p>
          {locations.length === 0 ? (
            <p className="text-sm text-zinc-500">No locations defined</p>
          ) : (
            <SiteChips
              locations={locations}
              selected={sitesDraft}
              onToggle={(id) =>
                setSitesDraft((prev) =>
                  prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
                )
              }
            />
          )}
          <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Button variant="ghost" onClick={() => setSitesToken(null)}>
              Cancel
            </Button>
            <Button type="submit" variant="primary">
              Save scope
            </Button>
          </div>
        </form>
      </FormDialog>
    </div>
  );
}
