import { Field } from "./ui";

export type FieldDef = {
  id: string;
  key: string;
  label: string;
  field_type: string;
  required: boolean;
  options: string[];
  sort_order?: number;
};

/** Build a validated custom_fields object from form state + definitions. */
export function buildCustomFields(
  defs: FieldDef[],
  values: Record<string, unknown>,
): { fields: Record<string, unknown>; error?: string } {
  const customFields: Record<string, unknown> = {};
  for (const def of defs) {
    const v = values[def.key];
    if (v === undefined || v === "" || v === null) {
      if (def.required) {
        return { fields: {}, error: `${def.label} is required` };
      }
      continue;
    }
    if (def.field_type === "number") {
      const n = Number(v);
      if (Number.isNaN(n)) {
        return { fields: {}, error: `${def.label} must be a number` };
      }
      customFields[def.key] = n;
    } else if (def.field_type === "boolean") {
      customFields[def.key] = Boolean(v);
    } else {
      customFields[def.key] = v;
    }
  }
  for (const [k, v] of Object.entries(values)) {
    if (!(k in customFields) && !defs.some((d) => d.key === k)) {
      customFields[k] = v;
    }
  }
  return { fields: customFields };
}

type Props = {
  defs: FieldDef[];
  values: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
};

export function CustomFieldInputs({ defs, values, onChange }: Props) {
  if (defs.length === 0) return null;

  const setVal = (key: string, v: unknown) => onChange({ ...values, [key]: v });

  const label = (def: FieldDef) => (
    <>
      {def.label}
      {def.required ? <span className="ml-0.5 text-red-400"> *</span> : null}
    </>
  );

  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-4">
      {defs.map((def) => {
        const val = values[def.key];
        if (def.field_type === "boolean") {
          return (
            <label
              key={def.id}
              className="flex items-center gap-2 self-end pb-2 text-sm text-zinc-300"
            >
              <input
                type="checkbox"
                checked={Boolean(val)}
                onChange={(e) => setVal(def.key, e.target.checked)}
              />
              {label(def)}
            </label>
          );
        }
        if (def.field_type === "select") {
          return (
            <Field key={def.id} label={label(def)}>
              <select
                className="wt-input w-full"
                value={val == null ? "" : String(val)}
                onChange={(e) => setVal(def.key, e.target.value)}
                required={def.required}
              >
                <option value="">—</option>
                {def.options.map((o) => (
                  <option key={o} value={o}>
                    {o}
                  </option>
                ))}
              </select>
            </Field>
          );
        }
        return (
          <Field key={def.id} label={label(def)}>
            <input
              type={def.field_type === "number" ? "number" : "text"}
              className="wt-input w-full"
              value={val == null ? "" : String(val)}
              onChange={(e) => setVal(def.key, e.target.value)}
              required={def.required}
            />
          </Field>
        );
      })}
    </div>
  );
}
