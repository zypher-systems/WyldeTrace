import type { FormEvent } from "react";
import { DEFAULT_VRF_ID, type Location, type VRF } from "../../api";
import { CustomFieldInputs, type FieldDef } from "../../components/CustomFieldInputs";
import { flattenTree, type TreeNode } from "./SubnetTree";

export type CreateSubnetFormProps = {
  cidr: string;
  setCidr: (v: string) => void;
  name: string;
  setName: (v: string) => void;
  vrfId: string;
  setVrfId: (v: string) => void;
  vlan: string;
  setVlan: (v: string) => void;
  locationId: string;
  setLocationId: (v: string) => void;
  tags: string;
  setTags: (v: string) => void;
  parentId: string;
  setParentId: (v: string) => void;
  locations: Location[];
  vrfs: VRF[];
  tree: TreeNode[];
  fieldDefs: FieldDef[];
  createFields: Record<string, unknown>;
  setCreateFields: (v: Record<string, unknown>) => void;
  onSubmit: (e: FormEvent) => void;
  onCancel?: () => void;
};

export function CreateSubnetForm(p: CreateSubnetFormProps) {
  return (
    <form
      id="create-subnet"
      onSubmit={p.onSubmit}
      className="space-y-5"
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block space-y-1.5">
          <span className="wt-label">CIDR</span>
          <input
            className="block w-full wt-input font-mono"
            value={p.cidr}
            onChange={(e) => p.setCidr(e.target.value)}
            placeholder="10.10.0.0/24"
            required
            autoFocus
          />
        </label>
        <label className="block space-y-1.5">
          <span className="wt-label">Name</span>
          <input
            className="block w-full wt-input"
            value={p.name}
            onChange={(e) => p.setName(e.target.value)}
            required
          />
        </label>
        <label className="block space-y-1.5">
          <span className="wt-label">VRF</span>
          <select
            className="block w-full wt-input"
            value={p.vrfId}
            onChange={(e) => p.setVrfId(e.target.value)}
          >
            {p.vrfs.length === 0 && <option value={DEFAULT_VRF_ID}>Default</option>}
            {p.vrfs.map((v) => (
              <option key={v.id} value={v.id}>
                {v.name}
              </option>
            ))}
          </select>
        </label>
        <label className="block space-y-1.5">
          <span className="wt-label">VLAN ID <span className="font-normal text-zinc-500">(optional)</span></span>
          <input
            className="block w-full wt-input font-mono"
            type="number"
            min={1}
            max={4094}
            value={p.vlan}
            onChange={(e) => p.setVlan(e.target.value)}
            placeholder="10"
          />
        </label>
        <label className="block space-y-1.5">
          <span className="wt-label">Site</span>
          <select
            className="block w-full wt-input"
            value={p.locationId}
            onChange={(e) => p.setLocationId(e.target.value)}
          >
            <option value="">Unassigned</option>
            {p.locations.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </label>
        <label className="block space-y-1.5">
          <span className="wt-label">Tags</span>
          <input
            className="block w-full wt-input font-mono"
            value={p.tags}
            onChange={(e) => p.setTags(e.target.value)}
            placeholder="prod, edge"
          />
        </label>
        <label className="block space-y-1.5">
          <span className="wt-label">Parent <span className="font-normal text-zinc-500">(optional)</span></span>
          <select
            className="block w-full wt-input"
            value={p.parentId}
            onChange={(e) => p.setParentId(e.target.value)}
          >
            <option value="">Auto-detect / none</option>
            {flattenTree(p.tree).map((s) => (
              <option key={s.id} value={s.id}>
                {s.cidr} — {s.name}
              </option>
            ))}
          </select>
        </label>
      </div>
      {p.fieldDefs.length > 0 && (
        <CustomFieldInputs defs={p.fieldDefs} values={p.createFields} onChange={p.setCreateFields} />
      )}
      <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
        {p.onCancel && (
          <button type="button" className="wt-btn wt-btn-ghost" onClick={p.onCancel}>
            Cancel
          </button>
        )}
        <button type="submit" className="wt-btn wt-btn-primary">
          Create subnet
        </button>
      </div>
    </form>
  );
}
