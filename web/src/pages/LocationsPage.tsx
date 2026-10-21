import { type ColumnDef } from "@tanstack/react-table";
import { MapPin, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { api, type Location } from "../api";
import { DataTable } from "../components/DataTable";
import { useConfirm } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Button, Field } from "../components/ui";
import { useAuth } from "../auth";

export function LocationsPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const [items, setItems] = useState<Location[]>([]);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [editing, setEditing] = useState<Location | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [error, setError] = useState("");
  const isAdmin = user?.role === "admin";

  async function load() {
    try {
      setItems(await api<Location[]>("/api/v1/locations"));
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load sites", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  function openCreate() {
    setEditing(null);
    setName("");
    setDescription("");
    setError("");
    setDialogOpen(true);
  }

  function openEdit(loc: Location) {
    setEditing(loc);
    setName(loc.name);
    setDescription(loc.description || "");
    setError("");
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    setEditing(null);
    setName("");
    setDescription("");
    setError("");
  }

  const columns = useMemo<ColumnDef<Location>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ row }) => (
          <Link
            className="inline-flex items-center gap-2 font-medium text-zinc-100 hover:text-brand"
            to={`/locations/${row.original.id}`}
          >
            <MapPin className="h-4 w-4 text-zinc-500" />
            {row.original.name}
          </Link>
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
          ] as ColumnDef<Location>[])
        : []),
    ],
    [isAdmin, items],
  );

  async function onSave(e: FormEvent) {
    e.preventDefault();
    try {
      if (editing) {
        await api(`/api/v1/locations/${editing.id}`, {
          method: "PUT",
          body: JSON.stringify({ name, description }),
        });
        toast(`Saved ${name}`, "success");
      } else {
        await api("/api/v1/locations", {
          method: "POST",
          body: JSON.stringify({ name, description }),
        });
        toast(`Created site ${name}`, "success");
      }
      closeDialog();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    }
  }

  async function onDelete(loc: Location) {
    if (
      !(await confirm({ title: `Delete site ${loc.name}?`, danger: true, confirmLabel: "Delete" }))
    )
      return;
    try {
      await api(`/api/v1/locations/${loc.id}`, { method: "DELETE" });
      if (editing?.id === loc.id) closeDialog();
      toast(`Deleted ${loc.name}`, "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Sites"
        subtitle={`${items.length} site${items.length === 1 ? "" : "s"} · physical or logical groupings for subnets and VLANs`}
        actions={
          isAdmin && (
            <Button variant="primary" icon={<Plus />} onClick={openCreate}>
              New site
            </Button>
          )
        }
      />

      <FormDialog open={dialogOpen} title={editing ? "Edit site" : "New site"} onClose={closeDialog}>
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Name">
              <input
                className="wt-input w-full"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="HQ, DC – Ashburn…"
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
              {editing ? "Save site" : "Create site"}
            </Button>
          </div>
        </form>
      </FormDialog>

      {loading ? (
        <TableSkeleton rows={5} cols={3} />
      ) : items.length === 0 ? (
        <div className="wt-card px-6 py-12 text-center">
          <MapPin className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">No sites yet</p>
          <p className="mt-1 text-xs text-zinc-500">
            {isAdmin
              ? "Add a site to group subnets and VLANs by where they live."
              : "An admin can add sites to group subnets and VLANs."}
          </p>
          {isAdmin && (
            <Button variant="primary" icon={<Plus />} className="mt-4" onClick={openCreate}>
              New site
            </Button>
          )}
        </div>
      ) : (
        <DataTable data={items} columns={columns} filterPlaceholder="Filter sites…" />
      )}
    </div>
  );
}
