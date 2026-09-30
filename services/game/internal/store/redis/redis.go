package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	"github.com/kungrem23/quizgo/services/game/internal/observability"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	Address  string
	Password string
	DB       int
	Logger   *slog.Logger
	Metrics  *observability.Metrics
}

type Store struct {
	client  *redis.Client
	logger  *slog.Logger
	metrics *observability.Metrics
}

var saveGame = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then
  return 0
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SET', KEYS[2], ARGV[3], 'PX', ARGV[2])
return 1
`)

var updateGame = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return 0
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SET', KEYS[2], ARGV[3], 'PX', ARGV[2])
return 1
`)

func New(config Config) *Store {
	return &Store{
		client: redis.NewClient(&redis.Options{
			Addr: config.Address, Password: config.Password, DB: config.DB, Protocol: 2,
		}),
		logger: config.Logger, metrics: config.Metrics,
	}
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.client == nil {
		return errors.New("redis store is not configured")
	}
	err := s.client.Ping(ctx).Err()
	s.observeRedisError(ctx, "ping", err)
	return err
}

func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *Store) Save(ctx context.Context, value game.Game, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal game: %w", err)
	}
	result, err := saveGame.Run(ctx, s.client,
		[]string{gameKey(value.ID), codeKey(value.Code)}, payload, strconv.FormatInt(ttl.Milliseconds(), 10), value.ID,
	).Int()
	if err != nil {
		s.observeRedisError(ctx, "save", err, "game_id", value.ID)
		return err
	}
	if result == 0 {
		return game.ErrJoinCodeTaken
	}
	return nil
}

func (s *Store) Update(ctx context.Context, value game.Game, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal game: %w", err)
	}
	result, err := updateGame.Run(ctx, s.client,
		[]string{gameKey(value.ID), codeKey(value.Code)}, payload, strconv.FormatInt(ttl.Milliseconds(), 10), value.ID,
	).Int()
	if err != nil {
		s.observeRedisError(ctx, "update", err, "game_id", value.ID)
		return err
	}
	if result == 0 {
		return game.ErrGameNotFound
	}
	return nil
}

func (s *Store) GetByID(ctx context.Context, id string) (game.Game, error) {
	payload, err := s.client.Get(ctx, gameKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return game.Game{}, game.ErrGameNotFound
		}
		s.observeRedisError(ctx, "get_by_id", err, "game_id", id)
		return game.Game{}, err
	}
	var result game.Game
	if err := json.Unmarshal(payload, &result); err != nil {
		s.observeRedisResultError(ctx, "get_by_id", err, "game_id", id)
		return game.Game{}, fmt.Errorf("decode game: %w", err)
	}
	return result, nil
}

func (s *Store) GetByCode(ctx context.Context, code string) (game.Game, error) {
	id, err := s.client.Get(ctx, codeKey(code)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return game.Game{}, game.ErrGameNotFound
		}
		s.observeRedisError(ctx, "get_by_code", err)
		return game.Game{}, err
	}
	return s.GetByID(ctx, id)
}

func (s *Store) observeRedisError(ctx context.Context, operation string, err error, attributes ...any) {
	if err == nil || errors.Is(err, redis.Nil) {
		return
	}
	if s.metrics != nil {
		s.metrics.RedisError(operation)
	}
	if s.logger != nil {
		fields := []any{"component", "redis", "operation", operation, "error", err}
		fields = append(fields, attributes...)
		s.logger.ErrorContext(ctx, "redis_operation_failed", fields...)
	}
}

func (s *Store) observeRedisResultError(ctx context.Context, operation string, err error, attributes ...any) {
	if err == nil {
		return
	}
	if s.metrics != nil {
		s.metrics.RedisError(operation)
	}
	if s.logger != nil {
		fields := []any{"component", "redis", "operation", operation, "error", err}
		fields = append(fields, attributes...)
		s.logger.ErrorContext(ctx, "redis_result_invalid", fields...)
	}
}

func gameKey(id string) string   { return "game:session:" + id }
func codeKey(code string) string { return "game:code:" + code }
