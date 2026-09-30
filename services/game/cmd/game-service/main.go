package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kungrem23/quizgo/services/game/internal/application"
	"github.com/kungrem23/quizgo/services/game/internal/client/quizgrpc"
	"github.com/kungrem23/quizgo/services/game/internal/config"
	"github.com/kungrem23/quizgo/services/game/internal/observability"
	redisstore "github.com/kungrem23/quizgo/services/game/internal/store/redis"
	httptransport "github.com/kungrem23/quizgo/services/game/internal/transport/http"
	websockettransport "github.com/kungrem23/quizgo/services/game/internal/transport/ws"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("game_service_failed", "component", "main", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	settings, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	quizConnection, err := quizgrpc.Dial(settings.Quiz.Address, quizgrpc.TLSConfig{
		CAFile: settings.Quiz.CAFile, CertFile: settings.Quiz.CertFile,
		KeyFile: settings.Quiz.KeyFile, ServerName: settings.Quiz.ServerName,
	})
	if err != nil {
		return err
	}
	defer quizConnection.Close()
	if settings.Quiz.CAFile == "" {
		logger.Warn("quiz_grpc_plaintext", "component", "quiz_grpc", "address", settings.Quiz.Address)
	}

	registry, metrics := observability.NewRegistry()
	games := redisstore.New(redisstore.Config{
		Address: settings.Redis.Address, Password: settings.Redis.Password, DB: settings.Redis.DB,
		Logger: logger, Metrics: metrics,
	})
	defer games.Close()
	gameService := application.New(quizgrpc.New(quizConnection, settings.Quiz.ServiceToken), games, settings.GameTTL)
	hub := application.NewDistributedHub(games, games, settings.GameTTL, application.OwnershipOptions{
		InstanceID: settings.Ownership.InstanceID, InternalURL: settings.Ownership.InternalURL,
		LeaseTTL: settings.Ownership.LeaseTTL, RenewInterval: settings.Ownership.RenewInterval,
		SafetyMargin: settings.Ownership.SafetyMargin, Logger: logger, Metrics: metrics,
	})
	defer hub.Close()
	healthClient := grpc_health_v1.NewHealthClient(quizConnection)
	websocketHandler := websockettransport.NewWithOptions(hub, logger, websockettransport.Options{
		MaxMessageBytes:     settings.WebSocket.MaxMessageBytes,
		HandshakeTimeout:    settings.WebSocket.HandshakeTimeout,
		CommandTimeout:      settings.WebSocket.CommandTimeout,
		WriteTimeout:        settings.WebSocket.WriteTimeout,
		PingInterval:        settings.WebSocket.PingInterval,
		PongTimeout:         settings.WebSocket.PongTimeout,
		PlayerCommandRate:   settings.WebSocket.PlayerCommandRate,
		PlayerCommandBurst:  settings.WebSocket.PlayerCommandBurst,
		HostCommandRate:     settings.WebSocket.HostCommandRate,
		HostCommandBurst:    settings.WebSocket.HostCommandBurst,
		InvalidMessageLimit: settings.WebSocket.InvalidMessageLimit,
		Metrics:             metrics,
	})
	realtimeRouter := websockettransport.NewRoomRouter(hub, websocketHandler, logger)
	router := httptransport.NewWithMetrics(gameService, realtimeRouter, observability.Handler(registry),
		httptransport.Dependency{Name: "redis", Check: withTimeout(games.Ping)},
		httptransport.Dependency{Name: "quiz", Check: withTimeout(func(ctx context.Context) error {
			response, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "quiz.v1.QuizCatalogService"})
			if err != nil {
				return err
			}
			if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
				return fmt.Errorf("quiz service status is %s", response.GetStatus())
			}
			return nil
		})},
	)
	server := &http.Server{
		Addr: ":" + settings.HTTPPort, Handler: router,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("game_service_started",
			"component", "main", "address", server.Addr, "instance_id", settings.Ownership.InstanceID,
			"internal_url", settings.Ownership.InternalURL, "lease_ttl", settings.Ownership.LeaseTTL,
			"lease_renew_interval", settings.Ownership.RenewInterval,
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve game HTTP: %w", err)
		}
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var serveErr error
	select {
	case <-signalCtx.Done():
		logger.Info("game_service_shutdown_started", "component", "main", "reason", signalCtx.Err())
	case serveErr = <-errCh:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Upgraded WebSocket connections are not drained by net/http. Stop room
	// actors first so leases are no longer renewed and can be released while
	// Redis is still available.
	hub.Close()
	if err := server.Shutdown(shutdownCtx); err != nil && serveErr == nil {
		serveErr = err
	}
	return serveErr
}

func withTimeout(check func(context.Context) error) func(context.Context) error {
	return func(parent context.Context) error {
		ctx, cancel := context.WithTimeout(parent, 2*time.Second)
		defer cancel()
		return check(ctx)
	}
}
