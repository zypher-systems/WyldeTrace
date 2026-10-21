import { type ColumnDef } from "@tanstack/react-table";
import {
  ArrowDownUp,
  Download,
  GitBranch,
  List,
  MoreHorizontal,
  Plus,
  ScanSearch,
  Tag,
  Upload,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState, type FormEvent, type MouseEvent as ReactMouseEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  api,
  DEFAULT_VRF_ID,
  downloadCSV,
  pageQuery,
  uploadCSV,
  type ImportSummary,
  type Location,
  type PageResult,
  type Subnet,
  type VRF,
} from "../api";
import {
  buildCustomFields,
  type FieldDef,
} from "../components/CustomFieldInputs";
import { useConfirm } from "../components/ConfirmDialog";
import { useContextMenu, type ContextMenuItem } from "../components/ContextMenu";
import { DataTable } from "../components/DataTable";
import { DropdownMenu, type DropdownItem } from "../components/DropdownMenu";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { Badge, Button, Meter, Segmented, cx } from "../components/ui";
import { EmptyState } from "../components/EmptyState";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { useAuth } from "../auth";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";
import { CreateSubnetForm } from "./subnets/CreateSubnetForm";
import { SubnetBulkBar } from "./subnets/SubnetBulkBar";
import { flattenTree, TreeList, type TreeNode } from "./subnets/SubnetTree";

export function SubnetsPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const toast = useToast();
  const { open: openMenu, menu } = useContextMenu();
  const [searchParams, setSearchParams] = useSearchParams();
  const page = Math.max(1, Number(searchParams.get("page") || "1") || 1);
  const filter = searchParams.get("q") || "";
  const sort = searchParams.get("sort") || "cidr";
  const order = (searchParams.get("order") === "desc" ? "desc" : "asc") as "asc" | "desc";
  const tagFilters = parseTagsParam(searchParams);
  const tagMatch = (searchParams.get("tag_match") === "or" ? "or" : "and") as "and" | "or";
  const [subnets, setSubnets] = useState<Subnet[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [availableTags, setAvailableTags] = useState<string[]>([]);
  const [tree, setTree] = useState<TreeNode[]>([]);
  const [view, setView] = useState<"table" | "tree">("table");
  const [createOpen, setCreateOpen] = useState(false);
  const [cidr, setCidr] = useState("");
  const [name, setName] = useState("");
  const [vlan, setVlan] = useState("");
  const [vrfId, setVrfId] = useState(DEFAULT_VRF_ID);
  const [locationId, setLocationId] = useState("");
  const [tags, setTags] = useState("");
  const [parentId, setParentId] = useState("");
  const [fieldDefs, setFieldDefs] = useState<FieldDef[]>([]);
  const [createFields, setCreateFields] = useState<Record<string, unknown>>({});
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [locations, setLocations] = useState<Location[]>([]);
  const [vrfs, setVrfs] = useState<VRF[]>([]);
  const [bulkTags, setBulkTags] = useState("");
  const [bulkTagMode, setBulkTagMode] = useState<"add" | "remove" | "set">("add");
  const [bulkLocationId, setBulkLocationId] = useState("");
  const [bulkFieldKey, setBulkFieldKey] = useState("");
  const [bulkFieldValue, setBulkFieldValue] = useState("");
  const subnetFileRef = useRef<HTMLInputElement>(null);
  const ipFileRef = useRef<HTMLInputElement>(null);
  const dhcpFileRef = useRef<HTMLInputElement>(null);
  const dnsFileRef = useRef<HTMLInputElement>(null);
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const pageSize = 25;
  const selectedIDs = Object.keys(selected).filter((id) => selected[id]);
  useUnsavedChanges(!!(cidr || name || vlan || tags || parentId || locationId));

  function patchParams(next: Record<string, string | null>, resetPage = false) {
    const sp = new URLSearchParams(searchParams);
    for (const [k, v] of Object.entries(next)) {
      if (v == null || v === "") sp.delete(k);
      else sp.set(k, v);
    }
    if (resetPage) {
      sp.delete("page");
    }
    if (sp.get("page") === "1") sp.delete("page");
    if (sp.get("sort") === "cidr") sp.delete("sort");
    if (sp.get("order") === "asc") sp.delete("order");
    if (sp.get("tag_match") === "and") sp.delete("tag_match");
    setSearchParams(sp, { replace: true });
  }

  async function loadTree() {
    try {
      setTree(await api<TreeNode[]>("/api/v1/subnets/tree"));
    } catch {
      /* tree is optional for table view */
    }
  }

  async function load(
    nextPage = page,
    q = filter,
    tags = tagFilters,
    match = tagMatch,
    sortBy = sort,
    sortOrder = order,
  ) {
    setLoading(true);
    try {
      const list = await api<PageResult<Subnet>>(
        `/api/v1/subnets${pageQuery({
          page: nextPage,
          pageSize,
          q,
          tags: tags.length ? tags.join(",") : undefined,
          tagMatch: tags.length ? match : undefined,
          sort: sortBy,
          order: sortOrder,
        })}`,
      );
      setSubnets(list.items);
      setTotal(list.total);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load subnets", "error");
    } finally {
      setLoading(false);
    }
  }

  async function reloadAll() {
    await Promise.all([load(), loadTree()]);
  }

  function toggleTag(tag: string) {
    const t = tag.toLowerCase();
    const next = tagFilters.includes(t) ? tagFilters.filter((x) => x !== t) : [...tagFilters, t];
    patchParams(
      {
        tags: next.length ? next.join(",") : null,
        tag_match: next.length && tagMatch === "or" ? "or" : null,
      },
      true,
    );
  }

  function clearTags() {
    patchParams({ tags: null, tag_match: null }, true);
  }

  useEffect(() => {
    void load(page, filter, tagFilters, tagMatch, sort, order);
  }, [page, filter, tagFilters.join(","), tagMatch, sort, order]);

  useEffect(() => {
    void loadTree();
  }, []);

  useEffect(() => {
    void api<FieldDef[]>("/api/v1/custom-fields?entity_type=subnet")
      .then(setFieldDefs)
      .catch(() => setFieldDefs([]));
    void api<string[]>("/api/v1/subnets/tags")
      .then(setAvailableTags)
      .catch(() => setAvailableTags([]));
    void api<VRF[]>("/api/v1/vrfs")
      .then(setVrfs)
      .catch(() => setVrfs([]));
    if (canEdit) {
      void api<Location[]>("/api/v1/locations")
        .then(setLocations)
        .catch(() => setLocations([]));
    }
  }, [canEdit]);

  function toggleSelect(id: string) {
    setSelected((prev) => ({ ...prev, [id]: !prev[id] }));
  }

  function toggleSelectAllOnPage() {
    const allSelected = subnets.length > 0 && subnets.every((s) => selected[s.id]);
    setSelected((prev) => {
      const next = { ...prev };
      for (const s of subnets) next[s.id] = !allSelected;
      return next;
    });
  }

  async function applyBulkTags() {
    const values = bulkTags
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean);
    if (!selectedIDs.length || !values.length) return;
    try {
      const body: Record<string, unknown> = { ids: selectedIDs };
      if (bulkTagMode === "add") body.add_tags = values;
      else if (bulkTagMode === "remove") body.remove_tags = values;
      else body.set_tags = values;
      const res = await api<{ updated: number; skipped: number }>("/api/v1/subnets/bulk", {
        method: "POST",
        body: JSON.stringify(body),
      });
      toast(`Tags updated on ${res.updated} subnet(s)` + (res.skipped ? ` · ${res.skipped} skipped` : ""), "success");
      setSelected({});
      await reloadAll();
      const tags = await api<string[]>("/api/v1/subnets/tags");
      setAvailableTags(tags);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Bulk tag update failed", "error");
    }
  }

  async function applyBulkLocation() {
    if (!selectedIDs.length) return;
    try {
      const res = await api<{ updated: number; skipped: number }>("/api/v1/subnets/bulk", {
        method: "POST",
        body: JSON.stringify({
          ids: selectedIDs,
          update_location: true,
          location_id: bulkLocationId || null,
        }),
      });
      toast(
        `Location updated on ${res.updated} subnet(s)` + (res.skipped ? ` · ${res.skipped} skipped` : ""),
        "success",
      );
      setSelected({});
      await reloadAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Bulk location update failed", "error");
    }
  }

  async function applyBulkCustomField() {
    if (!selectedIDs.length || !bulkFieldKey) return;
    try {
      const res = await api<{ updated: number; skipped: number }>("/api/v1/subnets/bulk", {
        method: "POST",
        body: JSON.stringify({
          ids: selectedIDs,
          set_custom_fields: { [bulkFieldKey]: bulkFieldValue },
        }),
      });
      toast(
        `Custom field set on ${res.updated} subnet(s)` + (res.skipped ? ` · ${res.skipped} skipped` : ""),
        "success",
      );
      setSelected({});
      await reloadAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Bulk field update failed", "error");
    }
  }

  const columns = useMemo<ColumnDef<Subnet>[]>(
    () => [
      ...(canEdit
        ? ([
            {
              id: "select",
              enableSorting: false,
              header: () => (
                <input
                  type="checkbox"
                  checked={subnets.length > 0 && subnets.every((s) => selected[s.id])}
                  onChange={toggleSelectAllOnPage}
                  aria-label="Select all on page"
                />
              ),
              cell: ({ row }) => (
                <input
                  type="checkbox"
                  checked={!!selected[row.original.id]}
                  onChange={() => toggleSelect(row.original.id)}
                  aria-label={`Select ${row.original.cidr}`}
                />
              ),
            },
          ] as ColumnDef<Subnet>[])
        : []),
      {
        accessorKey: "cidr",
        header: "CIDR",
        cell: ({ row }) => (
          <div onContextMenu={(e) => openSubnetMenu(e, row.original)}>
            <Link className="font-mono text-brand hover:underline" to={`/subnets/${row.original.id}`}>
              {row.original.cidr}
            </Link>
          </div>
        ),
      },
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ row }) => (
          <span className="inline-flex flex-wrap items-center gap-1.5">
            {row.original.name}
            {(row.original.tags ?? []).includes("lab") && (
              <Badge className="h-5 text-[10px] uppercase tracking-wide">Sample</Badge>
            )}
          </span>
        ),
      },
      {
        id: "tags",
        enableSorting: false,
        header: "Tags",
        cell: ({ row }) => {
          const rowTags = row.original.tags ?? [];
          if (!rowTags.length) return <span className="text-zinc-600">—</span>;
          return (
            <div className="flex flex-wrap gap-1">
              {rowTags.map((t) => (
                <button
                  key={t}
                  type="button"
                  className={cx(
                    "wt-badge h-5 cursor-pointer font-mono hover:border-brand/50 hover:text-brand",
                    tagFilters.includes(t) && "wt-badge-brand",
                  )}
                  onClick={() => toggleTag(t)}
                >
                  {t}
                </button>
              ))}
            </div>
          );
        },
      },
      {
        id: "util",
        header: "Utilization",
        accessorFn: (row) => row.percent ?? -1,
        cell: ({ row }) => {
          const s = row.original;
          if (s.used == null) return "—";
          if (s.sparse || s.total == null) {
            return (
              <div className="min-w-[140px] text-xs text-zinc-400">
                {s.used} used <span className="text-zinc-600">(sparse)</span>
              </div>
            );
          }
          const pct = Math.min(100, Math.round(s.percent ?? 0));
          return (
            <div className="flex min-w-[160px] items-center gap-3" title={`${s.used}/${s.total} used`}>
              <Meter value={pct} className="flex-1" />
              <span className="w-9 text-right text-xs tabular-nums text-zinc-300">{pct}%</span>
            </div>
          );
        },
      },
      {
        accessorKey: "vlan_vid",
        header: "VLAN",
        cell: ({ getValue }) =>
          getValue() != null ? (
            <span className="font-mono text-xs text-zinc-300">{String(getValue())}</span>
          ) : (
            <span className="text-zinc-600">—</span>
          ),
      },
      {
        id: "vrf",
        header: "VRF",
        accessorFn: (row) => row.vrf_id ?? "",
        cell: ({ row }) => {
          const v = vrfs.find((x) => x.id === row.original.vrf_id);
          return v?.name ?? "Default";
        },
      },
      {
        id: "location",
        header: "Site",
        accessorFn: (row) => row.location_id ?? "",
        cell: ({ row }) => {
          const lid = row.original.location_id;
          if (!lid) return "—";
          const loc = locations.find((l) => l.id === lid);
          return loc?.name ?? lid.slice(0, 8);
        },
      },
      {
        accessorKey: "parent_id",
        header: "Parent",
        cell: ({ row }) => {
          const pid = row.original.parent_id;
          if (!pid) return "—";
          const parent = subnets.find((s) => s.id === pid);
          return parent ? (
            <span className="font-mono text-xs text-zinc-400">{parent.cidr}</span>
          ) : (
            <span className="text-xs text-zinc-500">yes</span>
          );
        },
      },
      ...(canEdit
        ? ([
            {
              id: "actions",
              header: "",
              cell: ({ row }) => (
                <div className="flex justify-end">
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                    aria-label={`Actions for ${row.original.cidr}`}
                    title="Actions"
                    onClick={(e) => openSubnetMenu(e, row.original)}
                  >
                    <MoreHorizontal />
                  </button>
                </div>
              ),
            },
          ] as ColumnDef<Subnet>[])
        : []),
    ],
    [subnets, canEdit, tagFilters, selected, vrfs, locations],
  );

  function openSubnetMenu(e: ReactMouseEvent, s: Subnet) {
    const items: ContextMenuItem[] = [
      {
        id: "open",
        label: "Open",
        onSelect: () => navigate(`/subnets/${s.id}`),
      },
      {
        id: "map",
        label: "Address map",
        onSelect: () => navigate(`/subnets/${s.id}?tab=map`),
      },
      {
        id: "copy",
        label: "Copy CIDR",
        onSelect: () => void navigator.clipboard.writeText(s.cidr),
      },
    ];
    if (canEdit) {
      items.splice(2, 0, {
        id: "delete",
        label: "Soft-delete",
        danger: true,
        onSelect: () => void deleteSubnet(s),
      });
    }
    openMenu(e, items);
  }

  async function deleteSubnet(s: Subnet) {
    if (
      !(await confirm({
        title: `Soft-delete subnet ${s.cidr}?`,
        body: "You can restore it from Deleted subnets.",
        danger: true,
        confirmLabel: "Delete",
      }))
    )
      return;
    try {
      await api(`/api/v1/subnets/${s.id}`, { method: "DELETE" });
      await reloadAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  async function createSubnet(e: FormEvent) {
    e.preventDefault();
    const built = buildCustomFields(fieldDefs, createFields);
    if (built.error) {
      toast(built.error, "error");
      return;
    }
    const vlanVID = parseVlanID(vlan);
    if (vlan.trim() && vlanVID == null) {
      toast("VLAN must be an integer from 1 to 4094", "error");
      return;
    }
    try {
      await api("/api/v1/subnets", {
        method: "POST",
        body: JSON.stringify({
          cidr,
          name,
          description: "",
          vrf_id: vrfId || DEFAULT_VRF_ID,
          vlan_vid: vlanVID,
          location_id: locationId || null,
          parent_id: parentId || null,
          tags: tags
            .split(",")
            .map((t) => t.trim())
            .filter(Boolean),
          custom_fields: built.fields,
        }),
      });
      setCidr("");
      setName("");
      setVlan("");
      setVrfId(DEFAULT_VRF_ID);
      setLocationId("");
      setTags("");
      setParentId("");
      setCreateFields({});
      setCreateOpen(false);
      toast(`Created ${cidr}`, "success");
      await reloadAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Create failed", "error");
    }
  }

  async function onImport(kind: "subnets" | "ips" | "dhcp-leases", file: File | null) {
    if (!file) return;
    try {
      const summary = await uploadCSV<ImportSummary>(`/api/v1/import/${kind}.csv`, file);
      toast(
        `${kind} import: created ${summary.created}, updated ${summary.updated}, skipped ${summary.skipped}`,
        "success",
      );
      if (summary.errors?.length) {
        toast(summary.errors.slice(0, 3).join("; "), "error");
      }
      await reloadAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Import failed", "error");
    }
  }

  async function onImportDNS(file: File | null) {
    if (!file) return;
    try {
      const token = localStorage.getItem("wyldetrace_token");
      const res = await fetch("/api/v1/import/dns-hosts", {
        method: "POST",
        headers: {
          "Content-Type": "text/plain",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: await file.text(),
      });
      const summary = (await res.json().catch(() => ({}))) as ImportSummary & { error?: string };
      if (!res.ok) throw new Error(summary.error || res.statusText);
      toast(
        `dns import: created ${summary.created}, updated ${summary.updated}, skipped ${summary.skipped}`,
        "success",
      );
      if (summary.errors?.length) toast(summary.errors.slice(0, 3).join("; "), "error");
      await reloadAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "DNS import failed", "error");
    }
  }

  const dataItems: DropdownItem[] = [
    {
      label: "Export subnets CSV",
      icon: <Download />,
      onClick: () => void downloadCSV("/api/v1/export/subnets.csv", "subnets.csv"),
    },
    {
      label: "Export IPs CSV",
      icon: <Download />,
      onClick: () => void downloadCSV("/api/v1/export/ips.csv", "ips.csv"),
    },
    ...(canEdit
      ? [
          { label: "Import subnets CSV", icon: <Upload />, separated: true, onClick: () => subnetFileRef.current?.click() },
          { label: "Import IPs CSV", icon: <Upload />, onClick: () => ipFileRef.current?.click() },
          { label: "Import DHCP leases", icon: <Upload />, onClick: () => dhcpFileRef.current?.click() },
          { label: "Import DNS hosts", icon: <Upload />, onClick: () => dnsFileRef.current?.click() },
          {
            label: "Find subnets on the network…",
            icon: <ScanSearch />,
            separated: true,
            onClick: () => navigate("/discover-subnets"),
          },
        ]
      : []),
  ];

  return (
    <div className="space-y-6">
      {menu}
      <PageHeader
        title="Subnets"
        subtitle={`${total} prefix${total === 1 ? "" : "es"} · utilization and hierarchy`}
        actions={
          <>
            <Segmented
              label="View"
              value={view}
              onChange={setView}
              items={[
                { value: "table", label: "Table", icon: <List /> },
                { value: "tree", label: "Tree", icon: <GitBranch /> },
              ]}
            />
            <DropdownMenu label="Import & export" iconOnly icon={<ArrowDownUp />} items={dataItems} />
            {canEdit && (
              <Button variant="primary" icon={<Plus />} onClick={() => setCreateOpen(true)}>
                New subnet
              </Button>
            )}
          </>
        }
      />
      {canEdit && (
        <>
          <input
            ref={subnetFileRef}
            type="file"
            accept=".csv,text/csv"
            className="hidden"
            onChange={(e) => {
              void onImport("subnets", e.target.files?.[0] ?? null);
              e.target.value = "";
            }}
          />
          <input
            ref={ipFileRef}
            type="file"
            accept=".csv,text/csv"
            className="hidden"
            onChange={(e) => {
              void onImport("ips", e.target.files?.[0] ?? null);
              e.target.value = "";
            }}
          />
          <input
            ref={dhcpFileRef}
            type="file"
            accept=".csv,text/csv"
            className="hidden"
            onChange={(e) => {
              void onImport("dhcp-leases", e.target.files?.[0] ?? null);
              e.target.value = "";
            }}
          />
          <input
            ref={dnsFileRef}
            type="file"
            accept=".txt,.hosts,text/plain,.csv,text/csv"
            className="hidden"
            onChange={(e) => {
              void onImportDNS(e.target.files?.[0] ?? null);
              e.target.value = "";
            }}
          />
          <FormDialog open={createOpen} title="New subnet" onClose={() => setCreateOpen(false)} wide>
            <CreateSubnetForm
              cidr={cidr}
              setCidr={setCidr}
              name={name}
              setName={setName}
              vrfId={vrfId}
              setVrfId={setVrfId}
              vlan={vlan}
              setVlan={setVlan}
              locationId={locationId}
              setLocationId={setLocationId}
              tags={tags}
              setTags={setTags}
              parentId={parentId}
              setParentId={setParentId}
              locations={locations}
              vrfs={vrfs}
              tree={tree}
              fieldDefs={fieldDefs}
              createFields={createFields}
              setCreateFields={setCreateFields}
              onSubmit={createSubnet}
              onCancel={() => setCreateOpen(false)}
            />
          </FormDialog>
        </>
      )}

      {canEdit && selectedIDs.length > 0 && (
        <SubnetBulkBar
          selectedCount={selectedIDs.length}
          bulkTags={bulkTags}
          setBulkTags={setBulkTags}
          bulkTagMode={bulkTagMode}
          setBulkTagMode={setBulkTagMode}
          bulkLocationId={bulkLocationId}
          setBulkLocationId={setBulkLocationId}
          locations={locations}
          fieldDefs={fieldDefs}
          bulkFieldKey={bulkFieldKey}
          setBulkFieldKey={setBulkFieldKey}
          bulkFieldValue={bulkFieldValue}
          setBulkFieldValue={setBulkFieldValue}
          onApplyTags={() => void applyBulkTags()}
          onApplyLocation={() => void applyBulkLocation()}
          onApplyField={() => void applyBulkCustomField()}
          onClear={() => setSelected({})}
        />
      )}

      {availableTags.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 text-sm">
          <span className="mr-1 inline-flex items-center gap-1.5 text-xs text-zinc-500">
            <Tag className="h-3.5 w-3.5" />
            Tags:
          </span>
          <button
            type="button"
            aria-pressed={!tagFilters.length}
            className={cx("wt-badge cursor-pointer", !tagFilters.length && "wt-badge-brand")}
            onClick={clearTags}
          >
            All
          </button>
          {availableTags.map((t) => (
            <button
              key={t}
              type="button"
              aria-pressed={tagFilters.includes(t)}
              className={cx(
                "wt-badge cursor-pointer font-mono hover:border-zinc-500",
                tagFilters.includes(t) && "wt-badge-brand",
              )}
              onClick={() => toggleTag(t)}
            >
              {t}
            </button>
          ))}
          {tagFilters.length > 0 ? (
            <span className="ml-2 inline-flex items-center gap-1.5">
              <span className="text-xs text-zinc-500">match</span>
              <span className="wt-segment">
                <button
                  type="button"
                  aria-pressed={tagMatch === "and"}
                  onClick={() =>
                    patchParams(
                      { tags: tagFilters.length ? tagFilters.join(",") : null, tag_match: null },
                      true,
                    )
                  }
                >
                  AND
                </button>
                <button
                  type="button"
                  aria-pressed={tagMatch === "or"}
                  onClick={() =>
                    patchParams(
                      {
                        tags: tagFilters.length ? tagFilters.join(",") : null,
                        tag_match: tagFilters.length ? "or" : null,
                      },
                      true,
                    )
                  }
                >
                  OR
                </button>
              </span>
            </span>
          ) : null}
        </div>
      )}

      {!loading && total === 0 && !filter && tagFilters.length === 0 && (
        <EmptyState
          title="No subnets yet"
          detail="Create a /24 to start allocating IPs, or import a CSV when you are ready."
          actionLabel={canEdit ? "New subnet" : undefined}
          onAction={canEdit ? () => setCreateOpen(true) : undefined}
        />
      )}

      {loading && view === "table" ? (
        <TableSkeleton rows={8} cols={6} />
      ) : view === "table" ? (
        <DataTable
          data={subnets}
          columns={columns}
          filterPlaceholder="Filter subnets…"
          loading={loading}
          server={{
            page,
            pageSize,
            total,
            onPageChange: (p) => patchParams({ page: p <= 1 ? null : String(p) }),
            filter,
            onFilterChange: (q) => patchParams({ q: q || null }, true),
            sorting: sort ? [{ id: sort, desc: order === "desc" }] : [],
            onSortingChange: (next) => {
              const first = next[0];
              if (!first) {
                patchParams({ sort: null, order: null }, true);
              } else {
                patchParams(
                  {
                    sort: first.id === "cidr" ? null : first.id,
                    order: first.desc ? "desc" : null,
                  },
                  true,
                );
              }
            },
          }}
        />
      ) : (
        <div className="wt-card p-3 font-mono text-sm">
          {tree.length === 0 ? (
            <EmptyState title="No subnets" detail="Create your first prefix with New subnet." />
          ) : (
            <TreeList
              nodes={tree}
              flat={flattenTree(tree)}
              canEdit={canEdit}
              onContextMenu={openSubnetMenu}
              onReparent={async (id, parentId) => {
                const s = flattenTree(tree).find((x) => x.id === id);
                if (!s) return;
                try {
                  await api(`/api/v1/subnets/${id}`, {
                    method: "PUT",
                    body: JSON.stringify({
                      cidr: s.cidr,
                      name: s.name,
                      description: s.description,
                      location_id: s.location_id,
                      vrf_id: s.vrf_id || DEFAULT_VRF_ID,
                      vlan_vid: s.vlan_vid,
                      parent_id: parentId || null,
                      tags: s.tags ?? [],
                      custom_fields: s.custom_fields ?? {},
                    }),
                  });
                  toast("Parent updated", "success");
                  await reloadAll();
                } catch (err) {
                  toast(err instanceof Error ? err.message : "Reparent failed", "error");
                }
              }}
            />
          )}
        </div>
      )}
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

function parseTagsParam(sp: URLSearchParams): string[] {
  const raw = sp.get("tags") || sp.get("tag") || "";
  return [
    ...new Set(
      raw
        .split(",")
        .map((t) => t.trim().toLowerCase())
        .filter(Boolean),
    ),
  ];
}

