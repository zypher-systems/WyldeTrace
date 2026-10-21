package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
	"github.com/wyldetechnologies/wyldetrace/internal/mail"
	"github.com/wyldetechnologies/wyldetrace/internal/scope"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

type reportUtilRow struct {
	ID      uuid.UUID `json:"id"`
	CIDR    string    `json:"cidr"`
	Name    string    `json:"name"`
	Used    int       `json:"used"`
	Total   int       `json:"total"`
	Free    int       `json:"free"`
	Percent float64   `json:"percent"`
}

type reportSiteUtilRow struct {
	LocationID *uuid.UUID `json:"location_id"`
	Name       string     `json:"name"`
	Subnets    int        `json:"subnets"`
	Used       int        `json:"used"`
	Total      int        `json:"total"`
	Free       int        `json:"free"`
	Percent    float64    `json:"percent"`
}

type reportJobRow struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	SubnetID   uuid.UUID  `json:"subnet_id"`
	Enabled    bool       `json:"enabled"`
	LastRunAt  *time.Time `json:"last_run_at"`
	LastStatus string     `json:"last_status"`
	Alive      any        `json:"alive,omitempty"`
	NewAlive   any        `json:"new_alive,omitempty"`
}

type reportsSummary struct {
	GeneratedAt time.Time `json:"generated_at"`
	SiteScoped  bool      `json:"site_scoped"`
	SiteCount   int       `json:"site_count"`
	Subnets     struct {
		Total        int `json:"total"`
		WithLocation int `json:"with_location"`
	} `json:"subnets"`
	IPs struct {
		Total    int            `json:"total"`
		ByStatus map[string]int `json:"by_status"`
		WithMAC  int            `json:"with_mac"`
	} `json:"ips"`
	UtilizationTop []reportUtilRow     `json:"utilization_top"`
	BySite         []reportSiteUtilRow `json:"by_site"`
	Discovery      struct {
		Jobs      int            `json:"jobs"`
		Enabled   int            `json:"enabled"`
		RecentOK  int            `json:"recent_ok"`
		RecentErr int            `json:"recent_error"`
		Recent    []reportJobRow `json:"recent"`
		LastRunAt *time.Time     `json:"last_run_at,omitempty"`
	} `json:"discovery"`
	Inbox struct {
		OpenTotal    int        `json:"open_total"`
		OpenNewAlive int        `json:"open_new_alive"`
		OpenStale    int        `json:"open_stale"`
		LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	} `json:"inbox"`
}

func (a *API) ReportsSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := a.buildReportsSummaryFiltered(r)
	if err != nil {
		if err.Error() == "unauthorized" {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err.Error() == "forbidden" {
			httpx.Error(w, http.StatusForbidden, "location out of scope")
			return
		}
		if err.Error() == "bad location" {
			httpx.Error(w, http.StatusBadRequest, "invalid location_id")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, summary)
}

func (a *API) DispatchReportsSummary(w http.ResponseWriter, r *http.Request) {
	if a.Hooks == nil && a.Mail == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "notification channels unavailable")
		return
	}
	payload, err := a.unrestrictedReportsPayload(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if a.Hooks != nil {
		a.Hooks.Dispatch(r.Context(), webhook.EventReportsSummary, payload)
	}
	if a.Mail != nil {
		a.Mail.SendAlert(r.Context(), mail.AlertData{
			Event:     webhook.EventReportsSummary,
			Title:     "Reports summary",
			Body:      "Manual IPAM reports summary dispatch.",
			EntityKey: "reports",
		})
	}
	a.audit(r.Context(), "reports.summary.dispatch", "reports", "summary", map[string]any{"manual": true})
	httpx.JSON(w, http.StatusAccepted, map[string]any{"dispatched": true, "event": webhook.EventReportsSummary})
}

func (a *API) unrestrictedReportsPayload(ctx context.Context) (map[string]any, error) {
	summary, err := a.buildReportsSummaryScoped(ctx, scope.Scope{Unrestricted: true})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *API) ExportReportsSummaryCSV(w http.ResponseWriter, r *http.Request) {
	summary, err := a.buildReportsSummaryFiltered(r)
	if err != nil {
		if err.Error() == "unauthorized" {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err.Error() == "forbidden" {
			httpx.Error(w, http.StatusForbidden, "location out of scope")
			return
		}
		if err.Error() == "bad location" {
			httpx.Error(w, http.StatusBadRequest, "invalid location_id")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="wyldetrace-reports.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"section", "key", "value"})
	writeKV := func(section, key, value string) {
		_ = cw.Write([]string{section, key, value})
	}
	writeKV("meta", "generated_at", summary.GeneratedAt.UTC().Format(time.RFC3339))
	writeKV("meta", "site_scoped", strconv.FormatBool(summary.SiteScoped))
	writeKV("inventory", "subnets_total", strconv.Itoa(summary.Subnets.Total))
	writeKV("inventory", "subnets_with_location", strconv.Itoa(summary.Subnets.WithLocation))
	writeKV("inventory", "ips_total", strconv.Itoa(summary.IPs.Total))
	writeKV("inventory", "ips_with_mac", strconv.Itoa(summary.IPs.WithMAC))
	for status, n := range summary.IPs.ByStatus {
		writeKV("ip_status", status, strconv.Itoa(n))
	}
	writeKV("discovery", "jobs", strconv.Itoa(summary.Discovery.Jobs))
	writeKV("discovery", "enabled", strconv.Itoa(summary.Discovery.Enabled))
	writeKV("discovery", "recent_ok", strconv.Itoa(summary.Discovery.RecentOK))
	writeKV("discovery", "recent_error", strconv.Itoa(summary.Discovery.RecentErr))
	if summary.Discovery.LastRunAt != nil {
		writeKV("discovery", "last_run_at", summary.Discovery.LastRunAt.UTC().Format(time.RFC3339))
	}
	writeKV("inbox", "open_total", strconv.Itoa(summary.Inbox.OpenTotal))
	writeKV("inbox", "open_new_alive", strconv.Itoa(summary.Inbox.OpenNewAlive))
	writeKV("inbox", "open_stale", strconv.Itoa(summary.Inbox.OpenStale))
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"utilization_top", "cidr", "name", "used", "total", "free", "percent"})
	for _, u := range summary.UtilizationTop {
		_ = cw.Write([]string{
			"utilization_top", u.CIDR, u.Name,
			strconv.Itoa(u.Used), strconv.Itoa(u.Total), strconv.Itoa(u.Free),
			fmt.Sprintf("%.1f", u.Percent),
		})
	}
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"by_site", "location", "subnets", "used", "total", "free", "percent"})
	for _, s := range summary.BySite {
		_ = cw.Write([]string{
			"by_site", s.Name,
			strconv.Itoa(s.Subnets), strconv.Itoa(s.Used), strconv.Itoa(s.Total),
			strconv.Itoa(s.Free), fmt.Sprintf("%.1f", s.Percent),
		})
	}
	cw.Flush()
}

func (a *API) buildReportsSummary(ctx context.Context) (reportsSummary, error) {
	sc, err := scope.Load(ctx, a.Store)
	if err != nil {
		return reportsSummary{}, fmt.Errorf("unauthorized")
	}
	return a.buildReportsSummaryScoped(ctx, sc)
}

func (a *API) buildReportsSummaryFiltered(r *http.Request) (reportsSummary, error) {
	sc, err := scope.Load(r.Context(), a.Store)
	if err != nil {
		return reportsSummary{}, fmt.Errorf("unauthorized")
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("location_id")); raw != "" {
		lid, err := uuid.Parse(raw)
		if err != nil {
			return reportsSummary{}, fmt.Errorf("bad location")
		}
		if !sc.Allows(&lid) {
			return reportsSummary{}, fmt.Errorf("forbidden")
		}
		sc = scope.Scope{Unrestricted: false, LocationIDs: []uuid.UUID{lid}}
	}
	return a.buildReportsSummaryScoped(r.Context(), sc)
}

func (a *API) buildReportsSummaryScoped(ctx context.Context, sc scope.Scope) (reportsSummary, error) {
	subnets, err := a.Store.Queries.ListSubnets(ctx)
	if err != nil {
		return reportsSummary{}, err
	}
	counts, _ := a.Store.Queries.ListSubnetIPCounts(ctx)
	usedBy := map[uuid.UUID]int{}
	for _, c := range counts {
		usedBy[c.SubnetID] = int(c.UsedCount)
	}
	locs, _ := a.Store.Queries.ListLocations(ctx)
	locName := map[uuid.UUID]string{}
	for _, l := range locs {
		locName[l.ID] = l.Name
	}

	summary := reportsSummary{
		GeneratedAt: time.Now().UTC(),
		SiteScoped:  !sc.Unrestricted,
		SiteCount:   len(sc.LocationIDs),
	}
	summary.IPs.ByStatus = map[string]int{}
	utils := make([]reportUtilRow, 0)
	allowedSubnets := map[uuid.UUID]bool{}
	type siteAgg struct {
		id      pgtype.UUID
		subnets int
		used    int
		total   int
	}
	bySite := map[string]*siteAgg{}

	for _, s := range subnets {
		if !sc.AllowsNullUUID(s.LocationID) {
			continue
		}
		allowedSubnets[s.ID] = true
		summary.Subnets.Total++
		if s.LocationID.Valid {
			summary.Subnets.WithLocation++
		}
		siteKey := "unassigned"
		if lid := db.NullUUID(s.LocationID); lid != nil {
			siteKey = lid.String()
		}
		agg := bySite[siteKey]
		if agg == nil {
			agg = &siteAgg{id: s.LocationID}
			bySite[siteKey] = agg
		}
		agg.subnets++
		if prefix, err := ipam.ParsePrefix(s.Cidr); err == nil {
			// Utilization % only for dense prefixes with a meaningful denominator.
			if ipam.IsSparsePrefix(prefix) {
				continue
			}
			total, uerr := ipam.TotalUsable(prefix)
			if uerr != nil {
				continue
			}
			used := usedBy[s.ID]
			free := total - used
			if free < 0 {
				free = 0
			}
			pct := 0.0
			if total > 0 {
				pct = float64(used) / float64(total) * 100
			}
			utils = append(utils, reportUtilRow{
				ID: s.ID, CIDR: s.Cidr, Name: s.Name,
				Used: used, Total: total, Free: free, Percent: pct,
			})
			agg.used += used
			agg.total += total
		}
	}
	sort.Slice(utils, func(i, j int) bool { return utils[i].Percent > utils[j].Percent })
	if len(utils) > 10 {
		utils = utils[:10]
	}
	summary.UtilizationTop = utils

	siteRows := make([]reportSiteUtilRow, 0, len(bySite))
	for _, agg := range bySite {
		free := agg.total - agg.used
		if free < 0 {
			free = 0
		}
		pct := 0.0
		if agg.total > 0 {
			pct = float64(agg.used) / float64(agg.total) * 100
		}
		name := "Unassigned"
		locID := db.NullUUID(agg.id)
		if locID != nil {
			if n, ok := locName[*locID]; ok {
				name = n
			} else {
				name = locID.String()
			}
		}
		siteRows = append(siteRows, reportSiteUtilRow{
			LocationID: locID, Name: name, Subnets: agg.subnets,
			Used: agg.used, Total: agg.total, Free: free, Percent: pct,
		})
	}
	sort.Slice(siteRows, func(i, j int) bool { return siteRows[i].Percent > siteRows[j].Percent })
	summary.BySite = siteRows

	ips, err := a.Store.Queries.ListIPs(ctx)
	if err != nil {
		return reportsSummary{}, err
	}
	for _, ip := range ips {
		if !allowedSubnets[ip.SubnetID] {
			continue
		}
		summary.IPs.Total++
		summary.IPs.ByStatus[ip.Status]++
		if ip.MacAddress != "" {
			summary.IPs.WithMAC++
		}
	}

	jobs, err := a.Store.Queries.ListDiscoveryJobs(ctx, sqlcdb.ListDiscoveryJobsParams{
		Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(),
	})
	if err == nil {
		recent := make([]reportJobRow, 0)
		for _, j := range jobs {
			if !allowedSubnets[j.SubnetID] {
				continue
			}
			summary.Discovery.Jobs++
			if j.Enabled {
				summary.Discovery.Enabled++
			}
			switch j.LastStatus {
			case "ok":
				summary.Discovery.RecentOK++
			case "error":
				summary.Discovery.RecentErr++
			}
			row := reportJobRow{
				ID: j.ID, Name: j.Name, SubnetID: j.SubnetID, Enabled: j.Enabled,
				LastRunAt: timestamptzPtr(j.LastRunAt), LastStatus: j.LastStatus,
			}
			var m map[string]any
			if len(j.LastSummary) > 0 {
				_ = json.Unmarshal(j.LastSummary, &m)
				if m != nil {
					row.Alive = m["alive"]
					row.NewAlive = m["new_alive"]
				}
			}
			recent = append(recent, row)
			if t := timestamptzPtr(j.LastRunAt); t != nil {
				if summary.Discovery.LastRunAt == nil || t.After(*summary.Discovery.LastRunAt) {
					summary.Discovery.LastRunAt = t
				}
			}
		}
		sort.Slice(recent, func(i, j int) bool {
			ai, aj := recent[i].LastRunAt, recent[j].LastRunAt
			if ai == nil && aj == nil {
				return recent[i].Name < recent[j].Name
			}
			if ai == nil {
				return false
			}
			if aj == nil {
				return true
			}
			return ai.After(*aj)
		})
		if len(recent) > 8 {
			recent = recent[:8]
		}
		summary.Discovery.Recent = recent
	}

	findings, err := a.Store.Queries.ListOpenFindings(ctx, sqlcdb.ListOpenFindingsParams{
		Kind: "", RowLimit: 5000, RowOffset: 0,
		Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(),
	})
	if err == nil {
		for _, f := range findings {
			if !allowedSubnets[f.SubnetID] {
				continue
			}
			summary.Inbox.OpenTotal++
			switch f.Kind {
			case "new_alive":
				summary.Inbox.OpenNewAlive++
			case "stale":
				summary.Inbox.OpenStale++
			}
			if t := timestamptzPtr(f.LastSeenAt); t != nil {
				if summary.Inbox.LastSeenAt == nil || t.After(*summary.Inbox.LastSeenAt) {
					summary.Inbox.LastSeenAt = t
				}
			}
		}
	}

	return summary, nil
}
