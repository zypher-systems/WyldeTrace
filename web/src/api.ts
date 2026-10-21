const TOKEN_KEY = "wyldetrace_token";

export type Role = "admin" | "editor" | "readonly" | "reportview";

export type User = {
  id: string;
  email: string;
  role: Role;
  location_ids?: string[];
  site_scoped?: boolean;
  totp_enabled?: boolean;
  using_default_password?: boolean;
  created_at?: string;
  updated_at?: string;
};

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

function handleUnauthorized(status: number, path: string) {
  if (status !== 401) return;
  if (path.includes("/auth/login") || path.includes("/auth/oidc")) return;
  setToken(null);
  if (typeof window !== "undefined" && !window.location.pathname.startsWith("/login")) {
    const next = `/login?reason=expired`;
    window.location.assign(next);
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type") && init.body && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const res = await fetch(path, { ...init, headers });
  if (res.status === 204) return undefined as T;
  if (res.status === 401) {
    handleUnauthorized(401, path);
    throw new ApiError(401, "session expired");
  }
  const contentType = res.headers.get("Content-Type") || "";
  if (contentType.includes("text/csv")) {
    if (!res.ok) throw new ApiError(res.status, res.statusText);
    return (await res.text()) as T;
  }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(res.status, data.error || res.statusText);
  }
  return data as T;
}

export async function downloadCSV(path: string, filename: string) {
  const token = getToken();
  const res = await fetch(path, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (res.status === 401) {
    handleUnauthorized(401, path);
    throw new ApiError(401, "session expired");
  }
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new ApiError(res.status, data.error || res.statusText);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export async function uploadCSV<T>(path: string, file: File): Promise<T> {
  const token = getToken();
  const res = await fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "text/csv",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: await file.text(),
  });
  if (res.status === 401) {
    handleUnauthorized(401, path);
    throw new ApiError(401, "session expired");
  }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, data.error || res.statusText);
  return data as T;
}

export type Subnet = {
  id: string;
  cidr: string;
  name: string;
  description: string;
  location_id: string | null;
  vrf_id: string;
  vlan_id: string | null;
  vlan_vid: number | null;
  parent_id: string | null;
  tags?: string[];
  custom_fields?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  used?: number;
  total?: number;
  free?: number;
  percent?: number;
  sparse?: boolean;
  next_ip?: string;
};

export type VRF = {
  id: string;
  name: string;
  rd: string;
  description: string;
  created_at: string;
  updated_at: string;
};

export type VLAN = {
  id: string;
  vid: number;
  name: string;
  location_id: string | null;
  description: string;
  created_at: string;
  updated_at: string;
};

export type IPRange = {
  id: string;
  subnet_id: string;
  name: string;
  start_address: string;
  end_address: string;
  role: string;
  description: string;
  created_at: string;
  updated_at: string;
};

export const DEFAULT_VRF_ID = "00000000-0000-0000-0000-000000000001";

export type IPAddress = {
  id: string;
  address: string;
  subnet_id: string;
  hostname: string;
  status: string;
  device_type_id: string | null;
  location_id: string | null;
  notes: string;
  reserved_until: string | null;
  last_seen_at: string | null;
  mac_address: string;
  open_ports: string;
  interface_name?: string;
  switch_id?: string;
  vendor?: string;
  device_hint?: string;
  custom_fields?: Record<string, unknown>;
  deleted_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type Location = {
  id: string;
  name: string;
  description: string;
};

export type Utilization = {
  cidr: string;
  total?: number;
  used: number;
  free?: number;
  percent?: number;
  sparse?: boolean;
  next_ip?: string;
  next_ip_error?: string;
};

export type SweepResult = {
  ip: string;
  alive: boolean;
  rtt_ms?: number;
  hostname?: string;
  mac?: string;
  vendor?: string;
  device_hint?: string;
  open_ports?: number[];
  hints?: string[];
};

export type ReconcileHost = {
  ip: string;
  hostname?: string;
  mac?: string;
  vendor?: string;
  device_hint?: string;
  open_ports?: number[];
  hints?: string[];
  db_status?: string;
  db_id?: string;
};

export type SweepReport = {
  cidr: string;
  total: number;
  alive: number;
  results: SweepResult[];
  reconcile?: {
    new_alive: ReconcileHost[];
    matched: ReconcileHost[];
    stale: ReconcileHost[];
  };
};

export type ClaimResponse = {
  created: number;
  updated: number;
  skipped: number;
  conflicts: number;
  results: { address: string; action: string; detail?: string }[];
};

export type SearchResponse = {
  query: string;
  subnets: Subnet[];
  ips: IPAddress[];
  locations: Location[];
};

export type ImportSummary = {
  created: number;
  updated: number;
  skipped: number;
  errors: string[];
};

export type AuditLog = {
  id: string;
  actor_id: string | null;
  actor_email: string;
  action: string;
  entity_type: string;
  entity_id: string;
  details: Record<string, unknown>;
  created_at: string;
};

export type PageResult<T> = {
  items: T[];
  total: number;
  page: number;
  page_size: number;
};

export type APIToken = {
  id: string;
  name: string;
  prefix: string;
  role: Role;
  location_ids?: string[];
  site_scoped?: boolean;
  created_by: string | null;
  last_used_at: string | null;
  expires_at: string | null;
  revoked_at: string | null;
  created_at: string;
  token?: string;
};

export type DiscoveryJob = {
  id: string;
  name: string;
  subnet_id: string;
  interval_minutes: number;
  enabled: boolean;
  enrich: boolean;
  last_run_at: string | null;
  last_status: string;
  last_summary: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export function pageQuery(params: {
  page?: number;
  pageSize?: number;
  q?: string;
  status?: string;
  tag?: string;
  tags?: string;
  tagMatch?: "and" | "or";
  sort?: string;
  order?: "asc" | "desc";
}): string {
  const sp = new URLSearchParams();
  if (params.page) sp.set("page", String(params.page));
  if (params.pageSize) sp.set("page_size", String(params.pageSize));
  if (params.q) sp.set("q", params.q);
  if (params.status) sp.set("status", params.status);
  if (params.tags) sp.set("tags", params.tags);
  else if (params.tag) sp.set("tag", params.tag);
  if (params.tagMatch) sp.set("tag_match", params.tagMatch);
  if (params.sort) sp.set("sort", params.sort);
  if (params.order) sp.set("order", params.order);
  const qs = sp.toString();
  return qs ? `?${qs}` : "";
}
