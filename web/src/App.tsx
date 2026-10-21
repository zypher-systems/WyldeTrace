import { Navigate, createBrowserRouter, RouterProvider, useParams } from "react-router-dom";
import { useAuth } from "./auth";
import { AppShell } from "./components/AppShell";
import { SettingsLayout } from "./components/SettingsLayout";
import { AuditPage } from "./pages/AuditPage";
import { CustomFieldsPage } from "./pages/CustomFieldsPage";
import { DeletedIPsPage } from "./pages/DeletedIPsPage";
import { DeviceTypesPage } from "./pages/DeviceTypesPage";
import { DiscoverSubnetsPage } from "./pages/DiscoverSubnetsPage";
import { DiscoveryJobsPage } from "./pages/DiscoveryJobsPage";
import { InboxPage } from "./pages/InboxPage";
import { DeletedSubnetsPage } from "./pages/DeletedSubnetsPage";
import { IPHistoryPage } from "./pages/IPHistoryPage";
import { LocationDetailPage } from "./pages/LocationDetailPage";
import { LocationsPage } from "./pages/LocationsPage";
import { LoginPage } from "./pages/LoginPage";
import { OpsDashboardPage } from "./pages/OpsDashboardPage";
import { RequestsPage } from "./pages/RequestsPage";
import { SearchPage } from "./pages/SearchPage";
import { SettingsPage } from "./pages/SettingsPage";
import { SubnetDetailPage } from "./pages/SubnetDetailPage";
import { SubnetsPage } from "./pages/SubnetsPage";
import { TokensPage } from "./pages/TokensPage";
import { UsersPage } from "./pages/UsersPage";
import { VLANsPage } from "./pages/VLANsPage";
import { VRFsPage } from "./pages/VRFsPage";
import { WebhooksPage } from "./pages/WebhooksPage";

function Protected({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) {
    return (
      <div className="grid min-h-screen place-items-center">
        <img src="/logos/logo.svg" alt="Loading WyldeTrace" className="h-10 w-10 animate-pulse" />
      </div>
    );
  }
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function SubnetMapRedirect() {
  const { id } = useParams();
  return <Navigate to={`/subnets/${id}?tab=map`} replace />;
}

function BlockReportView({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();
  if (user?.role === "reportview") return <Navigate to="/" replace />;
  return <>{children}</>;
}

function AdminOnly({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();
  if (user?.role !== "admin") return <Navigate to="/settings/account" replace />;
  return <>{children}</>;
}

const inventory = (element: React.ReactNode) => <BlockReportView>{element}</BlockReportView>;
const admin = (element: React.ReactNode) => <AdminOnly>{element}</AdminOnly>;

const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  {
    path: "/",
    element: (
      <Protected>
        <AppShell />
      </Protected>
    ),
    children: [
      { index: true, element: <OpsDashboardPage /> },
      { path: "dashboard", element: <Navigate to="/" replace /> },
      { path: "reports", element: <Navigate to="/" replace /> },

      // Network
      { path: "subnets", element: inventory(<SubnetsPage />) },
      { path: "subnets/:id", element: inventory(<SubnetDetailPage />) },
      { path: "subnets/:id/map", element: inventory(<SubnetMapRedirect />) },
      { path: "locations", element: inventory(<LocationsPage />) },
      { path: "locations/:id", element: inventory(<LocationDetailPage />) },
      { path: "vrfs", element: inventory(<VRFsPage />) },
      { path: "vlans", element: inventory(<VLANsPage />) },
      { path: "device-types", element: inventory(<DeviceTypesPage />) },
      { path: "search", element: inventory(<SearchPage />) },

      // Discovery
      { path: "inbox", element: inventory(<InboxPage />) },
      { path: "discovery", element: inventory(<DiscoveryJobsPage />) },
      { path: "discover-subnets", element: inventory(<DiscoverSubnetsPage />) },

      { path: "requests", element: <RequestsPage /> },

      // Activity
      { path: "ip-history", element: inventory(<IPHistoryPage />) },
      { path: "audit", element: inventory(<AuditPage />) },
      { path: "deleted-ips", element: inventory(<DeletedIPsPage />) },
      { path: "deleted-subnets", element: inventory(<DeletedSubnetsPage />) },

      // Settings
      {
        path: "settings",
        element: <SettingsLayout />,
        children: [
          { index: true, element: <Navigate to="/settings/account" replace /> },
          { path: "account", element: <SettingsPage section="account" /> },
          { path: "general", element: admin(<SettingsPage section="general" />) },
          { path: "notifications", element: admin(<SettingsPage section="notifications" />) },
          { path: "integrations", element: admin(<SettingsPage section="integrations" />) },
          { path: "data", element: admin(<SettingsPage section="data" />) },
          { path: "users", element: admin(<UsersPage />) },
          { path: "tokens", element: admin(<TokensPage />) },
          { path: "webhooks", element: admin(<WebhooksPage />) },
          { path: "custom-fields", element: admin(<CustomFieldsPage />) },
        ],
      },
      // Pre-redesign admin URLs.
      { path: "users", element: <Navigate to="/settings/users" replace /> },
      { path: "tokens", element: <Navigate to="/settings/tokens" replace /> },
      { path: "webhooks", element: <Navigate to="/settings/webhooks" replace /> },
      { path: "custom-fields", element: <Navigate to="/settings/custom-fields" replace /> },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
]);

export default function App() {
  return <RouterProvider router={router} />;
}
