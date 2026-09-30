package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	redisstore "github.com/kungrem23/quizgo/services/game/internal/store/redis"
	websockettransport "github.com/kungrem23/quizgo/services/game/internal/transport/ws"
)

type distributedRoomRepository interface {
	application.GameRepository
	application.RoomOwnershipStore
}

type websocketOwnershipRepository struct {
	*repositoryStub
	mu     sync.Mutex
	owners map[string]websocketLease
	fences map[string]uint64
}

type websocketLease struct {
	lease     application.RoomLease
	expiresAt time.Time
}

func newWebSocketOwnershipRepository() *websocketOwnershipRepository {
	return &websocketOwnershipRepository{
		repositoryStub: &repositoryStub{},
		owners:         make(map[string]websocketLease),
		fences:         make(map[string]uint64),
	}
}

func (r *websocketOwnershipRepository) AcquireRoom(_ context.Context, gameID string, candidate application.RoomOwner, leaseTTL, _ time.Duration) (application.RoomLease, bool, error) {
	if _, err := r.GetByID(context.Background(), gameID); err != nil {
		return application.RoomLease{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, ok := r.current(gameID); ok {
		return current.lease, false, nil
	}
	r.fences[gameID]++
	candidate.Fence = r.fences[gameID]
	lease := application.RoomLease{GameID: gameID, Owner: candidate}
	r.owners[gameID] = websocketLease{lease: lease, expiresAt: time.Now().Add(leaseTTL)}
	return lease, true, nil
}

func (r *websocketOwnershipRepository) LookupRoomOwner(_ context.Context, gameID string) (application.RoomOwner, time.Duration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.current(gameID)
	if !ok {
		return application.RoomOwner{}, 0, nil
	}
	return current.lease.Owner, time.Until(current.expiresAt), nil
}

func (r *websocketOwnershipRepository) RenewRoom(_ context.Context, lease application.RoomLease, leaseTTL, _ time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.current(lease.GameID)
	if !ok || !sameWebSocketLease(current.lease, lease) {
		return false, nil
	}
	current.expiresAt = time.Now().Add(leaseTTL)
	r.owners[lease.GameID] = current
	return true, nil
}

func (r *websocketOwnershipRepository) ReleaseRoom(_ context.Context, lease application.RoomLease) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, ok := r.current(lease.GameID); ok && sameWebSocketLease(current.lease, lease) {
		delete(r.owners, lease.GameID)
	}
	return nil
}

func (r *websocketOwnershipRepository) UpdateOwned(ctx context.Context, value game.Game, ttl time.Duration, lease application.RoomLease) error {
	r.mu.Lock()
	current, ok := r.current(value.ID)
	if !ok || !sameWebSocketLease(current.lease, lease) {
		r.mu.Unlock()
		return application.ErrLeaseLost
	}
	err := r.repositoryStub.Update(ctx, value, ttl)
	r.mu.Unlock()
	return err
}

func (r *websocketOwnershipRepository) current(gameID string) (websocketLease, bool) {
	current, ok := r.owners[gameID]
	if ok && !time.Now().Before(current.expiresAt) {
		delete(r.owners, gameID)
		return websocketLease{}, false
	}
	return current, ok
}

func sameWebSocketLease(left, right application.RoomLease) bool {
	return left.GameID == right.GameID && left.Owner.LeaseID == right.Owner.LeaseID && left.Owner.Fence == right.Owner.Fence
}

func TestRoomRouterProxiesToOwnerAndReconnectsAfterTakeover(t *testing.T) {
	testRoomRouterProxiesToOwnerAndReconnectsAfterTakeover(t, newWebSocketOwnershipRepository())
}

func TestRoomRouterTwoReplicasWithRedis(t *testing.T) {
	address := os.Getenv("GAME_TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("GAME_TEST_REDIS_ADDRESS is not set")
	}
	repository := redisstore.New(redisstore.Config{Address: address})
	defer repository.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := repository.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	testRoomRouterProxiesToOwnerAndReconnectsAfterTakeover(t, repository)
}

func testRoomRouterProxiesToOwnerAndReconnectsAfterTakeover(t *testing.T, repository distributedRoomRepository) {
	t.Helper()
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	const gameTTL = 10 * time.Second
	created, err := application.New(catalogStub{snapshot: snapshot}, repository, gameTTL).CreateGame(
		context.Background(), application.CreateGameRequest{QuizID: 8, AccessToken: "token"},
	)
	if err != nil {
		t.Fatal(err)
	}

	hubA, serverA := newDistributedWebSocketServer(t, repository, gameTTL, "replica-a")
	hubB, serverB := newDistributedWebSocketServer(t, repository, gameTTL, "replica-b")
	defer func() {
		hubA.Close()
		hubB.Close()
		serverA.Close()
		serverB.Close()
	}()

	ownerSession, err := hubA.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ownerSession.Events:
	case <-time.After(time.Second):
		t.Fatal("owner actor did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	remoteURL := "ws" + strings.TrimPrefix(serverB.URL, "http") + "?game_id=" + url.QueryEscape(created.Game.ID)
	connection, _, err := websocket.Dial(ctx, remoteURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	writeClientMessage(t, ctx, connection, "host_auth", "remote-auth", map[string]any{
		"game_id": created.Game.ID, "ticket": created.HostTicket,
	})
	waitForMessage(t, ctx, connection, "authenticated")
	waitForMessage(t, ctx, connection, "state")
	_ = connection.Close(websocket.StatusNormalClosure, "first owner verified")

	joinURL := "ws" + strings.TrimPrefix(serverB.URL, "http") + "?code=" + url.QueryEscape(created.Game.Code)
	playerConnection, joined := joinPlayer(t, ctx, joinURL, created.Game.Code, "Alice")
	_ = playerConnection.Close(websocket.StatusNormalClosure, "reconnect through new owner")

	// Graceful owner shutdown releases the lease immediately. The same public
	// replica can then acquire the persisted room and authenticate the player.
	hubA.Close()
	reconnect, _, err := websocket.Dial(ctx, remoteURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reconnect.Close(websocket.StatusNormalClosure, "test complete")
	writeClientMessage(t, ctx, reconnect, "player_auth", "takeover-auth", map[string]any{
		"game_id": created.Game.ID, "participant_id": joined.PlayerID, "ticket": joined.Ticket,
	})
	waitForMessage(t, ctx, reconnect, "authenticated")
	state := waitForMessage(t, ctx, reconnect, "state")
	if !strings.Contains(string(state.Payload), joined.PlayerID) {
		t.Fatalf("restored state does not contain player: %s", state.Payload)
	}
	owner, _, err := repository.LookupRoomOwner(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if owner.InstanceID != "replica-b" {
		t.Fatalf("owner after takeover = %q, want replica-b", owner.InstanceID)
	}
}

func newDistributedWebSocketServer(t *testing.T, repository distributedRoomRepository, ttl time.Duration, instanceID string) (*application.Hub, *httptest.Server) {
	t.Helper()
	var realtime http.Handler
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		realtime.ServeHTTP(writer, request)
	}))
	internalURL := "http://" + server.Listener.Addr().String()
	hub := application.NewDistributedHub(repository, repository, ttl, application.OwnershipOptions{
		InstanceID: instanceID, InternalURL: internalURL,
		LeaseTTL: 2 * time.Second, RenewInterval: 200 * time.Millisecond, SafetyMargin: 50 * time.Millisecond,
	})
	handler := websockettransport.New(hub, testLogger())
	realtime = websockettransport.NewRoomRouter(hub, handler, testLogger())
	server.Start()
	return hub, server
}
