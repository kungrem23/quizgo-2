package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	"github.com/redis/go-redis/v9"
)

var acquireRoom = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 0 then
  return {-1}
end
if redis.call('EXISTS', KEYS[1]) == 1 then
  return {0, redis.call('HGET', KEYS[1], 'instance_id'), redis.call('HGET', KEYS[1], 'internal_url'), redis.call('HGET', KEYS[1], 'lease_id'), redis.call('HGET', KEYS[1], 'fence'), redis.call('PTTL', KEYS[1])}
end
local fence = redis.call('INCR', KEYS[2])
redis.call('HSET', KEYS[1], 'instance_id', ARGV[1], 'internal_url', ARGV[2], 'lease_id', ARGV[3], 'fence', fence)
redis.call('PEXPIRE', KEYS[1], ARGV[4])
redis.call('PEXPIRE', KEYS[2], ARGV[5])
return {1, ARGV[1], ARGV[2], ARGV[3], tostring(fence), ARGV[4]}
`)

var lookupRoomOwner = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return {0}
end
return {1, redis.call('HGET', KEYS[1], 'instance_id'), redis.call('HGET', KEYS[1], 'internal_url'), redis.call('HGET', KEYS[1], 'lease_id'), redis.call('HGET', KEYS[1], 'fence'), redis.call('PTTL', KEYS[1])}
`)

var renewRoom = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 0 then
  return 0
end
if redis.call('HGET', KEYS[1], 'lease_id') ~= ARGV[1] or redis.call('HGET', KEYS[1], 'fence') ~= ARGV[2] then
  return 0
end
redis.call('PEXPIRE', KEYS[1], ARGV[3])
redis.call('PEXPIRE', KEYS[2], ARGV[4])
return 1
`)

var releaseRoom = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'lease_id') ~= ARGV[1] or redis.call('HGET', KEYS[1], 'fence') ~= ARGV[2] then
  return 0
end
return redis.call('DEL', KEYS[1])
`)

var updateOwnedGame = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'lease_id') ~= ARGV[1] or redis.call('HGET', KEYS[1], 'fence') ~= ARGV[2] then
  return -1
end
if redis.call('EXISTS', KEYS[2]) == 0 then
  return 0
end
redis.call('SET', KEYS[2], ARGV[3], 'PX', ARGV[4])
redis.call('SET', KEYS[3], ARGV[5], 'PX', ARGV[4])
redis.call('PEXPIRE', KEYS[4], ARGV[4])
return 1
`)

func (s *Store) AcquireRoom(ctx context.Context, gameID string, candidate application.RoomOwner, leaseTTL, metadataTTL time.Duration) (application.RoomLease, bool, error) {
	result, err := acquireRoom.Run(ctx, s.client,
		[]string{ownerKey(gameID), fenceKey(gameID), gameKey(gameID)},
		candidate.InstanceID, candidate.InternalURL, candidate.LeaseID,
		strconv.FormatInt(leaseTTL.Milliseconds(), 10), strconv.FormatInt(metadataTTL.Milliseconds(), 10),
	).Slice()
	if err != nil {
		s.observeRedisError(ctx, "acquire_room", err, "game_id", gameID)
		return application.RoomLease{}, false, err
	}
	if len(result) == 0 {
		err := errors.New("redis returned an empty room ownership result")
		s.observeRedisResultError(ctx, "acquire_room", err)
		return application.RoomLease{}, false, err
	}
	status, err := redisInt64(result[0])
	if err != nil {
		s.observeRedisResultError(ctx, "acquire_room", err)
		return application.RoomLease{}, false, err
	}
	if status == -1 {
		return application.RoomLease{}, false, game.ErrGameNotFound
	}
	owner, _, err := decodeOwnerResult(result)
	if err != nil {
		s.observeRedisResultError(ctx, "acquire_room", err)
		return application.RoomLease{}, false, err
	}
	return application.RoomLease{GameID: gameID, Owner: owner}, status == 1, nil
}

func (s *Store) LookupRoomOwner(ctx context.Context, gameID string) (application.RoomOwner, time.Duration, error) {
	result, err := lookupRoomOwner.Run(ctx, s.client, []string{ownerKey(gameID)}).Slice()
	if err != nil {
		s.observeRedisError(ctx, "lookup_room_owner", err, "game_id", gameID)
		return application.RoomOwner{}, 0, err
	}
	if len(result) == 0 {
		err := errors.New("redis returned an empty room owner lookup")
		s.observeRedisResultError(ctx, "lookup_room_owner", err)
		return application.RoomOwner{}, 0, err
	}
	found, err := redisInt64(result[0])
	if err != nil {
		s.observeRedisResultError(ctx, "lookup_room_owner", err)
		return application.RoomOwner{}, 0, err
	}
	if found == 0 {
		return application.RoomOwner{}, 0, nil
	}
	owner, ttl, err := decodeOwnerResult(result)
	if err != nil {
		s.observeRedisResultError(ctx, "lookup_room_owner", err)
	}
	return owner, ttl, err
}

func (s *Store) RenewRoom(ctx context.Context, lease application.RoomLease, leaseTTL, metadataTTL time.Duration) (bool, error) {
	result, err := renewRoom.Run(ctx, s.client,
		[]string{ownerKey(lease.GameID), fenceKey(lease.GameID), gameKey(lease.GameID)},
		lease.Owner.LeaseID, strconv.FormatUint(lease.Owner.Fence, 10),
		strconv.FormatInt(leaseTTL.Milliseconds(), 10), strconv.FormatInt(metadataTTL.Milliseconds(), 10),
	).Int()
	s.observeRedisError(ctx, "renew_room", err, "game_id", lease.GameID)
	return result == 1, err
}

func (s *Store) ReleaseRoom(ctx context.Context, lease application.RoomLease) error {
	_, err := releaseRoom.Run(ctx, s.client, []string{ownerKey(lease.GameID)},
		lease.Owner.LeaseID, strconv.FormatUint(lease.Owner.Fence, 10),
	).Int()
	s.observeRedisError(ctx, "release_room", err, "game_id", lease.GameID)
	return err
}

func (s *Store) UpdateOwned(ctx context.Context, value game.Game, ttl time.Duration, lease application.RoomLease) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal game: %w", err)
	}
	result, err := updateOwnedGame.Run(ctx, s.client,
		[]string{ownerKey(value.ID), gameKey(value.ID), codeKey(value.Code), fenceKey(value.ID)},
		lease.Owner.LeaseID, strconv.FormatUint(lease.Owner.Fence, 10), payload,
		strconv.FormatInt(ttl.Milliseconds(), 10), value.ID,
	).Int()
	if err != nil {
		s.observeRedisError(ctx, "update_owned", err, "game_id", value.ID)
		return err
	}
	switch result {
	case -1:
		return application.ErrLeaseLost
	case 0:
		return game.ErrGameNotFound
	default:
		return nil
	}
}

func decodeOwnerResult(result []any) (application.RoomOwner, time.Duration, error) {
	if len(result) < 6 {
		return application.RoomOwner{}, 0, fmt.Errorf("invalid room owner result length %d", len(result))
	}
	instanceID, err := redisString(result[1])
	if err != nil {
		return application.RoomOwner{}, 0, err
	}
	internalURL, err := redisString(result[2])
	if err != nil {
		return application.RoomOwner{}, 0, err
	}
	leaseID, err := redisString(result[3])
	if err != nil {
		return application.RoomOwner{}, 0, err
	}
	fence, err := redisInt64(result[4])
	if err != nil || fence < 1 {
		return application.RoomOwner{}, 0, fmt.Errorf("invalid room fencing token %v", result[4])
	}
	remainingMilliseconds, err := redisInt64(result[5])
	if err != nil {
		return application.RoomOwner{}, 0, err
	}
	return application.RoomOwner{
		InstanceID: instanceID, InternalURL: internalURL, LeaseID: leaseID, Fence: uint64(fence),
	}, time.Duration(remainingMilliseconds) * time.Millisecond, nil
}

func redisString(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		return "", fmt.Errorf("unexpected Redis string value %T", value)
	}
}

func redisInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected Redis integer value %T", value)
	}
}

func ownerKey(id string) string { return "game:room-owner:" + id }
func fenceKey(id string) string { return "game:room-fence:" + id }
