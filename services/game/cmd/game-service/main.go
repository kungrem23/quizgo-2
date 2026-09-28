package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kungrem23/quizgo/services/game/internal/application"
	"github.com/kungrem23/quizgo/services/game/internal/client/quizgrpc"
	"github.com/kungrem23/quizgo/services/game/internal/config"
	redisstore "github.com/kungrem23/quizgo/services/game/internal/store/redis"
	httptransport "github.com/kungrem23/quizgo/services/game/internal/transport/http"
	websockettransport "github.com/kungrem23/quizgo/services/game/internal/transport/ws"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
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
		log.Print("warning: quiz gRPC transport is plaintext; configure mTLS outside a trusted development network")
	}

	games := redisstore.New(redisstore.Config{
		Address: settings.Redis.Address, Password: settings.Redis.Password, DB: settings.Redis.DB,
	})
	defer games.Close()
	gameService := application.New(quizgrpc.New(quizConnection, settings.Quiz.ServiceToken), games, settings.GameTTL)
	hub := application.NewHub(games, settings.GameTTL)
	defer hub.Close()
	healthClient := grpc_health_v1.NewHealthClient(quizConnection)
	router := httptransport.New(gameService, websockettransport.New(hub, log.Default()),
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
		log.Printf("game HTTP service started on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve game HTTP: %w", err)
		}
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var serveErr error
	select {
	case <-signalCtx.Done():
		log.Print("shutting down game service")
	case serveErr = <-errCh:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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
