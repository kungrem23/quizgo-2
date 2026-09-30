package redis_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	redisstore "github.com/kungrem23/quizgo/services/game/internal/store/redis"
)

// This test is optional in the regular suite and is exercised in CI or locally
// by pointing it at an isolated Redis instance.
func TestRoomLeaseExpiryAndFencing(t *testing.T) {
	address := os.Getenv("GAME_TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("GAME_TEST_REDIS_ADDRESS is not set")
	}
	store := redisstore.New(redisstore.Config{Address: address})
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	id := uuid.NewString()
	value := game.Game{ID: id, Code: "T" + id[:5], Phase: game.PhaseLobby}
	if err := store.Save(ctx, value, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	leaseA, acquired, err := store.AcquireRoom(ctx, id, application.RoomOwner{
		InstanceID: "replica-a", InternalURL: "http://replica-a:8081", LeaseID: uuid.NewString(),
	}, 150*time.Millisecond, 5*time.Second)
	if err != nil || !acquired {
		t.Fatalf("acquire A = %t, %v", acquired, err)
	}
	leaseB, acquired, err := store.AcquireRoom(ctx, id, application.RoomOwner{
		InstanceID: "replica-b", InternalURL: "http://replica-b:8081", LeaseID: uuid.NewString(),
	}, time.Second, 5*time.Second)
	if err != nil || acquired || leaseB.Owner.LeaseID != leaseA.Owner.LeaseID {
		t.Fatalf("competing acquire = %#v, %t, %v", leaseB, acquired, err)
	}

	time.Sleep(200 * time.Millisecond)
	leaseB, acquired, err = store.AcquireRoom(ctx, id, application.RoomOwner{
		InstanceID: "replica-b", InternalURL: "http://replica-b:8081", LeaseID: uuid.NewString(),
	}, time.Second, 5*time.Second)
	if err != nil || !acquired {
		t.Fatalf("takeover B = %t, %v", acquired, err)
	}
	if leaseB.Owner.Fence <= leaseA.Owner.Fence {
		t.Fatalf("fence did not increase: A=%d B=%d", leaseA.Owner.Fence, leaseB.Owner.Fence)
	}

	updated := value
	updated.Sequence = 1
	if err := store.UpdateOwned(ctx, updated, 5*time.Second, leaseA); !errors.Is(err, application.ErrLeaseLost) {
		t.Fatalf("stale write error = %v", err)
	}
	if err := store.UpdateOwned(ctx, updated, 5*time.Second, leaseB); err != nil {
		t.Fatalf("current owner write: %v", err)
	}
	if err := store.ReleaseRoom(ctx, leaseA); err != nil {
		t.Fatal(err)
	}
	if renewed, err := store.RenewRoom(ctx, leaseB, time.Second, 5*time.Second); err != nil || !renewed {
		t.Fatalf("stale release removed current lease: renewed=%t err=%v", renewed, err)
	}
	if err := store.ReleaseRoom(ctx, leaseB); err != nil {
		t.Fatal(err)
	}
	owner, _, err := store.LookupRoomOwner(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if owner.LeaseID != "" {
		t.Fatalf("owner remains after release: %#v", owner)
	}
}
