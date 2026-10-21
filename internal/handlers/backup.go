package handlers

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type backupPayload struct {
	Version     int                   `json:"version"`
	CreatedAt   time.Time             `json:"created_at"`
	Locations   []sqlcdb.Location     `json:"locations"`
	DeviceTypes []sqlcdb.DeviceType   `json:"device_types"`
	VRFs        []sqlcdb.Vrf          `json:"vrfs"`
	VLANs       []sqlcdb.Vlan         `json:"vlans"`
	Subnets     []backupSubnet        `json:"subnets"`
	IPRanges    []backupIPRange       `json:"ip_ranges"`
	IPs         []backupIP            `json:"ips"`
	IPRequests  []backupIPRequest     `json:"ip_requests"`
	Users       []backupUser          `json:"users"`
}

type backupUser struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	PasswordHash string `json:"password_hash"`
	// Never export totp_secret / recovery_codes.
}

type backupSubnet struct {
	ID          string  `json:"id"`
	CIDR        string  `json:"cidr"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	LocationID  *string `json:"location_id"`
	VrfID       string  `json:"vrf_id,omitempty"`
	VlanVID     *int32  `json:"vlan_id"` // VLAN tag (vid), not UUID FK
	ParentID    *string `json:"parent_id"`
}

type backupIP struct {
	ID           string  `json:"id"`
	Address      string  `json:"address"`
	SubnetID     string  `json:"subnet_id"`
	VrfID        string  `json:"vrf_id,omitempty"`
	Hostname     string  `json:"hostname"`
	Status       string  `json:"status"`
	DeviceTypeID *string `json:"device_type_id"`
	LocationID   *string `json:"location_id"`
	Notes        string  `json:"notes"`
	MACAddress   string  `json:"mac_address"`
	OpenPorts    string  `json:"open_ports"`
}

type backupIPRange struct {
	ID           string `json:"id"`
	SubnetID     string `json:"subnet_id"`
	Name         string `json:"name"`
	StartAddress string `json:"start_address"`
	EndAddress   string `json:"end_address"`
	Role         string `json:"role"`
	Description  string `json:"description"`
}

type backupIPRequest struct {
	ID               string  `json:"id"`
	RequesterID      string  `json:"requester_id"`
	SubnetID         string  `json:"subnet_id"`
	RangeID          *string `json:"range_id,omitempty"`
	Hostname         string  `json:"hostname"`
	Notes            string  `json:"notes"`
	RequestedAddress string  `json:"requested_address,omitempty"`
	Status           string  `json:"status"`
	ReviewNotes      string  `json:"review_notes,omitempty"`
}

func (a *API) ExportBackup(w http.ResponseWriter, r *http.Request) {
	locs, err := a.Store.Queries.ListLocations(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	dts, err := a.Store.Queries.ListDeviceTypes(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	vrfs, err := a.Store.Queries.ListVRFs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	vlans, err := a.Store.Queries.ListVLANs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	subs, err := a.Store.Queries.ListSubnets(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	ranges, err := a.Store.Queries.ListAllIPRanges(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	ips, err := a.Store.Queries.ListIPs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	reqs, err := a.Store.Queries.ListAllIPRequests(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	users, err := a.Store.Queries.ListUsers(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	vids := a.vlanVIDMap(r.Context())
	outSubs := make([]backupSubnet, 0, len(subs))
	for _, s := range subs {
		bs := backupSubnet{
			ID: s.ID.String(), CIDR: s.Cidr, Name: s.Name, Description: s.Description,
			VrfID: s.VrfID.String(),
		}
		if id := db.NullUUID(s.LocationID); id != nil {
			v := id.String()
			bs.LocationID = &v
		}
		if id := db.NullUUID(s.ParentID); id != nil {
			v := id.String()
			bs.ParentID = &v
		}
		if s.VlanID.Valid {
			if vid, ok := vids[uuid.UUID(s.VlanID.Bytes)]; ok {
				v := vid
				bs.VlanVID = &v
			}
		}
		outSubs = append(outSubs, bs)
	}
	outRanges := make([]backupIPRange, 0, len(ranges))
	for _, rng := range ranges {
		outRanges = append(outRanges, backupIPRange{
			ID: rng.ID.String(), SubnetID: rng.SubnetID.String(), Name: rng.Name,
			StartAddress: rng.StartAddress, EndAddress: rng.EndAddress,
			Role: rng.Role, Description: rng.Description,
		})
	}
	outIPs := make([]backupIP, 0, len(ips))
	for _, ip := range ips {
		bi := backupIP{
			ID: ip.ID.String(), Address: ip.Address, SubnetID: ip.SubnetID.String(),
			VrfID: ip.VrfID.String(), Hostname: ip.Hostname, Status: ip.Status, Notes: ip.Notes,
			MACAddress: ip.MacAddress, OpenPorts: ip.OpenPorts,
		}
		if id := db.NullUUID(ip.DeviceTypeID); id != nil {
			v := id.String()
			bi.DeviceTypeID = &v
		}
		if id := db.NullUUID(ip.LocationID); id != nil {
			v := id.String()
			bi.LocationID = &v
		}
		outIPs = append(outIPs, bi)
	}
	outReqs := make([]backupIPRequest, 0, len(reqs))
	for _, req := range reqs {
		br := backupIPRequest{
			ID: req.ID.String(), RequesterID: req.RequesterID.String(), SubnetID: req.SubnetID.String(),
			Hostname: req.Hostname, Notes: req.Notes, RequestedAddress: req.RequestedAddress,
			Status: req.Status, ReviewNotes: req.ReviewNotes,
		}
		if id := db.NullUUID(req.RangeID); id != nil {
			v := id.String()
			br.RangeID = &v
		}
		outReqs = append(outReqs, br)
	}
	outUsers := make([]backupUser, 0, len(users))
	for _, u := range users {
		outUsers = append(outUsers, backupUser{
			ID: u.ID.String(), Email: u.Email, Role: u.Role, PasswordHash: "", // never export hashes / MFA secrets
		})
	}

	payload := backupPayload{
		Version:     2,
		CreatedAt:   time.Now().UTC(),
		Locations:   locs,
		DeviceTypes: dts,
		VRFs:        vrfs,
		VLANs:       vlans,
		Subnets:     outSubs,
		IPRanges:    outRanges,
		IPs:         outIPs,
		IPRequests:  outReqs,
		Users:       outUsers,
	}
	a.audit(r.Context(), "backup.export", "system", "", map[string]any{
		"locations": len(locs), "subnets": len(subs), "ips": len(ips),
		"vrfs": len(vrfs), "vlans": len(vlans), "ip_ranges": len(ranges), "ip_requests": len(reqs),
	})
	w.Header().Set("Content-Disposition", "attachment; filename=wyldetrace-backup.json")
	httpx.JSON(w, http.StatusOK, payload)
}

type restoreDiff struct {
	WouldCreate map[string]int `json:"would_create"`
	WouldSkip   map[string]int `json:"would_skip"`
	InBackup    map[string]int `json:"in_backup"`
	Notes       []string       `json:"notes,omitempty"`
}

// PreviewRestoreBackup computes merge counts without writing.
func (a *API) PreviewRestoreBackup(w http.ResponseWriter, r *http.Request) {
	var payload backupPayload
	if err := httpx.Decode(r, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid backup json")
		return
	}
	httpx.JSON(w, http.StatusOK, a.computeRestoreDiff(r, payload))
}

// RestoreBackup merges inventory from a backup JSON.
// Requires header X-Confirm-Restore: true. Does not delete existing rows or restore users/MFA secrets.
func (a *API) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Confirm-Restore") != "true" {
		httpx.Error(w, http.StatusBadRequest, "set X-Confirm-Restore: true to proceed")
		return
	}
	var payload backupPayload
	if err := httpx.Decode(r, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid backup json")
		return
	}

	created := map[string]int{
		"locations": 0, "device_types": 0, "vrfs": 0, "vlans": 0,
		"subnets": 0, "ip_ranges": 0, "ips": 0, "ip_requests": 0,
	}
	locByName := map[string]uuid.UUID{}
	existingLocs, _ := a.Store.Queries.ListLocations(r.Context())
	for _, l := range existingLocs {
		locByName[l.Name] = l.ID
	}
	for _, l := range payload.Locations {
		if _, ok := locByName[l.Name]; ok {
			continue
		}
		item, err := a.Store.Queries.CreateLocation(r.Context(), sqlcdb.CreateLocationParams{
			Name: l.Name, Description: l.Description,
		})
		if err != nil {
			continue
		}
		locByName[item.Name] = item.ID
		created["locations"]++
	}

	dtByName := map[string]uuid.UUID{}
	existingDT, _ := a.Store.Queries.ListDeviceTypes(r.Context())
	for _, d := range existingDT {
		dtByName[d.Name] = d.ID
	}
	for _, d := range payload.DeviceTypes {
		if _, ok := dtByName[d.Name]; ok {
			continue
		}
		item, err := a.Store.Queries.CreateDeviceType(r.Context(), sqlcdb.CreateDeviceTypeParams{
			Name: d.Name, Description: d.Description,
		})
		if err != nil {
			continue
		}
		dtByName[item.Name] = item.ID
		created["device_types"]++
	}

	vrfByName := map[string]uuid.UUID{}
	existingVRFs, _ := a.Store.Queries.ListVRFs(r.Context())
	for _, v := range existingVRFs {
		vrfByName[v.Name] = v.ID
	}
	for _, v := range payload.VRFs {
		if v.ID == db.DefaultVRFID || v.Name == "Default" {
			vrfByName[v.Name] = db.DefaultVRFID
			continue
		}
		if _, ok := vrfByName[v.Name]; ok {
			continue
		}
		item, err := a.Store.Queries.CreateVRF(r.Context(), sqlcdb.CreateVRFParams{
			Name: v.Name, Rd: v.Rd, Description: v.Description,
		})
		if err != nil {
			continue
		}
		vrfByName[item.Name] = item.ID
		created["vrfs"]++
	}

	for _, v := range payload.VLANs {
		var locID *uuid.UUID
		if v.LocationID.Valid {
			if name := findLocationName(payload.Locations, uuid.UUID(v.LocationID.Bytes)); name != "" {
				if id, ok := locByName[name]; ok {
					locID = &id
				}
			}
		}
		if _, err := a.Store.Queries.GetVLANByVidLocation(r.Context(), sqlcdb.GetVLANByVidLocationParams{
			Vid: v.Vid, LocationID: db.PtrUUIDToNull(locID),
		}); err == nil {
			continue
		}
		if _, err := a.Store.Queries.CreateVLAN(r.Context(), sqlcdb.CreateVLANParams{
			Vid: v.Vid, Name: v.Name, LocationID: db.PtrUUIDToNull(locID), Description: v.Description,
		}); err == nil {
			created["vlans"]++
		}
	}

	subnetByCIDR := map[string]uuid.UUID{}
	subnetVrfByCIDR := map[string]uuid.UUID{}
	existingSubs, _ := a.Store.Queries.ListSubnets(r.Context())
	for _, s := range existingSubs {
		subnetByCIDR[s.Cidr] = s.ID
		subnetVrfByCIDR[s.Cidr] = s.VrfID
	}
	for _, s := range payload.Subnets {
		if _, ok := subnetByCIDR[s.CIDR]; ok {
			continue
		}
		var locID *uuid.UUID
		if s.LocationID != nil {
			if parsed, err := uuid.Parse(*s.LocationID); err == nil {
				if name := findLocationName(payload.Locations, parsed); name != "" {
					if id, ok := locByName[name]; ok {
						locID = &id
					}
				} else {
					locID = &parsed
				}
			}
		}
		vrfID := db.DefaultVRFID
		if s.VrfID != "" {
			if parsed, err := uuid.Parse(s.VrfID); err == nil {
				vrfID = parsed
				for _, v := range payload.VRFs {
					if v.ID == parsed {
						if id, ok := vrfByName[v.Name]; ok {
							vrfID = id
						}
						break
					}
				}
			}
		}
		vlanFK, _ := a.resolveVLANFK(r.Context(), s.VlanVID, db.PtrUUIDToNull(locID))
		item, err := a.Store.Queries.CreateSubnet(r.Context(), sqlcdb.CreateSubnetParams{
			Cidr: s.CIDR, Name: s.Name, Description: s.Description,
			LocationID: db.PtrUUIDToNull(locID), VrfID: vrfID, VlanID: vlanFK,
			Tags: []string{}, CustomFields: []byte(`{}`),
		})
		if err != nil {
			continue
		}
		subnetByCIDR[item.Cidr] = item.ID
		subnetVrfByCIDR[item.Cidr] = item.VrfID
		created["subnets"]++
	}

	for _, rng := range payload.IPRanges {
		cidr := findSubnetCIDR(payload.Subnets, rng.SubnetID)
		subnetID, ok := subnetByCIDR[cidr]
		if !ok {
			continue
		}
		existing, _ := a.Store.Queries.ListIPRangesBySubnet(r.Context(), subnetID)
		dup := false
		for _, e := range existing {
			if e.StartAddress == rng.StartAddress && e.EndAddress == rng.EndAddress {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		role := rng.Role
		if role == "" {
			role = "other"
		}
		if _, err := a.Store.Queries.CreateIPRange(r.Context(), sqlcdb.CreateIPRangeParams{
			SubnetID: subnetID, Name: rng.Name, StartAddress: rng.StartAddress,
			EndAddress: rng.EndAddress, Role: role, Description: rng.Description,
		}); err == nil {
			created["ip_ranges"]++
		}
	}

	for _, ip := range payload.IPs {
		cidr := findSubnetCIDR(payload.Subnets, ip.SubnetID)
		subnetID, ok := subnetByCIDR[cidr]
		if !ok {
			continue
		}
		vrfID := subnetVrfByCIDR[cidr]
		if vrfID == uuid.Nil {
			vrfID = db.DefaultVRFID
		}
		if ip.VrfID != "" {
			if parsed, err := uuid.Parse(ip.VrfID); err == nil {
				vrfID = parsed
			}
		}
		if _, err := a.Store.Queries.GetIPByAddress(r.Context(), sqlcdb.GetIPByAddressParams{
			Address: ip.Address, VrfID: vrfID,
		}); err == nil {
			continue
		}
		status := ip.Status
		if status == "" {
			status = "assigned"
		}
		_, err := a.Store.Queries.CreateIP(r.Context(), sqlcdb.CreateIPParams{
			Address: ip.Address, SubnetID: subnetID, VrfID: vrfID, Hostname: ip.Hostname, Status: status,
			Notes: ip.Notes, MacAddress: ip.MACAddress, OpenPorts: ip.OpenPorts,
			CustomFields: emptyJSON(),
		})
		if err == nil {
			created["ips"]++
		}
	}

	// IP requests: restore pending rows only when requester/subnet exist; skip secrets.
	for _, req := range payload.IPRequests {
		if req.Status != "" && req.Status != "pending" {
			continue
		}
		cidr := findSubnetCIDR(payload.Subnets, req.SubnetID)
		subnetID, ok := subnetByCIDR[cidr]
		if !ok {
			continue
		}
		requesterID, err := uuid.Parse(req.RequesterID)
		if err != nil {
			continue
		}
		if _, err := a.Store.Queries.GetUserByID(r.Context(), requesterID); err != nil {
			continue
		}
		var reqAddr *netip.Addr
		if req.RequestedAddress != "" {
			if parsed, perr := netip.ParseAddr(req.RequestedAddress); perr == nil {
				reqAddr = &parsed
			}
		}
		if _, err := a.Store.Queries.CreateIPRequest(r.Context(), sqlcdb.CreateIPRequestParams{
			RequesterID: requesterID, SubnetID: subnetID, Hostname: req.Hostname, Notes: req.Notes,
			RequestedAddress: reqAddr,
		}); err == nil {
			created["ip_requests"]++
		}
	}

	a.audit(r.Context(), "backup.restore", "system", "", map[string]any{"created": created})
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok", "created": created})
}

func (a *API) computeRestoreDiff(r *http.Request, payload backupPayload) restoreDiff {
	create := map[string]int{
		"locations": 0, "device_types": 0, "vrfs": 0, "vlans": 0,
		"subnets": 0, "ip_ranges": 0, "ips": 0, "ip_requests": 0,
	}
	skip := map[string]int{
		"locations": 0, "device_types": 0, "vrfs": 0, "vlans": 0,
		"subnets": 0, "ip_ranges": 0, "ips": 0, "ip_requests": 0,
	}
	inBackup := map[string]int{
		"locations": len(payload.Locations), "device_types": len(payload.DeviceTypes),
		"vrfs": len(payload.VRFs), "vlans": len(payload.VLANs),
		"subnets": len(payload.Subnets), "ip_ranges": len(payload.IPRanges),
		"ips": len(payload.IPs), "ip_requests": len(payload.IPRequests), "users": len(payload.Users),
	}
	notes := []string{
		"Users and MFA secrets (totp_secret/recovery) in the backup are never restored.",
		"Existing rows are skipped (merge only).",
	}

	locByName := map[string]struct{}{}
	if existingLocs, err := a.Store.Queries.ListLocations(r.Context()); err == nil {
		for _, l := range existingLocs {
			locByName[l.Name] = struct{}{}
		}
	}
	for _, l := range payload.Locations {
		if _, ok := locByName[l.Name]; ok {
			skip["locations"]++
		} else {
			create["locations"]++
		}
	}

	dtByName := map[string]struct{}{}
	if existingDT, err := a.Store.Queries.ListDeviceTypes(r.Context()); err == nil {
		for _, d := range existingDT {
			dtByName[d.Name] = struct{}{}
		}
	}
	for _, d := range payload.DeviceTypes {
		if _, ok := dtByName[d.Name]; ok {
			skip["device_types"]++
		} else {
			create["device_types"]++
		}
	}

	vrfByName := map[string]struct{}{}
	if existing, err := a.Store.Queries.ListVRFs(r.Context()); err == nil {
		for _, v := range existing {
			vrfByName[v.Name] = struct{}{}
		}
	}
	for _, v := range payload.VRFs {
		if v.Name == "Default" || v.ID == db.DefaultVRFID {
			skip["vrfs"]++
			continue
		}
		if _, ok := vrfByName[v.Name]; ok {
			skip["vrfs"]++
		} else {
			create["vrfs"]++
		}
	}

	create["vlans"] = len(payload.VLANs) // approximate; restore skips duplicates by vid+location

	subnetByCIDR := map[string]struct{}{}
	subnetVrf := map[string]uuid.UUID{}
	if existingSubs, err := a.Store.Queries.ListSubnets(r.Context()); err == nil {
		for _, s := range existingSubs {
			subnetByCIDR[s.Cidr] = struct{}{}
			subnetVrf[s.Cidr] = s.VrfID
		}
	}
	for _, s := range payload.Subnets {
		if _, ok := subnetByCIDR[s.CIDR]; ok {
			skip["subnets"]++
		} else {
			create["subnets"]++
			subnetByCIDR[s.CIDR] = struct{}{}
		}
	}

	create["ip_ranges"] = len(payload.IPRanges)

	ipOrphan := 0
	for _, ip := range payload.IPs {
		cidr := findSubnetCIDR(payload.Subnets, ip.SubnetID)
		if cidr == "" {
			ipOrphan++
			skip["ips"]++
			continue
		}
		if _, ok := subnetByCIDR[cidr]; !ok {
			ipOrphan++
			skip["ips"]++
			continue
		}
		vrfID := subnetVrf[cidr]
		if vrfID == uuid.Nil {
			vrfID = db.DefaultVRFID
		}
		if ip.VrfID != "" {
			if parsed, err := uuid.Parse(ip.VrfID); err == nil {
				vrfID = parsed
			}
		}
		if _, err := a.Store.Queries.GetIPByAddress(r.Context(), sqlcdb.GetIPByAddressParams{
			Address: ip.Address, VrfID: vrfID,
		}); err == nil {
			skip["ips"]++
			continue
		}
		create["ips"]++
	}
	if ipOrphan > 0 {
		notes = append(notes, "Some IPs have no matching subnet CIDR and would be skipped.")
	}
	for _, req := range payload.IPRequests {
		if req.Status != "" && req.Status != "pending" {
			skip["ip_requests"]++
			continue
		}
		create["ip_requests"]++
	}
	return restoreDiff{WouldCreate: create, WouldSkip: skip, InBackup: inBackup, Notes: notes}
}

func findSubnetCIDR(subs []backupSubnet, id string) string {
	for _, s := range subs {
		if s.ID == id {
			return s.CIDR
		}
	}
	return ""
}

func findLocationName(locs []sqlcdb.Location, id uuid.UUID) string {
	for _, l := range locs {
		if l.ID == id {
			return l.Name
		}
	}
	return ""
}
