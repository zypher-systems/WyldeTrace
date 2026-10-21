package handlers

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/scope"
)

func importVRFID(r *http.Request) uuid.UUID {
	if raw := strings.TrimSpace(r.URL.Query().Get("vrf_id")); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			return id
		}
	}
	return db.DefaultVRFID
}

// ImportDHCPLeasesCSV accepts CSV: address,mac,hostname[,status]
// IPs are matched to the tightest containing subnet and upserted as dhcp/assigned.
func (a *API) ImportDHCPLeasesCSV(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "read failed")
		return
	}
	cr := csv.NewReader(strings.NewReader(string(body)))
	cr.TrimLeadingSpace = true
	rows, err := cr.ReadAll()
	if err != nil || len(rows) == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid csv")
		return
	}
	start := 0
	idx := map[string]int{}
	if looksLikeHeader(rows[0], "address", "ip") {
		for i, h := range rows[0] {
			idx[strings.ToLower(strings.TrimSpace(h))] = i
		}
		start = 1
	} else {
		idx["address"] = 0
		idx["mac"] = 1
		idx["hostname"] = 2
		idx["status"] = 3
	}
	sc, err := scope.Load(r.Context(), a.Store)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	summary := importSummary{Errors: []string{}}
	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	for line := start; line < len(rows); line++ {
		row := rows[line]
		addrStr := col(row, idx, "address")
		if addrStr == "" {
			addrStr = col(row, idx, "ip")
		}
		addr, err := netip.ParseAddr(strings.TrimSpace(addrStr))
		if err != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: bad address", line+1))
			continue
		}
		vrfID := importVRFID(r)
		subnet, err := a.Store.Queries.FindSubnetContainingIP(r.Context(), sqlcdb.FindSubnetContainingIPParams{
			VrfID: vrfID, Address: addr.String(),
		})
		if err != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: no subnet for %s", line+1, addr))
			continue
		}
		if !sc.AllowsNullUUID(subnet.LocationID) {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: out of scope", line+1))
			continue
		}
		hostname := col(row, idx, "hostname")
		if hostname == "" {
			hostname = col(row, idx, "name")
		}
		mac := col(row, idx, "mac")
		if mac == "" {
			mac = col(row, idx, "mac_address")
		}
		status := col(row, idx, "status")
		if status == "" {
			status = "dhcp"
		}
		if !validIPStatus[status] {
			status = "dhcp"
		}
		existing, err := a.Store.Queries.GetIPByAddressAny(r.Context(), sqlcdb.GetIPByAddressAnyParams{
			Address: addr.String(), VrfID: subnet.VrfID,
		})
		if err == nil {
			if existing.DeletedAt.Valid {
				_, rerr := a.Store.Queries.RestoreIP(r.Context(), sqlcdb.RestoreIPParams{
					ID: existing.ID, SubnetID: subnet.ID, VrfID: subnet.VrfID, Hostname: firstNonEmpty(hostname, existing.Hostname),
					Status: status, DeviceTypeID: existing.DeviceTypeID, LocationID: subnet.LocationID,
					Notes: mergeNotes(existing.Notes, "imported from dhcp"), ReservedUntil: existing.ReservedUntil,
					LastSeenAt: now, MacAddress: firstNonEmpty(mac, existing.MacAddress), OpenPorts: existing.OpenPorts,
					InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
					CustomFields: existing.CustomFields,
				})
				if rerr != nil {
					summary.Skipped++
					continue
				}
				summary.Updated++
				continue
			}
			_, uerr := a.Store.Queries.UpdateIP(r.Context(), sqlcdb.UpdateIPParams{
				ID: existing.ID, Address: existing.Address, SubnetID: subnet.ID, VrfID: subnet.VrfID,
				Hostname: firstNonEmpty(hostname, existing.Hostname), Status: status,
				DeviceTypeID: existing.DeviceTypeID, LocationID: subnet.LocationID,
				Notes: mergeNotes(existing.Notes, "imported from dhcp"), ReservedUntil: existing.ReservedUntil,
				LastSeenAt: now, MacAddress: firstNonEmpty(mac, existing.MacAddress), OpenPorts: existing.OpenPorts,
				InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
				CustomFields: existing.CustomFields,
			})
			if uerr != nil {
				summary.Skipped++
				continue
			}
			summary.Updated++
			continue
		}
		_, cerr := a.Store.Queries.CreateIP(r.Context(), sqlcdb.CreateIPParams{
			Address: addr.String(), SubnetID: subnet.ID, VrfID: subnet.VrfID, Hostname: hostname, Status: status,
			DeviceTypeID: pgtype.UUID{}, LocationID: subnet.LocationID,
			Notes: "imported from dhcp", ReservedUntil: pgtype.Timestamptz{}, LastSeenAt: now,
			MacAddress: mac, OpenPorts: "",
			CustomFields: emptyJSON(),
		})
		if cerr != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line+1, cerr))
			continue
		}
		summary.Created++
	}
	a.audit(r.Context(), "import.dhcp", "ip_address", "", map[string]any{
		"created": summary.Created, "updated": summary.Updated, "skipped": summary.Skipped,
	})
	httpx.JSON(w, http.StatusOK, summary)
}

// ImportDNSHosts accepts hosts-file lines or CSV hostname,address
func (a *API) ImportDNSHosts(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "read failed")
		return
	}
	sc, err := scope.Load(r.Context(), a.Store)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ct := r.Header.Get("Content-Type")
	summary := importSummary{Errors: []string{}}
	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}

	if strings.Contains(ct, "csv") || looksCSV(string(body)) {
		cr := csv.NewReader(strings.NewReader(string(body)))
		rows, err := cr.ReadAll()
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid csv")
			return
		}
		start := 0
		idx := map[string]int{"hostname": 0, "address": 1, "ip": 1}
		if len(rows) > 0 && looksLikeHeader(rows[0], "hostname", "name", "address", "ip") {
			idx = map[string]int{}
			for i, h := range rows[0] {
				idx[strings.ToLower(strings.TrimSpace(h))] = i
			}
			start = 1
		}
		for line := start; line < len(rows); line++ {
			row := rows[line]
			host := col(row, idx, "hostname")
			if host == "" {
				host = col(row, idx, "name")
			}
			addrStr := col(row, idx, "address")
			if addrStr == "" {
				addrStr = col(row, idx, "ip")
			}
			a.importDNSRow(r, sc, &summary, line+1, host, addrStr, now)
		}
	} else {
		scanner := bufio.NewScanner(strings.NewReader(string(body)))
		line := 0
		for scanner.Scan() {
			line++
			raw := strings.TrimSpace(scanner.Text())
			if raw == "" || strings.HasPrefix(raw, "#") {
				continue
			}
			fields := strings.Fields(raw)
			if len(fields) < 2 {
				summary.Skipped++
				continue
			}
			// hosts file: IP hostname [aliases...]
			addrStr := fields[0]
			host := fields[1]
			a.importDNSRow(r, sc, &summary, line, host, addrStr, now)
		}
	}
	a.audit(r.Context(), "import.dns", "ip_address", "", map[string]any{
		"created": summary.Created, "updated": summary.Updated, "skipped": summary.Skipped,
	})
	httpx.JSON(w, http.StatusOK, summary)
}

func (a *API) importDNSRow(r *http.Request, sc scope.Scope, summary *importSummary, line int, hostname, addrStr string, now pgtype.Timestamptz) {
	addr, err := netip.ParseAddr(strings.TrimSpace(addrStr))
	if err != nil {
		summary.Skipped++
		summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: bad address", line))
		return
	}
	vrfID := importVRFID(r)
	subnet, err := a.Store.Queries.FindSubnetContainingIP(r.Context(), sqlcdb.FindSubnetContainingIPParams{
		VrfID: vrfID, Address: addr.String(),
	})
	if err != nil {
		summary.Skipped++
		summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: no subnet for %s", line, addr))
		return
	}
	if !sc.AllowsNullUUID(subnet.LocationID) {
		summary.Skipped++
		summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: out of scope", line))
		return
	}
	existing, err := a.Store.Queries.GetIPByAddressAny(r.Context(), sqlcdb.GetIPByAddressAnyParams{
		Address: addr.String(), VrfID: subnet.VrfID,
	})
	if err == nil && !existing.DeletedAt.Valid {
		_, uerr := a.Store.Queries.UpdateIP(r.Context(), sqlcdb.UpdateIPParams{
			ID: existing.ID, Address: existing.Address, SubnetID: subnet.ID, VrfID: subnet.VrfID,
			Hostname: firstNonEmpty(hostname, existing.Hostname), Status: firstNonEmpty(existing.Status, "assigned"),
			DeviceTypeID: existing.DeviceTypeID, LocationID: subnet.LocationID,
			Notes: mergeNotes(existing.Notes, "imported from dns"), ReservedUntil: existing.ReservedUntil,
			LastSeenAt: now, MacAddress: existing.MacAddress, OpenPorts: existing.OpenPorts,
			InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
			CustomFields: existing.CustomFields,
		})
		if uerr != nil {
			summary.Skipped++
			return
		}
		summary.Updated++
		return
	}
	if err == nil && existing.DeletedAt.Valid {
		_, rerr := a.Store.Queries.RestoreIP(r.Context(), sqlcdb.RestoreIPParams{
			ID: existing.ID, SubnetID: subnet.ID, VrfID: subnet.VrfID, Hostname: firstNonEmpty(hostname, existing.Hostname),
			Status: "assigned", DeviceTypeID: existing.DeviceTypeID, LocationID: subnet.LocationID,
			Notes: "imported from dns", ReservedUntil: existing.ReservedUntil, LastSeenAt: now,
			MacAddress: existing.MacAddress, OpenPorts: existing.OpenPorts,
			InterfaceName: existing.InterfaceName, SwitchID: existing.SwitchID,
			CustomFields: existing.CustomFields,
		})
		if rerr != nil {
			summary.Skipped++
			return
		}
		summary.Updated++
		return
	}
	_, cerr := a.Store.Queries.CreateIP(r.Context(), sqlcdb.CreateIPParams{
		Address: addr.String(), SubnetID: subnet.ID, VrfID: subnet.VrfID, Hostname: hostname, Status: "assigned",
		DeviceTypeID: pgtype.UUID{}, LocationID: subnet.LocationID,
		Notes: "imported from dns", ReservedUntil: pgtype.Timestamptz{}, LastSeenAt: now,
		CustomFields: emptyJSON(),
	})
	if cerr != nil {
		summary.Skipped++
		summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, cerr))
		return
	}
	summary.Created++
}

func looksLikeHeader(row []string, needles ...string) bool {
	joined := strings.ToLower(strings.Join(row, ","))
	for _, n := range needles {
		if strings.Contains(joined, n) {
			return true
		}
	}
	return false
}

func looksCSV(s string) bool {
	first := strings.SplitN(s, "\n", 2)[0]
	return strings.Count(first, ",") >= 1 && !strings.Contains(first, " ")
}
