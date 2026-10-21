import type { FormEvent } from "react";
import { KeyRound } from "lucide-react";
import { Button, Field } from "../../components/ui";

export function PasswordSection({
  currentPassword,
  setCurrentPassword,
  newPassword,
  setNewPassword,
  onSubmit,
}: {
  currentPassword: string;
  setCurrentPassword: (v: string) => void;
  newPassword: string;
  setNewPassword: (v: string) => void;
  onSubmit: (e: FormEvent) => void;
}) {
  return (
    <form onSubmit={onSubmit} className="flex h-full flex-col gap-4">
      <div className="flex items-start gap-3">
        <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-[var(--color-surface-overlay)] text-zinc-400">
          <KeyRound className="h-4 w-4" />
        </span>
        <div>
          <h2 className="text-sm font-semibold text-zinc-100">Change password</h2>
          <p className="mt-0.5 text-xs text-zinc-500">Local account password used for sign-in.</p>
        </div>
      </div>
      <div className="grid gap-4">
        <Field label="Current password">
          <input
            type="password"
            className="wt-input w-full"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </Field>
        <Field label="New password" hint="At least 8 characters">
          <input
            type="password"
            className="wt-input w-full"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            autoComplete="new-password"
            minLength={8}
            required
          />
        </Field>
      </div>
      <div className="mt-auto flex justify-end border-t border-[var(--color-border)] pt-4">
        <Button type="submit" variant="primary">
          Update password
        </Button>
      </div>
    </form>
  );
}
