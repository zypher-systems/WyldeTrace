import { useEffect, useState, type FormEvent } from "react";
import { api, pageQuery, type PageResult, type Subnet } from "../api";
import { Check, Plus, Send, Ticket, X } from "lucide-react";
import { useAuth } from "../auth";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { Pagination } from "../components/Pagination";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Button, Field, Segmented, StatusBadge } from "../components/ui";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";
import { relativeTime } from "../lib/time";

type IPRequest = {
  id: string;
  requester_id: string;
  requester_email?: string;
  subnet_id: string;
  subnet_cidr?: string;
  subnet_name?: string;
  range_id?: string;
  hostname: string;
  notes: string;
  requested_address?: string;
  status: "pending" | "approved" | "denied" | string;
  allocated_ip_id?: string;
  allocated_address?: string;
  review_notes?: string;
  created_at: string;
  reviewed_at?: string;
};

export function RequestsPage() {
  const { user } = useAuth();
  const toast = useToast();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [items, setItems] = useState<IPRequest[]>([]);
  const [subnets, setSubnets] = useState<Subnet[]>([]);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState(canEdit ? "pending" : "");
  const [page, setPage] = useState(1);
  const [subnetId, setSubnetId] = useState("");
  const [hostname, setHostname] = useState("");
  const [notes, setNotes] = useState("");
  const [requestedAddress, setRequestedAddress] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const isReportView = user?.role === "reportview";
  const pageSize = 50;
  useUnsavedChanges(!!(hostname || notes || requestedAddress || subnetId));

  async function load(nextPage = page, st = status) {
    setLoading(true);
    try {
      const q =
        `/api/v1/ip-requests${pageQuery({ page: nextPage, pageSize })}` +
        (st ? `&status=${encodeURIComponent(st)}` : "");
      const res = await api<PageResult<IPRequest>>(q);
      setItems(res.items);
      setTotal(res.total);
      setPage(res.page);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load requests", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load(page, status);
  }, [page, status]);

  useEffect(() => {
    if (user?.role === "reportview") return;
    void api<PageResult<Subnet>>(`/api/v1/subnets${pageQuery({ page: 1, pageSize: 200 })}`)
      .then((r) => setSubnets(r.items))
      .catch(() => setSubnets([]));
  }, [user?.role]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await api("/api/v1/ip-requests", {
        method: "POST",
        body: JSON.stringify({
          subnet_id: subnetId,
          hostname,
          notes,
          requested_address: requestedAddress || undefined,
        }),
      });
      setHostname("");
      setNotes("");
      setRequestedAddress("");
      setDialogOpen(false);
      toast("Request submitted", "success");
      await load(1, status);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Create failed", "error");
    } finally {
      setBusy(false);
    }
  }

  async function act(id: string, action: "approve" | "deny") {
    try {
      await api(`/api/v1/ip-requests/${id}/${action}`, {
        method: "POST",
        body: JSON.stringify({}),
      });
      toast(`${action} ok`, "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Action failed", "error");
    }
  }

  async function cancel(id: string) {
    try {
      await api(`/api/v1/ip-requests/${id}`, { method: "DELETE" });
      toast("Cancelled", "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Cancel failed", "error");
    }
  }

  const pages = Math.max(1, Math.ceil(total / pageSize));

  const formFields = (
    <div className="grid gap-4 sm:grid-cols-2">
      {isReportView ? (
        <Field label="Subnet ID" hint="Ask your network team for the subnet's ID">
          <input
            className="wt-input w-full font-mono text-xs"
            value={subnetId}
            onChange={(e) => setSubnetId(e.target.value)}
            placeholder="UUID of target subnet"
            required
          />
        </Field>
      ) : (
        <Field label="Subnet">
          <select
            className="wt-input w-full"
            value={subnetId}
            onChange={(e) => setSubnetId(e.target.value)}
            required
          >
            <option value="">Select subnet…</option>
            {subnets.map((s) => (
              <option key={s.id} value={s.id}>
                {s.cidr} — {s.name}
              </option>
            ))}
          </select>
        </Field>
      )}
      <Field label="Hostname">
        <input
          className="wt-input w-full"
          value={hostname}
          onChange={(e) => setHostname(e.target.value)}
          placeholder="e.g. printer-2f"
          required
        />
      </Field>
      <Field label="Requested address" hint="Optional — leave blank for the next free IP">
        <input
          className="wt-input w-full font-mono text-sm"
          value={requestedAddress}
          onChange={(e) => setRequestedAddress(e.target.value)}
          placeholder="next free"
        />
      </Field>
      <Field label="Notes" className="sm:col-span-2">
        <textarea
          className="wt-input w-full"
          rows={2}
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          placeholder="What is it for?"
        />
      </Field>
    </div>
  );

  return (
    <div className="space-y-6">
      <PageHeader
        title="IP requests"
        subtitle={
          canEdit
            ? `${total} ${status || "total"} · review requests and allocate from inventory`
            : "Request an address — an editor approves it and allocates from inventory"
        }
        actions={
          !isReportView ? (
            <Button variant="primary" icon={<Plus />} onClick={() => setDialogOpen(true)}>
              New request
            </Button>
          ) : undefined
        }
      />

      {isReportView ? (
        <form onSubmit={onCreate} className="wt-card max-w-3xl p-5">
          <div className="mb-4 flex items-start gap-3">
            <span className="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-brand/10 text-brand">
              <Send className="h-4 w-4" />
            </span>
            <div>
              <h2 className="text-sm font-semibold text-zinc-100">Request an IP address</h2>
              <p className="mt-0.5 text-xs text-zinc-500">
                Fill in the details and submit. You can track the status below.
              </p>
            </div>
          </div>
          {formFields}
          <div className="mt-4 flex justify-end border-t border-[var(--color-border)] pt-4">
            <Button type="submit" variant="primary" icon={<Send />} disabled={busy}>
              {busy ? "Submitting…" : "Submit request"}
            </Button>
          </div>
        </form>
      ) : (
        <FormDialog open={dialogOpen} title="New IP request" onClose={() => setDialogOpen(false)}>
          <form onSubmit={onCreate} className="space-y-4">
            {formFields}
            <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
              <Button variant="ghost" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" variant="primary" disabled={busy}>
                {busy ? "Submitting…" : "Submit request"}
              </Button>
            </div>
          </form>
        </FormDialog>
      )}

      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          {isReportView && <h2 className="text-sm font-semibold text-zinc-100">Your requests</h2>}
          <Segmented
            label="Status filter"
            value={status || "all"}
            onChange={(v) => {
              setPage(1);
              setStatus(v === "all" ? "" : v);
            }}
            items={[
              { value: "all", label: "All" },
              { value: "pending", label: "Pending" },
              { value: "approved", label: "Approved" },
              { value: "denied", label: "Denied" },
            ]}
          />
          {isReportView && <span className="text-xs text-zinc-500 tabular-nums">{total} shown</span>}
        </div>

        {loading ? (
          <TableSkeleton rows={6} cols={7} />
        ) : items.length === 0 ? (
          <div className="wt-card flex flex-col items-center px-6 py-12 text-center">
            <Ticket className="h-8 w-8 text-zinc-600" />
            <p className="mt-3 text-sm font-medium text-zinc-200">
              {status === "pending" ? "No pending requests" : "No requests"}
            </p>
            <p className="mt-1 max-w-sm text-xs text-zinc-500">
              {canEdit && status === "pending"
                ? "You're all caught up. New requests will show up here for review."
                : "Requests you submit appear here with their status."}
            </p>
          </div>
        ) : (
          <div className="wt-table-wrap">
            <table className="min-w-full text-left text-sm">
              <thead>
                <tr className="border-b border-[var(--color-border)]">
                  <th className="h-10 px-3">Status</th>
                  <th className="h-10 px-3">Hostname</th>
                  <th className="h-10 px-3">Subnet</th>
                  <th className="h-10 px-3">Address</th>
                  <th className="h-10 px-3">Requester</th>
                  <th className="h-10 px-3">Created</th>
                  <th className="h-10 px-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {items.map((item) => (
                  <tr key={item.id}>
                    <td className="h-11 px-3">
                      <StatusBadge status={item.status} />
                    </td>
                    <td className="h-11 px-3">
                      <div className="text-zinc-100">{item.hostname || "—"}</div>
                      {item.notes && (
                        <div className="max-w-[18rem] truncate text-xs text-zinc-500" title={item.notes}>
                          {item.notes}
                        </div>
                      )}
                    </td>
                    <td className="h-11 px-3 font-mono text-zinc-300" title={item.subnet_name}>
                      {item.subnet_cidr || item.subnet_id.slice(0, 8)}
                    </td>
                    <td className="h-11 px-3 font-mono">
                      {item.allocated_address ? (
                        <span className="text-zinc-100">{item.allocated_address}</span>
                      ) : item.requested_address ? (
                        <span className="text-zinc-300">{item.requested_address}</span>
                      ) : (
                        <span className="text-xs text-zinc-500">next free</span>
                      )}
                    </td>
                    <td className="h-11 px-3 text-zinc-400">{item.requester_email || "—"}</td>
                    <td
                      className="h-11 whitespace-nowrap px-3 text-xs text-zinc-400"
                      title={new Date(item.created_at).toLocaleString()}
                    >
                      {relativeTime(item.created_at)}
                    </td>
                    <td className="h-11 px-3">
                      <div className="flex items-center justify-end gap-1">
                        {canEdit && item.status === "pending" && (
                          <>
                            <Button size="sm" icon={<Check />} onClick={() => void act(item.id, "approve")}>
                              Approve
                            </Button>
                            <Button
                              size="sm"
                              variant="ghost"
                              className="hover:text-red-300"
                              onClick={() => void act(item.id, "deny")}
                            >
                              Deny
                            </Button>
                          </>
                        )}
                        {item.status === "pending" && (canEdit || item.requester_id === user?.id) && (
                          <button
                            type="button"
                            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                            aria-label={`Cancel request for ${item.hostname || "host"}`}
                            title="Cancel request"
                            onClick={() => void cancel(item.id)}
                          >
                            <X />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <Pagination page={page} pageCount={pages} total={total} onPageChange={setPage} />
      </div>
    </div>
  );
}
