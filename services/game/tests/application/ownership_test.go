package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	. "github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
)

type ownershipRepositoryStub struct {
	*repositoryStub
	ownerMu sync.Mutex
	owners  map[string]fakeLease
	fences  map[string]uint64
}

type fakeLease struct {
	lease     RoomLease
	expiresAt time.Time
}

func newOwnershipRepositoryStub() *ownershipRepositoryStub {
	return &ownershipRepositoryStub{
		repositoryStub: &repositoryStub{}, owners: make(map[string]fakeLease), fences: make(map[string]uint64),
	}
}

func (r *ownershipRepositoryStub) AcquireRoom(_ context.Context, gameID string, candidate RoomOwner, leaseTTL, _ time.Duration) (RoomLease, bool, error) {
	if _, err := r.GetByID(context.Background(), gameID); err != nil {
		return RoomLease{}, false, err
	}
	r.ownerMu.Lock()
	defer r.ownerMu.Unlock()
	if current, ok := r.currentLease(gameID, time.Now()); ok {
		return current.lease, false, nil
	}
	r.fences[gameID]++
	candidate.Fence = r.fences[gameID]
	lease := RoomLease{GameID: gameID, Owner: candidate}
	r.owners[gameID] = fakeLease{lease: lease, expiresAt: time.Now().Add(leaseTTL)}
	return lease, true, nil
}

func (r *ownershipRepositoryStub) LookupRoomOwner(_ context.Context, gameID string) (RoomOwner, time.Duration, error) {
	r.ownerMu.Lock()
	defer r.ownerMu.Unlock()
	current, ok := r.currentLease(gameID, time.Now())
	if !ok {
		return RoomOwner{}, 0, nil
	}
	return current.lease.Owner, time.Until(current.expiresAt), nil
}

func (r *ownershipRepositoryStub) RenewRoom(_ context.Context, lease RoomLease, leaseTTL, _ time.Duration) (bool, error) {
	r.ownerMu.Lock()
	defer r.ownerMu.Unlock()
	current, ok := r.currentLease(lease.GameID, time.Now())
	if !ok || !sameLease(current.lease, lease) {
		return false, nil
	}
	current.expiresAt = time.Now().Add(leaseTTL)
	r.owners[lease.GameID] = current
	return true, nil
}

func (r *ownershipRepositoryStub) ReleaseRoom(_ context.Context, lease RoomLease) error {
	r.ownerMu.Lock()
	defer r.ownerMu.Unlock()
	if current, ok := r.owners[lease.GameID]; ok && sameLease(current.lease, lease) {
		delete(r.owners, lease.GameID)
	}
	return nil
}

func (r *ownershipRepositoryStub) UpdateOwned(ctx context.Context, value game.Game, ttl time.Duration, lease RoomLease) error {
	r.ownerMu.Lock()
	current, ok := r.currentLease(value.ID, time.Now())
	if !ok || !sameLease(current.lease, lease) {
		r.ownerMu.Unlock()
		return ErrLeaseLost
	}
	err := r.repositoryStub.Update(ctx, value, ttl)
	r.ownerMu.Unlock()
	return err
}

func (r *ownershipRepositoryStub) currentLease(gameID string, now time.Time) (fakeLease, bool) {
	current, ok := r.owners[gameID]
	if ok && !now.Before(current.expiresAt) {
		delete(r.owners, gameID)
		return fakeLease{}, false
	}
	return current, ok
}

func (r *ownershipRepositoryStub) expireRoom(gameID string) {
	r.ownerMu.Lock()
	defer r.ownerMu.Unlock()
	current := r.owners[gameID]
	current.expiresAt = time.Now().Add(-time.Millisecond)
	r.owners[gameID] = current
}

func sameLease(left, right RoomLease) bool {
	return left.GameID == right.GameID && left.Owner.LeaseID == right.Owner.LeaseID && left.Owner.Fence == right.Owner.Fence
}

func distributedOptions(instanceID string) OwnershipOptions {
	return OwnershipOptions{
		InstanceID: instanceID, InternalURL: "http://" + instanceID,
		LeaseTTL: 500 * time.Millisecond, RenewInterval: 100 * time.Millisecond, SafetyMargin: 25 * time.Millisecond,
	}
}

func TestDistributedHubHasOneOwnerAndGracefulTakeover(t *testing.T) {
	repository := newOwnershipRepositoryStub()
	created := createOwnershipTestGame(t, repository, 10)
	hubA := NewDistributedHub(repository, repository, time.Hour, distributedOptions("replica-a"))
	hubB := NewDistributedHub(repository, repository, time.Hour, distributedOptions("replica-b"))
	defer hubB.Close()

	hostA, err := hubA.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, hostA.Events, "state", time.Second)
	if _, err := hubB.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket); err == nil {
		t.Fatal("second replica unexpectedly created another room owner")
	} else if notOwner, ok := IsNotRoomOwner(err); !ok || notOwner.Owner.InstanceID != "replica-a" {
		t.Fatalf("second replica error = %v", err)
	}

	hubA.Close()
	hostB, err := hubB.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatalf("take over after graceful release: %v", err)
	}
	state := waitForEvent(t, hostB.Events, "state", time.Second)
	if state.Payload.(StateView).GameID != created.Game.ID {
		t.Fatalf("unexpected recovered state: %#v", state.Payload)
	}
}

func TestDistributedStoreFencesExpiredOwner(t *testing.T) {
	repository := newOwnershipRepositoryStub()
	created := createOwnershipTestGame(t, repository, 10)
	leaseA, acquired, err := repository.AcquireRoom(context.Background(), created.Game.ID, RoomOwner{InstanceID: "a", LeaseID: "lease-a"}, 20*time.Millisecond, time.Hour)
	if err != nil || !acquired {
		t.Fatalf("acquire A = %t, %v", acquired, err)
	}
	time.Sleep(30 * time.Millisecond)
	leaseB, acquired, err := repository.AcquireRoom(context.Background(), created.Game.ID, RoomOwner{InstanceID: "b", LeaseID: "lease-b"}, time.Second, time.Hour)
	if err != nil || !acquired {
		t.Fatalf("acquire B = %t, %v", acquired, err)
	}
	candidate := created.Game
	if err := candidate.Finish(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateOwned(context.Background(), candidate, time.Hour, leaseA); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale owner update error = %v", err)
	}
	if err := repository.UpdateOwned(context.Background(), candidate, time.Hour, leaseB); err != nil {
		t.Fatalf("current owner update: %v", err)
	}
}

func TestDistributedHubRestoresTimerAfterOwnerLeaseExpires(t *testing.T) {
	repository := newOwnershipRepositoryStub()
	created := createOwnershipTestGame(t, repository, 1)
	hubA := NewDistributedHub(repository, repository, time.Hour, distributedOptions("replica-a"))
	defer hubA.Close()
	player, err := hubA.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	hostA, err := hubA.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hubA.DispatchCommand(context.Background(), hostA, Command{Type: "start", RequestID: "start-1"}); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, player.Session.Events, "countdown_started", time.Second)
	// Simulate the owner becoming unable to renew while its actor is still in
	// memory. The new owner must recover from Redis and fence the old actor.
	repository.expireRoom(created.Game.ID)

	hubB := NewDistributedHub(repository, repository, time.Hour, distributedOptions("replica-b"))
	defer hubB.Close()
	hostB, err := hubB.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	state := waitForEvent(t, hostB.Events, "state", time.Second)
	if state.Payload.(StateView).Phase != game.PhaseCountdown {
		t.Fatalf("recovered phase = %s", state.Payload.(StateView).Phase)
	}
	select {
	case <-player.Session.Done:
	case <-time.After(time.Second):
		t.Fatal("expired owner kept its existing session open")
	}
	opened := waitForEvent(t, hostB.Events, "question_opened", 4*time.Second)
	if opened.Payload.(StateView).Phase != game.PhaseQuestionOpen {
		t.Fatalf("timer did not recover: %#v", opened.Payload)
	}
}

func createOwnershipTestGame(t *testing.T, repository GameRepository, questionSeconds uint32) CreatedGame {
	t.Helper()
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: questionSeconds,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	created, err := New(catalogStub{snapshot: snapshot}, repository, time.Hour).CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	return created
}
