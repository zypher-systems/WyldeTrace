import { render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsPage } from "./SettingsPage";

const api = vi.fn();

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    api: (...args: unknown[]) => api(...args),
    getToken: () => "test-token",
  };
});

vi.mock("../auth", () => ({
  useAuth: () => ({
    user: { id: "u1", email: "admin@example.com", role: "admin", location_ids: [] },
    loading: false,
    refresh: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
  }),
}));

vi.mock("../hooks/useUnsavedChanges", () => ({
  useUnsavedChanges: () => {},
}));

describe("SettingsPage", () => {
  beforeEach(() => {
    api.mockReset();
    api.mockImplementation(async (path: string) => {
      if (path === "/api/v1/auth/oidc/status") return { enabled: false };
      if (path === "/api/v1/settings") {
        return {
          ip_deleted_retention_days: 90,
          subnet_deleted_retention_days: 90,
          audit_retention_days: 365,
          webhook_delivery_retention_days: 30,
          reports_summary_interval_hours: 24,
          subnet_utilization_alert_percent: 90,
          webhook_mute_hours: 6,
          powerdns_enabled: false,
          powerdns_url: "",
          powerdns_api_key_set: false,
          powerdns_dry_run: true,
          powerdns_server: "localhost",
          kea_enabled: false,
          kea_url: "",
          kea_dry_run: true,
          snmp_enabled: false,
          snmp_community_set: false,
          snmp_version: "2c",
          snmp_timeout_seconds: 5,
          smtp_enabled: false,
          smtp_host: "",
          smtp_port: 587,
          smtp_user: "",
          smtp_pass_set: false,
          smtp_from: "",
          smtp_to: "",
          smtp_subject_template: "",
          smtp_body_template: "",
        };
      }
      if (path === "/api/v1/instance") {
        return { version: "test", migration_version: "1", db_ok: true };
      }
      if (path === "/api/v1/auth/mfa") return { totp_enabled: false };
      throw new Error(`unexpected api call: ${path}`);
    });
  });

  it("renders deleted IP retention settings in the data section", async () => {
    render(
      <MemoryRouter>
        <SettingsPage section="data" />
      </MemoryRouter>,
    );

    const heading = await screen.findByRole("heading", { name: /deleted ip retention/i });
    const row = heading.parentElement!.parentElement!;
    expect(within(row).getByRole("button", { name: /save retention/i })).toBeInTheDocument();
    expect(within(row).getByRole("button", { name: /purge now/i })).toBeInTheDocument();

    await waitFor(() => {
      expect(api).toHaveBeenCalledWith("/api/v1/settings");
    });
    await waitFor(() => {
      expect(within(row).getByRole("spinbutton")).toHaveValue(90);
    });
  });

  it("shows the signed-in account in the account section", async () => {
    render(
      <MemoryRouter>
        <SettingsPage section="account" />
      </MemoryRouter>,
    );

    expect(await screen.findByText(/signed in as/i)).toBeInTheDocument();
    expect(screen.getByText("admin@example.com")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /change password/i })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /deleted ip retention/i })).not.toBeInTheDocument();
  });
});
