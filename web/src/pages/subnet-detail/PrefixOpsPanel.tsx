import { GitBranchPlus, Maximize2, Merge, Plus, Split } from "lucide-react";
import type { ReactNode } from "react";
import { api, type Subnet } from "../../api";
import { useConfirm } from "../../components/ConfirmDialog";
import { useToast } from "../../components/Toast";
import { Button, Field, Section } from "../../components/ui";

export function PrefixOpsPanel({
  subnetId,
  subnet,
  allSubnets,
  childPrefixLen,
  setChildPrefixLen,
  childName,
  setChildName,
  resizeCIDR,
  setResizeCIDR,
  mergeSiblingID,
  setMergeSiblingID,
  onReload,
}: {
  subnetId: string;
  subnet: Subnet;
  allSubnets: Subnet[];
  childPrefixLen: string;
  setChildPrefixLen: (v: string) => void;
  childName: string;
  setChildName: (v: string) => void;
  resizeCIDR: string;
  setResizeCIDR: (v: string) => void;
  mergeSiblingID: string;
  setMergeSiblingID: (v: string) => void;
  onReload: () => Promise<void>;
}) {
  const toast = useToast();
  const confirm = useConfirm();
  const id = subnetId;
  const load = onReload;
  return (
    <Section
      title="Prefix operations"
      description="Carve out children, split or merge halves, or resize this prefix in place."
      bodyClassName="p-0"
    >
      <div className="grid divide-y divide-[var(--color-border)] lg:grid-cols-3 lg:divide-x lg:divide-y-0">
        <OpBlock
          icon={<GitBranchPlus />}
          title="Allocate child"
          description="Create the first free child prefix of the given length."
        >
          <div className="grid grid-cols-[6rem_1fr] gap-3">
            <Field label="Prefix length">
              <input
                type="number"
                className="wt-input w-full font-mono"
                value={childPrefixLen}
                onChange={(e) => setChildPrefixLen(e.target.value)}
                placeholder="25"
              />
            </Field>
            <Field label="Name">
              <input
                className="wt-input w-full"
                value={childName}
                onChange={(e) => setChildName(e.target.value)}
                placeholder="optional"
              />
            </Field>
          </div>
          <Button
            icon={<Plus />}
            onClick={() =>
              void (async () => {
                if (!id) return;
                try {
                  const created = await api<Subnet>(`/api/v1/subnets/${id}/allocate-child`, {
                    method: "POST",
                    body: JSON.stringify({
                      prefix_length: Number(childPrefixLen),
                      name: childName || undefined,
                    }),
                  });
                  toast(`Allocated ${created.cidr}`, "success");
                  void load();
                } catch (e) {
                  toast(e instanceof Error ? e.message : "Allocate child failed", "error");
                }
              })()
            }
          >
            Allocate child
          </Button>
        </OpBlock>

        <OpBlock
          icon={<Split />}
          title="Split / merge"
          description="Splitting moves IPs into the halves and soft-deletes this subnet. Merging pulls a sibling's IPs in."
        >
          <Button
            icon={<Split />}
            onClick={() =>
              void (async () => {
                if (!id) return;
                const ok = await confirm({
                  title: `Split ${subnet.cidr} into two halves?`,
                  body: "IPs move into the containing half; this subnet is soft-deleted.",
                  confirmLabel: "Split",
                });
                if (!ok) return;
                try {
                  const res = await api<{ left: Subnet; right: Subnet }>(
                    `/api/v1/subnets/${id}/split`,
                    { method: "POST", body: "{}" },
                  );
                  toast(`Split → ${res.left.cidr} + ${res.right.cidr}`, "success");
                  window.location.assign(`/subnets/${res.left.id}`);
                } catch (e) {
                  toast(e instanceof Error ? e.message : "Split failed", "error");
                }
              })()
            }
          >
            Split in half
          </Button>
          <div className="flex items-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Field label="Merge sibling" className="min-w-0 flex-1">
              <select
                className="wt-input w-full font-mono text-[13px]"
                value={mergeSiblingID}
                onChange={(e) => setMergeSiblingID(e.target.value)}
              >
                <option value="">Select sibling…</option>
                {allSubnets
                  .filter((s) => s.id !== subnet.id && s.vrf_id === subnet.vrf_id)
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.cidr} — {s.name}
                    </option>
                  ))}
              </select>
            </Field>
            <Button
              icon={<Merge />}
              disabled={!mergeSiblingID}
              onClick={() =>
                void (async () => {
                  if (!id || !mergeSiblingID) return;
                  const ok = await confirm({
                    title: "Merge with sibling?",
                    body: "Primary keeps covering CIDR; sibling IPs move here; sibling is soft-deleted.",
                    confirmLabel: "Merge",
                  });
                  if (!ok) return;
                  try {
                    const merged = await api<Subnet>(`/api/v1/subnets/${id}/merge`, {
                      method: "POST",
                      body: JSON.stringify({ sibling_id: mergeSiblingID }),
                    });
                    toast(`Merged to ${merged.cidr}`, "success");
                    setMergeSiblingID("");
                    void load();
                  } catch (e) {
                    toast(e instanceof Error ? e.message : "Merge failed", "error");
                  }
                })()
              }
            >
              Merge
            </Button>
          </div>
        </OpBlock>

        <OpBlock
          icon={<Maximize2 />}
          title="Resize"
          description="Change the prefix when every existing IP stays contained."
        >
          <Field label="New CIDR">
            <input
              className="wt-input w-full font-mono"
              value={resizeCIDR}
              onChange={(e) => setResizeCIDR(e.target.value)}
              placeholder={subnet.cidr}
            />
          </Field>
          <Button
            icon={<Maximize2 />}
            onClick={() =>
              void (async () => {
                if (!id) return;
                try {
                  const resized = await api<Subnet>(`/api/v1/subnets/${id}/resize`, {
                    method: "POST",
                    body: JSON.stringify({ cidr: resizeCIDR }),
                  });
                  toast(`Resized to ${resized.cidr}`, "success");
                  setResizeCIDR("");
                  void load();
                } catch (e) {
                  toast(e instanceof Error ? e.message : "Resize failed", "error");
                }
              })()
            }
          >
            Resize
          </Button>
        </OpBlock>
      </div>
    </Section>
  );
}

function OpBlock({
  icon,
  title,
  description,
  children,
}: {
  icon: ReactNode;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4 p-5">
      <div>
        <h3 className="flex items-center gap-2 text-sm font-medium text-zinc-100 [&_svg]:h-4 [&_svg]:w-4 [&_svg]:text-zinc-500">
          {icon}
          {title}
        </h3>
        <p className="mt-1 text-xs text-zinc-500">{description}</p>
      </div>
      <div className="flex flex-col items-start gap-3 [&>*:not(button)]:w-full">{children}</div>
    </div>
  );
}
