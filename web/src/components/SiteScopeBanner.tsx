import { useAuth } from "../auth";

export function SiteScopeBanner() {
  const { user } = useAuth();
  if (!user?.site_scoped) return null;
  const n = user.location_ids?.length ?? 0;
  return (
    <div className="border-b border-sky-900/40 bg-sky-950/25 px-4 py-2 text-center text-sm text-sky-200">
      Site-scoped view · {n} assigned location{n === 1 ? "" : "s"} (unassigned subnets hidden)
    </div>
  );
}
