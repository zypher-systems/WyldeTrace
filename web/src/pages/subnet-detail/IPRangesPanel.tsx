import { Pencil, Plus, Rows3, Trash2 } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import type { IPRange } from "../../api";
import { FormDialog } from "../../components/FormDialog";
import { Badge, Button, Field } from "../../components/ui";

export function IPRangesPanel({
  canEdit,
  ranges,
  rangeName,
  setRangeName,
  rangeStart,
  setRangeStart,
  rangeEnd,
  setRangeEnd,
  rangeRole,
  setRangeRole,
  rangeDesc,
  setRangeDesc,
  editingRange,
  setEditingRange,
  onSubmit,
  onDelete,
}: {
  canEdit: boolean;
  ranges: IPRange[];
  rangeName: string;
  setRangeName: (v: string) => void;
  rangeStart: string;
  setRangeStart: (v: string) => void;
  rangeEnd: string;
  setRangeEnd: (v: string) => void;
  rangeRole: string;
  setRangeRole: (v: string) => void;
  rangeDesc: string;
  setRangeDesc: (v: string) => void;
  editingRange: IPRange | null;
  setEditingRange: (v: IPRange | null) => void;
  onSubmit: (e: FormEvent) => void;
  onDelete: (rng: IPRange) => void;
}) {
  const [open, setOpen] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  function resetFields() {
    setEditingRange(null);
    setRangeName("");
    setRangeStart("");
    setRangeEnd("");
    setRangeRole("dhcp");
    setRangeDesc("");
  }

  function openNew() {
    resetFields();
    setSubmitted(false);
    setOpen(true);
  }

  function openEdit(rng: IPRange) {
    setEditingRange(rng);
    setRangeName(rng.name);
    setRangeStart(rng.start_address);
    setRangeEnd(rng.end_address);
    setRangeRole(rng.role);
    setRangeDesc(rng.description || "");
    setSubmitted(false);
    setOpen(true);
  }

  function close() {
    setOpen(false);
    setSubmitted(false);
    resetFields();
  }

  // The parent clears the form state after a successful save; close the dialog then.
  useEffect(() => {
    if (open && submitted && !editingRange && rangeStart === "" && rangeEnd === "") {
      setOpen(false);
      setSubmitted(false);
    }
  }, [open, submitted, editingRange, rangeStart, rangeEnd]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-zinc-100">IP ranges</h2>
          <p className="mt-0.5 text-xs text-zinc-500">
            Named slices inside this subnet: DHCP pools, static blocks, reservations.
          </p>
        </div>
        {canEdit && (
          <Button variant="primary" icon={<Plus />} onClick={openNew}>
            New range
          </Button>
        )}
      </div>

      {ranges.length === 0 ? (
        <div className="wt-card px-6 py-10 text-center">
          <Rows3 className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">No ranges defined</p>
          <p className="mt-1 text-xs text-zinc-500">
            {canEdit
              ? "Carve out a DHCP pool or reserved block to document how this subnet is used."
              : "Nobody has documented ranges for this subnet yet."}
          </p>
          {canEdit && (
            <Button variant="primary" icon={<Plus />} className="mt-4" onClick={openNew}>
              New range
            </Button>
          )}
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                <th className="h-10 px-3">Name</th>
                <th className="h-10 px-3">Range</th>
                <th className="h-10 px-3">Role</th>
                <th className="h-10 px-3">Description</th>
                {canEdit && <th className="h-10 px-3" />}
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {ranges.map((rng) => (
                <tr key={rng.id}>
                  <td className="h-11 px-3 text-zinc-100">
                    {rng.name || <span className="text-zinc-600">Unnamed</span>}
                  </td>
                  <td className="h-11 whitespace-nowrap px-3 font-mono text-zinc-200">
                    {rng.start_address}
                    <span className="mx-1.5 text-zinc-600">–</span>
                    {rng.end_address}
                  </td>
                  <td className="h-11 px-3">
                    <Badge tone={roleTone[rng.role] ?? "neutral"}>{rng.role}</Badge>
                  </td>
                  <td className="h-11 max-w-md truncate px-3 text-zinc-400" title={rng.description || undefined}>
                    {rng.description || <span className="text-zinc-600">—</span>}
                  </td>
                  {canEdit && (
                    <td className="h-11 px-3">
                      <div className="flex justify-end gap-1">
                        <button
                          type="button"
                          className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                          aria-label={`Edit range ${rng.start_address}–${rng.end_address}`}
                          title="Edit"
                          onClick={() => openEdit(rng)}
                        >
                          <Pencil />
                        </button>
                        <button
                          type="button"
                          className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                          aria-label={`Delete range ${rng.start_address}–${rng.end_address}`}
                          title="Delete"
                          onClick={() => onDelete(rng)}
                        >
                          <Trash2 />
                        </button>
                      </div>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {canEdit && (
        <FormDialog open={open} title={editingRange ? "Edit range" : "New range"} onClose={close}>
          <form
            className="space-y-5"
            onSubmit={(e) => {
              setSubmitted(true);
              onSubmit(e);
            }}
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Name" className="sm:col-span-2">
                <input
                  className="wt-input w-full"
                  value={rangeName}
                  onChange={(e) => setRangeName(e.target.value)}
                  placeholder="DHCP pool"
                  autoFocus
                />
              </Field>
              <Field label="Start address">
                <input
                  className="wt-input w-full font-mono"
                  value={rangeStart}
                  onChange={(e) => setRangeStart(e.target.value)}
                  placeholder="10.0.0.10"
                  required
                />
              </Field>
              <Field label="End address">
                <input
                  className="wt-input w-full font-mono"
                  value={rangeEnd}
                  onChange={(e) => setRangeEnd(e.target.value)}
                  placeholder="10.0.0.200"
                  required
                />
              </Field>
              <Field label="Role">
                <select
                  className="wt-input w-full"
                  value={rangeRole}
                  onChange={(e) => setRangeRole(e.target.value)}
                >
                  <option value="dhcp">dhcp</option>
                  <option value="static">static</option>
                  <option value="reserved">reserved</option>
                  <option value="other">other</option>
                </select>
              </Field>
              <Field label="Description">
                <input
                  className="wt-input w-full"
                  value={rangeDesc}
                  onChange={(e) => setRangeDesc(e.target.value)}
                />
              </Field>
            </div>
            <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
              <Button variant="ghost" onClick={close}>
                Cancel
              </Button>
              <Button type="submit" variant="primary">
                {editingRange ? "Save range" : "Create range"}
              </Button>
            </div>
          </form>
        </FormDialog>
      )}
    </div>
  );
}

const roleTone: Record<string, "info" | "brand" | "warn" | "neutral"> = {
  dhcp: "info",
  static: "brand",
  reserved: "warn",
  other: "neutral",
};
