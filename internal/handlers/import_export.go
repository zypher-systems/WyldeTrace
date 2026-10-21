package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
)

func (a *API) ExportSubnetsCSV(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	items, err := a.Store.Queries.ListSubnets(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	locs, _ := a.Store.Queries.ListLocations(r.Context())
	locName := map[uuid.UUID]string{}
	for _, l := range locs {
		locName[l.ID] = l.Name
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="subnets.csv"`)
	cw := csv.NewWriter(w)
	vids := a.vlanVIDMap(r.Context())
	_ = cw.Write([]string{"cidr", "name", "description", "vlan_id", "location_id", "location", "parent_id", "tags", "custom_fields"})
	for _, item := range items {
		if !sc.AllowsNullUUID(item.LocationID) {
			continue
		}
		vlan := ""
		if item.VlanID.Valid {
			if vid, ok := vids[uuid.UUID(item.VlanID.Bytes)]; ok {
				vlan = fmt.Sprintf("%d", vid)
			}
		}
		locLabel := ""
		if item.LocationID.Valid {
			locLabel = locName[uuid.UUID(item.LocationID.Bytes)]
		}
		cf := strings.TrimSpace(string(item.CustomFields))
		if cf == "" {
			cf = "{}"
		}
		_ = cw.Write([]string{
			item.Cidr,
			item.Name,
			item.Description,
			vlan,
			nullUUIDString(item.LocationID),
			locLabel,
			nullUUIDString(item.ParentID),
			strings.Join(normalizeTags(item.Tags), ","),
			cf,
		})
	}
	cw.Flush()
}

func (a *API) ExportIPsCSV(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	items, err := a.Store.Queries.ListIPs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	subs, _ := a.Store.Queries.ListSubnets(r.Context())
	cidrByID := map[uuid.UUID]string{}
	locBySubnet := map[uuid.UUID]pgtype.UUID{}
	for _, s := range subs {
		cidrByID[s.ID] = s.Cidr
		locBySubnet[s.ID] = s.LocationID
	}
	locs, _ := a.Store.Queries.ListLocations(r.Context())
	locName := map[uuid.UUID]string{}
	for _, l := range locs {
		locName[l.ID] = l.Name
	}
	dts, _ := a.Store.Queries.ListDeviceTypes(r.Context())
	dtName := map[uuid.UUID]string{}
	for _, d := range dts {
		dtName[d.ID] = d.Name
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="ips.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"address", "subnet_id", "subnet_cidr", "hostname", "status", "notes",
		"location_id", "location", "device_type_id", "device_type",
		"mac_address", "open_ports", "custom_fields",
	})
	for _, item := range items {
		if !sc.AllowsNullUUID(item.LocationID) && !sc.AllowsNullUUID(locBySubnet[item.SubnetID]) {
			continue
		}
		locLabel, dtLabel := "", ""
		if item.LocationID.Valid {
			locLabel = locName[uuid.UUID(item.LocationID.Bytes)]
		}
		if item.DeviceTypeID.Valid {
			dtLabel = dtName[uuid.UUID(item.DeviceTypeID.Bytes)]
		}
		cf := strings.TrimSpace(string(item.CustomFields))
		if cf == "" {
			cf = "{}"
		}
		_ = cw.Write([]string{
			item.Address,
			item.SubnetID.String(),
			cidrByID[item.SubnetID],
			item.Hostname,
			item.Status,
			item.Notes,
			nullUUIDString(item.LocationID),
			locLabel,
			nullUUIDString(item.DeviceTypeID),
			dtLabel,
			item.MacAddress,
			item.OpenPorts,
			cf,
		})
	}
	cw.Flush()
}

type importSummary struct {
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

func (a *API) ImportSubnetsCSV(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	rows, err := readCSV(r.Body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(rows) < 2 {
		httpx.Error(w, http.StatusBadRequest, "csv must include header and at least one row")
		return
	}
	idx := headerIndex(rows[0])
	locs, _ := a.Store.Queries.ListLocations(r.Context())
	locByName := map[string]uuid.UUID{}
	for _, l := range locs {
		if !sc.Allows(&l.ID) {
			continue
		}
		locByName[strings.ToLower(l.Name)] = l.ID
	}
	summary := importSummary{Errors: []string{}}
	for i, row := range rows[1:] {
		line := i + 2
		cidr := col(row, idx, "cidr")
		name := col(row, idx, "name")
		if cidr == "" || name == "" {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: cidr and name required", line))
			continue
		}
		prefix, err := ipam.ParsePrefix(cidr)
		if err != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, err))
			continue
		}
		var parentPtr *uuid.UUID
		if p := col(row, idx, "parent_id"); p != "" {
			id, err := uuid.Parse(p)
			if err != nil {
				summary.Skipped++
				summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: bad parent_id", line))
				continue
			}
			parentPtr = &id
		}
		var vlan *int32
		if v := col(row, idx, "vlan_id"); v != "" {
			var n int32
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
				vlan = &n
			}
		}
		locID, locErr := resolveImportLocation(col(row, idx, "location_id"), col(row, idx, "location"), locByName)
		if locErr != "" {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %s", line, locErr))
			continue
		}
		if !sc.AllowsNullUUID(locID) {
			// Scoped users must assign an in-scope location; unscoped null location is allowed only when unrestricted.
			if !sc.Unrestricted {
				summary.Skipped++
				summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: location out of scope or required", line))
				continue
			}
		}
		_, hasTagsCol := idx["tags"]
		tags := normalizeTags(strings.Split(col(row, idx, "tags"), ","))
		description := col(row, idx, "description")
		_, hasCF := idx["custom_fields"]
		cfPatch, cfErr := parseCustomFieldsCol(col(row, idx, "custom_fields"))
		if cfErr != "" {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %s", line, cfErr))
			continue
		}
		if hasCF && cfPatch != nil {
			if err := a.validateCustomFields(r, "subnet", cfPatch); err != nil {
				summary.Skipped++
				summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, err))
				continue
			}
		}

		vrfID := db.DefaultVRFID
		vlanFK, verr := a.resolveVLANFK(r.Context(), vlan, locID)
		if verr != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, verr))
			continue
		}
		existing, gerr := a.Store.Queries.GetSubnetByCIDR(r.Context(), sqlcdb.GetSubnetByCIDRParams{
			Cidr: prefix.String(), VrfID: vrfID,
		})
		if gerr == nil && !existing.DeletedAt.Valid {
			if !sc.AllowsNullUUID(existing.LocationID) {
				summary.Skipped++
				summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: existing subnet out of scope", line))
				continue
			}
			parentID, err := a.resolveSubnetParent(r.Context(), prefix, existing.VrfID, parentPtr, existing.ID)
			if err != nil {
				summary.Skipped++
				summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, err))
				continue
			}
			cf := existing.CustomFields
			if len(cf) == 0 {
				cf = []byte(`{}`)
			}
			if hasCF {
				cf = mergeCustomFieldsJSON(existing.CustomFields, cfPatch)
			}
			if !hasTagsCol {
				tags = normalizeTags(existing.Tags)
			}
			_, err = a.Store.Queries.UpdateSubnet(r.Context(), sqlcdb.UpdateSubnetParams{
				ID: existing.ID, Cidr: prefix.String(), Name: name, Description: description,
				LocationID: locID, VrfID: existing.VrfID, VlanID: vlanFK, ParentID: parentID,
				Tags: tags, CustomFields: cf,
			})
			if err != nil {
				summary.Skipped++
				summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, err))
				continue
			}
			summary.Updated++
			continue
		}

		parentID, err := a.resolveSubnetParent(r.Context(), prefix, vrfID, parentPtr, uuid.Nil)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				summary.Skipped++
				continue
			}
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, err))
			continue
		}
		cf := emptyJSON()
		if hasCF {
			cf = mergeCustomFieldsJSON(nil, cfPatch)
		}
		_, err = a.Store.Queries.CreateSubnet(r.Context(), sqlcdb.CreateSubnetParams{
			Cidr:         prefix.String(),
			Name:         name,
			Description:  description,
			LocationID:   locID,
			VrfID:        vrfID,
			VlanID:       vlanFK,
			ParentID:     parentID,
			Tags:         tags,
			CustomFields: cf,
		})
		if err != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, err))
			continue
		}
		summary.Created++
	}
	httpx.JSON(w, http.StatusOK, summary)
}

func resolveImportLocation(idRaw, nameRaw string, byName map[string]uuid.UUID) (pgtype.UUID, string) {
	idRaw = strings.TrimSpace(idRaw)
	nameRaw = strings.TrimSpace(nameRaw)
	if idRaw != "" {
		id, err := uuid.Parse(idRaw)
		if err != nil {
			return pgtype.UUID{}, "bad location_id"
		}
		return db.UUIDToNull(id), ""
	}
	if nameRaw != "" {
		id, ok := byName[strings.ToLower(nameRaw)]
		if !ok {
			return pgtype.UUID{}, "unknown location name"
		}
		return db.UUIDToNull(id), ""
	}
	return pgtype.UUID{}, ""
}

func (a *API) ImportIPsCSV(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	rows, err := readCSV(r.Body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(rows) < 2 {
		httpx.Error(w, http.StatusBadRequest, "csv must include header and at least one row")
		return
	}
	idx := headerIndex(rows[0])
	locs, _ := a.Store.Queries.ListLocations(r.Context())
	locByName := map[string]uuid.UUID{}
	for _, l := range locs {
		if !sc.Allows(&l.ID) {
			continue
		}
		locByName[strings.ToLower(l.Name)] = l.ID
	}
	dts, _ := a.Store.Queries.ListDeviceTypes(r.Context())
	dtByName := map[string]uuid.UUID{}
	for _, d := range dts {
		dtByName[strings.ToLower(d.Name)] = d.ID
	}
	summary := importSummary{Errors: []string{}}
	for i, row := range rows[1:] {
		line := i + 2
		address := col(row, idx, "address")
		if address == "" {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: address required", line))
			continue
		}
		addr, err := netip.ParseAddr(address)
		if err != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: invalid address", line))
			continue
		}
		subnetID, serr := a.resolveImportSubnetID(r, col(row, idx, "subnet_id"), col(row, idx, "subnet_cidr"))
		if serr != "" {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %s", line, serr))
			continue
		}
		sub, serr2 := a.Store.Queries.GetSubnet(r.Context(), subnetID)
		if serr2 != nil || sub.DeletedAt.Valid || !sc.AllowsNullUUID(sub.LocationID) {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: subnet out of scope", line))
			continue
		}
		status := col(row, idx, "status")
		if status == "" {
			status = "assigned"
		}
		if !validIPStatus[status] {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: invalid status", line))
			continue
		}
		locID, locErr := resolveImportLocation(col(row, idx, "location_id"), col(row, idx, "location"), locByName)
		if locErr != "" {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %s", line, locErr))
			continue
		}
		deviceID, derr := resolveImportDeviceType(col(row, idx, "device_type_id"), col(row, idx, "device_type"), dtByName)
		if derr != "" {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %s", line, derr))
			continue
		}
		mac := col(row, idx, "mac_address")
		ports := col(row, idx, "open_ports")
		_, hasCF := idx["custom_fields"]
		cfPatch, cfErr := parseCustomFieldsCol(col(row, idx, "custom_fields"))
		if cfErr != "" {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %s", line, cfErr))
			continue
		}
		if hasCF && cfPatch != nil {
			if err := a.validateCustomFields(r, "ip", cfPatch); err != nil {
				summary.Skipped++
				summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, err))
				continue
			}
		}

		existing, err := a.Store.Queries.GetIPByAddress(r.Context(), sqlcdb.GetIPByAddressParams{
			Address: addr.String(), VrfID: sub.VrfID,
		})
		if err == nil {
			cf := existing.CustomFields
			if hasCF {
				cf = mergeCustomFieldsJSON(existing.CustomFields, cfPatch)
			}
			_, uerr := a.Store.Queries.UpdateIP(r.Context(), sqlcdb.UpdateIPParams{
				ID:            existing.ID,
				Address:       addr.String(),
				SubnetID:      subnetID,
				VrfID:         sub.VrfID,
				Hostname:      col(row, idx, "hostname"),
				Status:        status,
				DeviceTypeID:  deviceID,
				LocationID:    locID,
				Notes:         col(row, idx, "notes"),
				ReservedUntil: existing.ReservedUntil,
				LastSeenAt:    existing.LastSeenAt,
				MacAddress:    firstNonEmpty(mac, existing.MacAddress),
				OpenPorts:     firstNonEmpty(ports, existing.OpenPorts),
				InterfaceName: existing.InterfaceName,
				SwitchID:      existing.SwitchID,
				CustomFields:  cf,
			})
			if uerr != nil {
				summary.Skipped++
				summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, uerr))
				continue
			}
			summary.Updated++
			continue
		}
		cf := emptyJSON()
		if hasCF {
			cf = mergeCustomFieldsJSON(nil, cfPatch)
		}
		_, cerr := a.Store.Queries.CreateIP(r.Context(), sqlcdb.CreateIPParams{
			Address:       addr.String(),
			SubnetID:      subnetID,
			VrfID:         sub.VrfID,
			Hostname:      col(row, idx, "hostname"),
			Status:        status,
			DeviceTypeID:  deviceID,
			LocationID:    locID,
			Notes:         col(row, idx, "notes"),
			ReservedUntil: pgtype.Timestamptz{},
			LastSeenAt:    pgtype.Timestamptz{},
			MacAddress:    mac,
			OpenPorts:     ports,
			CustomFields:  cf,
		})
		if cerr != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, fmt.Sprintf("line %d: %v", line, cerr))
			continue
		}
		summary.Created++
	}
	httpx.JSON(w, http.StatusOK, summary)
}

func (a *API) resolveImportSubnetID(r *http.Request, idRaw, cidrRaw string) (uuid.UUID, string) {
	idRaw = strings.TrimSpace(idRaw)
	cidrRaw = strings.TrimSpace(cidrRaw)
	if idRaw != "" {
		id, err := uuid.Parse(idRaw)
		if err != nil {
			return uuid.Nil, "invalid subnet_id"
		}
		if sub, err := a.Store.Queries.GetSubnet(r.Context(), id); err != nil || sub.DeletedAt.Valid {
			return uuid.Nil, "subnet not found"
		}
		return id, ""
	}
	if cidrRaw == "" {
		return uuid.Nil, "subnet_id or subnet_cidr required"
	}
	prefix, err := ipam.ParsePrefix(cidrRaw)
	if err != nil {
		return uuid.Nil, "invalid subnet_cidr"
	}
	sub, err := a.Store.Queries.GetSubnetByCIDR(r.Context(), sqlcdb.GetSubnetByCIDRParams{
		Cidr: prefix.String(), VrfID: db.DefaultVRFID,
	})
	if err != nil || sub.DeletedAt.Valid {
		return uuid.Nil, "subnet_cidr not found"
	}
	return sub.ID, ""
}

func resolveImportDeviceType(idRaw, nameRaw string, byName map[string]uuid.UUID) (pgtype.UUID, string) {
	idRaw = strings.TrimSpace(idRaw)
	nameRaw = strings.TrimSpace(nameRaw)
	if idRaw != "" {
		id, err := uuid.Parse(idRaw)
		if err != nil {
			return pgtype.UUID{}, "bad device_type_id"
		}
		return db.UUIDToNull(id), ""
	}
	if nameRaw != "" {
		id, ok := byName[strings.ToLower(nameRaw)]
		if !ok {
			return pgtype.UUID{}, "unknown device_type name"
		}
		return db.UUIDToNull(id), ""
	}
	return pgtype.UUID{}, ""
}

func parseCustomFieldsCol(raw string) (map[string]any, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, "custom_fields must be JSON object"
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, ""
}

func mergeCustomFieldsJSON(existing []byte, patch map[string]any) []byte {
	base := decodeCustomFields(existing)
	for k, v := range patch {
		if v == nil {
			delete(base, k)
			continue
		}
		base[k] = v
	}
	return encodeCustomFields(base)
}

func readCSV(r io.Reader) ([][]string, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	return cr.ReadAll()
}

func headerIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return m
}

func col(row []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func nullUUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func parseOptionalUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return db.UUIDToNull(id)
}
