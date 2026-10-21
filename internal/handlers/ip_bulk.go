package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

type ipBulkRequest struct {
	IDs              []uuid.UUID    `json:"ids"`
	Status           *string        `json:"status"`
	UpdateDeviceType bool           `json:"update_device_type"`
	DeviceTypeID     *uuid.UUID     `json:"device_type_id"`
	SoftDelete       bool           `json:"soft_delete"`
	SetCustomFields  map[string]any `json:"set_custom_fields"`
}

type ipBulkResult struct {
	Updated int      `json:"updated"`
	Deleted int      `json:"deleted"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

func (a *API) BulkUpdateIPs(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	var req ipBulkRequest
	if err := httpx.Decode(r, &req); err != nil || len(req.IDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "ids required")
		return
	}
	if len(req.IDs) > 500 {
		httpx.Error(w, http.StatusBadRequest, "max 500 ids per request")
		return
	}
	hasCF := len(req.SetCustomFields) > 0
	if req.Status == nil && !req.UpdateDeviceType && !req.SoftDelete && !hasCF {
		httpx.Error(w, http.StatusBadRequest, "provide status, update_device_type, soft_delete, and/or set_custom_fields")
		return
	}
	if hasCF {
		if err := a.validateCustomFields(r, "ip", req.SetCustomFields); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if req.Status != nil && !validIPStatus[*req.Status] {
		httpx.Error(w, http.StatusBadRequest, "invalid status")
		return
	}
	if req.UpdateDeviceType && req.DeviceTypeID != nil {
		if _, err := a.Store.Queries.GetDeviceType(r.Context(), *req.DeviceTypeID); err != nil {
			httpx.Error(w, http.StatusBadRequest, "device type not found")
			return
		}
	}

	items, err := a.Store.Queries.ListIPsByIDs(r.Context(), req.IDs)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	byID := map[uuid.UUID]sqlcdb.ListIPsByIDsRow{}
	for _, item := range items {
		byID[item.ID] = item
	}

	result := ipBulkResult{}
	var deviceNull pgtype.UUID
	if req.UpdateDeviceType && req.DeviceTypeID != nil {
		deviceNull = pgtype.UUID{Bytes: *req.DeviceTypeID, Valid: true}
	}

	type histEvent struct {
		action string
		item   sqlcdb.SoftDeleteIPRow
	}
	var history []histEvent
	var deletedHooks []sqlcdb.SoftDeleteIPRow

	_ = a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		for _, id := range req.IDs {
			item, ok := byID[id]
			if !ok {
				result.Skipped++
				result.Errors = append(result.Errors, id.String()+": not found")
				continue
			}
			if !sc.AllowsNullUUID(item.LocationID) {
				subnet, err := q.GetSubnet(r.Context(), item.SubnetID)
				if err != nil || subnet.DeletedAt.Valid || !sc.AllowsNullUUID(subnet.LocationID) {
					result.Skipped++
					result.Errors = append(result.Errors, id.String()+": out of scope")
					continue
				}
			}
			if req.SoftDelete {
				deleted, err := q.SoftDeleteIP(r.Context(), id)
				if err != nil {
					result.Skipped++
					result.Errors = append(result.Errors, id.String()+": "+err.Error())
					continue
				}
				history = append(history, histEvent{action: "delete", item: deleted})
				deletedHooks = append(deletedHooks, deleted)
				result.Deleted++
				continue
			}
			changed := false
			if req.Status != nil {
				if err := q.BulkUpdateIPStatus(r.Context(), sqlcdb.BulkUpdateIPStatusParams{
					ID: id, Status: *req.Status,
				}); err != nil {
					result.Skipped++
					result.Errors = append(result.Errors, id.String()+": "+err.Error())
					continue
				}
				changed = true
			}
			if req.UpdateDeviceType {
				if err := q.BulkUpdateIPDeviceType(r.Context(), sqlcdb.BulkUpdateIPDeviceTypeParams{
					ID: id, DeviceTypeID: deviceNull,
				}); err != nil {
					result.Skipped++
					result.Errors = append(result.Errors, id.String()+": "+err.Error())
					continue
				}
				changed = true
			}
			if hasCF {
				merged := mergeCustomFieldsJSON(item.CustomFields, req.SetCustomFields)
				if err := q.BulkSetIPCustomFields(r.Context(), sqlcdb.BulkSetIPCustomFieldsParams{
					ID: id, CustomFields: merged,
				}); err != nil {
					result.Skipped++
					result.Errors = append(result.Errors, id.String()+": "+err.Error())
					continue
				}
				changed = true
			}
			if changed {
				result.Updated++
			}
		}
		return nil
	})
	for _, h := range history {
		a.recordIPHistory(r.Context(), h.action, h.item.ID, h.item.Address, h.item.SubnetID, h.item.Hostname, h.item.Status, h.item.DeviceTypeID, h.item.LocationID, h.item.Notes, h.item.MacAddress, h.item.OpenPorts, h.item.ReservedUntil, h.item.LastSeenAt, nil)
	}
	if a.Hooks != nil {
		for _, deleted := range deletedHooks {
			a.Hooks.Dispatch(r.Context(), webhook.EventIPDeleted, map[string]any{
				"id": deleted.ID.String(), "address": deleted.Address,
				"subnet_id": deleted.SubnetID.String(), "hostname": deleted.Hostname, "status": deleted.Status,
				"bulk": true,
			})
		}
	}

	a.audit(r.Context(), "ip.bulk_update", "ip_address", "", map[string]any{
		"ids": len(req.IDs), "updated": result.Updated, "deleted": result.Deleted, "skipped": result.Skipped,
		"soft_delete": req.SoftDelete, "status": req.Status, "update_device_type": req.UpdateDeviceType,
		"set_custom_fields": hasCF,
	})
	if len(result.Errors) > 10 {
		result.Errors = result.Errors[:10]
	}
	httpx.JSON(w, http.StatusOK, result)
}
