import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginPage } from "./LoginPage";

const api = vi.fn();
const setToken = vi.fn();
const refresh = vi.fn().mockResolvedValue(undefined);

vi.mock("../api", () => ({
  api: (...args: unknown[]) => api(...args),
  setToken: (...args: unknown[]) => setToken(...args),
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

vi.mock("../auth", () => ({
  useAuth: () => ({
    user: null,
    loading: false,
    refresh,
    login: vi.fn(),
    logout: vi.fn(),
  }),
}));

function renderLogin() {
  return render(
    <MemoryRouter>
      <LoginPage />
    </MemoryRouter>,
  );
}

function form() {
  const el = document.querySelector("form");
  if (!el) throw new Error("login form missing");
  return el;
}

describe("LoginPage", () => {
  beforeEach(() => {
    api.mockReset();
    setToken.mockReset();
    refresh.mockClear();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: new URL("http://app.example.com/login"),
    });
    api.mockImplementation(async (path: string) => {
      if (path === "/api/v1/auth/oidc/status") return { enabled: false };
      throw new Error(`unexpected api call: ${path}`);
    });
  });

  it("renders sign-in form", async () => {
    renderLogin();
    const f = within(form());
    expect(await f.findByRole("button", { name: /^sign in$/i })).toBeInTheDocument();
    expect(f.getByRole("textbox", { name: /^email$/i })).toBeInTheDocument();
    expect(form().querySelector('input[type="password"]')).toBeTruthy();
  });

  it("shows MFA step when login requires TOTP", async () => {
    const user = userEvent.setup();
    api.mockImplementation(async (path: string, init?: RequestInit) => {
      if (path === "/api/v1/auth/oidc/status") return { enabled: false };
      if (path === "/api/v1/auth/login" && init?.method === "POST") {
        return { mfa_required: true, mfa_token: "pending-mfa-token" };
      }
      throw new Error(`unexpected api call: ${path}`);
    });

    renderLogin();
    const f = within(form());
    await user.type(f.getByRole("textbox", { name: /^email$/i }), "user@example.com");
    await user.type(form().querySelector('input[type="password"]') as HTMLInputElement, "secret");
    await user.click(f.getByRole("button", { name: /^sign in$/i }));

    expect(await screen.findByRole("heading", { name: /two-factor authentication/i })).toBeInTheDocument();
    expect(f.getByLabelText(/authenticator code/i)).toBeInTheDocument();
    expect(f.getByRole("button", { name: /^verify$/i })).toBeInTheDocument();
    expect(f.getByRole("button", { name: /use a recovery code/i })).toBeInTheDocument();

    await user.click(f.getByRole("button", { name: /use a recovery code/i }));
    expect(f.getByLabelText(/recovery code/i)).toBeInTheDocument();
  });

  it("verifies MFA code and finishes login", async () => {
    const user = userEvent.setup();
    api.mockImplementation(async (path: string) => {
      if (path === "/api/v1/auth/oidc/status") return { enabled: false };
      if (path === "/api/v1/auth/login") {
        return { mfa_required: true, mfa_token: "pending-mfa-token" };
      }
      if (path === "/api/v1/auth/mfa/verify") {
        return { token: "session-token", user: { id: "1", email: "user@example.com", role: "editor" } };
      }
      throw new Error(`unexpected api call: ${path}`);
    });

    renderLogin();
    const f = within(form());
    await user.type(f.getByRole("textbox", { name: /^email$/i }), "user@example.com");
    await user.type(form().querySelector('input[type="password"]') as HTMLInputElement, "secret");
    await user.click(f.getByRole("button", { name: /^sign in$/i }));
    await f.findByLabelText(/authenticator code/i);
    await user.type(f.getByLabelText(/authenticator code/i), "123456");
    await user.click(f.getByRole("button", { name: /^verify$/i }));

    await waitFor(() => {
      expect(setToken).toHaveBeenCalledWith("session-token");
      expect(refresh).toHaveBeenCalled();
    });
  });
});
