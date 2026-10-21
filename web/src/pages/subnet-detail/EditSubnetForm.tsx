import { Trash2 } from "lucide-react";
import type { FormEvent } from "react";
import { DEFAULT_VRF_ID, type Location, type Subnet, type VRF } from "../../api";
import { CustomFieldInputs, type FieldDef } from "../../components/CustomFieldInputs";
import { Button, Field } from "../../components/ui";

export function EditSubnetForm({
  subnet,
  editName,
  setEditName,
  editDesc,
  setEditDesc,
  editVrfID,
  setEditVrfID,
  editVlan,
  setEditVlan,
  editLocationID,
  setEditLocationID,
  editTags,
  setEditTags,
  editParentID,
  setEditParentID,
  editFields,
  setEditFields,
  fieldDefs,
  locations,
  vrfs,
  allSubnets,
  onSubmit,
  onDelete,
  onCancel,
}: {
  subnet: Subnet;
  editName: string;
  setEditName: (v: string) => void;
  editDesc: string;
  setEditDesc: (v: string) => void;
  editVrfID: string;
  setEditVrfID: (v: string) => void;
  editVlan: string;
  setEditVlan: (v: string) => void;
  editLocationID: string;
  setEditLocationID: (v: string) => void;
  editTags: string;
  setEditTags: (v: string) => void;
  editParentID: string;
  setEditParentID: (v: string) => void;
  editFields: Record<string, unknown>;
  setEditFields: (v: Record<string, unknown>) => void;
  fieldDefs: FieldDef[];
  locations: Location[];
  vrfs: VRF[];
  allSubnets: Subnet[];
  onSubmit: (e: FormEvent) => void;
  onDelete: () => void;
  onCancel?: () => void;
}) {
  return (
    <form onSubmit={onSubmit} className="space-y-5">
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Name">
          <input
            className="wt-input w-full"
            value={editName}
            onChange={(e) => setEditName(e.target.value)}
            required
            autoFocus
          />
        </Field>
        <Field label="CIDR" hint="Use Resize under Child subnets to change the prefix">
          <input className="wt-input w-full font-mono text-zinc-400" value={subnet.cidr} readOnly disabled />
        </Field>
        <Field label="Description" className="sm:col-span-2">
          <input
            className="wt-input w-full"
            value={editDesc}
            onChange={(e) => setEditDesc(e.target.value)}
          />
        </Field>
        <Field label="VRF">
          <select
            className="wt-input w-full"
            value={editVrfID}
            onChange={(e) => setEditVrfID(e.target.value)}
          >
            {vrfs.length === 0 && <option value={DEFAULT_VRF_ID}>Default</option>}
            {vrfs.map((v) => (
              <option key={v.id} value={v.id}>
                {v.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="VLAN ID" hint="Optional, 1–4094">
          <input
            className="wt-input w-full font-mono"
            type="number"
            min={1}
            max={4094}
            value={editVlan}
            onChange={(e) => setEditVlan(e.target.value)}
            placeholder="optional"
          />
        </Field>
        <Field label="Site">
          <select
            className="wt-input w-full"
            value={editLocationID}
            onChange={(e) => setEditLocationID(e.target.value)}
          >
            <option value="">Unassigned</option>
            {locations.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Tags" hint="Comma-separated">
          <input
            className="wt-input w-full font-mono"
            value={editTags}
            onChange={(e) => setEditTags(e.target.value)}
            placeholder="prod, edge"
          />
        </Field>
        <Field label="Parent subnet" className="sm:col-span-2">
          <select
            className="wt-input w-full font-mono"
            value={editParentID}
            onChange={(e) => setEditParentID(e.target.value)}
          >
            <option value="">None (root)</option>
            {allSubnets
              .filter((s) => s.id !== subnet.id)
              .map((s) => (
                <option key={s.id} value={s.id}>
                  {s.cidr} — {s.name}
                </option>
              ))}
          </select>
        </Field>
      </div>
      {fieldDefs.length > 0 && (
        <div className="space-y-3 border-t border-[var(--color-border)] pt-4">
          <p className="wt-eyebrow text-zinc-500">Custom fields</p>
          <CustomFieldInputs defs={fieldDefs} values={editFields} onChange={setEditFields} />
        </div>
      )}
      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-[var(--color-border)] pt-4">
        <Button variant="danger" icon={<Trash2 />} onClick={() => onDelete()}>
          Delete subnet
        </Button>
        <div className="flex gap-2">
          {onCancel && (
            <Button variant="ghost" onClick={onCancel}>
              Cancel
            </Button>
          )}
          <Button type="submit" variant="primary">
            Save changes
          </Button>
        </div>
      </div>
    </form>
  );
}
