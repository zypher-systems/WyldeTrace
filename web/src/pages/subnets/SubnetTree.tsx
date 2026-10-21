import { ChevronRight, CornerDownRight, FolderInput } from "lucide-react";
import { useState, type MouseEvent as ReactMouseEvent } from "react";
import { Link } from "react-router-dom";
import type { Subnet } from "../../api";
import { Badge, Meter, cx } from "../../components/ui";

export type TreeNode = Subnet & { children: TreeNode[] };

export function flattenTree(nodes: TreeNode[]): Subnet[] {
  const out: Subnet[] = [];
  for (const n of nodes) {
    out.push(n);
    if (n.children?.length) out.push(...flattenTree(n.children));
  }
  return out;
}

type Props = {
  nodes: TreeNode[];
  flat: Subnet[];
  canEdit: boolean;
  onReparent: (id: string, parentId: string) => Promise<void>;
  onContextMenu: (e: ReactMouseEvent, s: Subnet) => void;
};

/** Collapsible prefix hierarchy with utilization and inline reparenting. */
export function TreeList(props: Props) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [moving, setMoving] = useState<string | null>(null);
  return (
    <div role="tree" aria-label="Subnet hierarchy">
      <div className="grid grid-cols-[minmax(0,1fr)_10rem_2.5rem] gap-3 border-b border-[var(--color-border)] px-3 pb-2 font-sans">
        <span className="wt-eyebrow">Prefix</span>
        <span className="wt-eyebrow">Utilization</span>
        <span />
      </div>
      <Branch
        {...props}
        depth={0}
        collapsed={collapsed}
        toggle={(id) => setCollapsed((c) => ({ ...c, [id]: !c[id] }))}
        moving={moving}
        setMoving={setMoving}
      />
    </div>
  );
}

function Branch({
  nodes,
  flat,
  canEdit,
  onReparent,
  onContextMenu,
  depth,
  collapsed,
  toggle,
  moving,
  setMoving,
}: Props & {
  depth: number;
  collapsed: Record<string, boolean>;
  toggle: (id: string) => void;
  moving: string | null;
  setMoving: (id: string | null) => void;
}) {
  return (
    <ul role="group">
      {nodes.map((n) => {
        const sparse = Boolean(n.sparse || (n.used != null && n.total == null));
        const pct = !sparse && n.total != null ? Math.min(100, Math.round(n.percent ?? 0)) : null;
        const hasKids = n.children?.length > 0;
        const open = !collapsed[n.id];
        return (
          <li key={n.id} role="treeitem" aria-expanded={hasKids ? open : undefined}>
            <div
              className="group grid grid-cols-[minmax(0,1fr)_10rem_2.5rem] items-center gap-3 rounded-lg px-3 py-1.5 transition hover:bg-white/[0.025]"
              onContextMenu={(e) => onContextMenu(e, n)}
            >
              <div className="flex min-w-0 items-center gap-1.5" style={{ paddingLeft: depth * 22 }}>
                {hasKids ? (
                  <button
                    type="button"
                    className="grid h-5 w-5 shrink-0 place-items-center rounded text-zinc-500 hover:bg-[var(--color-surface-overlay)] hover:text-zinc-200"
                    aria-label={open ? `Collapse ${n.cidr}` : `Expand ${n.cidr}`}
                    onClick={() => toggle(n.id)}
                  >
                    <ChevronRight className={cx("h-3.5 w-3.5 transition", open && "rotate-90")} />
                  </button>
                ) : (
                  <span className="grid h-5 w-5 shrink-0 place-items-center text-zinc-700">
                    {depth > 0 && <CornerDownRight className="h-3 w-3" />}
                  </span>
                )}
                <Link
                  className="shrink-0 font-mono text-[13px] text-zinc-50 hover:text-brand"
                  to={`/subnets/${n.id}`}
                >
                  {n.cidr}
                </Link>
                <span className="truncate font-sans text-[13px] text-zinc-500">{n.name}</span>
                {(n.tags ?? []).includes("lab") && (
                  <Badge className="h-5 font-sans text-[10px] uppercase tracking-wide">Sample</Badge>
                )}
                {n.vlan_vid != null && (
                  <Badge tone="info" className="h-5 font-sans">
                    VLAN {n.vlan_vid}
                  </Badge>
                )}
                {moving === n.id && canEdit && (
                  <select
                    autoFocus
                    className="wt-input ml-2 h-7 font-sans text-xs"
                    aria-label={`New parent for ${n.cidr}`}
                    value={n.parent_id || ""}
                    onBlur={() => setMoving(null)}
                    onChange={(e) => {
                      setMoving(null);
                      void onReparent(n.id, e.target.value);
                    }}
                  >
                    <option value="">Top level</option>
                    {flat
                      .filter((s) => s.id !== n.id)
                      .map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.cidr}
                        </option>
                      ))}
                  </select>
                )}
              </div>
              <div className="flex items-center gap-2 font-sans">
                {pct != null ? (
                  <>
                    <Meter value={pct} className="flex-1" />
                    <span className="w-9 text-right text-[11px] tabular-nums text-zinc-400">{pct}%</span>
                  </>
                ) : (
                  <span className="text-[11px] text-zinc-500">
                    {sparse && n.used != null ? `${n.used} used` : "—"}
                  </span>
                )}
              </div>
              <div className="flex justify-end">
                {canEdit ? (
                  <button
                    type="button"
                    className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm opacity-0 group-hover:opacity-100 focus:opacity-100"
                    title="Move under another prefix"
                    aria-label={`Move ${n.cidr}`}
                    onClick={() => setMoving(moving === n.id ? null : n.id)}
                  >
                    <FolderInput />
                  </button>
                ) : null}
              </div>
            </div>
            {hasKids && open && (
              <Branch
                nodes={n.children}
                flat={flat}
                canEdit={canEdit}
                onReparent={onReparent}
                onContextMenu={onContextMenu}
                depth={depth + 1}
                collapsed={collapsed}
                toggle={toggle}
                moving={moving}
                setMoving={setMoving}
              />
            )}
          </li>
        );
      })}
    </ul>
  );
}
