package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	quizv1 "github.com/kungrem23/quizgo/gen/quiz/v1"
	"github.com/kungrem23/quizgo/internal/authn"
	"github.com/kungrem23/quizgo/internal/config"
	"github.com/kungrem23/quizgo/internal/domain/quiz"
	quizgrpc "github.com/kungrem23/quizgo/internal/grpc/quizserver"
	quizhttp "github.com/kungrem23/quizgo/internal/http"
	"github.com/kungrem23/quizgo/internal/http/middleware"
	"github.com/kungrem23/quizgo/internal/platform/postgres"
	imagestore "github.com/kungrem23/quizgo/internal/store/s3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	settings, err := config.LoadQuizService()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	tokens, err := authn.NewManager(authn.Config{
		Secret: settings.JWT.Secret, Issuer: settings.JWT.Issuer,
		Audience: settings.JWT.Audience, TTL: settings.JWT.TTL,
	})
	if err != nil {
		return fmt.Errorf("configure access tokens: %w", err)
	}
	var images quiz.ImageStore
	if settings.S3.Bucket != "" {
		images, err = imagestore.New(context.Background(), settings.S3)
		if err != nil {
			return fmt.Errorf("configure image storage: %w", err)
		}
	} else {
		log.Print("S3_BUCKET is unset: image uploads are unavailable")
	}
	db, err := postgres.NewDBConnection(postgres.Config{
		Host: settings.Postgres.Host, Port: settings.Postgres.Port, User: settings.Postgres.User,
		Password: settings.Postgres.Password, Database: settings.Postgres.Database, SSLMode: settings.Postgres.SSLMode,
	}, images)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	service := quiz.NewServiceWithTokens(quiz.NewPostgresRepository(db), images, tokens)
	router := quizhttp.NewQuizRouterWithReadiness(service, db.PingContext)
	if directory := os.Getenv("QUIZ_FRONTEND_DIR"); directory != "" {
		router, err = quizhttp.WithFrontend(router, directory)
		if err != nil {
			return err
		}
	}
	router = middleware.RequestLogger(router, log.Default())
	httpServer := &http.Server{
		Addr:              ":" + settings.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	grpcOptions := []grpc.ServerOption{grpc.ChainUnaryInterceptor(
		quizgrpc.UnaryLoggingInterceptor(log.Default()),
		quizgrpc.UnaryRecoveryInterceptor(log.Default()),
		quizgrpc.UnaryAuthInterceptor(service, settings.GRPC.ServiceToken, settings.GRPC.AllowedClientURI),
	)}
	transportOption, err := quizgrpc.TransportCredentialsOption(
		settings.GRPC.TLSCertFile, settings.GRPC.TLSKeyFile, settings.GRPC.TLSClientCAFile,
	)
	if err != nil {
		return err
	}
	if transportOption != nil {
		grpcOptions = append(grpcOptions, transportOption)
	} else {
		log.Print("warning: gRPC transport is plaintext; configure mTLS before exposing it outside a trusted development network")
	}
	grpcServer := grpc.NewServer(grpcOptions...)
	quizv1.RegisterQuizCatalogServiceServer(grpcServer, quizgrpc.New(service))
	healthServer := health.NewServer()
	healthServer.SetServingStatus("quiz.v1.QuizCatalogService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	grpcListener, err := net.Listen("tcp", ":"+settings.GRPC.Port)
	if err != nil {
		return fmt.Errorf("listen gRPC: %w", err)
	}

	errCh := make(chan error, 2)
	go func() {
		log.Printf("quiz HTTP service started on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve HTTP: %w", err)
		}
	}()
	go func() {
		log.Printf("quiz gRPC service started on %s", grpcListener.Addr())
		if err := grpcServer.Serve(grpcListener); err != nil {
			errCh <- fmt.Errorf("serve gRPC: %w", err)
		}
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var serveErr error
	select {
	case <-signalCtx.Done():
		log.Print("shutting down quiz service")
	case serveErr = <-errCh:
	}
	healthServer.SetServingStatus("quiz.v1.QuizCatalogService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	return serveErr
}
