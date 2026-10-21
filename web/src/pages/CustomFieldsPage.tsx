import { useEffect, useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { ListPlus, Pencil, Plus, Trash2 } from "lucide-react";
import { api } from "../api";
import { useAuth } from "../auth";
import { useConfirm } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { useToast } from "../components/Toast";
import { Badge, Button, Field, Segmented } from "../components/ui";

type FieldDef = {
  id: string;
  entity_type: string;
  key: string;
  label: string;
  field_type: string;
  required: boolean;
  options: string[];
  sort_order: number;
};

const ENTITY_PLURAL = { subnet: "subnets", ip: "IP addresses" } as const;

export function CustomFieldsPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const [entityType, setEntityType] = useState<"subnet" | "ip">("subnet");
  const [items, setItems] = useState<FieldDef[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [key, setKey] = useState("");
  const [label, setLabel] = useState("");
  const [fieldType, setFieldType] = useState("string");
  const [required, setRequired] = useState(false);
  const [options, setOptions] = useState("");
  const [sortOrder, setSortOrder] = useState(0);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<FieldDef | null>(null);
  const [error, setError] = useState("");

  async function load(et = entityType) {
    try {
      setItems(await api<FieldDef[]>(`/api/v1/custom-fields?entity_type=${et}`));
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load", "error");
    } finally {
      setLoaded(true);
    }
  }

  useEffect(() => {
    if (user?.role === "admin") void load(entityType);
  }, [user?.role, entityType]);

  if (user?.role !== "admin") return <Navigate to="/" replace />;

  function openCreate() {
    setEditing(null);
    setKey("");
    setLabel("");
    setFieldType("string");
    setRequired(false);
    setOptions("");
    setSortOrder(0);
    setError("");
    setDialogOpen(true);
  }

  function openEdit(f: FieldDef) {
    setEditing(f);
    setKey(f.key);
    setLabel(f.label);
    setFieldType(f.field_type);
    setRequired(f.required);
    setOptions((f.options ?? []).join(", "));
    setSortOrder(f.sort_order);
    setError("");
    setDialogOpen(true);
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/api/v1/custom-fields", {
        method: "POST",
        body: JSON.stringify({
          entity_type: entityType,
          key,
          label,
          field_type: fieldType,
          required,
          options: options
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean),
          sort_order: sortOrder,
        }),
      });
      setKey("");
      setLabel("");
      setOptions("");
      setRequired(false);
      setDialogOpen(false);
      toast("Field definition created", "success");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create failed");
    }
  }

  async function onSaveEdit(e: FormEvent) {
    e.preventDefault();
    const f = editing;
    if (!f) return;
    try {
      await api(`/api/v1/custom-fields/${f.id}`, {
        method: "PUT",
        body: JSON.stringify({
          label: label.trim() || f.label,
          field_type: f.field_type,
          required,
          options: f.options,
          sort_order: f.sort_order,
        }),
      });
      setDialogOpen(false);
      setEditing(null);
      toast("Field definition updated", "success");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Update failed");
    }
  }

  async function onDelete(id: string) {
    if (
      !(await confirm({
        title: "Delete this field definition?",
        danger: true,
        confirmLabel: "Delete",
      }))
    )
      return;
    try {
      await api(`/api/v1/custom-fields/${id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Custom fields"
        subtitle="Typed field definitions for subnets and IPs, validated on create and update."
        actions={
          <Button variant="primary" icon={<Plus />} onClick={openCreate}>
            New {entityType === "ip" ? "IP" : "subnet"} field
          </Button>
        }
      />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Segmented
          label="Entity type"
          value={entityType}
          onChange={setEntityType}
          items={[
            { value: "subnet", label: "Subnets" },
            { value: "ip", label: "IP addresses" },
          ]}
        />
        <span className="text-xs text-zinc-500">
          {items.length} field{items.length === 1 ? "" : "s"}
        </span>
      </div>

      {loaded && items.length === 0 ? (
        <div className="wt-card flex flex-col items-center gap-2 px-6 py-12 text-center">
          <ListPlus className="h-6 w-6 text-zinc-500" />
          <p className="text-sm font-medium text-zinc-200">
            No custom fields for {ENTITY_PLURAL[entityType]}
          </p>
          <p className="text-xs text-zinc-500">
            Add fields like owner, environment, or ticket to capture extra metadata.
          </p>
          <Button variant="primary" icon={<Plus />} className="mt-2" onClick={openCreate}>
            New field
          </Button>
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                <th className="h-10 px-3">Key</th>
                <th className="h-10 px-3">Label</th>
                <th className="h-10 px-3">Type</th>
                <th className="h-10 px-3">Required</th>
                <th className="h-10 px-3">Options</th>
                <th className="h-10 px-3 text-right">Sort</th>
                <th className="h-10 px-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {items.map((f) => (
                <tr key={f.id}>
                  <td className="h-11 px-3 font-mono text-xs text-zinc-200">{f.key}</td>
                  <td className="h-11 px-3 text-zinc-100">{f.label}</td>
                  <td className="h-11 px-3">
                    <Badge tone={f.field_type === "select" ? "accent" : "neutral"} className="font-mono">
                      {f.field_type}
                    </Badge>
                  </td>
                  <td className="h-11 px-3">
                    {f.required ? <Badge tone="warn">required</Badge> : <span className="text-zinc-600">—</span>}
                  </td>
                  <td className="h-11 px-3">
                    {(f.options ?? []).length ? (
                      <div className="flex flex-wrap gap-1">
                        {f.options.map((o) => (
                          <span key={o} className="wt-badge font-mono">
                            {o}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <span className="text-zinc-600">—</span>
                    )}
                  </td>
                  <td className="h-11 px-3 text-right font-mono text-xs text-zinc-400 tabular-nums">
                    {f.sort_order}
                  </td>
                  <td className="h-11 px-3">
                    <div className="flex justify-end gap-0.5">
                      <button
                        type="button"
                        className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                        aria-label={`Edit ${f.key}`}
                        title="Edit"
                        onClick={() => openEdit(f)}
                      >
                        <Pencil />
                      </button>
                      <button
                        type="button"
                        className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300"
                        aria-label={`Delete ${f.key}`}
                        title="Delete"
                        onClick={() => void onDelete(f.id)}
                      >
                        <Trash2 />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {!loaded && (
                <tr>
                  <td colSpan={7} className="h-11 px-3 text-center text-zinc-500">
                    Loading…
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <FormDialog
        open={dialogOpen}
        title={
          editing
            ? `Edit field — ${editing.key}`
            : `New ${entityType === "ip" ? "IP" : "subnet"} field`
        }
        onClose={() => setDialogOpen(false)}
      >
        <form onSubmit={editing ? onSaveEdit : onCreate} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Key" hint={editing ? "Keys can't be changed" : "Stored identifier, e.g. owner"}>
              <input
                className="wt-input w-full font-mono disabled:opacity-50"
                value={key}
                onChange={(e) => setKey(e.target.value)}
                placeholder="owner"
                disabled={!!editing}
                autoFocus={!editing}
                required
              />
            </Field>
            <Field label="Label">
              <input
                className="wt-input w-full"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="Owner"
                autoFocus={!!editing}
                required
              />
            </Field>
            <Field label="Type">
              <select
                className="wt-input w-full disabled:opacity-50"
                value={fieldType}
                onChange={(e) => setFieldType(e.target.value)}
                disabled={!!editing}
              >
                <option value="string">string</option>
                <option value="number">number</option>
                <option value="boolean">boolean</option>
                <option value="select">select</option>
              </select>
            </Field>
            <Field label="Sort order">
              <input
                type="number"
                className="wt-input w-full disabled:opacity-50"
                value={sortOrder}
                onChange={(e) => setSortOrder(Number(e.target.value))}
                disabled={!!editing}
              />
            </Field>
            {fieldType === "select" && (
              <Field label="Options" hint="Comma-separated" className="sm:col-span-2">
                <input
                  className="wt-input w-full font-mono disabled:opacity-50"
                  value={options}
                  onChange={(e) => setOptions(e.target.value)}
                  placeholder="prod, staging, lab"
                  disabled={!!editing}
                />
              </Field>
            )}
          </div>
          <label className="flex items-center gap-2 text-sm text-zinc-300">
            <input type="checkbox" checked={required} onChange={(e) => setRequired(e.target.checked)} />
            Required
          </label>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Button variant="ghost" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="primary">
              {editing ? "Save field" : "Add field"}
            </Button>
          </div>
        </form>
      </FormDialog>
    </div>
  );
}
