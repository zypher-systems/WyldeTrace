import { useEffect, useState, type FormEvent } from "react";
import { Navigate, useSearchParams } from "react-router-dom";
import { ApiError, api, setToken, type User } from "../api";
import { useAuth } from "../auth";
import {
  AlertCircle,
  ArrowLeft,
  Clock,
  FlaskConical,
  Info,
  KeyRound,
  Loader2,
  Lock,
  Mail,
} from "lucide-react";

type OIDCStatus = {
  enabled: boolean;
  issuer?: string;
  default_role?: string;
  auto_provision?: boolean;
};

type LoginResult = {
  token?: string;
  user?: User;
  mfa_required?: boolean;
  mfa_token?: string;
};

export function LoginPage() {
  const { user, loading, refresh } = useAuth();
  const [params] = useSearchParams();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [labMode, setLabMode] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [oidc, setOidc] = useState<OIDCStatus>({ enabled: false });
  const [ssoBusy, setSsoBusy] = useState(false);
  const [mfaToken, setMfaToken] = useState("");
  const [mfaCode, setMfaCode] = useState("");
  const [recoveryCode, setRecoveryCode] = useState("");
  const [useRecovery, setUseRecovery] = useState(false);

  useEffect(() => {
    void api<OIDCStatus>("/api/v1/auth/oidc/status")
      .then(setOidc)
      .catch(() => setOidc({ enabled: false }));
    const host = window.location.hostname;
    const isLab = host === "localhost" || host === "127.0.0.1" || host.endsWith(".local");
    if (isLab) {
      setLabMode(true);
      setEmail("admin@wyldetech.local");
      setPassword("changeme");
    }
  }, []);

  useEffect(() => {
    const code = params.get("code");
    const legacyToken = params.get("token");
    const err = params.get("error");
    if (err) setError(err);
    if (params.get("reason") === "expired") {
      setNotice("Your session expired. Please sign in again.");
    }
    if (params.get("role_synced") === "1") {
      const from = params.get("from") || "previous";
      const to = params.get("to") || "updated";
      setNotice(`Your SSO role was updated (${from} → ${to}). Signed in with the new role.`);
    }
    if (code) {
      setSsoBusy(true);
      void api<{ token: string }>("/api/v1/auth/oidc/exchange", {
        method: "POST",
        body: JSON.stringify({ code }),
      })
        .then((res) => {
          setToken(res.token);
          return refresh();
        })
        .catch((e) => setError(e instanceof Error ? e.message : "SSO exchange failed"))
        .finally(() => setSsoBusy(false));
      return;
    }
    if (!legacyToken) return;
    setToken(legacyToken);
    setSsoBusy(true);
    void refresh().finally(() => setSsoBusy(false));
  }, [params, refresh]);

  if (!loading && user) return <Navigate to="/" replace />;

  async function finishLogin(token: string) {
    setToken(token);
    await refresh();
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (mfaToken) {
        const res = await api<LoginResult>("/api/v1/auth/mfa/verify", {
          method: "POST",
          body: JSON.stringify({
            mfa_token: mfaToken,
            code: useRecovery ? undefined : mfaCode,
            recovery_code: useRecovery ? recoveryCode : undefined,
          }),
        });
        if (!res.token) throw new Error("MFA verification failed");
        await finishLogin(res.token);
        return;
      }
      const res = await api<LoginResult>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      if (res.mfa_required && res.mfa_token) {
        setMfaToken(res.mfa_token);
        setNotice("Enter the code from your authenticator app.");
        return;
      }
      if (!res.token) throw new Error("Login failed");
      await finishLogin(res.token);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Login failed");
    } finally {
      setBusy(false);
    }
  }

  const expired = params.get("reason") === "expired";

  return (
    <div className="relative isolate flex min-h-screen flex-col overflow-hidden bg-[var(--color-canvas)]">
      {/* Backdrop: faint grid, masked to fade out, plus brand glows */}
      <div
        aria-hidden
        className="wt-grid-bg pointer-events-none absolute inset-0 -z-10"
        style={{
          maskImage: "radial-gradient(ellipse 70% 60% at 50% 40%, black 30%, transparent 75%)",
          WebkitMaskImage: "radial-gradient(ellipse 70% 60% at 50% 40%, black 30%, transparent 75%)",
        }}
      />
      <div
        aria-hidden
        className="pointer-events-none absolute left-1/2 top-[-18rem] -z-10 h-[36rem] w-[56rem] -translate-x-1/2 rounded-full opacity-60 blur-3xl"
        style={{
          background:
            "radial-gradient(closest-side, color-mix(in srgb, var(--color-brand) 28%, transparent), transparent)",
        }}
      />
      <div
        aria-hidden
        className="pointer-events-none absolute bottom-[-16rem] right-[-10rem] -z-10 h-[32rem] w-[32rem] rounded-full opacity-40 blur-3xl"
        style={{
          background:
            "radial-gradient(closest-side, color-mix(in srgb, var(--color-accent-blue) 22%, transparent), transparent)",
        }}
      />

      <main className="flex flex-1 items-center justify-center px-4 py-12">
        <div className="w-full max-w-[400px]">
          <div className="mb-8 flex flex-col items-center text-center wt-animate-fade">
            <div className="relative">
              <div
                aria-hidden
                className="absolute inset-0 -z-10 scale-150 rounded-full opacity-70 blur-2xl"
                style={{ background: "color-mix(in srgb, var(--color-brand) 25%, transparent)" }}
              />
              <img src="/logos/logo.svg" alt="" className="h-14 w-14 rounded-2xl shadow-lg" />
            </div>
            <p className="mt-4 text-2xl font-semibold tracking-tight text-zinc-50">
              Wylde<span className="text-brand">Trace</span>
            </p>
            <p className="mt-1 text-sm text-zinc-500">IP address management console</p>
          </div>

          <form
            onSubmit={onSubmit}
            className="relative space-y-5 rounded-2xl border border-[var(--color-border-strong)] bg-[var(--color-surface-raised)]/90 p-7 shadow-[var(--shadow-pop)] backdrop-blur wt-animate-in"
            style={{ animationDelay: "60ms" }}
          >
            <div
              aria-hidden
              className="pointer-events-none absolute inset-x-8 -top-px h-px"
              style={{
                background:
                  "linear-gradient(90deg, transparent, color-mix(in srgb, var(--color-brand) 70%, transparent), transparent)",
              }}
            />
            <div>
              <h1 className="text-lg font-semibold tracking-tight text-zinc-50">
                {mfaToken ? "Two-factor authentication" : "Sign in"}
              </h1>
              <p className="mt-1 text-sm text-zinc-400">
                {mfaToken
                  ? "Confirm with your authenticator app or a recovery code."
                  : oidc.enabled
                    ? "Use single sign-on or your local account."
                    : "Welcome back. Sign in to your account."}
              </p>
            </div>

            {notice && (
              <div
                role="status"
                className={
                  expired && !mfaToken
                    ? "flex items-start gap-2.5 rounded-lg border border-amber-400/30 bg-amber-400/10 px-3 py-2.5 text-sm text-amber-100"
                    : "flex items-start gap-2.5 rounded-lg border border-brand/30 bg-brand/10 px-3 py-2.5 text-sm text-brand"
                }
              >
                {expired && !mfaToken ? (
                  <Clock className="mt-0.5 h-4 w-4 shrink-0" />
                ) : (
                  <Info className="mt-0.5 h-4 w-4 shrink-0" />
                )}
                <span>{notice}</span>
              </div>
            )}

            {!mfaToken && oidc.enabled && (
              <div className="space-y-3">
                <a
                  href="/api/v1/auth/oidc/login"
                  className="wt-btn wt-btn-secondary h-10 w-full justify-center"
                  onClick={() => setSsoBusy(true)}
                >
                  {ssoBusy ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <KeyRound className="h-4 w-4 text-brand" />
                  )}
                  {ssoBusy ? "Completing SSO…" : "Sign in with SSO"}
                </a>
                {oidc.issuer && (
                  <p className="truncate text-center text-xs text-zinc-500" title={oidc.issuer}>
                    via {oidc.issuer.replace(/^https?:\/\//, "")}
                    {oidc.default_role ? ` · default ${oidc.default_role}` : ""}
                  </p>
                )}
                <div className="flex items-center gap-3 text-[11px] uppercase tracking-wider text-zinc-600">
                  <span className="h-px flex-1 bg-[var(--color-border)]" />
                  or use password
                  <span className="h-px flex-1 bg-[var(--color-border)]" />
                </div>
              </div>
            )}

            {!mfaToken ? (
              <div className="space-y-4">
                <label className="block space-y-1.5">
                  <span className="wt-label">Email</span>
                  <div className="relative">
                    <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
                    <input
                      className="wt-input h-10 w-full pl-9"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      type="email"
                      autoComplete="username"
                      placeholder="you@company.com"
                      autoFocus
                      required
                    />
                  </div>
                </label>
                <label className="block space-y-1.5">
                  <span className="wt-label">Password</span>
                  <div className="relative">
                    <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
                    <input
                      className="wt-input h-10 w-full pl-9"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      type="password"
                      autoComplete="current-password"
                      required
                    />
                  </div>
                </label>
                {labMode && (
                  <p className="flex items-start gap-2 rounded-lg border border-dashed border-[var(--color-border-strong)] px-3 py-2 text-xs text-zinc-500">
                    <FlaskConical className="mt-px h-3.5 w-3.5 shrink-0 text-zinc-500" />
                    Lab host detected — bootstrap admin credentials are prefilled. Change the
                    password after first sign-in.
                  </p>
                )}
              </div>
            ) : (
              <div className="space-y-3">
                {!useRecovery ? (
                  <label className="block space-y-1.5">
                    <span className="wt-label">Authenticator code</span>
                    <input
                      className="wt-input h-12 w-full text-center font-mono text-lg tracking-[0.5em]"
                      value={mfaCode}
                      onChange={(e) => setMfaCode(e.target.value)}
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      placeholder="000000"
                      autoFocus
                      required={!useRecovery}
                    />
                  </label>
                ) : (
                  <label className="block space-y-1.5">
                    <span className="wt-label">Recovery code</span>
                    <input
                      className="wt-input h-10 w-full font-mono"
                      value={recoveryCode}
                      onChange={(e) => setRecoveryCode(e.target.value)}
                      autoFocus
                      required={useRecovery}
                    />
                  </label>
                )}
                <div className="flex items-center justify-between gap-2">
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-200"
                    onClick={() => {
                      setMfaToken("");
                      setMfaCode("");
                      setRecoveryCode("");
                      setNotice("");
                    }}
                  >
                    <ArrowLeft className="h-3.5 w-3.5" />
                    Back to password
                  </button>
                  <button
                    type="button"
                    className="text-xs text-brand/90 hover:text-brand"
                    onClick={() => setUseRecovery((v) => !v)}
                  >
                    {useRecovery ? "Use authenticator code" : "Use a recovery code"}
                  </button>
                </div>
              </div>
            )}

            {error && (
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-lg border border-red-400/30 bg-red-500/10 px-3 py-2.5 text-sm text-red-200"
              >
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            <button
              type="submit"
              disabled={busy}
              className="wt-btn wt-btn-primary h-10 w-full justify-center disabled:opacity-60"
            >
              {busy && <Loader2 className="h-4 w-4 animate-spin" aria-hidden />}
              {busy ? "Signing in…" : mfaToken ? "Verify" : "Sign in"}
            </button>
          </form>
        </div>
      </main>

      <footer className="pb-6 text-center text-xs text-zinc-600">
        Wylde Technologies
      </footer>
    </div>
  );
}
