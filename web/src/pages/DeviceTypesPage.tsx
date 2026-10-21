import { type ColumnDef } from "@tanstack/react-table";
import { Cpu, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { api } from "../api";
import { DataTable } from "../components/DataTable";
import { useConfirm } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Button, Field } from "../components/ui";
import { useAuth } from "../auth";

type DeviceType = {
  id: string;
  name: string;
  description: string;
};

export function DeviceTypesPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const isAdmin = user?.role === "admin";
  const [items, setItems] = useState<DeviceType[]>([]);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [editing, setEditing] = useState<DeviceType | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [error, setError] = useState("");

  async function load() {
    try {
      setItems(await api<DeviceType[]>("/api/v1/device-types"));
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load device types", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (user?.role !== "reportview") void load();
  }, [user?.role]);

  function resetForm() {
    setEditing(null);
    setName("");
    setDescription("");
    setError("");
  }

  function openCreate() {
    resetForm();
    setDialogOpen(true);
  }

  function openEdit(t: DeviceType) {
    setEditing(t);
    setName(t.name);
    setDescription(t.description || "");
    setError("");
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    resetForm();
  }

  const columns = useMemo<ColumnDef<DeviceType>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ getValue }) => (
          <span className="font-medium text-zinc-100">{getValue() as string}</span>
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
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                    aria-label={`Delete ${row.original.name}`}
                    title="Delete"
                    onClick={() => void onDelete(row.original)}
                  >
                    <Trash2 />
                  </button>
                </div>
              ),
            },
          ] as ColumnDef<DeviceType>[])
        : []),
    ],
    [isAdmin, items],
  );

  if (user?.role === "reportview") return <Navigate to="/" replace />;

  async function onSave(e: FormEvent) {
    e.preventDefault();
    try {
      if (editing) {
        await api(`/api/v1/device-types/${editing.id}`, {
          method: "PUT",
          body: JSON.stringify({ name, description }),
        });
        toast(`Saved ${name}`, "success");
      } else {
        await api("/api/v1/device-types", {
          method: "POST",
          body: JSON.stringify({ name, description }),
        });
        toast(`Created device type ${name}`, "success");
      }
      closeDialog();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    }
  }

  async function onDelete(t: DeviceType) {
    if (
      !(await confirm({ title: `Delete device type ${t.name}?`, danger: true, confirmLabel: "Delete" }))
    )
      return;
    try {
      await api(`/api/v1/device-types/${t.id}`, { method: "DELETE" });
      if (editing?.id === t.id) closeDialog();
      toast(`Deleted ${t.name}`, "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Device types"
        subtitle={`${items.length} type${items.length === 1 ? "" : "s"} · catalog used for IP classification hints`}
        actions={
          isAdmin && (
            <Button variant="primary" icon={<Plus />} onClick={openCreate}>
              New device type
            </Button>
          )
        }
      />

      <FormDialog
        open={dialogOpen}
        title={editing ? "Edit device type" : "New device type"}
        onClose={closeDialog}
      >
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Name">
              <input
                className="wt-input w-full"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Printer, Access point…"
                required
                autoFocus
              />
            </Field>
            <Field label="Description">
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
              {editing ? "Save device type" : "Create device type"}
            </Button>
          </div>
        </form>
      </FormDialog>

      {loading ? (
        <TableSkeleton rows={5} cols={3} />
      ) : items.length === 0 ? (
        <div className="wt-card px-6 py-12 text-center">
          <Cpu className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">No device types</p>
          <p className="mt-1 text-xs text-zinc-500">
            Device types classify IP records (router, printer, camera…).
          </p>
          {isAdmin && (
            <Button variant="primary" icon={<Plus />} className="mt-4" onClick={openCreate}>
              New device type
            </Button>
          )}
        </div>
      ) : (
        <DataTable data={items} columns={columns} filterPlaceholder="Filter device types…" />
      )}
    </div>
  );
}
