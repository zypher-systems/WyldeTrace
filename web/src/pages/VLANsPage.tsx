import { type ColumnDef } from "@tanstack/react-table";
import { Layers, MapPin, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { api, type Location, type VLAN } from "../api";
import { DataTable } from "../components/DataTable";
import { useConfirm } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Badge, Button, Field } from "../components/ui";
import { useAuth } from "../auth";

export function VLANsPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const [items, setItems] = useState<VLAN[]>([]);
  const [locations, setLocations] = useState<Location[]>([]);
  const [loading, setLoading] = useState(true);
  const [vid, setVid] = useState("");
  const [name, setName] = useState("");
  const [locationId, setLocationId] = useState("");
  const [description, setDescription] = useState("");
  const [editing, setEditing] = useState<VLAN | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [error, setError] = useState("");
  const canEdit = user?.role === "admin" || user?.role === "editor";

  async function load() {
    try {
      const [vlans, locs] = await Promise.all([
        api<VLAN[]>("/api/v1/vlans"),
        api<Location[]>("/api/v1/locations"),
      ]);
      setItems(vlans);
      setLocations(locs);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load VLANs", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const locName = useMemo(() => {
    const m: Record<string, string> = {};
    for (const l of locations) m[l.id] = l.name;
    return m;
  }, [locations]);

  function resetForm() {
    setEditing(null);
    setVid("");
    setName("");
    setLocationId("");
    setDescription("");
    setError("");
  }

  function openCreate() {
    resetForm();
    setDialogOpen(true);
  }

  function openEdit(v: VLAN) {
    setEditing(v);
    setVid(String(v.vid));
    setName(v.name);
    setLocationId(v.location_id || "");
    setDescription(v.description || "");
    setError("");
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    resetForm();
  }

  const columns = useMemo<ColumnDef<VLAN>[]>(
    () => [
      {
        accessorKey: "vid",
        header: "VID",
        cell: ({ getValue }) => (
          <Badge tone="info" className="font-mono tabular-nums">
            {getValue() as number}
          </Badge>
        ),
      },
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ getValue }) =>
          (getValue() as string) ? (
            <span className="font-medium text-zinc-100">{getValue() as string}</span>
          ) : (
            <span className="text-zinc-600">—</span>
          ),
      },
      {
        id: "location",
        header: "Site",
        accessorFn: (row) => (row.location_id ? (locName[row.location_id] ?? row.location_id) : ""),
        cell: ({ row }) =>
          row.original.location_id ? (
            <Link
              to={`/locations/${row.original.location_id}`}
              className="inline-flex items-center gap-1.5 text-zinc-300 hover:text-brand"
            >
              <MapPin className="h-3.5 w-3.5 text-zinc-500" />
              {locName[row.original.location_id] ?? row.original.location_id.slice(0, 8)}
            </Link>
          ) : (
            <span className="text-zinc-500">Global</span>
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
      ...(canEdit
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
                    aria-label={`Edit VLAN ${row.original.vid}`}
                    title="Edit"
                    onClick={() => openEdit(row.original)}
                  >
                    <Pencil />
                  </button>
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                    aria-label={`Delete VLAN ${row.original.vid}`}
                    title="Delete"
                    onClick={() => void onDelete(row.original)}
                  >
                    <Trash2 />
                  </button>
                </div>
              ),
            },
          ] as ColumnDef<VLAN>[])
        : []),
    ],
    [canEdit, items, locName],
  );

  async function onSave(e: FormEvent) {
    e.preventDefault();
    const n = Number(vid);
    if (!Number.isInteger(n) || n < 1 || n > 4094) {
      setError("VID must be an integer from 1 to 4094");
      return;
    }
    const body = {
      vid: n,
      name,
      location_id: locationId || null,
      description,
    };
    try {
      if (editing) {
        await api(`/api/v1/vlans/${editing.id}`, {
          method: "PUT",
          body: JSON.stringify(body),
        });
        toast(`Saved VLAN ${n}`, "success");
      } else {
        await api("/api/v1/vlans", {
          method: "POST",
          body: JSON.stringify(body),
        });
        toast(`Created VLAN ${n}`, "success");
      }
      closeDialog();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    }
  }

  async function onDelete(v: VLAN) {
    if (!(await confirm({ title: `Delete VLAN ${v.vid}?`, danger: true, confirmLabel: "Delete" })))
      return;
    try {
      await api(`/api/v1/vlans/${v.id}`, { method: "DELETE" });
      if (editing?.id === v.id) closeDialog();
      toast(`Deleted VLAN ${v.vid}`, "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="VLANs"
        subtitle={`${items.length} VLAN${items.length === 1 ? "" : "s"} · inventory by VID and site`}
        actions={
          canEdit && (
            <Button variant="primary" icon={<Plus />} onClick={openCreate}>
              New VLAN
            </Button>
          )
        }
      />

      <FormDialog open={dialogOpen} title={editing ? "Edit VLAN" : "New VLAN"} onClose={closeDialog}>
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="VID" hint="1–4094">
              <input
                className="wt-input w-full font-mono"
                type="number"
                min={1}
                max={4094}
                value={vid}
                onChange={(e) => setVid(e.target.value)}
                required
                autoFocus
              />
            </Field>
            <Field label="Name">
              <input
                className="wt-input w-full"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            <Field label="Site">
              <select
                className="wt-input w-full"
                value={locationId}
                onChange={(e) => setLocationId(e.target.value)}
              >
                <option value="">Global / unscoped</option>
                {locations.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name}
                  </option>
                ))}
              </select>
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
              {editing ? "Save VLAN" : "Create VLAN"}
            </Button>
          </div>
        </form>
      </FormDialog>

      {loading ? (
        <TableSkeleton rows={6} cols={5} />
      ) : items.length === 0 ? (
        <div className="wt-card px-6 py-12 text-center">
          <Layers className="mx-auto h-6 w-6 text-zinc-600" />
          <p className="mt-2 text-sm text-zinc-300">No VLANs yet</p>
          <p className="mt-1 text-xs text-zinc-500">
            Record VLANs by VID, optionally scoped to a site, then attach them to subnets.
          </p>
          {canEdit && (
            <Button variant="primary" icon={<Plus />} className="mt-4" onClick={openCreate}>
              New VLAN
            </Button>
          )}
        </div>
      ) : (
        <DataTable data={items} columns={columns} filterPlaceholder="Filter VLANs…" />
      )}
    </div>
  );
}
