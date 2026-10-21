import { type ColumnDef } from "@tanstack/react-table";
import { Network, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { api, DEFAULT_VRF_ID, type VRF } from "../api";
import { DataTable } from "../components/DataTable";
import { useConfirm } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Badge, Button, Field } from "../components/ui";
import { useAuth } from "../auth";

export function VRFsPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const [items, setItems] = useState<VRF[]>([]);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState("");
  const [rd, setRd] = useState("");
  const [description, setDescription] = useState("");
  const [editing, setEditing] = useState<VRF | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [error, setError] = useState("");
  const isAdmin = user?.role === "admin";

  async function load() {
    try {
      setItems(await api<VRF[]>("/api/v1/vrfs"));
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load VRFs", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  function resetForm() {
    setEditing(null);
    setName("");
    setRd("");
    setDescription("");
    setError("");
  }

  function openCreate() {
    resetForm();
    setDialogOpen(true);
  }

  function openEdit(v: VRF) {
    setEditing(v);
    setName(v.name);
    setRd(v.rd || "");
    setDescription(v.description || "");
    setError("");
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    resetForm();
  }

  const columns = useMemo<ColumnDef<VRF>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ row }) => (
          <span className="inline-flex items-center gap-2 font-medium text-zinc-100">
            {row.original.name}
            {row.original.id === DEFAULT_VRF_ID && <Badge className="h-5 text-[10px]">default</Badge>}
          </span>
        ),
      },
      {
        accessorKey: "rd",
        header: "Route distinguisher",
        cell: ({ getValue }) =>
          (getValue() as string) ? (
            <span className="font-mono text-xs text-zinc-300">{getValue() as string}</span>
          ) : (
            <span className="text-zinc-600">—</span>
          ),
      },
      {
        accessorKey: "description",
        header: "Description",
        cell: ({ getValue }) =>
          (getValue() as string) ? (
            <span className="text-zinc-400">{getValue() as string}</span>
          ) : (
            <span className="text-zinc-600">—</span>
          ),
      },
      ...(isAdmin
        ? ([
            {
              id: "actions",
              header: "",
              enableSorting: false,
              cell: ({ row }) => (
                <div className="flex justify-end gap-1">
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                    aria-label={`Edit ${row.original.name}`}
                    title="Edit"
                    onClick={() => openEdit(row.original)}
                  >
                    <Pencil />
                  </button>
                  {row.original.id !== DEFAULT_VRF_ID ? (
                    <button
                      type="button"
                      className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                      aria-label={`Delete ${row.original.name}`}
                      title="Delete"
                      onClick={() => void onDelete(row.original)}
                    >
                      <Trash2 />
                    </button>
                  ) : (
                    <span className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm invisible" aria-hidden>
                      <Trash2 />
                    </span>
                  )}
                </div>
              ),
            },
          ] as ColumnDef<VRF>[])
        : []),
    ],
    [isAdmin, items],
  );

  async function onSave(e: FormEvent) {
    e.preventDefault();
    try {
      if (editing) {
        await api(`/api/v1/vrfs/${editing.id}`, {
          method: "PUT",
          body: JSON.stringify({ name, rd, description }),
        });
        toast(`Saved VRF ${name}`, "success");
      } else {
        await api("/api/v1/vrfs", {
          method: "POST",
          body: JSON.stringify({ name, rd, description }),
        });
        toast(`Created VRF ${name}`, "success");
      }
      closeDialog();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    }
  }

  async function onDelete(v: VRF) {
    if (!(await confirm({ title: `Delete VRF ${v.name}?`, danger: true, confirmLabel: "Delete" })))
      return;
    try {
      await api(`/api/v1/vrfs/${v.id}`, { method: "DELETE" });
      if (editing?.id === v.id) closeDialog();
      toast(`Deleted VRF ${v.name}`, "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="VRFs"
        subtitle={`${items.length} routing domain${items.length === 1 ? "" : "s"} · overlapping CIDR spaces stay separate per VRF`}
        actions={
          isAdmin && (
            <Button variant="primary" icon={<Plus />} onClick={openCreate}>
              New VRF
            </Button>
          )
        }
      />

      <FormDialog open={dialogOpen} title={editing ? "Edit VRF" : "New VRF"} onClose={closeDialog}>
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Name">
              <input
                className="wt-input w-full"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                autoFocus
              />
            </Field>
            <Field label="Route distinguisher" hint="Optional, e.g. ASN:number">
              <input
                className="wt-input w-full font-mono"
                value={rd}
                onChange={(e) => setRd(e.target.value)}
                placeholder="65000:1"
              />
            </Field>
            <Field label="Description" className="sm:col-span-2">
              <input
                className="wt-input w-full"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </Field>
          </div>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Button variant="ghost" onClick={closeDialog}>
              Cancel
            </Button>
            <Button type="submit" variant="primary">
              {editing ? "Save VRF" : "Create VRF"}
            </Button>
          </div>
        </form>
      </FormDialog>

      {loading ? (
        <TableSkeleton rows={4} cols={4} />
      ) : items.length === 0 ? (
        <div className="wt-card px-6 py-12 text-center">
          <Network className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">No VRFs</p>
          <p className="mt-1 text-xs text-zinc-500">
            Add a VRF when you need overlapping address space kept apart.
          </p>
          {isAdmin && (
            <Button variant="primary" icon={<Plus />} className="mt-4" onClick={openCreate}>
              New VRF
            </Button>
          )}
        </div>
      ) : (
        <DataTable data={items} columns={columns} filterPlaceholder="Filter VRFs…" />
      )}
    </div>
  );
}
