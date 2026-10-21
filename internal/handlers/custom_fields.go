package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type customFieldDefDTO struct {
	ID         uuid.UUID `json:"id"`
	EntityType string    `json:"entity_type"`
	Key        string    `json:"key"`
	Label      string    `json:"label"`
	FieldType  string    `json:"field_type"`
	Required   bool      `json:"required"`
	Options    []string  `json:"options"`
	SortOrder  int32     `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type customFieldDefRequest struct {
	EntityType string   `json:"entity_type"`
	Key        string   `json:"key"`
	Label      string   `json:"label"`
	FieldType  string   `json:"field_type"`
	Required   bool     `json:"required"`
	Options    []string `json:"options"`
	SortOrder  int32    `json:"sort_order"`
}

var validFieldTypes = map[string]bool{
	"string": true, "number": true, "boolean": true, "select": true,
}

func toCustomFieldDefDTO(d sqlcdb.CustomFieldDefinition) customFieldDefDTO {
	opts := d.Options
	if opts == nil {
		opts = []string{}
	}
	return customFieldDefDTO{
		ID: d.ID, EntityType: d.EntityType, Key: d.Key, Label: d.Label,
		FieldType: d.FieldType, Required: d.Required, Options: opts,
		SortOrder: d.SortOrder, CreatedAt: d.CreatedAt.Time, UpdatedAt: d.UpdatedAt.Time,
	}
}

func (a *API) ListCustomFieldDefs(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("entity_type")
	if entity == "" {
		entity = "subnet"
	}
	items, err := a.Store.Queries.ListCustomFieldDefs(r.Context(), entity)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]customFieldDefDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toCustomFieldDefDTO(item))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CreateCustomFieldDef(w http.ResponseWriter, r *http.Request) {
	var req customFieldDefRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.EntityType == "" {
		req.EntityType = "subnet"
	}
	if req.EntityType != "subnet" && req.EntityType != "ip" {
		httpx.Error(w, http.StatusBadRequest, "entity_type must be subnet or ip")
		return
	}
	req.Key = strings.TrimSpace(strings.ToLower(req.Key))
	req.Label = strings.TrimSpace(req.Label)
	if req.Key == "" || req.Label == "" || !validFieldTypes[req.FieldType] {
		httpx.Error(w, http.StatusBadRequest, "key, label, and valid field_type required")
		return
	}
	if req.Options == nil {
		req.Options = []string{}
	}
	item, err := a.Store.Queries.CreateCustomFieldDef(r.Context(), sqlcdb.CreateCustomFieldDefParams{
		EntityType: req.EntityType, Key: req.Key, Label: req.Label,
		FieldType: req.FieldType, Required: req.Required, Options: req.Options, SortOrder: req.SortOrder,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "custom_field.create", "custom_field_definition", item.ID.String(), map[string]any{"key": item.Key})
	httpx.JSON(w, http.StatusCreated, toCustomFieldDefDTO(item))
}

func (a *API) UpdateCustomFieldDef(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req customFieldDefRequest
	if err := httpx.Decode(r, &req); err != nil || strings.TrimSpace(req.Label) == "" || !validFieldTypes[req.FieldType] {
		httpx.Error(w, http.StatusBadRequest, "label and valid field_type required")
		return
	}
	if req.Options == nil {
		req.Options = []string{}
	}
	item, err := a.Store.Queries.UpdateCustomFieldDef(r.Context(), sqlcdb.UpdateCustomFieldDefParams{
		ID: id, Label: strings.TrimSpace(req.Label), FieldType: req.FieldType,
		Required: req.Required, Options: req.Options, SortOrder: req.SortOrder,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, toCustomFieldDefDTO(item))
}

func (a *API) DeleteCustomFieldDef(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.Queries.DeleteCustomFieldDef(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) validateSubnetCustomFields(r *http.Request, fields map[string]any) error {
	return a.validateCustomFields(r, "subnet", fields)
}

func (a *API) validateCustomFields(r *http.Request, entityType string, fields map[string]any) error {
	defs, err := a.Store.Queries.ListCustomFieldDefs(r.Context(), entityType)
	if err != nil {
		return err
	}
	if fields == nil {
		fields = map[string]any{}
	}
	for _, d := range defs {
		val, ok := fields[d.Key]
		if !ok || val == nil || val == "" {
			if d.Required {
				return fmt.Errorf("custom field %q is required", d.Key)
			}
			continue
		}
		switch d.FieldType {
		case "string", "select":
			s, ok := val.(string)
			if !ok {
				return fmt.Errorf("custom field %q must be a string", d.Key)
			}
			if d.FieldType == "select" && len(d.Options) > 0 {
				found := false
				for _, opt := range d.Options {
					if opt == s {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("custom field %q must be one of: %s", d.Key, strings.Join(d.Options, ", "))
				}
			}
		case "number":
			switch v := val.(type) {
			case float64, int, int32, int64:
				// ok
			case string:
				if _, err := strconv.ParseFloat(v, 64); err != nil {
					return fmt.Errorf("custom field %q must be a number", d.Key)
				}
			default:
				return fmt.Errorf("custom field %q must be a number", d.Key)
			}
		case "boolean":
			switch val.(type) {
			case bool:
			case string:
				s := strings.ToLower(val.(string))
				if s != "true" && s != "false" {
					return fmt.Errorf("custom field %q must be boolean", d.Key)
				}
			default:
				return fmt.Errorf("custom field %q must be boolean", d.Key)
			}
		}
	}
	return nil
}
