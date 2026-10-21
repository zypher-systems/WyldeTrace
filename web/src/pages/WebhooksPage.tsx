import { useEffect, useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { Check, Eye, Plus, RotateCcw, ScrollText, Send, Trash2, Webhook as WebhookIcon } from "lucide-react";
import { api, pageQuery, type PageResult } from "../api";
import { useAuth } from "../auth";
import { useConfirm } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { Pagination } from "../components/Pagination";
import { Sheet } from "../components/Sheet";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Badge, Button, Field, StatusBadge, Switch, cx } from "../components/ui";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";
import { relativeTime } from "../lib/time";

type Webhook = {
  id: string;
  name: string;
  url: string;
  secret_set: boolean;
  events: string[];
  format: "json" | "slack" | "teams";
  enabled: boolean;
  last_status: string;
  last_error: string;
  last_sent_at: string | null;
};

type Delivery = {
  id: string;
  webhook_id: string;
  event: string;
  status: string;
  status_code: number;
  error: string;
  created_at: string;
};

const DEFAULT_EVENTS = [
  "discovery.new_alive",
  "discovery.stale",
  "discovery.error",
  "ip.deleted",
  "ip.claimed",
  "subnet.utilization_high",
  "reports.summary",
];

export function WebhooksPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const [items, setItems] = useState<Webhook[]>([]);
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [deliveryTotal, setDeliveryTotal] = useState(0);
  const [deliveryPage, setDeliveryPage] = useState(1);
  const [filterWebhook, setFilterWebhook] = useState("");
  const deliveryPageSize = 50;
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [secret, setSecret] = useState("");
  const [events, setEvents] = useState(DEFAULT_EVENTS.join(","));
  const [format, setFormat] = useState<"json" | "slack" | "teams">("json");
  const [previewEvent, setPreviewEvent] = useState("discovery.new_alive");
  const [previewFormat, setPreviewFormat] = useState<"json" | "slack" | "teams">("json");
  const [previewJSON, setPreviewJSON] = useState("");
  const [previewMeta, setPreviewMeta] = useState("");
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [logOpen, setLogOpen] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);
  useUnsavedChanges(!!(name || url || secret));

  async function load() {
    setLoading(true);
    try {
      setItems(await api<Webhook[]>("/api/v1/webhooks"));
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load webhooks", "error");
    } finally {
      setLoading(false);
    }
  }

  async function loadDeliveries(webhookID = filterWebhook, page = deliveryPage) {
    try {
      const path = webhookID
        ? `/api/v1/webhooks/${webhookID}/deliveries${pageQuery({ page, pageSize: deliveryPageSize })}`
        : `/api/v1/webhooks/deliveries${pageQuery({ page, pageSize: deliveryPageSize })}`;
      const res = await api<PageResult<Delivery>>(path);
      setDeliveries(res.items);
      setDeliveryTotal(res.total);
      setDeliveryPage(res.page);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load deliveries", "error");
    }
  }

  useEffect(() => {
    if (user?.role === "admin") {
      void load();
      void loadDeliveries();
    }
  }, [user?.role]);

  useEffect(() => {
    if (user?.role === "admin") {
      setDeliveryPage(1);
      void loadDeliveries(filterWebhook, 1);
    }
  }, [filterWebhook, user?.role]);

  if (user?.role !== "admin") return <Navigate to="/" replace />;

  const nameByID = Object.fromEntries(items.map((w) => [w.id, w.name]));
  const selectedEvents = events
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);

  function toggleEvent(ev: string) {
    const next = selectedEvents.includes(ev)
      ? selectedEvents.filter((x) => x !== ev)
      : [...selectedEvents, ev];
    setEvents(next.join(","));
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/api/v1/webhooks", {
        method: "POST",
        body: JSON.stringify({
          name,
          url,
          secret: secret || undefined,
          events: events.split(",").map((s) => s.trim()).filter(Boolean),
          format,
          enabled: true,
        }),
      });
      setName("");
      setUrl("");
      setSecret("");
      setFormat("json");
      setCreateOpen(false);
      toast("Webhook created", "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Create failed", "error");
    }
  }

  async function toggle(w: Webhook) {
    try {
      await api(`/api/v1/webhooks/${w.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: w.name,
          url: w.url,
          events: w.events,
          format: w.format || "json",
          enabled: !w.enabled,
        }),
      });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Update failed", "error");
    }
  }

  async function changeFormat(w: Webhook, next: string) {
    try {
      await api(`/api/v1/webhooks/${w.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: w.name,
          url: w.url,
          events: w.events,
          format: next,
          enabled: w.enabled,
        }),
      });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Format update failed", "error");
    }
  }

  async function test(id: string) {
    try {
      await api(`/api/v1/webhooks/${id}/test`, { method: "POST" });
      toast("Test delivery sent", "success");
      await load();
      await loadDeliveries(filterWebhook);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Test failed", "error");
    }
  }

  async function preview(opts?: { webhookId?: string; format?: string }) {
    try {
      const path = opts?.webhookId
        ? `/api/v1/webhooks/${opts.webhookId}/preview`
        : "/api/v1/webhooks/preview";
      const res = await api<{ format: string; event: string; payload: unknown; raw: string }>(path, {
        method: "POST",
        body: JSON.stringify({
          format: opts?.format || previewFormat,
          event: previewEvent,
        }),
      });
      setPreviewJSON(JSON.stringify(res.payload, null, 2));
      setPreviewMeta(
        `${opts?.webhookId ? `${nameByID[opts.webhookId] ?? "Webhook"} · ` : ""}${res.format} · ${res.event}`,
      );
      setPreviewOpen(true);
      toast(`Preview (${res.format} · ${res.event}) — not sent`, "success");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Preview failed", "error");
    }
  }

  async function onDelete(id: string) {
    if (!(await confirm({ title: "Delete this webhook?", danger: true, confirmLabel: "Delete" })))
      return;
    try {
      await api(`/api/v1/webhooks/${id}`, { method: "DELETE" });
      await load();
      await loadDeliveries(filterWebhook);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  async function retry(id: string) {
    try {
      await api(`/api/v1/webhooks/deliveries/${id}/retry`, { method: "POST" });
      toast("Delivery retried", "success");
      await load();
      await loadDeliveries(filterWebhook);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Retry failed", "error");
    }
  }

  function openLog(webhookID: string) {
    if (webhookID === filterWebhook) void loadDeliveries(webhookID, 1);
    else setFilterWebhook(webhookID);
    setLogOpen(true);
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Webhooks"
        subtitle="Push events as JSON, Slack, or Teams payloads, HMAC-signed."
        actions={
          <>
            <Button icon={<Eye />} onClick={() => setPreviewOpen(true)}>
              Preview payload
            </Button>
            <Button icon={<ScrollText />} onClick={() => openLog("")}>
              Delivery log
              {deliveryTotal > 0 && <span className="wt-badge h-5 px-1.5">{deliveryTotal}</span>}
            </Button>
            <Button variant="primary" icon={<Plus />} onClick={() => setCreateOpen(true)}>
              New webhook
            </Button>
          </>
        }
      />

      {loading ? (
        <TableSkeleton rows={5} cols={6} />
      ) : items.length === 0 ? (
        <div className="wt-card flex flex-col items-center gap-2 px-6 py-12 text-center">
          <WebhookIcon className="h-6 w-6 text-zinc-500" />
          <p className="text-sm font-medium text-zinc-200">No webhooks configured</p>
          <p className="text-xs text-zinc-500">
            Send alerts to Slack, Teams, or any HTTPS endpoint when inventory changes.
          </p>
          <Button variant="primary" icon={<Plus />} className="mt-2" onClick={() => setCreateOpen(true)}>
            New webhook
          </Button>
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                <th className="h-10 w-12 px-3">
                  <span className="sr-only">Enabled</span>
                </th>
                <th className="h-10 px-3">Webhook</th>
                <th className="h-10 px-3">Format</th>
                <th className="h-10 px-3">Events</th>
                <th className="h-10 px-3">Last delivery</th>
                <th className="h-10 px-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {items.map((w) => (
                <tr key={w.id}>
                  <td className="h-14 px-3">
                    <Switch
                      checked={w.enabled}
                      onChange={() => void toggle(w)}
                      label={`${w.enabled ? "Disable" : "Enable"} ${w.name}`}
                    />
                  </td>
                  <td className="max-w-[15rem] px-3 py-2">
                    <div className="flex items-center gap-2">
                      <span className={cx("font-medium", w.enabled ? "text-zinc-100" : "text-zinc-400")}>
                        {w.name}
                      </span>
                      {w.secret_set && (
                        <Badge className="h-5 px-1.5">signed</Badge>
                      )}
                    </div>
                    <div className="truncate font-mono text-xs text-zinc-500" title={w.url}>
                      {w.url}
                    </div>
                  </td>
                  <td className="px-3 py-2">
                    <select
                      aria-label={`Format for ${w.name}`}
                      className="wt-input h-8 px-2 font-mono text-xs"
                      value={w.format || "json"}
                      onChange={(e) => void changeFormat(w, e.target.value)}
                    >
                      <option value="json">json</option>
                      <option value="slack">slack</option>
                      <option value="teams">teams</option>
                    </select>
                  </td>
                  <td className="px-3 py-2">
                    <div className="flex gap-1 whitespace-nowrap" title={w.events.join(", ")}>
                      {w.events.slice(0, 1).map((ev) => (
                        <span key={ev} className="wt-badge font-mono">
                          {ev}
                        </span>
                      ))}
                      {w.events.length > 1 && (
                        <span className="wt-badge">+{w.events.length - 1}</span>
                      )}
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-3 py-2">
                    {w.last_status ? (
                      <div className="flex items-center gap-2" title={w.last_error || undefined}>
                        <StatusBadge status={w.last_status} />
                        {w.last_sent_at && (
                          <span
                            className="text-xs text-zinc-500"
                            title={new Date(w.last_sent_at).toLocaleString()}
                          >
                            {relativeTime(w.last_sent_at)}
                          </span>
                        )}
                      </div>
                    ) : (
                      <span className="text-xs text-zinc-600">Never sent</span>
                    )}
                    {w.last_error && (
                      <div className="mt-0.5 max-w-[10rem] truncate text-xs text-red-300/80" title={w.last_error}>
                        {w.last_error}
                      </div>
                    )}
                  </td>
                  <td className="px-3 py-2">
                    <div className="flex justify-end gap-0.5">
                      <button
                        type="button"
                        className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                        aria-label={`Preview payload for ${w.name}`}
                        title="Preview payload (not sent)"
                        onClick={() => void preview({ webhookId: w.id, format: w.format })}
                      >
                        <Eye />
                      </button>
                      <button
                        type="button"
                        className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                        aria-label={`Send test to ${w.name}`}
                        title="Send test delivery"
                        onClick={() => void test(w.id)}
                      >
                        <Send />
                      </button>
                      <button
                        type="button"
                        className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                        aria-label={`Deliveries for ${w.name}`}
                        title="Delivery log"
                        onClick={() => openLog(w.id)}
                      >
                        <ScrollText />
                      </button>
                      <button
                        type="button"
                        className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                        aria-label={`Delete ${w.name}`}
                        title="Delete"
                        onClick={() => void onDelete(w.id)}
                      >
                        <Trash2 />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <FormDialog open={createOpen} title="New webhook" onClose={() => setCreateOpen(false)} wide>
        <form onSubmit={onCreate} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Name">
              <input
                className="wt-input w-full"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="ops-slack"
                autoFocus
                required
              />
            </Field>
            <Field label="Format">
              <select
                className="wt-input w-full"
                value={format}
                onChange={(e) => setFormat(e.target.value as "json" | "slack" | "teams")}
              >
                <option value="json">JSON envelope</option>
                <option value="slack">Slack blocks</option>
                <option value="teams">Teams MessageCard</option>
              </select>
            </Field>
            <Field label="URL" className="sm:col-span-2">
              <input
                className="wt-input w-full font-mono text-xs"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://hooks.example/…"
                required
              />
            </Field>
            <Field
              label="Signing secret"
              hint="Optional — signs the body as X-WyldeTrace-Signature"
              className="sm:col-span-2"
            >
              <input
                className="wt-input w-full font-mono text-xs"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
              />
            </Field>
          </div>
          <div className="space-y-2">
            <span className="wt-label">Events</span>
            <div className="flex flex-wrap gap-1.5">
              {DEFAULT_EVENTS.map((ev) => {
                const on = selectedEvents.includes(ev);
                return (
                  <button
                    key={ev}
                    type="button"
                    aria-pressed={on}
                    className={cx("wt-badge cursor-pointer font-mono", on && "wt-badge-brand")}
                    onClick={() => toggleEvent(ev)}
                  >
                    {on && <Check className="h-3 w-3" />}
                    {ev}
                  </button>
                );
              })}
            </div>
            <input
              aria-label="Events (comma-separated)"
              className="wt-input w-full font-mono text-xs"
              value={events}
              onChange={(e) => setEvents(e.target.value)}
            />
            <span className="wt-hint block">Comma-separated event names; toggle common events above.</span>
          </div>
          <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="primary">
              Add webhook
            </Button>
          </div>
        </form>
      </FormDialog>

      <Sheet
        open={previewOpen}
        title="Payload preview"
        subtitle="Dry-run sample body — nothing is delivered"
        onClose={() => setPreviewOpen(false)}
      >
        <div className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Event">
              <select
                className="wt-input w-full font-mono text-xs"
                value={previewEvent}
                onChange={(e) => setPreviewEvent(e.target.value)}
              >
                {DEFAULT_EVENTS.map((ev) => (
                  <option key={ev} value={ev}>
                    {ev}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Format">
              <select
                className="wt-input w-full"
                value={previewFormat}
                onChange={(e) => setPreviewFormat(e.target.value as "json" | "slack" | "teams")}
              >
                <option value="json">json</option>
                <option value="slack">slack</option>
                <option value="teams">teams</option>
              </select>
            </Field>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" icon={<Eye />} onClick={() => void preview()}>
              Preview
            </Button>
            <Button
              onClick={() => {
                setPreviewFormat(format);
                void preview({ format });
              }}
            >
              Preview create format
            </Button>
          </div>
          {previewJSON ? (
            <div className="space-y-2">
              {previewMeta && <p className="font-mono text-xs text-zinc-500">{previewMeta}</p>}
              <pre className="max-h-[60vh] overflow-auto rounded-lg border border-[var(--color-border)] bg-[var(--color-canvas)] p-3 font-mono text-xs leading-relaxed text-zinc-300">
                {previewJSON}
              </pre>
            </div>
          ) : (
            <p className="text-sm text-zinc-500">Choose an event and format, then preview.</p>
          )}
        </div>
      </Sheet>

      <Sheet
        open={logOpen}
        title="Delivery log"
        subtitle={`${deliveryTotal} recent deliveries${filterWebhook ? ` · ${nameByID[filterWebhook] ?? ""}` : ""}`}
        onClose={() => setLogOpen(false)}
        width="max-w-3xl"
      >
        <div className="space-y-4">
          <div className="flex flex-wrap items-end justify-between gap-3">
            <Field label="Webhook">
              <select
                className="wt-input w-56"
                value={filterWebhook}
                onChange={(e) => setFilterWebhook(e.target.value)}
              >
                <option value="">All webhooks</option>
                {items.map((w) => (
                  <option key={w.id} value={w.id}>
                    {w.name}
                  </option>
                ))}
              </select>
            </Field>
            <Pagination
              page={deliveryPage}
              pageCount={Math.max(1, Math.ceil(deliveryTotal / deliveryPageSize) || 1)}
              total={deliveryTotal}
              onPageChange={(p) => void loadDeliveries(filterWebhook, p)}
            />
          </div>
          <div className="wt-table-wrap">
            <table className="min-w-full text-left text-sm">
              <thead>
                <tr className="border-b border-[var(--color-border)]">
                  <th className="h-10 px-3">When</th>
                  <th className="h-10 px-3">Webhook</th>
                  <th className="h-10 px-3">Event</th>
                  <th className="h-10 px-3">Status</th>
                  <th className="h-10 px-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {deliveries.map((d) => (
                  <tr key={d.id} className="align-top">
                    <td
                      className="whitespace-nowrap px-3 py-2.5 text-xs text-zinc-400"
                      title={new Date(d.created_at).toLocaleString()}
                    >
                      {relativeTime(d.created_at)}
                    </td>
                    <td className="px-3 py-2.5 text-xs text-zinc-200">
                      {nameByID[d.webhook_id] || d.webhook_id.slice(0, 8)}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-xs text-brand">{d.event}</td>
                    <td className="px-3 py-2.5">
                      <div className="flex items-center gap-1.5">
                        <StatusBadge status={d.status} />
                        {d.status_code ? (
                          <span className="font-mono text-xs text-zinc-500">{d.status_code}</span>
                        ) : null}
                      </div>
                      {d.error && (
                        <div className="mt-1 max-w-[16rem] break-words text-xs text-red-300/80">
                          {d.error}
                        </div>
                      )}
                    </td>
                    <td className="px-3 py-2.5 text-right">
                      <button
                        type="button"
                        className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                        aria-label="Retry delivery"
                        title="Retry"
                        onClick={() => void retry(d.id)}
                      >
                        <RotateCcw />
                      </button>
                    </td>
                  </tr>
                ))}
                {deliveries.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-3 py-10 text-center text-sm text-zinc-500">
                      No deliveries yet
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </Sheet>
    </div>
  );
}
