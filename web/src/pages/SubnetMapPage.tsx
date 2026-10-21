import { useEffect, useLayoutEffect, useRef, useState, type FormEvent } from "react";
import { X } from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { api, pageQuery, type IPAddress, type PageResult, type Subnet } from "../api";
import {
  buildCustomFields,
  CustomFieldInputs,
  type FieldDef,
} from "../components/CustomFieldInputs";
import { IPStatusGrid, type IPMapCell, type IPMapData } from "../components/IPStatusGrid";
import { PageHeader } from "../components/PageHeader";
import { useAuth } from "../auth";
import { useFocusTrap } from "../hooks/useFocusTrap";

type DeviceType = { id: string; name: string; description: string };

type PopoverState = {
  cell: IPMapCell;
  top: number;
  left: number;
};

/** Occupancy grid for a subnet; `embedded` renders it inside the subnet detail tabs. */
export function SubnetMapPage({ embedded = false }: { embedded?: boolean }) {
  const { id } = useParams();
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [subnet, setSubnet] = useState<Subnet | null>(null);
  const [ipMap, setIPMap] = useState<IPMapData | null>(null);
  const [deviceTypes, setDeviceTypes] = useState<DeviceType[]>([]);
  const [ipFieldDefs, setIPFieldDefs] = useState<FieldDef[]>([]);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  const [popover, setPopover] = useState<PopoverState | null>(null);
  const [editingIP, setEditingIP] = useState<IPAddress | null>(null);
  const [address, setAddress] = useState("");
  const [hostname, setHostname] = useState("");
  const [status, setStatus] = useState("assigned");
  const [deviceTypeID, setDeviceTypeID] = useState("");
  const [ipFields, setIPFields] = useState<Record<string, unknown>>({});
  const popoverRef = useRef<HTMLDivElement | null>(null);
  useFocusTrap(!!popover, popoverRef);

  async function load() {
    if (!id) return;
    try {
      const [s, map, dts, defs] = await Promise.all([
        api<Subnet>(`/api/v1/subnets/${id}`),
        api<IPMapData>(`/api/v1/subnets/${id}/ip-map`),
        api<DeviceType[]>("/api/v1/device-types").catch(() => []),
        api<FieldDef[]>("/api/v1/custom-fields?entity_type=ip").catch(() => []),
      ]);
      setSubnet(s);
      setIPMap(map);
      setDeviceTypes(dts);
      setIPFieldDefs(defs);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load address map");
    }
  }

  useEffect(() => {
    void load();
  }, [id]);

  useLayoutEffect(() => {
    if (!popover || !popoverRef.current) return;
    const el = popoverRef.current;
    const rect = el.getBoundingClientRect();
    const pad = 12;
    let top = popover.top;
    let left = popover.left;
    if (top + rect.height > window.innerHeight - pad) {
      top = Math.max(pad, window.innerHeight - rect.height - pad);
    }
    if (left + rect.width > window.innerWidth - pad) {
      left = Math.max(pad, window.innerWidth - rect.width - pad);
    }
    if (top !== popover.top || left !== popover.left) {
      setPopover((p) => (p ? { ...p, top, left } : p));
    }
  }, [popover?.cell.address, editingIP, canEdit]);

  function closePopover() {
    setPopover(null);
    setEditingIP(null);
    setAddress("");
    setHostname("");
    setStatus("assigned");
    setDeviceTypeID("");
    setIPFields({});
  }

  useEffect(() => {
    if (!popover) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") closePopover();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [popover]);

  function fillIPForm(row: IPAddress) {
    setEditingIP(row);
    setAddress(row.address);
    setHostname(row.hostname || "");
    setStatus(row.status);
    setDeviceTypeID(row.device_type_id || "");
    setIPFields({ ...(row.custom_fields ?? {}) });
  }

  function fillAddForm(cell: IPMapCell) {
    setEditingIP(null);
    setAddress(cell.address);
    setHostname(cell.hostname || "");
    setStatus("assigned");
    setDeviceTypeID("");
    setIPFields({});
  }

  async function onSelect(cell: IPMapCell, anchor: DOMRect) {
    if (cell.status === "special") return;
    const top = Math.min(anchor.bottom + 8, window.innerHeight - 40);
    const left = Math.min(anchor.left, window.innerWidth - 360);
    setPopover({ cell, top, left });
    setMsg("");
    setError("");
    try {
      if (cell.id && id) {
        const res = await api<PageResult<IPAddress>>(
          `/api/v1/subnets/${id}/ips${pageQuery({ page: 1, pageSize: 5, q: cell.address })}`,
        );
        const row =
          res.items.find((ip) => ip.id === cell.id) ||
          res.items.find((ip) => ip.address === cell.address);
        if (row) fillIPForm(row);
        else fillAddForm(cell);
      } else {
        fillAddForm(cell);
      }
    } catch {
      fillAddForm(cell);
    }
  }

  async function saveIP(e: FormEvent) {
    e.preventDefault();
    if (!id || !canEdit) return;
    const built = buildCustomFields(ipFieldDefs, ipFields);
    if (built.error) {
      setError(built.error);
      return;
    }
    setBusy(true);
    try {
      const body: Record<string, unknown> = {
        address,
        hostname,
        status,
        notes: status === "reserved" ? "manual reservation" : "",
        device_type_id: deviceTypeID || null,
        custom_fields: built.fields,
      };
      if (status === "reserved") {
        body.reserved_until = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString();
      }
      if (editingIP) {
        await api(`/api/v1/ips/${editingIP.id}`, {
          method: "PUT",
          body: JSON.stringify({
            ...body,
            address: editingIP.address,
            mac_address: editingIP.mac_address,
            open_ports: editingIP.open_ports,
            notes: editingIP.notes || body.notes,
            location_id: editingIP.location_id,
            reserved_until: editingIP.reserved_until,
          }),
        });
        setMsg(`Updated ${editingIP.address}`);
      } else {
        await api(`/api/v1/subnets/${id}/ips`, {
          method: "POST",
          body: JSON.stringify(body),
        });
        setMsg(`Added ${address}`);
      }
      closePopover();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      {!embedded && (
        <div>
          <Link to={`/subnets/${id}`} className="text-sm text-zinc-400 hover:text-zinc-200">
            ← Back to subnet
          </Link>
          <PageHeader
            title={subnet ? `${subnet.cidr} map` : "Address map"}
            subtitle={subnet?.name || "Full-view occupancy grid"}
          />
        </div>
      )}

      {error && <p className="text-sm text-red-400">{error}</p>}
      {msg && <p className="text-sm text-brand">{msg}</p>}

      {ipMap && (
        <IPStatusGrid
          data={ipMap}
          fullView
          childLinkSuffix={embedded ? "?tab=map" : "/map"}
          selectedAddress={popover?.cell.address}
          onSelect={(cell, anchor) => void onSelect(cell, anchor)}
        />
      )}

      {popover && (
        <>
          <button
            type="button"
            className="fixed inset-0 z-40 cursor-default bg-black/30 wt-animate-fade"
            aria-label="Close"
            onClick={closePopover}
          />
          <div
            ref={popoverRef}
            className="fixed z-50 w-[min(22rem,calc(100vw-1.5rem))] rounded-xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)] p-4 shadow-[var(--shadow-pop)] wt-animate-pop"
            style={{ top: popover.top, left: popover.left }}
            role="dialog"
            aria-label={editingIP ? "Edit IP" : "Add IP"}
          >
            <div className="mb-3 flex items-start justify-between gap-2">
              <div>
                <p className="wt-eyebrow">{editingIP ? "Edit address" : "Add address"}</p>
                <p className="mt-0.5 font-mono text-base text-zinc-50">{address || popover.cell.address}</p>
                {popover.cell.hostname && !hostname && (
                  <p className="text-xs text-zinc-400">{popover.cell.hostname}</p>
                )}
              </div>
              <button
                type="button"
                className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm -mr-1.5 -mt-1"
                aria-label="Close"
                onClick={closePopover}
              >
                <X />
              </button>
            </div>

            {canEdit ? (
              <form onSubmit={(e) => void saveIP(e)} className="space-y-3">
                <label className="block space-y-1.5">
                  <span className="wt-label">Hostname / client</span>
                  <input
                    className="wt-input w-full"
                    value={hostname}
                    onChange={(e) => setHostname(e.target.value)}
                    autoFocus
                    placeholder="hostname"
                  />
                </label>
                <label className="block space-y-1.5">
                  <span className="wt-label">Status</span>
                  <select
                    className="wt-input w-full"
                    value={status}
                    onChange={(e) => setStatus(e.target.value)}
                  >
                    <option value="assigned">assigned</option>
                    <option value="reserved">reserved</option>
                    <option value="dhcp">dhcp</option>
                    <option value="available">available</option>
                  </select>
                </label>
                <label className="block space-y-1.5">
                  <span className="wt-label">Device type</span>
                  <select
                    className="wt-input w-full"
                    value={deviceTypeID}
                    onChange={(e) => setDeviceTypeID(e.target.value)}
                  >
                    <option value="">—</option>
                    {deviceTypes.map((d) => (
                      <option key={d.id} value={d.id}>
                        {d.name}
                      </option>
                    ))}
                  </select>
                </label>
                {ipFieldDefs.length > 0 && (
                  <CustomFieldInputs defs={ipFieldDefs} values={ipFields} onChange={setIPFields} />
                )}
                <div className="flex justify-end gap-2 pt-1">
                  <button type="button" className="wt-btn wt-btn-ghost" onClick={closePopover}>
                    Cancel
                  </button>
                  <button type="submit" disabled={busy} className="wt-btn wt-btn-primary disabled:opacity-60">
                    {busy ? "Saving…" : editingIP ? "Update" : "Add"}
                  </button>
                </div>
              </form>
            ) : (
              <dl className="space-y-1 text-xs text-zinc-400">
                <div className="flex justify-between gap-3">
                  <dt>Status</dt>
                  <dd className="font-mono text-zinc-200">{popover.cell.status}</dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt>Client</dt>
                  <dd className="text-zinc-200">{popover.cell.hostname || "—"}</dd>
                </div>
              </dl>
            )}
          </div>
        </>
      )}
    </div>
  );
}
