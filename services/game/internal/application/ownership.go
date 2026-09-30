package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	"github.com/kungrem23/quizgo/services/game/internal/observability"
)

var ErrLeaseLost = errors.New("room ownership lease lost")

// RoomOwner is the discoverable identity of the replica currently responsible
// for a room. LeaseID changes on every acquisition, including reacquisition by
// the same process, so an older actor cannot be mistaken for the current owner.
type RoomOwner struct {
	InstanceID  string
	InternalURL string
	LeaseID     string
	Fence       uint64
}

type RoomLease struct {
	GameID string
	Owner  RoomOwner
}

type RoomRoute struct {
	GameID string
	Local  bool
	Owner  RoomOwner
}

type NotRoomOwnerError struct {
	GameID string
	Owner  RoomOwner
}

func (e *NotRoomOwnerError) Error() string {
	return fmt.Sprintf("room %s is owned by replica %s", e.GameID, e.Owner.InstanceID)
}

// RoomOwnershipStore persists short-lived ownership independently from the
// longer-lived game snapshot. UpdateOwned must atomically reject a stale lease.
type RoomOwnershipStore interface {
	AcquireRoom(context.Context, string, RoomOwner, time.Duration, time.Duration) (RoomLease, bool, error)
	LookupRoomOwner(context.Context, string) (RoomOwner, time.Duration, error)
	RenewRoom(context.Context, RoomLease, time.Duration, time.Duration) (bool, error)
	ReleaseRoom(context.Context, RoomLease) error
	UpdateOwned(context.Context, game.Game, time.Duration, RoomLease) error
}

type OwnershipOptions struct {
	InstanceID    string
	InternalURL   string
	LeaseTTL      time.Duration
	RenewInterval time.Duration
	SafetyMargin  time.Duration
	Logger        *slog.Logger
	Metrics       *observability.Metrics
}

func IsNotRoomOwner(err error) (*NotRoomOwnerError, bool) {
	var target *NotRoomOwnerError
	ok := errors.As(err, &target)
	return target, ok
}
