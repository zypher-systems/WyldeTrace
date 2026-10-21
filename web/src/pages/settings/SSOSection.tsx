import type { ReactNode } from "react";
import { Fingerprint } from "lucide-react";
import { api } from "../../api";
import { useToast } from "../../components/Toast";
import { Badge, Button, Field } from "../../components/ui";

type OidcInfo = {
  enabled: boolean;
  issuer?: string;
  auto_provision?: boolean;
  default_role?: string;
  groups_claim?: string;
  role_claim?: string;
  role_map?: Record<string, string[]>;
  sync_role?: boolean;
};

export function SSOSection({
  oidc,
  isAdmin,
  currentRole,
  previewGroups,
  setPreviewGroups,
  previewRole,
  setPreviewRole,
  previewResult,
  setPreviewResult,
}: {
  oidc: OidcInfo;
  isAdmin: boolean;
  currentRole?: string;
  previewGroups: string;
  setPreviewGroups: (v: string) => void;
  previewRole: string;
  setPreviewRole: (v: string) => void;
  previewResult: {
    role: string;
    source: string;
    matched_groups: string[];
    would_sync?: boolean;
  } | null;
  setPreviewResult: (
    v: {
      role: string;
      source: string;
      matched_groups: string[];
      would_sync?: boolean;
    } | null,
  ) => void;
}) {
  const toast = useToast();
  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-[var(--color-surface-overlay)] text-zinc-400">
            <Fingerprint className="h-4 w-4" />
          </span>
          <div>
            <h2 className="text-sm font-semibold text-zinc-100">Single sign-on (OIDC)</h2>
            <p className="mt-0.5 text-xs text-zinc-500">
              {oidc.enabled
                ? "Identity provider settings are managed via server configuration."
                : "OIDC SSO is disabled."}
            </p>
          </div>
        </div>
        <Badge tone={oidc.enabled ? "brand" : "neutral"}>{oidc.enabled ? "Enabled" : "Disabled"}</Badge>
      </div>

      {oidc.enabled && (
        <dl className="divide-y divide-[var(--color-border)] rounded-xl border border-[var(--color-border)] px-4 text-sm">
          <SsoRow label="Issuer">
            <span className="block truncate font-mono text-xs text-zinc-300" title={oidc.issuer}>
              {oidc.issuer || "—"}
            </span>
          </SsoRow>
          <SsoRow label="Default role">
            <Badge className="font-mono">{oidc.default_role || "readonly"}</Badge>
          </SsoRow>
          <SsoRow label="Auto-provision">
            <YesNo value={!!oidc.auto_provision} />
          </SsoRow>
          <SsoRow label="Sync role on login">
            <YesNo value={!!oidc.sync_role} />
          </SsoRow>
          <SsoRow label="Claims">
            <span className="font-mono text-xs text-zinc-300">
              groups={oidc.groups_claim || "groups"} · role={oidc.role_claim || "role"}
            </span>
          </SsoRow>
          {oidc.role_map && Object.keys(oidc.role_map).length > 0 && (
            <SsoRow label="Role map">
              <div className="space-y-1 text-right">
                {Object.entries(oidc.role_map).map(([role, groups]) => (
                  <div key={role} className="flex flex-wrap items-center justify-end gap-1.5">
                    <Badge tone="accent" className="font-mono">
                      {role}
                    </Badge>
                    <span className="font-mono text-xs text-zinc-400">
                      ← {(groups ?? []).join(" | ") || "—"}
                    </span>
                  </div>
                ))}
              </div>
            </SsoRow>
          )}
        </dl>
      )}

      {isAdmin && oidc.enabled && (
        <div className="space-y-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
          <div>
            <h3 className="text-sm font-medium text-zinc-200">Claim preview</h3>
            <p className="mt-0.5 text-xs text-zinc-500">
              Dry-run role resolution from sample {oidc.groups_claim || "groups"} /{" "}
              {oidc.role_claim || "role"} values (no IdP call).
            </p>
          </div>
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_10rem_auto] sm:items-end">
            <Field label="Groups">
              <input
                className="wt-input w-full font-mono text-xs"
                value={previewGroups}
                onChange={(e) => setPreviewGroups(e.target.value)}
                placeholder="wt-editors, users"
              />
            </Field>
            <Field label="Role claim">
              <input
                className="wt-input w-full font-mono text-xs"
                value={previewRole}
                onChange={(e) => setPreviewRole(e.target.value)}
                placeholder="optional"
              />
            </Field>
            <Button
              onClick={() =>
                void (async () => {
                  try {
                    const res = await api<{
                      role: string;
                      source: string;
                      matched_groups: string[];
                      would_sync?: boolean;
                    }>("/api/v1/auth/oidc/preview", {
                      method: "POST",
                      body: JSON.stringify({
                        groups: previewGroups
                          .split(",")
                          .map((g) => g.trim())
                          .filter(Boolean),
                        role: previewRole,
                        current_role: currentRole,
                      }),
                    });
                    setPreviewResult(res);
                  } catch (err) {
                    toast(err instanceof Error ? err.message : "Preview failed", "error");
                  }
                })()
              }
            >
              Preview
            </Button>
          </div>
          {previewResult && (
            <div className="flex flex-wrap items-center gap-2 rounded-lg border border-[var(--color-border)] bg-[var(--color-canvas)] px-3 py-2 text-xs">
              <span className="text-zinc-500">Resolves to</span>
              <Badge tone="brand" className="font-mono">
                {previewResult.role}
              </Badge>
              <span className="text-zinc-500">via</span>
              <span className="font-mono text-zinc-300">{previewResult.source}</span>
              {previewResult.matched_groups?.length ? (
                <span className="font-mono text-zinc-400">
                  · matched {previewResult.matched_groups.join(", ")}
                </span>
              ) : null}
              {previewResult.would_sync ? (
                <Badge tone="info">would sync on next SSO login</Badge>
              ) : null}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function SsoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2.5">
      <dt className="shrink-0 text-zinc-400">{label}</dt>
      <dd className="min-w-0">{children}</dd>
    </div>
  );
}

function YesNo({ value }: { value: boolean }) {
  return <Badge tone={value ? "brand" : "neutral"}>{value ? "Yes" : "No"}</Badge>;
}
