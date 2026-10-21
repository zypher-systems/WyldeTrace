import type { Location } from "../../api";
import type { FieldDef } from "../../components/CustomFieldInputs";

export type SubnetBulkBarProps = {
  selectedCount: number;
  bulkTags: string;
  setBulkTags: (v: string) => void;
  bulkTagMode: "add" | "remove" | "set";
  setBulkTagMode: (v: "add" | "remove" | "set") => void;
  bulkLocationId: string;
  setBulkLocationId: (v: string) => void;
  locations: Location[];
  fieldDefs: FieldDef[];
  bulkFieldKey: string;
  setBulkFieldKey: (v: string) => void;
  bulkFieldValue: string;
  setBulkFieldValue: (v: string) => void;
  onApplyTags: () => void;
  onApplyLocation: () => void;
  onApplyField: () => void;
  onClear: () => void;
};

export function SubnetBulkBar(p: SubnetBulkBarProps) {
  return (
    <div className="flex flex-wrap items-end gap-3 rounded-xl border border-brand/25 bg-brand/[0.06] px-4 py-3 wt-animate-pop">
      <div className="text-sm text-zinc-300">
        <span className="font-medium text-brand">{p.selectedCount}</span> selected
      </div>
      <label className="block space-y-1.5">
        <span className="wt-label">Tags</span>
        <input
          className="block wt-input font-mono"
          value={p.bulkTags}
          onChange={(e) => p.setBulkTags(e.target.value)}
          placeholder="prod, edge"
        />
      </label>
      <label className="block space-y-1.5">
        <span className="wt-label">Tag mode</span>
        <select
          className="block wt-input"
          value={p.bulkTagMode}
          onChange={(e) => p.setBulkTagMode(e.target.value as "add" | "remove" | "set")}
        >
          <option value="add">Add</option>
          <option value="remove">Remove</option>
          <option value="set">Replace</option>
        </select>
      </label>
      <button
        type="button"
        className="wt-btn wt-btn-secondary"
        onClick={() => p.onApplyTags()}
      >
        Apply tags
      </button>
      <label className="block space-y-1.5">
        <span className="wt-label">Location</span>
        <select
          className="block wt-input"
          value={p.bulkLocationId}
          onChange={(e) => p.setBulkLocationId(e.target.value)}
        >
          <option value="">Unassigned</option>
          {p.locations.map((l) => (
            <option key={l.id} value={l.id}>
              {l.name}
            </option>
          ))}
        </select>
      </label>
      <button
        type="button"
        className="wt-btn wt-btn-secondary"
        onClick={() => p.onApplyLocation()}
      >
        Assign location
      </button>
      {p.fieldDefs.length > 0 && (
        <>
          <label className="block space-y-1.5">
            <span className="wt-label">Custom field</span>
            <select
              className="block wt-input"
              value={p.bulkFieldKey}
              onChange={(e) => p.setBulkFieldKey(e.target.value)}
            >
              <option value="">Select…</option>
              {p.fieldDefs.map((d) => (
                <option key={d.key} value={d.key}>
                  {d.label || d.key}
                </option>
              ))}
            </select>
          </label>
          <label className="block space-y-1.5">
            <span className="wt-label">Value</span>
            <input
              className="block wt-input"
              value={p.bulkFieldValue}
              onChange={(e) => p.setBulkFieldValue(e.target.value)}
            />
          </label>
          <button
            type="button"
            className="wt-btn wt-btn-secondary"
            onClick={() => p.onApplyField()}
          >
            Set field
          </button>
        </>
      )}
      <button
        type="button"
        className="wt-btn wt-btn-ghost"
        onClick={() => p.onClear()}
      >
        Clear
      </button>
    </div>
  );
}
