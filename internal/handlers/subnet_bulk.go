package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type subnetBulkRequest struct {
	IDs              []uuid.UUID    `json:"ids"`
	AddTags          []string       `json:"add_tags"`
	RemoveTags       []string       `json:"remove_tags"`
	SetTags          []string       `json:"set_tags"`
	UpdateLocation   bool           `json:"update_location"`
	LocationID       *uuid.UUID     `json:"location_id"`
	SetCustomFields  map[string]any `json:"set_custom_fields"`
}

type subnetBulkResult struct {
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

func (a *API) BulkUpdateSubnets(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireScope(w, r)
	if !ok {
		return
	}
	var req subnetBulkRequest
	if err := httpx.Decode(r, &req); err != nil || len(req.IDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "ids required")
		return
	}
	if len(req.IDs) > 500 {
		httpx.Error(w, http.StatusBadRequest, "max 500 ids per request")
		return
	}
	hasTagOp := len(req.AddTags) > 0 || len(req.RemoveTags) > 0 || req.SetTags != nil
	hasCF := len(req.SetCustomFields) > 0
	if !hasTagOp && !req.UpdateLocation && !hasCF {
		httpx.Error(w, http.StatusBadRequest, "provide tag changes, update_location, and/or set_custom_fields")
		return
	}
	if hasCF {
		if err := a.validateSubnetCustomFields(r, req.SetCustomFields); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if req.UpdateLocation && req.LocationID != nil {
		if !sc.Allows(req.LocationID) {
			httpx.Error(w, http.StatusForbidden, "location out of scope")
			return
		}
		if _, err := a.Store.Queries.GetLocation(r.Context(), *req.LocationID); err != nil {
			httpx.Error(w, http.StatusBadRequest, "location not found")
			return
		}
	}

	items, err := a.Store.Queries.ListSubnetsByIDs(r.Context(), req.IDs)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	byID := map[uuid.UUID]sqlcdb.ListSubnetsByIDsRow{}
	for _, item := range items {
		byID[item.ID] = item
	}

	addTags := normalizeTags(req.AddTags)
	removeTags := normalizeTags(req.RemoveTags)
	var setTags []string
	if req.SetTags != nil {
		setTags = normalizeTags(req.SetTags)
	}

	result := subnetBulkResult{}
	var locNull pgtype.UUID
	if req.UpdateLocation {
		if req.LocationID != nil {
			locNull = pgtype.UUID{Bytes: *req.LocationID, Valid: true}
		}
	}

	_ = a.Store.WithTx(r.Context(), func(q *sqlcdb.Queries) error {
		for _, id := range req.IDs {
			item, ok := byID[id]
			if !ok {
				result.Skipped++
				result.Errors = append(result.Errors, id.String()+": not found")
				continue
			}
			if !sc.AllowsNullUUID(item.LocationID) {
				result.Skipped++
				result.Errors = append(result.Errors, id.String()+": out of scope")
				continue
			}
			changed := false
			if hasTagOp {
				tags := normalizeTags(item.Tags)
				switch {
				case req.SetTags != nil:
					tags = setTags
				default:
					if len(addTags) > 0 {
						seen := map[string]bool{}
						for _, t := range tags {
							seen[t] = true
						}
						for _, t := range addTags {
							if !seen[t] {
								tags = append(tags, t)
								seen[t] = true
							}
						}
					}
					if len(removeTags) > 0 {
						drop := map[string]bool{}
						for _, t := range removeTags {
							drop[t] = true
						}
						kept := make([]string, 0, len(tags))
						for _, t := range tags {
							if !drop[t] {
								kept = append(kept, t)
							}
						}
						tags = kept
					}
				}
				if err := q.BulkSetSubnetTags(r.Context(), sqlcdb.BulkSetSubnetTagsParams{
					ID: id, Tags: tags,
				}); err != nil {
					result.Skipped++
					result.Errors = append(result.Errors, id.String()+": "+err.Error())
					continue
				}
				changed = true
			}
			if req.UpdateLocation {
				if err := q.BulkSetSubnetLocation(r.Context(), sqlcdb.BulkSetSubnetLocationParams{
					ID: id, LocationID: locNull,
				}); err != nil {
					result.Skipped++
					result.Errors = append(result.Errors, id.String()+": "+err.Error())
					continue
				}
				changed = true
			}
			if hasCF {
				merged := mergeCustomFieldsJSON(item.CustomFields, req.SetCustomFields)
				if err := q.BulkSetSubnetCustomFields(r.Context(), sqlcdb.BulkSetSubnetCustomFieldsParams{
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

	a.audit(r.Context(), "subnet.bulk_update", "subnet", "", map[string]any{
		"ids": len(req.IDs), "updated": result.Updated, "skipped": result.Skipped,
		"add_tags": addTags, "remove_tags": removeTags, "set_tags": req.SetTags != nil,
		"update_location": req.UpdateLocation, "set_custom_fields": hasCF,
	})
	if len(result.Errors) > 10 {
		result.Errors = result.Errors[:10]
	}
	httpx.JSON(w, http.StatusOK, result)
}
