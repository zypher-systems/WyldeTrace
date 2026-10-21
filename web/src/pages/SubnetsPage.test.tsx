import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "../components/Toast";
import { SubnetsPage } from "./SubnetsPage";

const api = vi.fn();

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    api: (...args: unknown[]) => api(...args),
    downloadCSV: vi.fn(),
    uploadCSV: vi.fn(),
  };
});

vi.mock("../auth", () => ({
  useAuth: () => ({
    user: { id: "u1", email: "editor@example.com", role: "editor", location_ids: [] },
    loading: false,
    refresh: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
  }),
}));

vi.mock("../hooks/useUnsavedChanges", () => ({
  useUnsavedChanges: () => {},
}));

function renderPage(entry = "/") {
  return render(
    <ToastProvider>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="*" element={<SubnetsPage />} />
        </Routes>
      </MemoryRouter>
    </ToastProvider>,
  );
}

describe("SubnetsPage", () => {
  beforeEach(() => {
    api.mockReset();
    api.mockImplementation(async (path: string) => {
      if (path.startsWith("/api/v1/subnets/tree")) return [];
      if (path.startsWith("/api/v1/subnets/tags")) return ["prod", "lab"];
      if (path.startsWith("/api/v1/custom-fields")) return [];
      if (path.startsWith("/api/v1/vrfs")) {
        return [{ id: "00000000-0000-0000-0000-000000000001", name: "Default", rd: "" }];
      }
      if (path.startsWith("/api/v1/locations")) return [{ id: "loc1", name: "HQ" }];
      if (path.startsWith("/api/v1/subnets")) {
        const q = new URL(path, "http://local").searchParams.get("q") || "";
        const items = q.includes("10.0")
          ? [
              {
                id: "s1",
                cidr: "10.0.0.0/24",
                name: "Core",
                description: "",
                tags: ["prod"],
                location_id: "loc1",
                vrf_id: "00000000-0000-0000-0000-000000000001",
                created_at: new Date().toISOString(),
                updated_at: new Date().toISOString(),
              },
            ]
          : [
              {
                id: "s1",
                cidr: "10.0.0.0/24",
                name: "Core",
                description: "",
                tags: ["prod"],
                location_id: "loc1",
                vrf_id: "00000000-0000-0000-0000-000000000001",
                created_at: new Date().toISOString(),
                updated_at: new Date().toISOString(),
              },
              {
                id: "s2",
                cidr: "192.168.1.0/24",
                name: "Lab",
                description: "",
                tags: ["lab"],
                location_id: "loc1",
                vrf_id: "00000000-0000-0000-0000-000000000001",
                created_at: new Date().toISOString(),
                updated_at: new Date().toISOString(),
              },
            ];
        return { items, total: items.length, page: 1, page_size: 25 };
      }
      throw new Error(`unexpected api call: ${path}`);
    });
  });

  it("loads list and applies text filter via URL", async () => {
    renderPage("/?q=10.0");

    expect(await screen.findByPlaceholderText(/filter subnets/i)).toBeInTheDocument();
    await waitFor(() => {
      const calls = api.mock.calls.map((c) => String(c[0]));
      expect(calls.some((p) => p.includes("q=10.0") || p.includes("q=10%2E0"))).toBe(true);
    });
    expect(await screen.findByText("10.0.0.0/24")).toBeInTheDocument();
  });

  it("toggles tag filter and match mode", async () => {
    const user = userEvent.setup();
    renderPage();

    const tags = await screen.findByText(/Tags:/i);
    const tagBar = tags.parentElement!;
    await user.click(within(tagBar).getByRole("button", { name: "prod" }));

    await waitFor(() => {
      const calls = api.mock.calls.map((c) => String(c[0]));
      expect(calls.some((p) => p.includes("tags=prod"))).toBe(true);
    });

    expect(within(tagBar).getByRole("button", { name: "AND" })).toBeInTheDocument();
    await user.click(within(tagBar).getByRole("button", { name: "OR" }));
    await waitFor(() => {
      const calls = api.mock.calls.map((c) => String(c[0]));
      expect(calls.some((p) => p.includes("tag_match=or"))).toBe(true);
    });
  });
});
