package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	Address  string
	Password string
	DB       int
}

type Store struct {
	client *redis.Client
}

var saveGame = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then
  return 0
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SET', KEYS[2], ARGV[3], 'PX', ARGV[2])
return 1
`)

func New(config Config) *Store {
	return &Store{client: redis.NewClient(&redis.Options{
		Addr: config.Address, Password: config.Password, DB: config.DB, Protocol: 2,
	})}
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.client == nil {
		return errors.New("redis store is not configured")
	}
	return s.client.Ping(ctx).Err()
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
		return err
	}
	if result == 0 {
		return game.ErrJoinCodeTaken
	}
	return nil
}

func (s *Store) GetByID(ctx context.Context, id string) (game.Game, error) {
	payload, err := s.client.Get(ctx, gameKey(id)).Bytes()
	if err != nil {
		return game.Game{}, err
	}
	var result game.Game
	if err := json.Unmarshal(payload, &result); err != nil {
		return game.Game{}, fmt.Errorf("decode game: %w", err)
	}
	return result, nil
}

func gameKey(id string) string   { return "game:session:" + id }
func codeKey(code string) string { return "game:code:" + code }
