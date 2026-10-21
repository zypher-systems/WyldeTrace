import { useEffect, useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { KeyRound, MapPin, Pencil, Plus, Trash2, Users } from "lucide-react";
import { api, type Location, type Role, type User } from "../api";
import { useAuth } from "../auth";
import { useConfirm } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { useToast } from "../components/Toast";
import { Badge, Button, Field } from "../components/ui";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";

const ROLES: Role[] = ["readonly", "reportview", "editor", "admin"];

const ROLE_TONE: Record<Role, "accent" | "brand" | "info" | "neutral"> = {
  admin: "accent",
  editor: "brand",
  reportview: "info",
  readonly: "neutral",
};

const ROLE_HINT: Record<Role, string> = {
  readonly: "Read inventory in assigned sites",
  reportview: "Overview and IP requests only",
  editor: "Read and write in assigned sites",
  admin: "Global access, including settings",
};

function RoleBadge({ role }: { role: Role }) {
  return <Badge tone={ROLE_TONE[role] ?? "neutral"}>{role}</Badge>;
}

export function UsersPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const [users, setUsers] = useState<User[]>([]);
  const [locations, setLocations] = useState<Location[]>([]);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<Role>("readonly");
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<User | null>(null);
  const [editRole, setEditRole] = useState<Role>("readonly");
  const [sitesFor, setSitesFor] = useState<string | null>(null);
  const [resetFor, setResetFor] = useState<User | null>(null);
  const [resetPassword, setResetPassword] = useState("");
  const [resetError, setResetError] = useState("");

  const createDirty = !!(email || password);
  useUnsavedChanges(createDirty);

  async function load() {
    setLoading(true);
    try {
      const [u, locs] = await Promise.all([
        api<User[]>("/api/v1/users"),
        api<Location[]>("/api/v1/locations"),
      ]);
      setUsers(u);
      setLocations(locs);
    } catch (e) {
      toast(e instanceof Error ? e.message : "Failed to load users", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (user?.role === "admin") void load();
  }, [user?.role]);

  if (user?.role !== "admin") return <Navigate to="/" replace />;

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/api/v1/users", {
        method: "POST",
        body: JSON.stringify({ email, password, role }),
      });
      setEmail("");
      setPassword("");
      setRole("readonly");
      setCreateOpen(false);
      toast("User created", "success");
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Create failed", "error");
    }
  }

  async function onRoleChange(id: string, email: string, next: Role) {
    try {
      await api(`/api/v1/users/${id}`, {
        method: "PUT",
        body: JSON.stringify({ email, role: next }),
      });
      await load();
      return true;
    } catch (err) {
      toast(err instanceof Error ? err.message : "Update failed", "error");
      return false;
    }
  }

  async function onResetPassword(e: FormEvent) {
    e.preventDefault();
    if (!resetFor) return;
    const next = resetPassword;
    if (next.length < 8) {
      setResetError("Password must be at least 8 characters");
      return;
    }
    const { id, email, role } = resetFor;
    try {
      await api(`/api/v1/users/${id}`, {
        method: "PUT",
        body: JSON.stringify({ email, role, password: next }),
      });
      toast(`Password reset for ${email}`, "success");
      closeReset();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Password reset failed", "error");
    }
  }

  function closeReset() {
    setResetFor(null);
    setResetPassword("");
    setResetError("");
  }

  async function onDelete(id: string) {
    if (!(await confirm({ title: "Delete this user?", danger: true, confirmLabel: "Delete" })))
      return;
    try {
      await api(`/api/v1/users/${id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Delete failed", "error");
    }
  }

  async function toggleLocation(userId: string, current: string[], locId: string) {
    const next = current.includes(locId)
      ? current.filter((id) => id !== locId)
      : [...current, locId];
    try {
      await api(`/api/v1/users/${userId}/locations`, {
        method: "PUT",
        body: JSON.stringify({ location_ids: next }),
      });
      toast(
        next.length
          ? "Site scope updated"
          : "Site scope cleared — non-admins with no sites see nothing",
        "success",
      );
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Site update failed", "error");
    }
  }

  const locName = Object.fromEntries(locations.map((l) => [l.id, l.name]));
  const sitesUser = users.find((u) => u.id === sitesFor) ?? null;
  const sitesUserLocs = sitesUser?.location_ids ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Users"
        subtitle="Roles and site scope. Non-admins with no sites assigned see nothing; admins are global."
        actions={
          <Button variant="primary" icon={<Plus />} onClick={() => setCreateOpen(true)}>
            Add user
          </Button>
        }
      />

      {loading ? (
        <TableSkeleton rows={6} cols={4} />
      ) : users.length === 0 ? (
        <div className="wt-card flex flex-col items-center gap-2 px-6 py-12 text-center">
          <Users className="h-6 w-6 text-zinc-500" />
          <p className="text-sm font-medium text-zinc-200">No users yet</p>
          <p className="text-xs text-zinc-500">Invite teammates with a role and site scope.</p>
          <Button variant="primary" icon={<Plus />} className="mt-2" onClick={() => setCreateOpen(true)}>
            Add user
          </Button>
        </div>
      ) : (
        <div className="wt-table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)]">
                <th className="h-10 px-3">User</th>
                <th className="h-10 px-3">Role</th>
                <th className="h-10 px-3">Sites</th>
                <th className="h-10 px-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {users.map((u) => {
                const locs = u.location_ids ?? [];
                const isSelf = u.id === user?.id;
                return (
                  <tr key={u.id}>
                    <td className="h-11 px-3">
                      <div className="flex items-center gap-2">
                        <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-[var(--color-surface-overlay)] text-xs font-semibold uppercase text-zinc-300">
                          {u.email.slice(0, 1)}
                        </span>
                        <span className="text-zinc-100">{u.email}</span>
                        {isSelf && <Badge tone="brand">you</Badge>}
                        {u.totp_enabled && <Badge tone="info">2FA</Badge>}
                      </div>
                    </td>
                    <td className="h-11 px-3">
                      <RoleBadge role={u.role} />
                    </td>
                    <td className="h-11 px-3">
                      {u.role === "admin" ? (
                        <span className="text-xs text-zinc-500">All sites</span>
                      ) : locs.length === 0 ? (
                        <button
                          type="button"
                          className="wt-badge wt-badge-warn cursor-pointer"
                          onClick={() => setSitesFor(u.id)}
                          title="No sites assigned — this user sees nothing. Click to assign."
                        >
                          No sites · blocked
                        </button>
                      ) : (
                        <button
                          type="button"
                          className="flex flex-wrap items-center gap-1 text-left"
                          onClick={() => setSitesFor(u.id)}
                          title="Edit site access"
                        >
                          {locs.slice(0, 3).map((id) => (
                            <Badge key={id}>{locName[id] || id.slice(0, 8)}</Badge>
                          ))}
                          {locs.length > 3 && (
                            <span className="text-xs text-zinc-500">+{locs.length - 3}</span>
                          )}
                        </button>
                      )}
                    </td>
                    <td className="h-11 px-3">
                      <div className="flex justify-end gap-0.5">
                        <button
                          type="button"
                          className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm disabled:opacity-40"
                          aria-label={`Change role for ${u.email}`}
                          title={isSelf ? "You can't change your own role" : "Change role"}
                          disabled={isSelf}
                          onClick={() => {
                            setEditing(u);
                            setEditRole(u.role);
                          }}
                        >
                          <Pencil />
                        </button>
                        {u.role !== "admin" && (
                          <button
                            type="button"
                            className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                            aria-label={`Site access for ${u.email}`}
                            title="Site access"
                            onClick={() => setSitesFor(u.id)}
                          >
                            <MapPin />
                          </button>
                        )}
                        <button
                          type="button"
                          className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm"
                          aria-label={`Reset password for ${u.email}`}
                          title="Reset password"
                          onClick={() => setResetFor(u)}
                        >
                          <KeyRound />
                        </button>
                        <button
                          type="button"
                          className="wt-btn wt-btn-ghost wt-btn-icon wt-btn-sm hover:text-red-300 disabled:opacity-40"
                          aria-label={`Delete ${u.email}`}
                          title={isSelf ? "You can't delete yourself" : "Delete"}
                          disabled={isSelf}
                          onClick={() => void onDelete(u.id)}
                        >
                          <Trash2 />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <FormDialog open={createOpen} title="New user" onClose={() => setCreateOpen(false)}>
        <form onSubmit={onCreate} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Email" className="sm:col-span-2">
              <input
                className="wt-input w-full"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoFocus
                required
              />
            </Field>
            <Field label="Password" hint="At least 8 characters">
              <input
                className="wt-input w-full"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                minLength={8}
                autoComplete="new-password"
                required
              />
            </Field>
            <Field label="Role" hint={ROLE_HINT[role]}>
              <select
                className="wt-input w-full"
                value={role}
                onChange={(e) => setRole(e.target.value as Role)}
              >
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
            </Field>
          </div>
          {role !== "admin" && (
            <p className="wt-hint">
              New non-admin users start with no sites. Assign sites from the user list after
              creating.
            </p>
          )}
          <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="primary">
              Add user
            </Button>
          </div>
        </form>
      </FormDialog>

      <FormDialog
        open={!!editing}
        title={editing ? `Change role — ${editing.email}` : "Change role"}
        onClose={() => setEditing(null)}
      >
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (!editing) return;
            void onRoleChange(editing.id, editing.email, editRole).then((ok) => {
              if (ok) {
                toast(`Role updated to ${editRole}`, "success");
                setEditing(null);
              }
            });
          }}
          className="space-y-4"
        >
          <Field label="Role" hint={ROLE_HINT[editRole]}>
            <select
              className="wt-input w-full"
              value={editRole}
              onChange={(e) => setEditRole(e.target.value as Role)}
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          </Field>
          <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Button variant="ghost" onClick={() => setEditing(null)}>
              Cancel
            </Button>
            <Button type="submit" variant="primary">
              Save role
            </Button>
          </div>
        </form>
      </FormDialog>

      <FormDialog
        open={!!sitesUser}
        title={sitesUser ? `Site access — ${sitesUser.email}` : "Site access"}
        onClose={() => setSitesFor(null)}
      >
        <div className="space-y-4">
          <p className="wt-hint">
            Changes save immediately. A non-admin with no sites sees nothing.
          </p>
          {locations.length === 0 ? (
            <p className="text-sm text-zinc-500">No locations defined</p>
          ) : (
            <div className="grid gap-1 sm:grid-cols-2">
              {locations.map((loc) => (
                <label
                  key={loc.id}
                  className="flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm text-zinc-200 hover:bg-[var(--color-surface-overlay)]"
                >
                  <input
                    type="checkbox"
                    checked={sitesUserLocs.includes(loc.id)}
                    onChange={() =>
                      sitesUser && void toggleLocation(sitesUser.id, sitesUserLocs, loc.id)
                    }
                  />
                  {loc.name}
                </label>
              ))}
            </div>
          )}
          <div className="flex items-center justify-between gap-2 border-t border-[var(--color-border)] pt-4">
            <span className="text-xs text-zinc-500">
              {sitesUserLocs.length} of {locations.length} site(s)
            </span>
            <Button variant="primary" onClick={() => setSitesFor(null)}>
              Done
            </Button>
          </div>
        </div>
      </FormDialog>

      <FormDialog
        open={!!resetFor}
        title={resetFor ? `Reset password — ${resetFor.email}` : "Reset password"}
        onClose={closeReset}
      >
        <form onSubmit={onResetPassword} className="space-y-4" noValidate>
          <Field label="New password" hint="At least 8 characters">
            <input
              type="password"
              className="wt-input w-full"
              value={resetPassword}
              onChange={(e) => {
                setResetPassword(e.target.value);
                setResetError("");
              }}
              autoComplete="new-password"
              autoFocus
            />
          </Field>
          {resetError && <p className="text-sm text-red-400">{resetError}</p>}
          <div className="flex justify-end gap-2 border-t border-[var(--color-border)] pt-4">
            <Button variant="ghost" onClick={closeReset}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" disabled={!resetPassword}>
              Reset password
            </Button>
          </div>
        </form>
      </FormDialog>
    </div>
  );
}
