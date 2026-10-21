package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
)

func Seed(ctx context.Context, store *Store, adminEmail, adminPassword string) error {
	count, err := store.Queries.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := auth.HashPassword(adminPassword)
	if err != nil {
		return err
	}
	if _, err := store.Queries.CreateUser(ctx, sqlcdb.CreateUserParams{
		Email:        adminEmail,
		PasswordHash: hash,
		Role:         string(auth.RoleAdmin),
	}); err != nil {
		return fmt.Errorf("create admin: %w", err)
	}

	loc, err := store.Queries.CreateLocation(ctx, sqlcdb.CreateLocationParams{
		Name:        "HQ",
		Description: "Default headquarters location",
	})
	if err != nil {
		return fmt.Errorf("seed location: %w", err)
	}

	for _, name := range []string{"Server", "Workstation", "Network Device", "Printer"} {
		if _, err := store.Queries.CreateDeviceType(ctx, sqlcdb.CreateDeviceTypeParams{
			Name:        name,
			Description: name,
		}); err != nil {
			return fmt.Errorf("seed device type %s: %w", name, err)
		}
	}

	parent, err := store.Queries.CreateSubnet(ctx, sqlcdb.CreateSubnetParams{
		Cidr:         "10.0.0.0/16",
		Name:         "Lab Supernet",
		Description:  "Sample parent CIDR",
		LocationID:   UUIDToNull(loc.ID),
		VrfID:        DefaultVRFID,
		VlanID:       pgtype.UUID{},
		ParentID:     pgtype.UUID{},
		Tags:         []string{"lab"},
		CustomFields: []byte(`{}`),
	})
	if err != nil {
		return fmt.Errorf("seed parent subnet: %w", err)
	}
	vlan, err := store.Queries.CreateVLAN(ctx, sqlcdb.CreateVLANParams{
		Vid: 10, Name: "VLAN 10", LocationID: UUIDToNull(loc.ID), Description: "Lab LAN",
	})
	if err != nil {
		return fmt.Errorf("seed vlan: %w", err)
	}
	_, err = store.Queries.CreateSubnet(ctx, sqlcdb.CreateSubnetParams{
		Cidr:         "10.0.0.0/24",
		Name:         "Lab LAN",
		Description:  "Sample lab subnet",
		LocationID:   UUIDToNull(loc.ID),
		VrfID:        DefaultVRFID,
		VlanID:       UUIDToNull(vlan.ID),
		ParentID:     UUIDToNull(parent.ID),
		Tags:         []string{"lab", "lan"},
		CustomFields: []byte(`{"env":"dev"}`),
	})
	if err != nil {
		return fmt.Errorf("seed subnet: %w", err)
	}
	return nil
}
