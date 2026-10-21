package handlers

import (
	"net/http"
	"strings"

	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type searchResponse struct {
	Query     string        `json:"query"`
	Subnets   []subnetDTO   `json:"subnets"`
	IPs       []ipDTO       `json:"ips"`
	Locations []locationDTO `json:"locations"`
}

func (a *API) Search(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 1 {
		httpx.Error(w, http.StatusBadRequest, "q is required")
		return
	}

	resp := searchResponse{
		Query:     q,
		Subnets:   []subnetDTO{},
		IPs:       []ipDTO{},
		Locations: []locationDTO{},
	}
	scoped := !sc.Unrestricted
	locs := sc.LocationIDsOrEmpty()

	subnets, err := a.Store.Queries.SearchSubnets(r.Context(), sqlcdb.SearchSubnetsParams{
		Query: q, Scoped: scoped, LocationIds: locs,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	vids := a.vlanVIDMap(r.Context())
	for _, item := range subnets {
		resp.Subnets = append(resp.Subnets, toSubnetDTO(item.ID, item.Cidr, item.Name, item.Description, item.LocationID, item.VrfID, item.VlanID, item.ParentID, item.Tags, item.CustomFields, item.CreatedAt, item.UpdatedAt, vids))
	}

	ips, err := a.Store.Queries.SearchIPs(r.Context(), sqlcdb.SearchIPsParams{
		Query: q, Scoped: scoped, LocationIds: locs,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, item := range ips {
		resp.IPs = append(resp.IPs, toIPDTOFromRow(item.ID, item.Address, item.SubnetID, item.Hostname, item.Status, item.DeviceTypeID, item.LocationID, item.Notes, item.ReservedUntil, item.LastSeenAt, item.MacAddress, item.OpenPorts, item.InterfaceName, item.SwitchID, item.CustomFields, item.CreatedAt, item.UpdatedAt))
	}

	locations, err := a.Store.Queries.SearchLocations(r.Context(), sqlcdb.SearchLocationsParams{
		Query: q, Scoped: scoped, LocationIds: locs,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, item := range locations {
		resp.Locations = append(resp.Locations, toLocationDTO(item))
	}

	httpx.JSON(w, http.StatusOK, resp)
}
