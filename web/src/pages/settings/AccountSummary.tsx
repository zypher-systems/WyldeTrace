import type { ReactNode } from "react";
import type { User } from "../../api";

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2.5">
      <dt className="text-zinc-400">{label}</dt>
      <dd className="text-right text-zinc-200">{children}</dd>
    </div>
  );
}

export function AccountSummary({ user }: { user: User | null }) {
  if (!user) return null;
  return (
    <dl className="divide-y divide-[var(--color-border)] text-sm">
      <Row label="Signed in as">{user.email}</Row>
      <Row label="Role">
        <span className="font-mono">{user.role}</span>
      </Row>
    </dl>
  );
}
