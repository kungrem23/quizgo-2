package ws

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/kungrem23/quizgo/services/game/internal/application"
)

const (
	internalGameHeader        = "X-Quizgo-Room-Id"
	internalLeaseHeader       = "X-Quizgo-Room-Lease"
	internalCorrelationHeader = "X-Quizgo-Correlation-Id"
)

type expectedRoom struct {
	gameID string
	code   string
}

type expectedRoomContextKey struct{}

// RoomRouter keeps the public endpoint replica-agnostic. It acquires an
// unowned room locally or proxies the untouched Upgrade request to its owner.
type RoomRouter struct {
	hub     *application.Hub
	handler http.Handler
	logger  *slog.Logger
}

func NewRoomRouter(hub *application.Hub, handler http.Handler, logger *slog.Logger) *RoomRouter {
	return &RoomRouter{hub: hub, handler: handler, logger: logger}
}

func (r *RoomRouter) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if r == nil || r.hub == nil || r.handler == nil {
		http.Error(writer, "realtime service unavailable", http.StatusServiceUnavailable)
		return
	}
	if gameID := request.Header.Get(internalGameHeader); gameID != "" {
		r.serveInternal(writer, request, gameID)
		return
	}
	request.Header.Set(internalCorrelationHeader, uuid.NewString())

	gameID := strings.TrimSpace(request.URL.Query().Get("game_id"))
	code := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("code")))
	if gameID == "" && code == "" {
		// Backward-compatible path. If the room is remote, the handler returns a
		// retryable ownership error after inspecting the first message.
		r.handler.ServeHTTP(writer, request)
		return
	}
	if gameID != "" && code != "" {
		http.Error(writer, "provide either game_id or code", http.StatusBadRequest)
		return
	}

	var route application.RoomRoute
	var err error
	if gameID != "" {
		route, err = r.hub.RouteByID(request.Context(), gameID)
	} else {
		route, err = r.hub.RouteByCode(request.Context(), code)
	}
	if err != nil {
		http.Error(writer, "room unavailable", http.StatusServiceUnavailable)
		return
	}
	expected := expectedRoom{gameID: route.GameID, code: code}
	if route.Local {
		r.handler.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), expectedRoomContextKey{}, expected)))
		return
	}
	r.proxyToOwner(writer, request, route, expected)
}

func (r *RoomRouter) serveInternal(writer http.ResponseWriter, request *http.Request, gameID string) {
	leaseID := request.Header.Get(internalLeaseHeader)
	if leaseID == "" {
		http.Error(writer, "missing room lease", http.StatusBadRequest)
		return
	}
	route, err := r.hub.RouteByID(request.Context(), gameID)
	if err != nil || !route.Local || route.Owner.LeaseID != leaseID {
		if r.logger != nil {
			r.logger.WarnContext(request.Context(), "websocket_internal_route_rejected",
				"component", "websocket_router", "connection_id", request.Header.Get(internalCorrelationHeader),
				"game_id", gameID, "error", err,
			)
		}
		http.Error(writer, "room owner changed", http.StatusServiceUnavailable)
		return
	}
	expected := expectedRoom{
		gameID: route.GameID,
		code:   strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("code"))),
	}
	request.Header.Del(internalGameHeader)
	request.Header.Del(internalLeaseHeader)
	r.handler.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), expectedRoomContextKey{}, expected)))
}

func (r *RoomRouter) proxyToOwner(writer http.ResponseWriter, request *http.Request, route application.RoomRoute, expected expectedRoom) {
	target, err := url.Parse(route.Owner.InternalURL)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		http.Error(writer, "room owner is unavailable", http.StatusServiceUnavailable)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	direct := proxy.Director
	proxy.Director = func(outbound *http.Request) {
		direct(outbound)
		outbound.Header.Set(internalGameHeader, expected.gameID)
		outbound.Header.Set(internalLeaseHeader, route.Owner.LeaseID)
	}
	if r.logger != nil {
		r.logger.DebugContext(request.Context(), "websocket_proxying_to_room_owner",
			"component", "websocket_router", "connection_id", request.Header.Get(internalCorrelationHeader),
			"game_id", expected.gameID, "owner_instance_id", route.Owner.InstanceID,
		)
	}
	proxy.ErrorHandler = func(response http.ResponseWriter, failedRequest *http.Request, proxyErr error) {
		if r.logger != nil {
			r.logger.ErrorContext(failedRequest.Context(), "websocket_owner_proxy_failed",
				"component", "websocket_router", "connection_id", failedRequest.Header.Get(internalCorrelationHeader),
				"game_id", expected.gameID, "owner_instance_id", route.Owner.InstanceID, "error", proxyErr,
			)
		}
		http.Error(response, "room owner is unavailable", http.StatusServiceUnavailable)
	}
	proxy.ServeHTTP(writer, request)
}

func roomExpectedByContext(ctx context.Context) expectedRoom {
	value, _ := ctx.Value(expectedRoomContextKey{}).(expectedRoom)
	return value
}
