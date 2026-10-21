package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/kea"
	"github.com/wyldetechnologies/wyldetrace/internal/powerdns"
	"github.com/wyldetechnologies/wyldetrace/internal/snmp"
)

const (
	settingPowerDNSEnabled = "powerdns_enabled"
	settingPowerDNSURL     = "powerdns_url"
	settingPowerDNSAPIKey  = "powerdns_api_key"
	settingPowerDNSDryRun  = "powerdns_dry_run"
	settingPowerDNSServer  = "powerdns_server"
	settingKeaEnabled      = "kea_enabled"
	settingKeaURL          = "kea_url"
	settingKeaDryRun       = "kea_dry_run"
	settingSNMPEnabled     = "snmp_enabled"
	settingSNMPCommunity   = "snmp_community"
	settingSNMPVersion     = "snmp_version"
	settingSNMPTimeout     = "snmp_timeout_seconds"
)

func (a *API) settingBool(ctx context.Context, key string, fallback bool) bool {
	row, err := a.Store.Queries.GetSetting(ctx, key)
	if err != nil {
		return fallback
	}
	v := strings.TrimSpace(strings.ToLower(row.Value))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (a *API) settingString(ctx context.Context, key, fallback string) string {
	row, err := a.Store.Queries.GetSetting(ctx, key)
	if err != nil {
		return fallback
	}
	return row.Value
}

func (a *API) settingSecret(ctx context.Context, key, fallback string) string {
	raw := a.settingString(ctx, key, fallback)
	plain, err := a.openSecret(raw)
	if err != nil {
		log.Printf("secretbox: open %s: %v", key, err)
		return ""
	}
	return plain
}

func (a *API) powerDNSClient(ctx context.Context) *powerdns.Client {
	return &powerdns.Client{
		BaseURL:  strings.TrimSpace(a.settingString(ctx, settingPowerDNSURL, "")),
		APIKey:   a.settingSecret(ctx, settingPowerDNSAPIKey, ""),
		ServerID: a.settingString(ctx, settingPowerDNSServer, "localhost"),
		DryRun:   a.settingBool(ctx, settingPowerDNSDryRun, true),
	}
}

func (a *API) keaClient(ctx context.Context) *kea.Client {
	return &kea.Client{
		URL:    strings.TrimSpace(a.settingString(ctx, settingKeaURL, "")),
		DryRun: a.settingBool(ctx, settingKeaDryRun, true),
	}
}

func (a *API) logIntegration(ctx context.Context, provider, action, status string, code int, dryRun bool, reqBody []byte, respBody, errMsg string) {
	if len(reqBody) == 0 {
		reqBody = []byte(`{}`)
	}
	_, err := a.Store.Queries.CreateIntegrationDelivery(ctx, sqlcdb.CreateIntegrationDeliveryParams{
		Provider:     provider,
		Action:       action,
		Status:       status,
		StatusCode:   int32(code),
		Error:        errMsg,
		DryRun:       dryRun,
		RequestBody:  reqBody,
		ResponseBody: respBody,
	})
	if err != nil {
		log.Printf("integrations: log %s/%s: %v", provider, action, err)
	}
}

// syncDNSForIP writes A/AAAA + PTR on create/update, or deletes on remove.
// Runs asynchronously; failures are logged to integration_deliveries.
func (a *API) syncDNSForIP(ctx context.Context, action, hostname, address string) {
	if !a.settingBool(ctx, settingPowerDNSEnabled, false) {
		return
	}
	hostname = strings.TrimSpace(hostname)
	addr, err := netip.ParseAddr(address)
	if err != nil || (hostname == "" && action != "delete") {
		return
	}
	// Capture settings for async path; re-validate URL before any request.
	pdnsURL := strings.TrimSpace(a.settingString(ctx, settingPowerDNSURL, ""))
	client := a.powerDNSClient(ctx)
	go func() {
		bg := context.Background()
		if err := a.validateIntegrationURL(pdnsURL); err != nil {
			a.finishDNS(bg, action, powerdns.Result{}, err)
			return
		}
		// Re-read client in case settings changed; still check URL from settings.
		liveURL := strings.TrimSpace(a.settingString(bg, settingPowerDNSURL, ""))
		if err := a.validateIntegrationURL(liveURL); err != nil {
			a.finishDNS(bg, action, powerdns.Result{}, err)
			return
		}
		client = a.powerDNSClient(bg)
		switch action {
		case "delete":
			if hostname != "" {
				res, err := client.DeleteForward(bg, hostname, addr)
				a.finishDNS(bg, "delete-forward", res, err)
			}
			res, err := client.DeletePTR(bg, addr)
			a.finishDNS(bg, "delete-ptr", res, err)
		default:
			if hostname == "" {
				return
			}
			res, err := client.UpsertForward(bg, hostname, addr, 300)
			a.finishDNS(bg, "upsert-forward", res, err)
			res, err = client.UpsertPTR(bg, addr, hostname, 300)
			a.finishDNS(bg, "upsert-ptr", res, err)
		}
	}()
}

func (a *API) finishDNS(ctx context.Context, action string, res powerdns.Result, err error) {
	status := "ok"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
		if res.Error != "" {
			errMsg = res.Error
		}
		log.Printf("powerdns %s: %v", action, err)
	} else if res.DryRun {
		status = "dry-run"
	}
	a.logIntegration(ctx, "powerdns", action, status, res.StatusCode, res.DryRun, res.RequestBody, res.ResponseBody, errMsg)
}

// SyncKea pushes DHCP pools (ip_ranges role=dhcp) and reserved/assigned hosts to Kea.
// Site-scoped callers only sync ranges/IPs in their locations.
func (a *API) SyncKea(w http.ResponseWriter, r *http.Request) {
	if !a.settingBool(r.Context(), settingKeaEnabled, false) {
		httpx.Error(w, http.StatusBadRequest, "kea integration disabled")
		return
	}
	url := strings.TrimSpace(a.settingString(r.Context(), settingKeaURL, ""))
	if url == "" {
		httpx.Error(w, http.StatusBadRequest, "kea_url not configured")
		return
	}
	if err := a.validateIntegrationURL(url); err != nil {
		httpx.Error(w, http.StatusBadRequest, "kea_url: "+err.Error())
		return
	}
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}

	ranges, err := a.Store.Queries.ListIPRangesByRole(r.Context(), "dhcp")
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	ips, err := a.Store.Queries.ListReservedOrAssignedIPs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	subnets, err := a.Store.Queries.ListSubnets(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	allowedSubnet := map[uuid.UUID]bool{}
	cidrByID := map[uuid.UUID]string{}
	for _, s := range subnets {
		if !sc.AllowsNullUUID(s.LocationID) {
			continue
		}
		allowedSubnet[s.ID] = true
		cidrByID[s.ID] = s.Cidr
	}

	keaRanges := make([]kea.DHCPRange, 0, len(ranges))
	for _, rng := range ranges {
		if !allowedSubnet[rng.SubnetID] {
			continue
		}
		keaRanges = append(keaRanges, kea.DHCPRange{
			SubnetCIDR: cidrByID[rng.SubnetID],
			PoolStart:  rng.StartAddress,
			PoolEnd:    rng.EndAddress,
		})
	}
	reservations := make([]kea.Reservation, 0, len(ips))
	for _, ip := range ips {
		if !allowedSubnet[ip.SubnetID] {
			continue
		}
		if ip.Status != "reserved" && ip.Status != "assigned" {
			continue
		}
		if ip.MacAddress == "" {
			continue
		}
		reservations = append(reservations, kea.Reservation{
			IPAddress: ip.Address,
			HWAddress: ip.MacAddress,
			Hostname:  ip.Hostname,
		})
	}

	client := a.keaClient(r.Context())
	res, err := client.SyncPools(r.Context(), keaRanges, reservations)
	status := "ok"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
	} else if res.DryRun {
		status = "dry-run"
	}
	a.logIntegration(r.Context(), "kea", "sync", status, res.StatusCode, res.DryRun, res.RequestBody, res.ResponseBody, errMsg)
	a.audit(r.Context(), "integration.kea.sync", "kea", "", map[string]any{
		"pools": len(keaRanges), "reservations": len(reservations), "status": status, "dry_run": res.DryRun,
		"site_scoped": !sc.Unrestricted,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, errMsg)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status": status, "dry_run": res.DryRun, "pools": len(keaRanges), "reservations": len(reservations),
	})
}

// SNMPEnrich walks a switch via SNMP and enriches matching IPs/findings in scope.
func (a *API) SNMPEnrich(w http.ResponseWriter, r *http.Request) {
	if !a.settingBool(r.Context(), settingSNMPEnabled, false) {
		httpx.Error(w, http.StatusBadRequest, "snmp integration disabled")
		return
	}
	var req struct {
		Target    string `json:"target"`
		Community string `json:"community"`
		Version   string `json:"version"`
	}
	if err := httpx.Decode(r, &req); err != nil || strings.TrimSpace(req.Target) == "" {
		httpx.Error(w, http.StatusBadRequest, "target required")
		return
	}
	if err := validateSNMPTarget(req.Target); err != nil {
		httpx.Error(w, http.StatusBadRequest, "target: "+err.Error())
		return
	}
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	community := strings.TrimSpace(req.Community)
	if community == "" {
		community = a.settingSecret(r.Context(), settingSNMPCommunity, "public")
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = a.settingString(r.Context(), settingSNMPVersion, "2c")
	}
	timeoutSec, _ := strconv.Atoi(a.settingString(r.Context(), settingSNMPTimeout, "5"))
	if timeoutSec <= 0 {
		timeoutSec = 5
	}

	walker := snmp.New()
	snap, err := walker.Walk(r.Context(), req.Target, snmp.Options{
		Community: community,
		Version:   version,
		Timeout:   time.Duration(timeoutSec) * time.Second,
	})
	if err != nil {
		a.logIntegration(r.Context(), "snmp", "enrich", "error", 0, false,
			mustJSON(map[string]any{"target": req.Target}), "", err.Error())
		httpx.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	switchID := snap.SysName
	if switchID == "" {
		switchID = req.Target
	}

	ipByAddr := map[string]uuid.UUID{}
	allIPs, _ := a.Store.Queries.ListIPs(r.Context())
	for _, ip := range allIPs {
		if !sc.AllowsNullUUID(ip.LocationID) {
			continue
		}
		ipByAddr[ip.Address] = ip.ID
	}

	updatedIPs := 0
	for _, arp := range snap.ARP {
		id, ok := ipByAddr[arp.IP]
		if !ok {
			continue
		}
		ifName := arp.IfName
		if err := a.Store.Queries.UpdateIPEnrichment(r.Context(), sqlcdb.UpdateIPEnrichmentParams{
			ID: id, InterfaceName: ifName, SwitchID: switchID, MacAddress: arp.MAC,
		}); err == nil {
			updatedIPs++
		}
	}

	findingsUpdated := 0
	findings, _ := a.Store.Queries.ListOpenFindings(r.Context(), sqlcdb.ListOpenFindingsParams{
		Kind: "", Scoped: !sc.Unrestricted, LocationIds: sc.LocationIDsOrEmpty(), RowLimit: 5000, RowOffset: 0,
	})
	findingByAddr := map[string]uuid.UUID{}
	for _, f := range findings {
		findingByAddr[f.Address] = f.ID
	}
	for _, arp := range snap.ARP {
		fid, ok := findingByAddr[arp.IP]
		if !ok {
			continue
		}
		if err := a.Store.Queries.UpdateFindingEnrichment(r.Context(), sqlcdb.UpdateFindingEnrichmentParams{
			ID: fid, InterfaceName: arp.IfName, SwitchID: switchID, MacAddress: arp.MAC,
		}); err == nil {
			findingsUpdated++
		}
	}

	a.logIntegration(r.Context(), "snmp", "enrich", "ok", 200, false,
		mustJSON(map[string]any{"target": req.Target, "switch_id": switchID}),
		mustJSONString(map[string]any{"arp": len(snap.ARP), "interfaces": len(snap.Interfaces), "lldp": len(snap.LLDP)}),
		"")
	a.audit(r.Context(), "integration.snmp.enrich", "snmp", req.Target, map[string]any{
		"switch_id": switchID, "ips_updated": updatedIPs, "findings_updated": findingsUpdated,
		"site_scoped": !sc.Unrestricted,
	})
	httpx.JSON(w, http.StatusOK, map[string]any{
		"switch_id":        switchID,
		"interfaces":       len(snap.Interfaces),
		"arp_entries":      len(snap.ARP),
		"lldp_neighbors":   len(snap.LLDP),
		"ips_updated":      updatedIPs,
		"findings_updated": findingsUpdated,
	})
}

func (a *API) ListIntegrationDeliveries(w http.ResponseWriter, r *http.Request) {
	pq := httpx.ParsePage(r)
	provider := r.URL.Query().Get("provider")
	total, err := a.Store.Queries.CountIntegrationDeliveries(r.Context(), provider)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListIntegrationDeliveries(r.Context(), sqlcdb.ListIntegrationDeliveriesParams{
		Provider: provider, RowLimit: pq.PageSize, RowOffset: pq.Offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[sqlcdb.IntegrationDelivery]{
		Items: items, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return []byte(`{}`)
	}
	return b
}

func mustJSONString(v any) string {
	return string(mustJSON(v))
}
