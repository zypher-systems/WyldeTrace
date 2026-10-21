import {
  Activity,
  Bell,
  Boxes,
  Building2,
  Cable,
  Cpu,
  Database,
  FileClock,
  Gauge,
  GitBranch,
  KeyRound,
  ListChecks,
  Mail,
  Network,
  Plug,
  Radar,
  ScanSearch,
  ScrollText,
  Send,
  Settings,
  SlidersHorizontal,
  Tags,
  Trash2,
  UserRound,
  Users,
  Webhook,
  type LucideIcon,
} from "lucide-react";
import type { Role } from "./api";

export type Counts = { inbox: number; requests: number };

export type NavTab = {
  to: string;
  label: string;
  icon: LucideIcon;
  end?: boolean;
  count?: keyof Counts;
};

export type Area = {
  key: string;
  label: string;
  icon: LucideIcon;
  to: string;
  /** Path prefixes owned by this area (first match wins, longest first). */
  match: string[];
  tabs?: NavTab[];
  count?: keyof Counts;
};

type Perms = { role: Role | undefined; canEdit: boolean; isAdmin: boolean; reportOnly: boolean };

export function permsFor(role: Role | undefined): Perms {
  return {
    role,
    canEdit: role === "admin" || role === "editor",
    isAdmin: role === "admin",
    reportOnly: role === "reportview",
  };
}

export function settingsTabs(p: Perms): Array<NavTab & { group: string }> {
  const account = [
    { group: "Personal", to: "/settings/account", label: "Account & security", icon: UserRound },
  ];
  if (!p.isAdmin) return account;
  return [
    ...account,
    { group: "Organization", to: "/settings/general", label: "General", icon: SlidersHorizontal },
    { group: "Organization", to: "/settings/users", label: "Users", icon: Users },
    { group: "Organization", to: "/settings/tokens", label: "API tokens", icon: KeyRound },
    { group: "Organization", to: "/settings/custom-fields", label: "Custom fields", icon: Tags },
    { group: "Alerts & integrations", to: "/settings/notifications", label: "Notifications", icon: Mail },
    { group: "Alerts & integrations", to: "/settings/webhooks", label: "Webhooks", icon: Webhook },
    { group: "Alerts & integrations", to: "/settings/integrations", label: "DNS, DHCP & SNMP", icon: Plug },
    { group: "Data", to: "/settings/data", label: "Retention & backup", icon: Database },
  ];
}

export function areasFor(p: Perms): Area[] {
  const settings: Area = {
    key: "settings",
    label: "Settings",
    icon: Settings,
    to: "/settings",
    match: ["/settings"],
  };
  if (p.reportOnly) {
    return [
      { key: "home", label: "Overview", icon: Gauge, to: "/", match: ["/"] },
      { key: "requests", label: "IP requests", icon: Send, to: "/requests", match: ["/requests"], count: "requests" },
      settings,
    ];
  }
  const activityTabs: NavTab[] = [{ to: "/ip-history", label: "IP history", icon: FileClock }];
  if (p.isAdmin) activityTabs.push({ to: "/audit", label: "Audit log", icon: ScrollText });
  if (p.canEdit) {
    activityTabs.push(
      { to: "/deleted-ips", label: "Deleted IPs", icon: Trash2 },
      { to: "/deleted-subnets", label: "Deleted subnets", icon: Trash2 },
    );
  }
  const discoveryTabs: NavTab[] = [
    { to: "/inbox", label: "Inbox", icon: Bell, count: "inbox" },
    { to: "/discovery", label: "Scan jobs", icon: ListChecks },
  ];
  if (p.canEdit) {
    discoveryTabs.push({ to: "/discover-subnets", label: "Find subnets", icon: ScanSearch });
  }
  return [
    { key: "home", label: "Overview", icon: Gauge, to: "/", match: ["/"] },
    {
      key: "network",
      label: "Network",
      icon: Network,
      to: "/subnets",
      match: ["/subnets", "/locations", "/vrfs", "/vlans", "/device-types", "/search"],
      tabs: [
        { to: "/subnets", label: "Subnets", icon: GitBranch },
        { to: "/locations", label: "Sites", icon: Building2 },
        { to: "/vrfs", label: "VRFs", icon: Boxes },
        { to: "/vlans", label: "VLANs", icon: Cable },
        { to: "/device-types", label: "Device types", icon: Cpu },
      ],
    },
    {
      key: "discovery",
      label: "Discovery",
      icon: Radar,
      to: "/inbox",
      match: ["/inbox", "/discovery", "/discover-subnets"],
      tabs: discoveryTabs,
      count: "inbox",
    },
    {
      key: "requests",
      label: "IP requests",
      icon: Send,
      to: "/requests",
      match: ["/requests"],
      count: "requests",
    },
    {
      key: "activity",
      label: "Activity",
      icon: Activity,
      to: "/ip-history",
      match: ["/ip-history", "/audit", "/deleted-ips", "/deleted-subnets"],
      tabs: activityTabs,
    },
    settings,
  ];
}

export function activeArea(areas: Area[], pathname: string): Area | undefined {
  let best: Area | undefined;
  let bestLen = -1;
  for (const a of areas) {
    for (const m of a.match) {
      const hit = m === "/" ? pathname === "/" : pathname === m || pathname.startsWith(m + "/");
      if (hit && m.length > bestLen) {
        best = a;
        bestLen = m.length;
      }
    }
  }
  return best;
}

