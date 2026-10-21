package scope

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAllows_unrestricted(t *testing.T) {
	s := Scope{Unrestricted: true}
	id := uuid.New()
	if !s.Allows(&id) {
		t.Fatal("unrestricted should allow any location")
	}
	if !s.Allows(nil) {
		t.Fatal("unrestricted should allow nil location")
	}
}

func TestAllows_failClosedEmptyMembership(t *testing.T) {
	// Non-admin with zero memberships: Unrestricted=false, empty LocationIDs.
	s := Scope{Unrestricted: false, LocationIDs: []uuid.UUID{}}
	id := uuid.New()
	if s.Allows(&id) {
		t.Fatal("fail-closed empty membership must deny all locations")
	}
	if s.Allows(nil) {
		t.Fatal("fail-closed empty membership must deny nil location")
	}
	if s.AllowsNullUUID(pgtype.UUID{}) {
		t.Fatal("fail-closed must deny null UUID")
	}
}

func TestAllows_membership(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	s := Scope{Unrestricted: false, LocationIDs: []uuid.UUID{a}}
	if !s.Allows(&a) {
		t.Fatal("expected allow for member location")
	}
	if s.Allows(&b) {
		t.Fatal("expected deny for non-member location")
	}
	nu := pgtype.UUID{Bytes: a, Valid: true}
	if !s.AllowsNullUUID(nu) {
		t.Fatal("expected allow for member null-uuid")
	}
}

func TestLocationIDsOrEmpty(t *testing.T) {
	s := Scope{}
	if s.LocationIDsOrEmpty() == nil {
		t.Fatal("nil LocationIDs should become empty slice")
	}
	id := uuid.New()
	s.LocationIDs = []uuid.UUID{id}
	got := s.LocationIDsOrEmpty()
	if len(got) != 1 || got[0] != id {
		t.Fatalf("unexpected ids: %v", got)
	}
}
