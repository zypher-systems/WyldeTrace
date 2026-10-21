import { useEffect, useState, type FormEvent, type MouseEvent as ReactMouseEvent } from "react";
import { History, LayoutTemplate, Pencil, Play, Plus, Radar, ScanSearch, Trash2 } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { api, type DiscoveryJob, type PageResult, type Subnet, pageQuery } from "../api";
import { useAuth } from "../auth";
import { useConfirm } from "../components/ConfirmDialog";
import { useContextMenu, type ContextMenuItem } from "../components/ContextMenu";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { Sheet } from "../components/Sheet";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Badge, Button, Field, Section, StatusBadge, Switch, cx } from "../components/ui";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";
import { relativeTime } from "../lib/time";

function summaryCount(summary: Record<string, unknown> | undefined, key: string): number | null {
  if (!summary) return null;
  const v = summary[key];
  if (typeof v === "number" && Number.isFinite(v)) return v;
  if (typeof v === "string" && v.trim() !== "" && Number.isFinite(Number(v))) return Number(v);
  return null;
}

function formatInterval(min: number) {
  if (min % 1440 === 0) return `every ${min / 1440}d`;
  if (min % 60 === 0) return `every ${min / 60}h`;
  return `every ${min}m`;
}

type JobTemplate = {
  id: string;
  name: string;
  interval_minutes: number;
  enrich: boolean;
  enabled_default: boolean;
};

export function DiscoveryJobsPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const navigate = useNavigate();
  const { open: openMenu, menu } = useContextMenu();
  const [jobs, setJobs] = useState<DiscoveryJob[]>([]);
  const [templates, setTemplates] = useState<JobTemplate[]>([]);
  const [subnets, setSubnets] = useState<Subnet[]>([]);
  const [name, setName] = useState("");
  const [subnetId, setSubnetId] = useState("");
  const [interval, setIntervalMinutes] = useState(60);
  const [enrich, setEnrich] = useState(true);
  const [templateId, setTemplateId] = useState("");
  const [tplName, setTplName] = useState("");
  const [tplInterval, setTplInterval] = useState(60);
  const [tplEnrich, setTplEnrich] = useState(true);
  const [editingId, setEditingId] = useState("");
  const [editEnabled, setEditEnabled] = useState(true);
  const [runsJobId, setRunsJobId] = useState("");
  const [runs, setRuns] = useState<
    { id: string; status: string; summary: Record<string, unknown>; started_at: string }[]
  >([]);
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [loading, setLoading] = useState(true);
  const [jobDialogOpen, setJobDialogOpen] = useState(false);
  const [tplDialogOpen, setTplDialogOpen] = useState(false);
  useUnsavedChanges(!!editingId || !!(name || subnetId || tplName));

  async function load() {
    setLoading(true);
    try {
      const [j, s, t] = await Promise.all([
        api<DiscoveryJob[]>("/api/v1/discovery-jobs"),
        api<PageResult<Subnet>>(`/api/v1/subnets${pageQuery({ page: 1, pageSize: 200 })}`),
        api<JobTemplate[]>("/api/v1/discovery-jobs/templates"),
      ]);
      setJobs(j);
      setSubnets(s.items);
      setTemplates(t);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load jobs", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  function applyTemplate(id: string) {
    setTemplateId(id);
    const t = templates.find((x) => x.id === id);
    if (!t) return;
    setIntervalMinutes(t.interval_minutes);
    setEnrich(t.enrich);
    if (!name) setName(t.name);
  }

  function startEdit(job: DiscoveryJob) {
    setEditingId(job.id);
    setTemplateId("");
    setName(job.name);
    setSubnetId(job.subnet_id);
    setIntervalMinutes(job.interval_minutes);
    setEnrich(job.enrich);
    setEditEnabled(job.enabled);
    setJobDialogOpen(true);
  }

  function startCreate(tplId = "") {
    cancelEdit();
    if (tplId) {
      const t = templates.find((x) => x.id === tplId);
      setTemplateId(tplId);
      if (t) {
        setIntervalMinutes(t.interval_minutes);
        setEnrich(t.enrich);
        setName(t.name);
      }
    }
    setJobDialogOpen(true);
  }

  function cancelEdit() {
    setEditingId("");
    setName("");
    setSubnetId("");
    setIntervalMinutes(60);
    setEnrich(true);
    setEditEnabled(true);
    setTemplateId("");
    setJobDialogOpen(false);
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    try {
      if (editingId) {
        await api(`/api/v1/discovery-jobs/${editingId}`, {
          method: "PUT",
          body: JSON.stringify({
            name,
            subnet_id: subnetId,
            interval_minutes: interval,
            enrich,
            enabled: editEnabled,
          }),
        });
        cancelEdit();
        toast("Job updated", "success");
        await load();
        return;
      }
      if (templateId) {
        await api("/api/v1/discovery-jobs/from-template", {
          method: "POST",
          body: JSON.stringify({
            template_id: templateId,
            subnet_id: subnetId,
            name: name || undefined,
          }),
        });
      } else {
        await api("/api/v1/discovery-jobs", {
          method: "POST",
          body: JSON.stringify({
            name,
            subnet_id: subnetId,
            interval_minutes: interval,
            enrich,
            enabled: true,
          }),
        });
      }
      setName("");
      setSubnetId("");
      setIntervalMinutes(60);
      setEnrich(true);
      setTemplateId("");
      setJobDialogOpen(false);
      toast("Job created", "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : editingId ? "Update failed" : "Create failed", "error");
    }
  }

  async function onCreateTemplate(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/api/v1/discovery-jobs/templates", {
        method: "POST",
        body: JSON.stringify({
          name: tplName,
          interval_minutes: tplInterval,
          enrich: tplEnrich,
          enabled_default: true,
        }),
      });
      setTplName("");
      setTplDialogOpen(false);
      toast("Template saved", "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Template create failed", "error");
    }
  }

  async function deleteTemplate(id: string) {
    if (!(await confirm({ title: "Delete this template?", danger: true, confirmLabel: "Delete" })))
      return;
    try {
      await api(`/api/v1/discovery-jobs/templates/${id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  async function toggleEnabled(job: DiscoveryJob) {
    try {
      await api(`/api/v1/discovery-jobs/${job.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: job.name,
          subnet_id: job.subnet_id,
          interval_minutes: job.interval_minutes,
          enrich: job.enrich,
          enabled: !job.enabled,
        }),
      });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Update failed", "error");
    }
  }

  async function runNow(id: string) {
    toast("Running…", "info");
    try {
      const job = await api<DiscoveryJob>(`/api/v1/discovery-jobs/${id}/run`, { method: "POST" });
      const neu = summaryCount(job.last_summary, "new_alive");
      const stale = summaryCount(job.last_summary, "stale");
      const parts = [`Run finished: ${job.last_status || "ok"}`];
      if (neu != null) parts.push(`${neu} new`);
      if (stale != null) parts.push(`${stale} stale`);
      const inboxQs = new URLSearchParams({ subnet_id: job.subnet_id });
      if (neu != null && neu > 0) inboxQs.set("kind", "new_alive");
      toast(parts.join(" · "), {
        tone: "success",
        action: {
          label: "Open inbox",
          onClick: () => navigate(`/inbox?${inboxQs.toString()}`),
        },
      });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Run failed", "error");
    }
  }

  async function onDelete(id: string) {
    if (
      !(await confirm({ title: "Delete this discovery job?", danger: true, confirmLabel: "Delete" }))
    )
      return;
    try {
      await api(`/api/v1/discovery-jobs/${id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  function openJobMenu(e: ReactMouseEvent, j: DiscoveryJob) {
    const items: ContextMenuItem[] = [];
    if (canEdit) {
      items.push({
        id: "run",
        label: "Run now",
        onSelect: () => void runNow(j.id),
      });
    }
    items.push({
      id: "history",
      label: "History",
      onSelect: () => void loadRuns(j.id),
    });
    if (canEdit) {
      items.push(
        {
          id: "edit",
          label: "Edit",
          onSelect: () => startEdit(j),
        },
        {
          id: "delete",
          label: "Delete",
          danger: true,
          onSelect: () => void onDelete(j.id),
        },
      );
    }
    openMenu(e, items);
  }

  async function loadRuns(jobId: string) {
    setRunsJobId(jobId);
    try {
      setRuns(
        await api(`/api/v1/discovery-jobs/${jobId}/runs`),
      );
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to load runs", "error");
    }
  }

  const subnetName = (id: string) => subnets.find((s) => s.id === id)?.cidr ?? id.slice(0, 8);

  const runsJob = jobs.find((j) => j.id === runsJobId);

  return (
    <div className="space-y-6">
      {menu}
      <PageHeader
        title="Scan jobs"
        subtitle={`${jobs.length} scheduled sweep${jobs.length === 1 ? "" : "s"} · findings land in the Inbox`}
        actions={
          canEdit ? (
            <>
              <Link to="/discover-subnets" className="wt-btn wt-btn-secondary">
                <ScanSearch />
                Find subnets
              </Link>
              <Button variant="primary" icon={<Plus />} onClick={() => startCreate()}>
                New scan job
              </Button>
            </>
          ) : undefined
        }
      />

      {loading ? (
        <TableSkeleton rows={6} cols={6} />
      ) : jobs.length === 0 ? (
        <div className="wt-card flex flex-col items-center px-6 py-14 text-center">
          <Radar className="h-8 w-8 text-zinc-600" />
          <p className="mt-3 text-sm font-medium text-zinc-200">No scan jobs yet</p>
          <p className="mt-1 max-w-sm text-xs text-zinc-500">
            Schedule a sweep of a subnet. New hosts and stale records show up in the Inbox after each run.
          </p>
          {canEdit && (
            <Button variant="primary" size="sm" className="mt-4" icon={<Plus />} onClick={() => startCreate()}>
              New scan job
            </Button>
          )}
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                <th className="h-10 px-3">Name</th>
                <th className="h-10 px-3">Subnet</th>
                <th className="h-10 px-3">Schedule</th>
                <th className="h-10 px-3">Last run</th>
                <th className="h-10 px-3">Result</th>
                <th className="h-10 px-3">Enabled</th>
                <th className="h-10 px-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {jobs.map((j) => {
                const alive = summaryCount(j.last_summary, "alive");
                const neu = summaryCount(j.last_summary, "new_alive");
                const stale = summaryCount(j.last_summary, "stale");
                return (
                  <tr key={j.id} onContextMenu={(e) => openJobMenu(e, j)}>
                    <td className="h-11 px-3">
                      <span className="inline-flex items-center gap-2 font-medium text-zinc-100">
                        <span
                          className={cx(
                            "wt-status-dot",
                            !j.enabled
                              ? "bg-zinc-600"
                              : !j.last_run_at
                                ? "bg-zinc-400"
                                : j.last_status === "ok"
                                  ? "bg-brand"
                                  : "bg-red-400",
                          )}
                        />
                        {j.name}
                        {j.enrich && <Badge className="h-5 text-[10px]">enrich</Badge>}
                      </span>
                    </td>
                    <td className="h-11 px-3">
                      <Link className="font-mono text-brand hover:underline" to={`/subnets/${j.subnet_id}`}>
                        {subnetName(j.subnet_id)}
                      </Link>
                    </td>
                    <td className="h-11 px-3 text-zinc-300">{formatInterval(j.interval_minutes)}</td>
                    <td
                      className="h-11 whitespace-nowrap px-3 text-xs text-zinc-400"
                      title={j.last_run_at ? new Date(j.last_run_at).toLocaleString() : undefined}
                    >
                      {j.last_run_at ? relativeTime(j.last_run_at) : <span className="text-zinc-600">never</span>}
                    </td>
                    <td className="h-11 px-3">
                      {j.last_status ? (
                        <span className="inline-flex flex-wrap items-center gap-2">
                          <StatusBadge status={j.last_status} />
                          {alive != null && (
                            <span className="text-xs text-zinc-400 tabular-nums">
                              {alive} alive
                              {neu ? <span className="text-brand"> · {neu} new</span> : null}
                              {stale ? <span className="text-amber-300"> · {stale} stale</span> : null}
                            </span>
                          )}
                        </span>
                      ) : (
                        <span className="text-zinc-600">—</span>
                      )}
                    </td>
                    <td className="h-11 px-3">
                      <Switch
                        checked={j.enabled}
                        disabled={!canEdit}
                        label={j.enabled ? `Disable ${j.name}` : `Enable ${j.name}`}
                        onChange={canEdit ? () => void toggleEnabled(j) : undefined}
                      />
                    </td>
                    <td className="h-11 px-3">
                      <div className="flex items-center justify-end gap-1">
                        {canEdit && (
                          <Button size="sm" icon={<Play />} onClick={() => void runNow(j.id)}>
                            Run now
                          </Button>
                        )}
                        <button
                          type="button"
                          className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                          aria-label={`Run history for ${j.name}`}
                          title="Run history"
                          onClick={() => void loadRuns(j.id)}
                        >
                          <History />
                        </button>
                        {canEdit && (
                          <>
                            <button
                              type="button"
                              className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                              aria-label={`Edit ${j.name}`}
                              title="Edit"
                              onClick={() => startEdit(j)}
                            >
                              <Pencil />
                            </button>
                            <button
                              type="button"
                              className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                              aria-label={`Delete ${j.name}`}
                              title="Delete"
                              onClick={() => void onDelete(j.id)}
                            >
                              <Trash2 />
                            </button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {canEdit && (
        <Section
          title="Templates"
          description="Reusable schedules — start a new job from one"
          actions={
            <Button size="sm" icon={<Plus />} onClick={() => setTplDialogOpen(true)}>
              New template
            </Button>
          }
        >
          {templates.length === 0 ? (
            <p className="py-4 text-center text-sm text-zinc-500">
              No templates yet. Save a common interval to reuse it.
            </p>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              {templates.map((t) => (
                <div
                  key={t.id}
                  className="group flex items-start gap-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-3"
                >
                  <LayoutTemplate className="mt-0.5 h-4 w-4 shrink-0 text-zinc-500" />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-zinc-100">{t.name}</p>
                    <p className="mt-0.5 text-xs text-zinc-500">
                      {formatInterval(t.interval_minutes)}
                      {t.enrich ? " · enrich" : ""}
                    </p>
                    <button
                      type="button"
                      className="mt-2 text-xs font-medium text-brand hover:underline"
                      onClick={() => startCreate(t.id)}
                    >
                      Use template
                    </button>
                  </div>
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                    aria-label={`Delete template ${t.name}`}
                    title="Delete template"
                    onClick={() => void deleteTemplate(t.id)}
                  >
                    <Trash2 />
                  </button>
                </div>
              ))}
            </div>
          )}
        </Section>
      )}

      {canEdit && (
        <FormDialog
          open={jobDialogOpen}
          title={editingId ? "Edit scan job" : "New scan job"}
          onClose={cancelEdit}
        >
          <form onSubmit={onCreate} className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              {!editingId && (
                <Field label="Template" className="sm:col-span-2" hint="Optional — prefills the schedule">
                  <select
                    className="wt-input w-full"
                    value={templateId}
                    onChange={(e) => applyTemplate(e.target.value)}
                  >
                    <option value="">Custom…</option>
                    {templates.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.name} ({t.interval_minutes}m{t.enrich ? " +enrich" : ""})
                      </option>
                    ))}
                  </select>
                </Field>
              )}
              <Field label="Name">
                <input
                  className="wt-input w-full"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required={!templateId || !!editingId}
                  placeholder={templateId && !editingId ? "Optional (auto)" : "e.g. Office LAN sweep"}
                />
              </Field>
              <Field label="Subnet">
                <select
                  className="wt-input w-full"
                  value={subnetId}
                  onChange={(e) => setSubnetId(e.target.value)}
                  required
                >
                  <option value="">Select…</option>
                  {subnets.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.cidr} — {s.name}
                    </option>
                  ))}
                </select>
              </Field>
              {(editingId || !templateId) && (
                <>
                  <Field label="Interval (minutes)">
                    <input
                      type="number"
                      min={1}
                      className="wt-input w-full"
                      value={interval}
                      onChange={(e) => setIntervalMinutes(Number(e.target.value))}
                    />
                  </Field>
                  <label className="flex items-center gap-2 self-end pb-2 text-sm text-zinc-300">
                    <input type="checkbox" checked={enrich} onChange={(e) => setEnrich(e.target.checked)} />
                    Enrich
                  </label>
                </>
              )}
              {editingId && (
                <label className="flex items-center gap-2 text-sm text-zinc-300">
                  <input
                    type="checkbox"
                    checked={editEnabled}
                    onChange={(e) => setEditEnabled(e.target.checked)}
                  />
                  Enabled
                </label>
              )}
            </div>
            <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
              <Button variant="ghost" onClick={cancelEdit}>
                Cancel
              </Button>
              <Button type="submit" variant="primary">
                {editingId ? "Save changes" : templateId ? "Create from template" : "Create job"}
              </Button>
            </div>
          </form>
        </FormDialog>
      )}

      {canEdit && (
        <FormDialog open={tplDialogOpen} title="New template" onClose={() => setTplDialogOpen(false)}>
          <form onSubmit={onCreateTemplate} className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Name">
                <input
                  className="wt-input w-full"
                  value={tplName}
                  onChange={(e) => setTplName(e.target.value)}
                  required
                />
              </Field>
              <Field label="Interval (minutes)">
                <input
                  type="number"
                  min={1}
                  className="wt-input w-full"
                  value={tplInterval}
                  onChange={(e) => setTplInterval(Number(e.target.value))}
                />
              </Field>
              <label className="flex items-center gap-2 text-sm text-zinc-300">
                <input
                  type="checkbox"
                  checked={tplEnrich}
                  onChange={(e) => setTplEnrich(e.target.checked)}
                />
                Enrich
              </label>
            </div>
            <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
              <Button variant="ghost" onClick={() => setTplDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" variant="primary">
                Save template
              </Button>
            </div>
          </form>
        </FormDialog>
      )}

      <Sheet
        open={!!runsJobId}
        title="Run history"
        subtitle={runsJob ? `${runsJob.name} · ${subnetName(runsJob.subnet_id)}` : runsJobId.slice(0, 8)}
        onClose={() => {
          setRunsJobId("");
          setRuns([]);
        }}
      >
        {runs.length === 0 ? (
          <div className="flex flex-col items-center py-12 text-center">
            <History className="h-7 w-7 text-zinc-600" />
            <p className="mt-3 text-sm text-zinc-400">No runs recorded yet</p>
          </div>
        ) : (
          <ul className="divide-y divide-[var(--color-border)]">
            {runs.map((r) => (
              <li key={r.id} className="flex items-center justify-between gap-3 py-3">
                <div className="min-w-0">
                  <p className="text-sm text-zinc-200" title={new Date(r.started_at).toLocaleString()}>
                    {relativeTime(r.started_at)}
                    <span className="ml-2 text-xs text-zinc-500">{new Date(r.started_at).toLocaleString()}</span>
                  </p>
                  <p className="mt-0.5 text-xs text-zinc-400 tabular-nums">
                    alive {String(r.summary?.alive ?? "—")} · new {String(r.summary?.new_alive ?? "—")} · stale{" "}
                    {String(r.summary?.stale ?? "—")}
                  </p>
                </div>
                <StatusBadge status={r.status} />
              </li>
            ))}
          </ul>
        )}
      </Sheet>
    </div>
  );
}
