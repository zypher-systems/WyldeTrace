import { useState } from "react";
import { Check, Copy, ShieldCheck } from "lucide-react";
import { api } from "../../api";
import { useToast } from "../../components/Toast";
import { Badge, Button, Field } from "../../components/ui";

export function MFASection({
  mfaEnabled,
  setMfaEnabled,
  mfaSecret,
  setMfaSecret,
  mfaURL,
  setMfaURL,
  mfaCode,
  setMfaCode,
  mfaRecovery,
  setMfaRecovery,
  mfaDisablePassword,
  setMfaDisablePassword,
  mfaDisableCode,
  setMfaDisableCode,
  refresh,
}: {
  mfaEnabled: boolean;
  setMfaEnabled: (v: boolean) => void;
  mfaSecret: string;
  setMfaSecret: (v: string) => void;
  mfaURL: string;
  setMfaURL: (v: string) => void;
  mfaCode: string;
  setMfaCode: (v: string) => void;
  mfaRecovery: string[] | null;
  setMfaRecovery: (v: string[] | null) => void;
  mfaDisablePassword: string;
  setMfaDisablePassword: (v: string) => void;
  mfaDisableCode: string;
  setMfaDisableCode: (v: string) => void;
  refresh: () => Promise<void>;
}) {
  const toast = useToast();
  const [copied, setCopied] = useState<string | null>(null);

  async function copy(text: string, what: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(what);
      window.setTimeout(() => setCopied(null), 1500);
    } catch {
      toast("Copy failed", "error");
    }
  }

  return (
    <div className="flex h-full flex-col gap-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-[var(--color-surface-overlay)] text-zinc-400">
            <ShieldCheck className="h-4 w-4" />
          </span>
          <div>
            <h2 className="text-sm font-semibold text-zinc-100">Two-factor authentication</h2>
            <p className="mt-0.5 text-xs text-zinc-500">
              TOTP for local password accounts. SSO users should rely on IdP MFA.
            </p>
          </div>
        </div>
        <Badge tone={mfaEnabled ? "brand" : "neutral"}>{mfaEnabled ? "Enabled" : "Disabled"}</Badge>
      </div>

      {!mfaEnabled && !mfaSecret && (
        <div className="mt-auto flex items-center justify-between gap-3 border-t border-[var(--color-border)] pt-4">
          <p className="text-xs text-zinc-500">Use any authenticator app (1Password, Authy, Google…).</p>
          <Button
            variant="primary"
            onClick={() =>
              void (async () => {
                try {
                  const res = await api<{ secret: string; otpauth_url: string }>(
                    "/api/v1/auth/mfa/enroll/begin",
                    { method: "POST" },
                  );
                  setMfaSecret(res.secret);
                  setMfaURL(res.otpauth_url);
                } catch (err) {
                  toast(err instanceof Error ? err.message : "MFA enroll failed", "error");
                }
              })()
            }
          >
            Begin enrollment
          </Button>
        </div>
      )}

      {mfaSecret && !mfaEnabled && (
        <div className="space-y-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
          <div className="space-y-1.5">
            <span className="wt-label">1. Add this secret to your authenticator</span>
            <div className="flex items-center gap-2">
              <code className="min-w-0 flex-1 select-all break-all rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-canvas)] px-3 py-2 font-mono text-xs text-zinc-100">
                {mfaSecret}
              </code>
              <Button
                size="sm"
                icon={copied === "secret" ? <Check /> : <Copy />}
                aria-label="Copy secret"
                title="Copy secret"
                onClick={() => void copy(mfaSecret, "secret")}
              />
            </div>
            {mfaURL && (
              <p className="truncate font-mono text-[10px] text-zinc-600" title={mfaURL}>
                {mfaURL}
              </p>
            )}
          </div>
          <div className="flex flex-wrap items-end gap-2">
            <Field label="2. Enter the 6-digit code" className="min-w-[10rem] flex-1">
              <input
                className="wt-input w-full font-mono tracking-widest"
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                placeholder="000000"
              />
            </Field>
            <Button
              variant="primary"
              onClick={() =>
                void (async () => {
                  try {
                    const res = await api<{ recovery_codes: string[] }>(
                      "/api/v1/auth/mfa/enroll/confirm",
                      { method: "POST", body: JSON.stringify({ code: mfaCode }) },
                    );
                    setMfaEnabled(true);
                    setMfaSecret("");
                    setMfaURL("");
                    setMfaCode("");
                    setMfaRecovery(res.recovery_codes);
                    toast("MFA enabled — store recovery codes safely", "success");
                    await refresh();
                  } catch (err) {
                    toast(err instanceof Error ? err.message : "MFA confirm failed", "error");
                  }
                })()
              }
            >
              Confirm & enable
            </Button>
          </div>
        </div>
      )}

      {mfaRecovery && (
        <div className="rounded-xl border border-amber-400/30 bg-amber-400/5 p-4 text-sm">
          <div className="mb-3 flex items-center justify-between gap-2">
            <p className="font-medium text-amber-100">Recovery codes — shown once</p>
            <div className="flex gap-1">
              <Button
                size="sm"
                variant="ghost"
                icon={copied === "recovery" ? <Check /> : <Copy />}
                onClick={() => void copy(mfaRecovery.join("\n"), "recovery")}
              >
                Copy
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setMfaRecovery(null)}>
                Dismiss
              </Button>
            </div>
          </div>
          <ul className="grid grid-cols-2 gap-x-4 gap-y-1 font-mono text-xs text-amber-50/90">
            {mfaRecovery.map((c) => (
              <li key={c}>{c}</li>
            ))}
          </ul>
        </div>
      )}

      {mfaEnabled && (
        <div className="mt-auto space-y-3 border-t border-[var(--color-border)] pt-4">
          <p className="text-xs text-zinc-500">
            To disable, confirm with your password and a current authenticator code.
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Current password">
              <input
                type="password"
                className="wt-input w-full"
                value={mfaDisablePassword}
                onChange={(e) => setMfaDisablePassword(e.target.value)}
                autoComplete="current-password"
              />
            </Field>
            <Field label="Authenticator code">
              <input
                className="wt-input w-full font-mono tracking-widest"
                value={mfaDisableCode}
                onChange={(e) => setMfaDisableCode(e.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
              />
            </Field>
          </div>
          <div className="flex justify-end">
            <Button
              variant="danger"
              onClick={() =>
                void (async () => {
                  try {
                    await api("/api/v1/auth/mfa/disable", {
                      method: "POST",
                      body: JSON.stringify({
                        password: mfaDisablePassword,
                        code: mfaDisableCode,
                      }),
                    });
                    setMfaEnabled(false);
                    setMfaDisablePassword("");
                    setMfaDisableCode("");
                    toast("MFA disabled", "success");
                    await refresh();
                  } catch (err) {
                    toast(err instanceof Error ? err.message : "Disable MFA failed", "error");
                  }
                })()
              }
            >
              Disable MFA
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
