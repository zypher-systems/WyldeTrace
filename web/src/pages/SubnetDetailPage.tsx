import { type ColumnDef, type SortingState } from "@tanstack/react-table";
import {
  BellRing,
  ChevronRight,
  Columns3,
  Copy,
  GitBranch,
  Grid3x3,
  List,
  MapPin,
  MoreHorizontal,
  Pencil,
  Plus,
  Radar,
  Rows3,
  Scissors,
  Trash2,
  Zap,
} from "lucide-react";
import { useEffect, useMemo, useState, type FormEvent, type MouseEvent as ReactMouseEvent } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import {
  api,
  DEFAULT_VRF_ID,
  pageQuery,
  type ClaimResponse,
  type IPAddress,
  type IPRange,
  type Location,
  type PageResult,
  type Subnet,
  type SweepReport,
  type Utilization,
  type VRF,
} from "../api";
import {
  buildCustomFields,
  CustomFieldInputs,
  type FieldDef,
} from "../components/CustomFieldInputs";
import { useConfirm } from "../components/ConfirmDialog";
import { useContextMenu, type ContextMenuItem } from "../components/ContextMenu";
import { DataTable } from "../components/DataTable";
import { FormDialog } from "../components/FormDialog";
import { DropdownMenu, type DropdownItem } from "../components/DropdownMenu";
import { Badge, Button, Field, Meter, StatusBadge, Tabs } from "../components/ui";
import { SubnetMapPage } from "./SubnetMapPage";
import { useToast } from "../components/Toast";
import { useAuth } from "../auth";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";
import { EditSubnetForm } from "./subnet-detail/EditSubnetForm";
import { IPRangesPanel } from "./subnet-detail/IPRangesPanel";
import { PrefixOpsPanel } from "./subnet-detail/PrefixOpsPanel";
import { IPHistoryPanel, type IPHistoryRow } from "./subnet-detail/IPHistoryPanel";

const CF_PINS_KEY = "wyldetrace.ip_cf_columns";

function loadCFPins(): string[] {
  try {
    const raw = localStorage.getItem(CF_PINS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) ? parsed.filter((x) => typeof x === "string") : [];
  } catch {
    return [];
  }
}

function saveCFPins(keys: string[]) {
  localStorage.setItem(CF_PINS_KEY, JSON.stringify(keys));
}

type DeviceType = { id: string; name: string; description: string };

type SubnetTab = "addresses" | "map" | "ranges" | "children" | "discovery";

export function SubnetDetailPage() {
  const { id } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const { open: openMenu, menu } = useContextMenu();
  const [subnet, setSubnet] = useState<Subnet | null>(null);
  const [util, setUtil] = useState<Utilization | null>(null);
  const [ips, setIps] = useState<IPAddress[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [filter, setFilter] = useState(() => searchParams.get("q") || "");
  const statusFilter = searchParams.get("status") || "";
  const [sorting, setSorting] = useState<SortingState>([{ id: "address", desc: false }]);
  const [ipsLoading, setIpsLoading] = useState(false);

  function patchIPParams(next: Record<string, string | null>) {
    const sp = new URLSearchParams(searchParams);
    for (const [k, v] of Object.entries(next)) {
      if (v == null || v === "") sp.delete(k);
      else sp.set(k, v);
    }
    setSearchParams(sp, { replace: true });
  }
  const [children, setChildren] = useState<Subnet[]>([]);
  const [address, setAddress] = useState("");
  const [hostname, setHostname] = useState("");
  const [status, setStatus] = useState("assigned");
  const [deviceTypeID, setDeviceTypeID] = useState("");
  const [deviceTypes, setDeviceTypes] = useState<DeviceType[]>([]);
  const [fieldDefs, setFieldDefs] = useState<FieldDef[]>([]);
  const [ipFieldDefs, setIPFieldDefs] = useState<FieldDef[]>([]);
  const [ipFields, setIPFields] = useState<Record<string, unknown>>({});
  const [sweep, setSweep] = useState<SweepReport | null>(null);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [selectedIPs, setSelectedIPs] = useState<Record<string, boolean>>({});
  const [bulkStatus, setBulkStatus] = useState("assigned");
  const [bulkDeviceTypeID, setBulkDeviceTypeID] = useState("");
  const [bulkFieldKey, setBulkFieldKey] = useState("");
  const [bulkFieldValue, setBulkFieldValue] = useState("");
  const [historyAddr, setHistoryAddr] = useState("");
  const [history, setHistory] = useState<IPHistoryRow[]>([]);
  const [editName, setEditName] = useState("");
  const [editDesc, setEditDesc] = useState("");
  const [editVlan, setEditVlan] = useState("");
  const [editVrfID, setEditVrfID] = useState(DEFAULT_VRF_ID);
  const [editLocationID, setEditLocationID] = useState("");
  const [editTags, setEditTags] = useState("");
  const [editParentID, setEditParentID] = useState("");
  const [allSubnets, setAllSubnets] = useState<Subnet[]>([]);
  const [locations, setLocations] = useState<Location[]>([]);
  const [vrfs, setVrfs] = useState<VRF[]>([]);
  const [ranges, setRanges] = useState<IPRange[]>([]);
  const [rangeName, setRangeName] = useState("");
  const [rangeStart, setRangeStart] = useState("");
  const [rangeEnd, setRangeEnd] = useState("");
  const [rangeRole, setRangeRole] = useState("dhcp");
  const [rangeDesc, setRangeDesc] = useState("");
  const [editingRange, setEditingRange] = useState<IPRange | null>(null);
  const [ipMac, setIpMac] = useState("");
  const [ipPorts, setIpPorts] = useState("");
  const [ipNotes, setIpNotes] = useState("");
  const [ipReservedUntil, setIpReservedUntil] = useState("");
  const [editFields, setEditFields] = useState<Record<string, unknown>>({});
  const [visibleCFKeys, setVisibleCFKeys] = useState<string[]>(() => loadCFPins());
  const [editingIP, setEditingIP] = useState<IPAddress | null>(null);
  const [subnetEditOpen, setSubnetEditOpen] = useState(false);
  const [ipFormOpen, setIPFormOpen] = useState(false);
  const [utilAlertPercent, setUtilAlertPercent] = useState("");
  const [globalUtilAlert, setGlobalUtilAlert] = useState(90);
  const [childPrefixLen, setChildPrefixLen] = useState("");
  const [childName, setChildName] = useState("");
  const [resizeCIDR, setResizeCIDR] = useState("");
  const [mergeSiblingID, setMergeSiblingID] = useState("");
  const [alertOpen, setAlertOpen] = useState(false);
  const [scanning, setScanning] = useState(false);
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const pageSize = 50;
  const deviceNameByID = useMemo(() => {
    const m: Record<string, string> = {};
    for (const d of deviceTypes) m[d.id] = d.name;
    return m;
  }, [deviceTypes]);

  async function loadHistory(addr: string) {
    setHistoryAddr(addr);
    try {
      setHistory(await api<IPHistoryRow[]>(`/api/v1/ip-history?address=${encodeURIComponent(addr)}`));
    } catch {
      setHistory([]);
    }
  }

  async function loadMeta() {
    if (!id) return;
    try {
      const [s, u, kids, settings, rng] = await Promise.all([
        api<Subnet>(`/api/v1/subnets/${id}`),
        api<Utilization>(`/api/v1/subnets/${id}/utilization`),
        api<Subnet[]>(`/api/v1/subnets/${id}/children`),
        api<{
          util_alert_percent: number | null;
          global_util_alert_percent: number;
        }>(`/api/v1/subnets/${id}/settings`).catch(() => ({
          util_alert_percent: null,
          global_util_alert_percent: 90,
        })),
        api<IPRange[]>(`/api/v1/subnets/${id}/ranges`).catch(() => []),
      ]);
      setSubnet(s);
      syncSubnetEditFields(s);
      setUtil(u);
      setGlobalUtilAlert(settings.global_util_alert_percent);
      setUtilAlertPercent(
        settings.util_alert_percent != null ? String(settings.util_alert_percent) : "",
      );
      setChildren(kids);
      setRanges(rng);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load", "error");
    }
  }

  async function loadIPs(nextPage = page, q = filter, st = statusFilter, sortState = sorting) {
    if (!id) return;
    const sortCol = sortState[0]?.id || "address";
    const sortOrder = sortState[0]?.desc ? "desc" : "asc";
    setIpsLoading(true);
    try {
      const list = await api<PageResult<IPAddress>>(
        `/api/v1/subnets/${id}/ips${pageQuery({
          page: nextPage,
          pageSize,
          q,
          status: st,
          sort: sortCol,
          order: sortOrder,
        })}`,
      );
      setIps(list.items);
      setTotal(list.total);
      setPage(list.page);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load IPs", "error");
    } finally {
      setIpsLoading(false);
    }
  }

  async function load() {
    await Promise.all([loadMeta(), loadIPs()]);
  }

  const editDirty = useMemo(() => {
    if (!subnet) return false;
    const tagsJoined = (subnet.tags ?? []).join(", ");
    const vlanStr = subnet.vlan_vid != null ? String(subnet.vlan_vid) : "";
    return (
      editName !== subnet.name ||
      editDesc !== (subnet.description || "") ||
      editVlan !== vlanStr ||
      editVrfID !== (subnet.vrf_id || DEFAULT_VRF_ID) ||
      editLocationID !== (subnet.location_id || "") ||
      editTags !== tagsJoined ||
      editParentID !== (subnet.parent_id || "") ||
      JSON.stringify(editFields) !== JSON.stringify(subnet.custom_fields ?? {})
    );
  }, [
    subnet,
    editName,
    editDesc,
    editVlan,
    editVrfID,
    editLocationID,
    editTags,
    editParentID,
    editFields,
  ]);
  useUnsavedChanges(subnetEditOpen && editDirty);

  function syncSubnetEditFields(s: Subnet) {
    setEditName(s.name);
    setEditDesc(s.description || "");
    setEditVlan(s.vlan_vid != null ? String(s.vlan_vid) : "");
    setEditVrfID(s.vrf_id || DEFAULT_VRF_ID);
    setEditLocationID(s.location_id || "");
    setEditTags((s.tags ?? []).join(", "));
    setEditParentID(s.parent_id || "");
    setEditFields({ ...(s.custom_fields ?? {}) });
  }

  function openSubnetEdit() {
    if (!subnet) return;
    syncSubnetEditFields(subnet);
    setSubnetEditOpen(true);
  }

  function closeSubnetEdit() {
    if (subnet) syncSubnetEditFields(subnet);
    setSubnetEditOpen(false);
  }

  function resetIPForm() {
    setEditingIP(null);
    setAddress("");
    setHostname("");
    setStatus("assigned");
    setDeviceTypeID("");
    setIpMac("");
    setIpPorts("");
    setIpNotes("");
    setIpReservedUntil("");
    setIPFields({});
  }

  function openAddIP() {
    resetIPForm();
    setIPFormOpen(true);
  }

  function closeIPForm() {
    resetIPForm();
    setIPFormOpen(false);
  }

  useEffect(() => {
    const q = searchParams.get("q") || "";
    setFilter((prev) => (prev === q ? prev : q));
  }, [searchParams]);

  useEffect(() => {
    void loadMeta();
  }, [id]);

  useEffect(() => {
    void loadIPs(page, filter, statusFilter, sorting);
  }, [id, page, filter, statusFilter, sorting]);

  useEffect(() => {
    void (async () => {
      try {
        const [types, subnetDefs, ipDefs, subs, locs, vrfList] = await Promise.all([
          api<DeviceType[]>("/api/v1/device-types"),
          api<FieldDef[]>("/api/v1/custom-fields?entity_type=subnet"),
          api<FieldDef[]>("/api/v1/custom-fields?entity_type=ip"),
          api<PageResult<Subnet>>(`/api/v1/subnets${pageQuery({ page: 1, pageSize: 200 })}`),
          api<Location[]>("/api/v1/locations"),
          api<VRF[]>("/api/v1/vrfs"),
        ]);
        setDeviceTypes(types);
        setAllSubnets(subs.items);
        setFieldDefs(subnetDefs);
        setIPFieldDefs(ipDefs);
        setLocations(locs);
        setVrfs(vrfList);
      } catch {
        /* optional metadata */
      }
    })();
  }, []);

  const selectedIPIDs = Object.keys(selectedIPs).filter((id) => selectedIPs[id]);

  function toggleIPSelect(ipID: string) {
    setSelectedIPs((prev) => ({ ...prev, [ipID]: !prev[ipID] }));
  }

  function toggleIPSelectAll() {
    const all = ips.length > 0 && ips.every((ip) => selectedIPs[ip.id]);
    setSelectedIPs((prev) => {
      const next = { ...prev };
      for (const ip of ips) next[ip.id] = !all;
      return next;
    });
  }

  async function applyBulkIPs(kind: "status" | "device" | "delete" | "field") {
    if (!selectedIPIDs.length) return;
    try {
      const body: Record<string, unknown> = { ids: selectedIPIDs };
      if (kind === "status") body.status = bulkStatus;
      if (kind === "device") {
        body.update_device_type = true;
        body.device_type_id = bulkDeviceTypeID || null;
      }
      if (kind === "field") {
        if (!bulkFieldKey) return;
        body.set_custom_fields = { [bulkFieldKey]: bulkFieldValue };
      }
      if (kind === "delete") {
        if (
          !(await confirm({
            title: `Soft-delete ${selectedIPIDs.length} IP(s)?`,
            danger: true,
            confirmLabel: "Delete",
          }))
        )
          return;
        body.soft_delete = true;
      }
      const res = await api<{ updated: number; deleted: number; skipped: number }>("/api/v1/ips/bulk", {
        method: "POST",
        body: JSON.stringify(body),
      });
      toast(
        kind === "delete"
          ? `Deleted ${res.deleted} IP(s)` + (res.skipped ? ` · ${res.skipped} skipped` : "")
          : `Updated ${res.updated} IP(s)` + (res.skipped ? ` · ${res.skipped} skipped` : ""),
        "success",
      );
      setSelectedIPs({});
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Bulk IP update failed", "error");
    }
  }

  const columns = useMemo<ColumnDef<IPAddress>[]>(
    () => [
      ...(canEdit
        ? ([
            {
              id: "select",
              enableSorting: false,
              header: () => (
                <input
                  type="checkbox"
                  checked={ips.length > 0 && ips.every((ip) => selectedIPs[ip.id])}
                  onChange={toggleIPSelectAll}
                  aria-label="Select all IPs on page"
                />
              ),
              cell: ({ row }) => (
                <input
                  type="checkbox"
                  checked={!!selectedIPs[row.original.id]}
                  onChange={() => toggleIPSelect(row.original.id)}
                  aria-label={`Select ${row.original.address}`}
                />
              ),
            },
          ] as ColumnDef<IPAddress>[])
        : []),
      {
        accessorKey: "address",
        header: "Address",
        cell: ({ row }) => (
          <div onContextMenu={(e) => openIPMenu(e, row.original)}>
            <button
              type="button"
              className="font-mono text-brand hover:underline"
              onClick={() => void loadHistory(row.original.address)}
            >
              {row.original.address}
            </button>
          </div>
        ),
      },
      {
        accessorKey: "hostname",
        header: "Hostname",
        cell: ({ getValue }) => (
          <span className={getValue() ? "text-zinc-100" : "text-zinc-600"}>
            {String(getValue() || "—")}
          </span>
        ),
      },
      {
        accessorKey: "status",
        header: "Status",
        cell: ({ getValue }) => <StatusBadge status={String(getValue())} />,
      },
      {
        id: "device_type",
        enableSorting: false,
        header: "Type",
        cell: ({ row }) => (
          <span className="text-xs text-zinc-300">
            {row.original.device_type_id
              ? deviceNameByID[row.original.device_type_id] || "—"
              : "—"}
          </span>
        ),
      },
      ...(visibleCFKeys.length
        ? visibleCFKeys.map(
            (key) =>
              ({
                id: `cf_${key}`,
                enableSorting: false,
                header: ipFieldDefs.find((d) => d.key === key)?.label || key,
                cell: ({ row }) => {
                  const v = row.original.custom_fields?.[key];
                  return (
                    <span className="text-xs text-zinc-300">
                      {v === undefined || v === null || v === "" ? "—" : String(v)}
                    </span>
                  );
                },
              }) as ColumnDef<IPAddress>,
          )
        : ipFieldDefs.length === 0
          ? []
          : ([
            {
              id: "custom_fields",
              enableSorting: false,
              header: "Fields",
              cell: ({ row }) => {
                const cf = row.original.custom_fields ?? {};
                const entries = Object.entries(cf).filter(([, v]) => v !== "" && v != null);
                if (entries.length === 0) return <span className="text-zinc-600">—</span>;
                const labelByKey = Object.fromEntries(ipFieldDefs.map((d) => [d.key, d.label]));
                return (
                  <div className="max-w-[200px] space-y-0.5 text-xs text-zinc-400">
                    {entries.slice(0, 3).map(([k, v]) => (
                      <div key={k} className="truncate">
                        <span className="text-zinc-500">{labelByKey[k] || k}:</span>{" "}
                        <span className="text-zinc-300">{String(v)}</span>
                      </div>
                    ))}
                    {entries.length > 3 && (
                      <div className="text-zinc-600">+{entries.length - 3} more</div>
                    )}
                  </div>
                );
              },
            },
          ] as ColumnDef<IPAddress>[])),
      {
        accessorKey: "mac_address",
        enableSorting: false,
        header: "MAC",
        cell: ({ getValue }) => (
          <span className="font-mono text-xs text-zinc-400">{String(getValue() || "—")}</span>
        ),
      },
      {
        accessorKey: "vendor",
        enableSorting: false,
        header: "Vendor",
        cell: ({ row }) => (
          <span className="whitespace-nowrap text-xs text-zinc-400">
            {row.original.vendor || "—"}
            {row.original.device_hint ? (
              <span className="text-zinc-500"> · {row.original.device_hint}</span>
            ) : null}
          </span>
        ),
      },
      {
        accessorKey: "open_ports",
        enableSorting: false,
        header: "Ports",
        cell: ({ getValue }) => (
          <span className="font-mono text-xs text-zinc-400">{String(getValue() || "—")}</span>
        ),
      },
      {
        accessorKey: "last_seen_at",
        header: "Last seen",
        cell: ({ getValue }) => {
          const v = getValue();
          return v ? (
            <span className="whitespace-nowrap text-xs text-zinc-400" title={new Date(String(v)).toLocaleString()}>
              {new Date(String(v)).toLocaleDateString()}
            </span>
          ) : (
            <span className="text-zinc-600">—</span>
          );
        },
      },
      {
        accessorKey: "notes",
        enableSorting: false,
        header: "Notes",
        cell: ({ getValue }) => (
          <span className="block max-w-[14rem] truncate text-xs text-zinc-400" title={String(getValue() || "")}>
            {String(getValue() || "")}
          </span>
        ),
      },
      ...(canEdit
        ? ([
            {
              id: "actions",
              enableSorting: false,
              header: "",
              cell: ({ row }) => (
                <div className="flex justify-end gap-0.5">
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                    aria-label={`Edit ${row.original.address}`}
                    title="Edit"
                    onClick={() => startEditIP(row.original)}
                  >
                    <Pencil />
                  </button>
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                    aria-label={`More actions for ${row.original.address}`}
                    title="More"
                    onClick={(e) => openIPMenu(e, row.original)}
                  >
                    <MoreHorizontal />
                  </button>
                </div>
              ),
            },
          ] as ColumnDef<IPAddress>[])
        : []),
    ],
    [canEdit, deviceNameByID, ipFieldDefs, ips, selectedIPs, visibleCFKeys],
  );

  function startEditIP(ip: IPAddress) {
    setEditingIP(ip);
    setAddress(ip.address);
    setHostname(ip.hostname || "");
    setStatus(ip.status);
    setDeviceTypeID(ip.device_type_id || "");
    setIpMac(ip.mac_address || "");
    setIpPorts(ip.open_ports || "");
    setIpNotes(ip.notes || "");
    setIpReservedUntil(
      ip.reserved_until ? new Date(ip.reserved_until).toISOString().slice(0, 16) : "",
    );
    setIPFields({ ...(ip.custom_fields ?? {}) });
    setIPFormOpen(true);
  }

  function openIPMenu(e: ReactMouseEvent, ip: IPAddress) {
    const items: ContextMenuItem[] = [
      {
        id: "copy",
        label: "Copy address",
        onSelect: () => void navigator.clipboard.writeText(ip.address),
      },
      {
        id: "history",
        label: "History",
        onSelect: () => void loadHistory(ip.address),
      },
    ];
    if (canEdit) {
      items.unshift(
        {
          id: "edit",
          label: "Edit",
          onSelect: () => startEditIP(ip),
        },
        {
          id: "delete",
          label: "Soft-delete",
          danger: true,
          onSelect: () => void deleteIP(ip.id),
        },
      );
    }
    openMenu(e, items);
  }

  async function deleteIP(ipID: string) {
    if (!(await confirm({ title: "Soft-delete this IP?", danger: true, confirmLabel: "Delete" })))
      return;
    try {
      await api(`/api/v1/ips/${ipID}`, { method: "DELETE" });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  async function saveSubnet(e: FormEvent) {
    e.preventDefault();
    if (!subnet) return;
    const built = buildCustomFields(fieldDefs, editFields);
    if (built.error) {
      toast(built.error, "error");
      return;
    }
    const vlanVID = parseVlanID(editVlan);
    if (editVlan.trim() && vlanVID == null) {
      toast("VLAN must be an integer from 1 to 4094", "error");
      return;
    }
    try {
      await api(`/api/v1/subnets/${subnet.id}`, {
        method: "PUT",
        body: JSON.stringify({
          cidr: subnet.cidr,
          name: editName,
          description: editDesc,
          location_id: editLocationID || null,
          vrf_id: editVrfID || DEFAULT_VRF_ID,
          vlan_vid: vlanVID,
          parent_id: editParentID || null,
          tags: editTags
            .split(",")
            .map((t) => t.trim())
            .filter(Boolean),
          custom_fields: built.fields,
        }),
      });
      toast("Subnet updated", "success");
      setSubnetEditOpen(false);
      await loadMeta();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Subnet update failed", "error");
    }
  }

  async function saveRange(e: FormEvent) {
    e.preventDefault();
    if (!id) return;
    const body = {
      name: rangeName,
      start_address: rangeStart,
      end_address: rangeEnd,
      role: rangeRole,
      description: rangeDesc,
    };
    const updating = !!editingRange;
    try {
      if (editingRange) {
        await api(`/api/v1/subnets/${id}/ranges/${editingRange.id}`, {
          method: "PUT",
          body: JSON.stringify(body),
        });
      } else {
        await api(`/api/v1/subnets/${id}/ranges`, {
          method: "POST",
          body: JSON.stringify(body),
        });
      }
      setEditingRange(null);
      setRangeName("");
      setRangeStart("");
      setRangeEnd("");
      setRangeRole("dhcp");
      setRangeDesc("");
      toast(updating ? "Range updated" : "Range created", "info");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Range save failed", "error");
    }
  }

  async function deleteRange(range: IPRange) {
    if (!id) return;
    if (
      !(await confirm({
        title: `Delete range ${range.start_address}–${range.end_address}?`,
        danger: true,
        confirmLabel: "Delete",
      }))
    )
      return;
    try {
      await api(`/api/v1/subnets/${id}/ranges/${range.id}`, { method: "DELETE" });
      if (editingRange?.id === range.id) setEditingRange(null);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Range delete failed", "error");
    }
  }

  async function deleteSubnet() {
    if (!subnet) return;
    if (
      !(await confirm({
        title: `Soft-delete subnet ${subnet.cidr}?`,
        body: "You can restore it from Deleted subnets.",
        danger: true,
        confirmLabel: "Delete",
      }))
    )
      return;
    try {
      await api(`/api/v1/subnets/${subnet.id}`, { method: "DELETE" });
      window.location.href = "/subnets";
    } catch (err) {
      toast(err instanceof Error ? err.message : "Subnet delete failed", "error");
    }
  }

  async function createIP(e: FormEvent): Promise<boolean> {
    e.preventDefault();
    if (!id) return false;
    const built = buildCustomFields(ipFieldDefs, ipFields);
    if (built.error) {
      toast(built.error, "error");
      return false;
    }
    try {
      const notes =
        ipNotes || (status === "reserved" && !editingIP ? "manual reservation" : "");
      let reservedUntil: string | null = ipReservedUntil
        ? new Date(ipReservedUntil).toISOString()
        : null;
      if (status === "reserved" && !reservedUntil) {
        reservedUntil = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString();
      }
      if (status !== "reserved") reservedUntil = null;
      const body: Record<string, unknown> = {
        address,
        hostname,
        status,
        notes,
        mac_address: ipMac,
        open_ports: ipPorts,
        device_type_id: deviceTypeID || null,
        reserved_until: reservedUntil,
        custom_fields: built.fields,
      };
      if (editingIP) {
        await api(`/api/v1/ips/${editingIP.id}`, {
          method: "PUT",
          body: JSON.stringify({
            ...body,
            address: editingIP.address,
            location_id: editingIP.location_id,
          }),
        });
        toast(`Updated ${editingIP.address}`, "success");
      } else {
        await api(`/api/v1/subnets/${id}/ips`, {
          method: "POST",
          body: JSON.stringify(body),
        });
        toast(`Created ${address}`, "success");
      }
      closeIPForm();
      await load();
      return true;
    } catch (err) {
      toast(err instanceof Error ? err.message : "Save failed", "error");
      return false;
    }
  }

  async function allocateNext() {
    if (!id) return;
    try {
      const ip = await api<IPAddress>(`/api/v1/subnets/${id}/ips/next`, {
        method: "POST",
        body: JSON.stringify({ hostname: "", status: "assigned" }),
      });
      toast(`Allocated ${ip.address} (${ip.status})`, "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Allocate failed", "error");
    }
  }

  async function runSweep() {
    if (!subnet || !id) return;
    toast("Scanning + enriching devices…", "info");
    setSweep(null);
    setSelected({});
    try {
      const report = await api<SweepReport>("/api/v1/scanner/sweep", {
        method: "POST",
        body: JSON.stringify({ cidr: subnet.cidr, subnet_id: id, enrich: true }),
      });
      setSweep(report);
      const pick: Record<string, boolean> = {};
      for (const h of report.reconcile?.new_alive ?? []) {
        pick[h.ip] = true;
      }
      setSelected(pick);
      const stale = report.reconcile?.stale?.length ?? 0;
      const neu = report.reconcile?.new_alive?.length ?? 0;
      toast(
        `Sweep: ${report.alive}/${report.total} alive · ${neu} new · ${stale} stale in DB`,
        "info",
      );
    } catch (err) {
      toast(err instanceof Error ? err.message : "Sweep failed", "error");
    }
  }

  async function claimSelected() {
    if (!id || !sweep) return;
    const hosts = (sweep.reconcile?.new_alive ?? [])
      .concat(sweep.reconcile?.matched ?? [])
      .filter((h) => selected[h.ip])
      .map((h) => ({
        address: h.ip,
        hostname: h.hostname || "",
        mac: h.mac || "",
        open_ports: (h.open_ports ?? []).join(","),
      }));
    if (hosts.length === 0) {
      toast("Select at least one host to claim", "error");
      return;
    }
    try {
      const res = await api<ClaimResponse>("/api/v1/scanner/claim", {
        method: "POST",
        body: JSON.stringify({
          subnet_id: id,
          hosts,
          status: "assigned",
          notes: "claimed from discovery sweep",
        }),
      });
      toast(
        `Claimed: ${res.created} created, ${res.updated} updated, ${res.conflicts} conflicts`,
        "success",
      );
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Claim failed", "error");
    }
  }

  const sparseUtil = Boolean(util && (util.sparse || util.total == null));
  const pct = sparseUtil ? null : Math.min(100, Math.round(util?.percent ?? 0));
  const discoveryRows = [
    ...(sweep?.reconcile?.new_alive ?? []).map((h) => ({ ...h, kind: "new" as const })),
    ...(sweep?.reconcile?.matched ?? []).map((h) => ({ ...h, kind: "matched" as const })),
    ...(sweep?.reconcile?.stale ?? []).map((h) => ({ ...h, kind: "stale" as const })),
  ];
  const claimCount = Object.values(selected).filter(Boolean).length;
  const parent = subnet?.parent_id ? allSubnets.find((s) => s.id === subnet.parent_id) : undefined;
  const vrfName = subnet?.vrf_id ? vrfs.find((v) => v.id === subnet.vrf_id)?.name ?? "Default" : null;
  const siteName = subnet?.location_id
    ? locations.find((l) => l.id === subnet.location_id)?.name ?? subnet.location_id.slice(0, 8)
    : null;
  const tab = (searchParams.get("tab") as SubnetTab | null) ?? "addresses";
  function setTab(next: SubnetTab) {
    patchIPParams({ tab: next === "addresses" ? null : next });
  }
  const alertLabel = utilAlertPercent ? `${utilAlertPercent}%` : `${globalUtilAlert}% (global)`;

  async function scanNow() {
    setTab("discovery");
    setScanning(true);
    try {
      await runSweep();
    } finally {
      setScanning(false);
    }
  }

  async function saveAlertThreshold() {
    if (!id) return;
    try {
      if (!utilAlertPercent.trim()) {
        await api(`/api/v1/subnets/${id}/settings`, {
          method: "PUT",
          body: JSON.stringify({ clear: true }),
        });
        toast("Using global utilization alert threshold", "info");
      } else {
        const n = Number(utilAlertPercent);
        await api(`/api/v1/subnets/${id}/settings`, {
          method: "PUT",
          body: JSON.stringify({ util_alert_percent: n }),
        });
        toast(`Subnet alert threshold set to ${n}%`, "info");
      }
      setAlertOpen(false);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Threshold update failed", "error");
    }
  }

  const moreItems: DropdownItem[] = [
    ...(canEdit && subnet
      ? [
          { label: "Edit details", icon: <Pencil />, onClick: openSubnetEdit },
          { label: "Alert threshold…", icon: <BellRing />, onClick: () => setAlertOpen(true) },
          { label: "Split, resize or merge…", icon: <Scissors />, onClick: () => setTab("children") },
        ]
      : []),
    {
      label: "Copy CIDR",
      icon: <Copy />,
      onClick: () => subnet && void navigator.clipboard.writeText(subnet.cidr),
      separated: canEdit,
    },
    ...(canEdit && subnet
      ? [
          {
            label: "Delete subnet",
            icon: <Trash2 />,
            danger: true,
            separated: true,
            onClick: () => void deleteSubnet(),
          },
        ]
      : []),
  ];

  return (
    <div className="space-y-6">
      {menu}

      {/* ---- Header ---- */}
      <div className="space-y-5">
        <nav className="flex items-center gap-1.5 text-xs text-zinc-500" aria-label="Breadcrumb">
          <Link to="/subnets" className="hover:text-zinc-200">
            Subnets
          </Link>
          {parent && (
            <>
              <ChevronRight className="h-3 w-3" />
              <Link to={`/subnets/${parent.id}`} className="font-mono hover:text-zinc-200">
                {parent.cidr}
              </Link>
            </>
          )}
          <ChevronRight className="h-3 w-3" />
          <span className="font-mono text-zinc-300">{subnet?.cidr ?? "…"}</span>
        </nav>

        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0 wt-animate-in">
            <h1 className="wt-mono-hero text-3xl text-zinc-50 md:text-[2.25rem]">
              {subnet?.cidr ?? "…"}
            </h1>
            <p className="mt-1 text-[15px] text-zinc-400">{subnet?.name || "Unnamed subnet"}</p>
            <div className="mt-3 flex flex-wrap items-center gap-1.5">
              {vrfName && <Badge tone="accent">VRF {vrfName}</Badge>}
              {subnet?.vlan_vid != null && <Badge tone="info">VLAN {subnet.vlan_vid}</Badge>}
              {siteName && subnet?.location_id && (
                <Link to={`/locations/${subnet.location_id}`}>
                  <Badge className="hover:border-zinc-500">
                    <MapPin className="h-3 w-3" />
                    {siteName}
                  </Badge>
                </Link>
              )}
              {subnet?.tags?.map((t) => (
                <Link key={t} to={`/subnets?tags=${encodeURIComponent(t)}`}>
                  <Badge className="font-mono hover:border-brand/50 hover:text-brand">#{t}</Badge>
                </Link>
              ))}
            </div>
            {subnet?.description && (
              <p className="mt-3 max-w-2xl text-sm text-zinc-500">{subnet.description}</p>
            )}
          </div>
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            {canEdit && (
              <>
                <Button icon={<Radar />} onClick={() => void scanNow()} disabled={scanning}>
                  {scanning ? "Scanning…" : "Scan"}
                </Button>
                <Button icon={<Plus />} onClick={openAddIP}>
                  Add IP
                </Button>
                <Button variant="primary" icon={<Zap />} onClick={() => void allocateNext()}>
                  Allocate next
                </Button>
              </>
            )}
            <DropdownMenu label="More actions" iconOnly items={moreItems} />
          </div>
        </div>

        {/* ---- Stats strip ---- */}
        {util && (
          <div className="wt-card grid gap-px overflow-hidden bg-[var(--color-border)] sm:grid-cols-2 lg:grid-cols-4">
            <div className="bg-[var(--color-surface-raised)] p-5 lg:col-span-2">
              <div className="flex items-baseline justify-between gap-3">
                <span className="wt-eyebrow">Utilization</span>
                {canEdit ? (
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 text-[11px] text-zinc-500 hover:text-zinc-300"
                    onClick={() => setAlertOpen(true)}
                  >
                    <BellRing className="h-3 w-3" /> alert at {alertLabel}
                  </button>
                ) : (
                  <span className="text-[11px] text-zinc-500">alert at {alertLabel}</span>
                )}
              </div>
              {sparseUtil ? (
                <p className="mt-2 text-sm text-zinc-300">
                  <span className="text-2xl font-semibold tabular-nums text-zinc-50">{util.used}</span>{" "}
                  registered · sparse prefix, no percentage
                </p>
              ) : (
                <>
                  <div className="mt-2 flex items-baseline gap-2">
                    <span className="text-3xl font-semibold tabular-nums tracking-tight text-zinc-50">
                      {pct}%
                    </span>
                    <span className="text-sm text-zinc-500 tabular-nums">
                      {util.used} of {util.total} used · {util.free} free
                    </span>
                  </div>
                  <Meter value={pct ?? 0} className="mt-3 h-2" />
                </>
              )}
            </div>
            <div className="bg-[var(--color-surface-raised)] p-5">
              <span className="wt-eyebrow">Next free</span>
              <div className="mt-2 flex items-center gap-2">
                <span className="font-mono text-lg text-zinc-50">{util.next_ip || "—"}</span>
                {util.next_ip && (
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                    aria-label="Copy next free address"
                    title="Copy"
                    onClick={() => void navigator.clipboard.writeText(util.next_ip!)}
                  >
                    <Copy />
                  </button>
                )}
              </div>
              <p className="mt-1 text-xs text-zinc-500">
                {util.next_ip_error || (canEdit ? "Allocate next claims it" : "First unassigned address")}
              </p>
            </div>
            <div className="grid grid-cols-2 gap-px bg-[var(--color-border)]">
              <button
                type="button"
                className="bg-[var(--color-surface-raised)] p-5 text-left transition hover:bg-[var(--color-surface-hover)]"
                onClick={() => setTab("ranges")}
              >
                <span className="wt-eyebrow">Ranges</span>
                <div className="mt-2 text-lg font-semibold tabular-nums text-zinc-50">{ranges.length}</div>
              </button>
              <button
                type="button"
                className="bg-[var(--color-surface-raised)] p-5 text-left transition hover:bg-[var(--color-surface-hover)]"
                onClick={() => setTab("children")}
              >
                <span className="wt-eyebrow">Children</span>
                <div className="mt-2 text-lg font-semibold tabular-nums text-zinc-50">{children.length}</div>
              </button>
            </div>
          </div>
        )}
      </div>

      {/* ---- Tabs ---- */}
      <Tabs<SubnetTab>
        value={tab}
        onChange={setTab}
        items={[
          { value: "addresses", label: "Addresses", icon: <List />, count: total },
          { value: "map", label: "Map", icon: <Grid3x3 /> },
          { value: "ranges", label: "Ranges", icon: <Rows3 />, count: ranges.length },
          { value: "children", label: "Child subnets", icon: <GitBranch />, count: children.length },
          ...(canEdit
            ? [{ value: "discovery" as const, label: "Discovery", icon: <Radar />, count: discoveryRows.length }]
            : []),
        ]}
      />

      {tab === "addresses" && (
        <div className="space-y-3">
          <DataTable
            data={ips}
            columns={columns}
            filterPlaceholder="Filter by address, hostname, MAC…"
            loading={ipsLoading}
            emptyMessage={
              filter || statusFilter ? (
                "No addresses match these filters"
              ) : (
                <span>
                  No addresses recorded yet.
                  {canEdit && (
                    <>
                      {" "}
                      <button type="button" className="text-brand hover:underline" onClick={() => void allocateNext()}>
                        Allocate the first one
                      </button>{" "}
                      or{" "}
                      <button type="button" className="text-brand hover:underline" onClick={() => void scanNow()}>
                        scan the subnet
                      </button>
                      .
                    </>
                  )}
                </span>
              )
            }
            toolbar={
              ipFieldDefs.length > 0 ? (
                <DropdownMenu
                  label="Columns"
                  icon={<Columns3 />}
                  items={ipFieldDefs.map((d) => {
                    const on = visibleCFKeys.includes(d.key);
                    return {
                      label: `${on ? "✓ " : ""}${d.label || d.key}`,
                      onClick: () =>
                        setVisibleCFKeys((prev) => {
                          const next = on ? prev.filter((k) => k !== d.key) : [...prev, d.key];
                          saveCFPins(next);
                          return next;
                        }),
                    };
                  })}
                />
              ) : undefined
            }
            banner={
              canEdit && selectedIPIDs.length > 0 ? (
                <div className="flex flex-wrap items-center gap-2 rounded-xl border border-brand/25 bg-brand/[0.06] px-3 py-2 wt-animate-pop">
                  <span className="mr-1 text-sm text-zinc-200">
                    <span className="font-semibold text-brand tabular-nums">{selectedIPIDs.length}</span>{" "}
                    selected
                  </span>
                  <span className="h-5 w-px bg-[var(--color-border-strong)]" />
                  <select
                    className="wt-input h-8 text-xs"
                    aria-label="Bulk status"
                    value={bulkStatus}
                    onChange={(e) => setBulkStatus(e.target.value)}
                  >
                    <option value="assigned">assigned</option>
                    <option value="reserved">reserved</option>
                    <option value="dhcp">dhcp</option>
                    <option value="available">available</option>
                  </select>
                  <Button size="sm" onClick={() => void applyBulkIPs("status")}>
                    Set status
                  </Button>
                  <select
                    className="wt-input h-8 text-xs"
                    aria-label="Bulk device type"
                    value={bulkDeviceTypeID}
                    onChange={(e) => setBulkDeviceTypeID(e.target.value)}
                  >
                    <option value="">No device type</option>
                    {deviceTypes.map((d) => (
                      <option key={d.id} value={d.id}>
                        {d.name}
                      </option>
                    ))}
                  </select>
                  <Button size="sm" onClick={() => void applyBulkIPs("device")}>
                    Set type
                  </Button>
                  {ipFieldDefs.length > 0 && (
                    <>
                      <select
                        className="wt-input h-8 text-xs"
                        aria-label="Bulk custom field"
                        value={bulkFieldKey}
                        onChange={(e) => setBulkFieldKey(e.target.value)}
                      >
                        <option value="">Field…</option>
                        {ipFieldDefs.map((d) => (
                          <option key={d.key} value={d.key}>
                            {d.label || d.key}
                          </option>
                        ))}
                      </select>
                      <input
                        className="wt-input h-8 w-32 text-xs"
                        aria-label="Bulk custom field value"
                        placeholder="Value"
                        value={bulkFieldValue}
                        onChange={(e) => setBulkFieldValue(e.target.value)}
                      />
                      <Button size="sm" onClick={() => void applyBulkIPs("field")}>
                        Set field
                      </Button>
                    </>
                  )}
                  <div className="ml-auto flex items-center gap-1">
                    <Button size="sm" variant="danger" icon={<Trash2 />} onClick={() => void applyBulkIPs("delete")}>
                      Delete
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setSelectedIPs({})}>
                      Clear
                    </Button>
                  </div>
                </div>
              ) : undefined
            }
            server={{
              page,
              pageSize,
              total,
              onPageChange: setPage,
              filter,
              onFilterChange: (q) => {
                setPage(1);
                setFilter(q);
                patchIPParams({ q: q || null });
              },
              status: statusFilter,
              onStatusChange: (st) => {
                setPage(1);
                patchIPParams({ status: st || null });
              },
              statusOptions: [
                { value: "assigned", label: "assigned" },
                { value: "reserved", label: "reserved" },
                { value: "dhcp", label: "dhcp" },
                { value: "available", label: "available" },
              ],
              sorting,
              onSortingChange: (next) => {
                setSorting(next.length ? next : [{ id: "address", desc: false }]);
                setPage(1);
              },
            }}
          />
        </div>
      )}

      {tab === "map" && <SubnetMapPage embedded />}

      {tab === "ranges" && (
        <IPRangesPanel
          canEdit={canEdit}
          ranges={ranges}
          rangeName={rangeName}
          setRangeName={setRangeName}
          rangeStart={rangeStart}
          setRangeStart={setRangeStart}
          rangeEnd={rangeEnd}
          setRangeEnd={setRangeEnd}
          rangeRole={rangeRole}
          setRangeRole={setRangeRole}
          rangeDesc={rangeDesc}
          setRangeDesc={setRangeDesc}
          editingRange={editingRange}
          setEditingRange={setEditingRange}
          onSubmit={saveRange}
          onDelete={(rng) => void deleteRange(rng)}
        />
      )}

      {tab === "children" && (
        <div className="space-y-6">
          {children.length > 0 ? (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {children.map((c) => (
                <Link
                  key={c.id}
                  to={`/subnets/${c.id}`}
                  className="wt-card wt-card-interactive group flex items-center justify-between gap-3 p-4"
                >
                  <div className="min-w-0">
                    <div className="font-mono text-[15px] text-zinc-50">{c.cidr}</div>
                    <div className="truncate text-xs text-zinc-500">{c.name || "Unnamed"}</div>
                  </div>
                  <div className="flex items-center gap-2">
                    {c.vlan_vid != null && <Badge tone="info">VLAN {c.vlan_vid}</Badge>}
                    <ChevronRight className="h-4 w-4 text-zinc-600 transition group-hover:translate-x-0.5 group-hover:text-brand" />
                  </div>
                </Link>
              ))}
            </div>
          ) : (
            <div className="wt-card px-6 py-10 text-center">
              <GitBranch className="mx-auto h-6 w-6 text-zinc-600" />
              <p className="mt-2 text-sm text-zinc-300">No child subnets</p>
              <p className="mt-1 text-xs text-zinc-500">
                {canEdit ? "Carve one out below, or split this prefix in half." : "This prefix has not been subdivided."}
              </p>
            </div>
          )}
          {canEdit && subnet && id && (
            <PrefixOpsPanel
              subnetId={id}
              subnet={subnet}
              allSubnets={allSubnets}
              childPrefixLen={childPrefixLen}
              setChildPrefixLen={setChildPrefixLen}
              childName={childName}
              setChildName={setChildName}
              resizeCIDR={resizeCIDR}
              setResizeCIDR={setResizeCIDR}
              mergeSiblingID={mergeSiblingID}
              setMergeSiblingID={setMergeSiblingID}
              onReload={load}
            />
          )}
        </div>
      )}

      {tab === "discovery" && canEdit && (
        <div className="space-y-4">
          <div className="wt-card flex flex-wrap items-center justify-between gap-4 p-5">
            <div>
              <h2 className="text-sm font-semibold text-zinc-100">Live scan</h2>
              <p className="mt-0.5 text-xs text-zinc-500">
                Ping-sweep {subnet?.cidr} and enrich responders, then claim new hosts into inventory.
                Scheduled scans land in the{" "}
                <Link to="/inbox" className="text-brand hover:underline">
                  discovery inbox
                </Link>
                .
              </p>
              {sweep && (
                <div className="mt-3 flex flex-wrap gap-1.5">
                  <Badge>{sweep.alive}/{sweep.total} alive</Badge>
                  <Badge tone="brand">{sweep.reconcile?.new_alive?.length ?? 0} new</Badge>
                  <Badge>{sweep.reconcile?.matched?.length ?? 0} matched</Badge>
                  <Badge tone="warn">{sweep.reconcile?.stale?.length ?? 0} stale</Badge>
                </div>
              )}
            </div>
            <div className="flex gap-2">
              {sweep && (
                <Button onClick={() => void claimSelected()} disabled={claimCount === 0}>
                  Claim {claimCount || ""} selected
                </Button>
              )}
              <Button variant="primary" icon={<Radar />} onClick={() => void scanNow()} disabled={scanning}>
                {scanning ? "Scanning…" : sweep ? "Scan again" : "Scan now"}
              </Button>
            </div>
          </div>
          {scanning && !sweep && (
            <div className="wt-card flex items-center justify-center gap-3 py-12 text-sm text-zinc-400">
              <span className="h-4 w-4 animate-spin rounded-full border-2 border-zinc-600 border-t-brand" />
              Sweeping and enriching hosts…
            </div>
          )}
          {discoveryRows.length > 0 && (
            <div className="wt-table-wrap max-h-[32rem] overflow-auto">
              <table className="min-w-full text-left text-sm">
                <thead className="sticky top-0 z-10">
                  <tr className="border-b border-[var(--color-border)]">
                    <th className="h-10 px-3">Claim</th>
                    <th className="px-3">State</th>
                    <th className="px-3">IP</th>
                    <th className="px-3">Hostname</th>
                    <th className="px-3">MAC / vendor</th>
                    <th className="px-3">Ports / hints</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-border)]">
                  {discoveryRows.map((r) => (
                    <tr key={`${r.kind}-${r.ip}`}>
                      <td className="h-11 px-3">
                        {r.kind !== "stale" ? (
                          <input
                            type="checkbox"
                            aria-label={`Claim ${r.ip}`}
                            checked={!!selected[r.ip]}
                            onChange={(e) => setSelected((prev) => ({ ...prev, [r.ip]: e.target.checked }))}
                          />
                        ) : (
                          <span className="text-xs text-zinc-600">—</span>
                        )}
                      </td>
                      <td className="px-3">
                        <Badge tone={r.kind === "new" ? "brand" : r.kind === "stale" ? "warn" : "neutral"}>
                          {r.kind}
                          {r.db_status ? ` · ${r.db_status}` : ""}
                        </Badge>
                      </td>
                      <td className="px-3 font-mono text-zinc-100">{r.ip}</td>
                      <td className="px-3 text-zinc-300">{r.hostname || "—"}</td>
                      <td className="px-3 font-mono text-xs text-zinc-300">
                        {r.mac || "—"}
                        {r.vendor ? <div className="font-sans text-zinc-500">{r.vendor}</div> : null}
                      </td>
                      <td className="px-3 font-mono text-xs text-zinc-400">
                        {(r.open_ports ?? []).join(",") || "—"}
                        {r.device_hint ? ` · ${r.device_hint}` : ""}
                        {r.hints?.length ? ` · ${r.hints.join(",")}` : ""}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {sweep && discoveryRows.length === 0 && (
            <div className="wt-card py-10 text-center text-sm text-zinc-500">
              Scan finished — nothing new, matched, or stale.
            </div>
          )}
        </div>
      )}

      {/* ---- Dialogs & sheets ---- */}
      {canEdit && subnet && (
        <FormDialog open={subnetEditOpen} title={`Edit ${subnet.cidr}`} onClose={closeSubnetEdit} wide>
          <EditSubnetForm
            subnet={subnet}
            editName={editName}
            setEditName={setEditName}
            editDesc={editDesc}
            setEditDesc={setEditDesc}
            editVrfID={editVrfID}
            setEditVrfID={setEditVrfID}
            editVlan={editVlan}
            setEditVlan={setEditVlan}
            editLocationID={editLocationID}
            setEditLocationID={setEditLocationID}
            editTags={editTags}
            setEditTags={setEditTags}
            editParentID={editParentID}
            setEditParentID={setEditParentID}
            editFields={editFields}
            setEditFields={setEditFields}
            fieldDefs={fieldDefs}
            locations={locations}
            vrfs={vrfs}
            allSubnets={allSubnets}
            onSubmit={saveSubnet}
            onDelete={() => void deleteSubnet()}
            onCancel={closeSubnetEdit}
          />
        </FormDialog>
      )}

      {canEdit && (
        <FormDialog open={alertOpen} title="Utilization alert" onClose={() => setAlertOpen(false)}>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              void saveAlertThreshold();
            }}
          >
            <p className="text-sm text-zinc-400">
              Notify when this subnet reaches the threshold. Leave blank to follow the global
              setting ({globalUtilAlert}%); a global value of 0 disables alerts.
            </p>
            <Field label="Alert at (%)">
              <input
                type="number"
                min={1}
                max={100}
                autoFocus
                className="wt-input w-32"
                value={utilAlertPercent}
                onChange={(e) => setUtilAlertPercent(e.target.value)}
                placeholder={String(globalUtilAlert)}
              />
            </Field>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setAlertOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" variant="primary">
                Save threshold
              </Button>
            </div>
          </form>
        </FormDialog>
      )}

      {canEdit && (
        <FormDialog
          open={ipFormOpen}
          title={editingIP ? `Edit ${editingIP.address}` : "Add IP address"}
          onClose={closeIPForm}
          wide
        >
          <form
            onSubmit={(e) => {
              void createIP(e);
            }}
            className="space-y-5"
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Address">
                <input
                  className="wt-input w-full font-mono"
                  value={address}
                  onChange={(e) => setAddress(e.target.value)}
                  required
                  readOnly={!!editingIP}
                  autoFocus={!editingIP}
                  placeholder={util?.next_ip || ""}
                />
              </Field>
              <Field label="Hostname">
                <input
                  className="wt-input w-full"
                  value={hostname}
                  onChange={(e) => setHostname(e.target.value)}
                  autoFocus={!!editingIP}
                />
              </Field>
              <Field label="Status">
                <select className="wt-input w-full" value={status} onChange={(e) => setStatus(e.target.value)}>
                  <option value="assigned">assigned</option>
                  <option value="reserved">reserved</option>
                  <option value="dhcp">dhcp</option>
                  <option value="available">available</option>
                </select>
              </Field>
              <Field label="Device type">
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
              </Field>
              <Field label="MAC address">
                <input
                  className="wt-input w-full font-mono"
                  value={ipMac}
                  onChange={(e) => setIpMac(e.target.value)}
                  placeholder="aa:bb:cc:dd:ee:ff"
                />
              </Field>
              <Field label="Open ports">
                <input
                  className="wt-input w-full font-mono"
                  value={ipPorts}
                  onChange={(e) => setIpPorts(e.target.value)}
                  placeholder="22,80,443"
                />
              </Field>
              {status === "reserved" && (
                <Field label="Reserved until" hint="Defaults to 30 days when left blank">
                  <input
                    type="datetime-local"
                    className="wt-input w-full"
                    value={ipReservedUntil}
                    onChange={(e) => setIpReservedUntil(e.target.value)}
                  />
                </Field>
              )}
              <Field label="Notes" className="sm:col-span-2">
                <input className="wt-input w-full" value={ipNotes} onChange={(e) => setIpNotes(e.target.value)} />
              </Field>
            </div>
            {ipFieldDefs.length > 0 && (
              <CustomFieldInputs defs={ipFieldDefs} values={ipFields} onChange={setIPFields} />
            )}
            <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
              <Button variant="ghost" onClick={closeIPForm}>
                Cancel
              </Button>
              <Button type="submit" variant="primary">
                {editingIP ? "Save changes" : "Add address"}
              </Button>
            </div>
          </form>
        </FormDialog>
      )}

      <IPHistoryPanel
        historyAddr={historyAddr}
        history={history}
        onClose={() => {
          setHistoryAddr("");
          setHistory([]);
        }}
      />
    </div>
  );
}

function parseVlanID(raw: string): number | null {
  const t = raw.trim();
  if (!t) return null;
  const n = Number(t);
  if (!Number.isInteger(n) || n < 1 || n > 4094) return null;
  return n;
}
