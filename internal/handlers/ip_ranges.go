package handlers

import (
	"fmt"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
)

type ipRangeDTO struct {
	ID           uuid.UUID `json:"id"`
	SubnetID     uuid.UUID `json:"subnet_id"`
	Name         string    `json:"name"`
	StartAddress string    `json:"start_address"`
	EndAddress   string    `json:"end_address"`
	Role         string    `json:"role"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ipRangeRequest struct {
	Name         string `json:"name"`
	StartAddress string `json:"start_address"`
	EndAddress   string `json:"end_address"`
	Role         string `json:"role"`
	Description  string `json:"description"`
}

func toIPRangeDTO(id, subnetID uuid.UUID, name, start, end, role, description string, createdAt, updatedAt time.Time) ipRangeDTO {
	return ipRangeDTO{
		ID: id, SubnetID: subnetID, Name: name,
		StartAddress: start, EndAddress: end, Role: role, Description: description,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

func normalizeRangeRole(role string) string {
	switch role {
	case "dhcp", "static", "reserved", "other":
		return role
	case "":
		return "other"
	default:
		return ""
	}
}

func validateRangeAddrs(startRaw, endRaw string) (string, string, error) {
	start, err := netip.ParseAddr(startRaw)
	if err != nil {
		return "", "", err
	}
	end, err := netip.ParseAddr(endRaw)
	if err != nil {
		return "", "", err
	}
	if end.Less(start) {
		return "", "", fmt.Errorf("end_address must be >= start_address")
	}
	return start.String(), end.String(), nil
}

func validateRangeAddrsInPrefix(prefix netip.Prefix, startRaw, endRaw string) (string, string, error) {
	start, end, err := validateRangeAddrs(startRaw, endRaw)
	if err != nil {
		return "", "", fmt.Errorf("invalid address range")
	}
	s := netip.MustParseAddr(start)
	e := netip.MustParseAddr(end)
	if !prefix.Contains(s) || !prefix.Contains(e) {
		return "", "", fmt.Errorf("range must be inside the subnet")
	}
	return start, end, nil
}

func (a *API) ListIPRanges(w http.ResponseWriter, r *http.Request) {
	subnetID, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet id")
		return
	}
	if !a.requireSubnetAccess(w, r, subnetID) {
		return
	}
	items, err := a.Store.Queries.ListIPRangesBySubnet(r.Context(), subnetID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]ipRangeDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toIPRangeDTO(item.ID, item.SubnetID, item.Name, item.StartAddress, item.EndAddress, item.Role, item.Description, item.CreatedAt.Time, item.UpdatedAt.Time))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CreateIPRange(w http.ResponseWriter, r *http.Request) {
	subnetID, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet id")
		return
	}
	if !a.requireSubnetAccess(w, r, subnetID) {
		return
	}
	subnet, err := a.Store.Queries.GetSubnet(r.Context(), subnetID)
	if err != nil || subnet.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	prefix, err := ipam.ParsePrefix(subnet.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet cidr")
		return
	}
	var req ipRangeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	role := normalizeRangeRole(req.Role)
	if role == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid role")
		return
	}
	start, end, err := validateRangeAddrsInPrefix(prefix, req.StartAddress, req.EndAddress)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := a.Store.Queries.CreateIPRange(r.Context(), sqlcdb.CreateIPRangeParams{
		SubnetID: subnetID, Name: req.Name, StartAddress: start, EndAddress: end,
		Role: role, Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, toIPRangeDTO(item.ID, item.SubnetID, item.Name, item.StartAddress, item.EndAddress, item.Role, item.Description, item.CreatedAt.Time, item.UpdatedAt.Time))
}

func (a *API) UpdateIPRange(w http.ResponseWriter, r *http.Request) {
	subnetID, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet id")
		return
	}
	rangeID, err := uuid.Parse(chiURLParam(r, "rangeID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid range id")
		return
	}
	if !a.requireSubnetAccess(w, r, subnetID) {
		return
	}
	existing, err := a.Store.Queries.GetIPRange(r.Context(), rangeID)
	if err != nil || existing.SubnetID != subnetID {
		httpx.Error(w, http.StatusNotFound, "range not found")
		return
	}
	subnet, err := a.Store.Queries.GetSubnet(r.Context(), subnetID)
	if err != nil || subnet.DeletedAt.Valid {
		httpx.Error(w, http.StatusNotFound, "subnet not found")
		return
	}
	prefix, err := ipam.ParsePrefix(subnet.Cidr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet cidr")
		return
	}
	var req ipRangeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	role := normalizeRangeRole(req.Role)
	if role == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid role")
		return
	}
	start, end, err := validateRangeAddrsInPrefix(prefix, req.StartAddress, req.EndAddress)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := a.Store.Queries.UpdateIPRange(r.Context(), sqlcdb.UpdateIPRangeParams{
		ID: rangeID, Name: req.Name, StartAddress: start, EndAddress: end,
		Role: role, Description: req.Description,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, toIPRangeDTO(item.ID, item.SubnetID, item.Name, item.StartAddress, item.EndAddress, item.Role, item.Description, item.CreatedAt.Time, item.UpdatedAt.Time))
}

func (a *API) DeleteIPRange(w http.ResponseWriter, r *http.Request) {
	subnetID, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid subnet id")
		return
	}
	rangeID, err := uuid.Parse(chiURLParam(r, "rangeID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid range id")
		return
	}
	if !a.requireSubnetAccess(w, r, subnetID) {
		return
	}
	existing, err := a.Store.Queries.GetIPRange(r.Context(), rangeID)
	if err != nil || existing.SubnetID != subnetID {
		httpx.Error(w, http.StatusNotFound, "range not found")
		return
	}
	if err := a.Store.Queries.DeleteIPRange(r.Context(), rangeID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
