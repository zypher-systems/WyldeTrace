package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type subnetSettingsDTO struct {
	SubnetID               uuid.UUID `json:"subnet_id"`
	UtilAlertPercent       *int32    `json:"util_alert_percent"`
	GlobalUtilAlertPercent int       `json:"global_util_alert_percent"`
}

type subnetSettingsReq struct {
	UtilAlertPercent *int32 `json:"util_alert_percent"`
	Clear            bool   `json:"clear"`
}

func (a *API) GetSubnetSettings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !a.requireSubnetAccess(w, r, id) {
		return
	}
	global, _ := a.subnetUtilizationAlertPercent(r.Context())
	dto := subnetSettingsDTO{SubnetID: id, GlobalUtilAlertPercent: global}
	row, err := a.Store.Queries.GetSubnetSettings(r.Context(), id)
	if err == nil && row.UtilAlertPercent.Valid {
		v := row.UtilAlertPercent.Int32
		dto.UtilAlertPercent = &v
	} else if err != nil && err != pgx.ErrNoRows {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, dto)
}

func (a *API) UpdateSubnetSettings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !a.requireSubnetAccess(w, r, id) {
		return
	}
	var req subnetSettingsReq
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	global, _ := a.subnetUtilizationAlertPercent(r.Context())
	if req.Clear || req.UtilAlertPercent == nil {
		_ = a.Store.Queries.ClearSubnetUtilAlertPercent(r.Context(), id)
		a.audit(r.Context(), "subnet.settings", "subnet", id.String(), map[string]any{
			"util_alert_percent": nil,
		})
		httpx.JSON(w, http.StatusOK, subnetSettingsDTO{
			SubnetID: id, UtilAlertPercent: nil, GlobalUtilAlertPercent: global,
		})
		return
	}
	v := *req.UtilAlertPercent
	if v < 1 || v > 100 {
		httpx.Error(w, http.StatusBadRequest, "util_alert_percent must be 1–100, or clear to use global")
		return
	}
	row, err := a.Store.Queries.UpsertSubnetUtilAlertPercent(r.Context(), sqlcdb.UpsertSubnetUtilAlertPercentParams{
		SubnetID: id, UtilAlertPercent: db.Int4Ptr(&v),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "subnet.settings", "subnet", id.String(), map[string]any{
		"util_alert_percent": v,
	})
	out := subnetSettingsDTO{SubnetID: id, GlobalUtilAlertPercent: global}
	if row.UtilAlertPercent.Valid {
		n := row.UtilAlertPercent.Int32
		out.UtilAlertPercent = &n
	}
	httpx.JSON(w, http.StatusOK, out)
}
