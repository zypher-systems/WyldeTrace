//go:build integration

package scheduler

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
)

func TestWithSessionLock_busyAndRelease(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	store, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("db unavailable: %v", err)
	}
	t.Cleanup(store.Close)

	s := &Scheduler{Store: store}
	lockKey := int64(87231405) // schedulerTickLockKey

	held := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.withSessionLock(ctx,
			`SELECT pg_try_advisory_lock($1)`,
			`SELECT pg_advisory_unlock($1)`,
			[]any{lockKey},
			func(context.Context) error {
				close(held)
				<-release
				return nil
			},
		)
		if err != nil {
			t.Errorf("holder lock: %v", err)
		}
	}()

	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for lock holder")
	}

	err = s.withSessionLock(ctx,
		`SELECT pg_try_advisory_lock($1)`,
		`SELECT pg_advisory_unlock($1)`,
		[]any{lockKey},
		func(context.Context) error { return nil },
	)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("want ErrBusy while held, got %v", err)
	}

	close(release)
	wg.Wait()

	err = s.withSessionLock(ctx,
		`SELECT pg_try_advisory_lock($1)`,
		`SELECT pg_advisory_unlock($1)`,
		[]any{lockKey},
		func(context.Context) error { return nil },
	)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
}

func TestJobLockObjID_stable(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := jobLockObjID(id)
	b := jobLockObjID(id)
	if a != b {
		t.Fatalf("unstable hash: %d vs %d", a, b)
	}
	other := jobLockObjID(uuid.MustParse("22222222-2222-2222-2222-222222222222"))
	if a == other {
		t.Fatal("expected distinct obj ids for different jobs")
	}
}
