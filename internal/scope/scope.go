package scope

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	"github.com/wyldetechnologies/wyldetrace/internal/middleware"
)

// Scope describes site (location) visibility for the current actor.
// Unrestricted means all locations (admins, or API tokens with no memberships).
// Non-admin users with zero user_locations rows are fail-closed (see nothing).
type Scope struct {
	Unrestricted bool
	LocationIDs  []uuid.UUID
}

func Load(ctx context.Context, store *db.Store) (Scope, error) {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok {
		return Scope{}, auth.ErrInvalidToken
	}
	// API tokens: empty location list = global; otherwise site-scoped.
	if claims.IsAPIToken {
		ids, err := store.Queries.ListLocationIDsByAPIToken(ctx, claims.TokenID)
		if err != nil {
			return Scope{}, err
		}
		if len(ids) == 0 {
			return Scope{Unrestricted: true}, nil
		}
		return Scope{Unrestricted: false, LocationIDs: ids}, nil
	}
	if claims.Role == auth.RoleAdmin || claims.UserID == uuid.Nil {
		return Scope{Unrestricted: true}, nil
	}
	ids, err := store.Queries.ListLocationIDsByUser(ctx, claims.UserID)
	if err != nil {
		return Scope{}, err
	}
	if len(ids) == 0 {
		// Fail-closed: no memberships = no site access (not global).
		return Scope{Unrestricted: false, LocationIDs: []uuid.UUID{}}, nil
	}
	return Scope{Unrestricted: false, LocationIDs: ids}, nil
}

func (s Scope) Allows(locationID *uuid.UUID) bool {
	if s.Unrestricted {
		return true
	}
	if locationID == nil {
		return false
	}
	for _, id := range s.LocationIDs {
		if id == *locationID {
			return true
		}
	}
	return false
}

func (s Scope) AllowsNullUUID(u pgtype.UUID) bool {
	if s.Unrestricted {
		return true
	}
	if !u.Valid {
		return false
	}
	id := uuid.UUID(u.Bytes)
	return s.Allows(&id)
}

func (s Scope) LocationIDsOrEmpty() []uuid.UUID {
	if s.LocationIDs == nil {
		return []uuid.UUID{}
	}
	return s.LocationIDs
}
