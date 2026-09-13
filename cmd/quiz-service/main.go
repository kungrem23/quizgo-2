package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/kungrem23/quizgo/internal/config"
	"github.com/kungrem23/quizgo/internal/domain/quiz"
	quizhttp "github.com/kungrem23/quizgo/internal/http"
	"github.com/kungrem23/quizgo/internal/platform/postgres"
	imagestore "github.com/kungrem23/quizgo/internal/store/s3"
)

func main() {
	config, err := config.LoadQuizService()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	var images quiz.ImageStore
	if config.S3.Bucket != "" {
		images, err = imagestore.New(context.Background(), config.S3)
		if err != nil {
			log.Fatalf("configure image storage: %v", err)
		}
	} else {
		log.Print("S3_BUCKET is unset: image uploads are unavailable")
	}
	db, err := postgres.NewDBConnection(postgres.Config{
		Host:     config.Postgres.Host,
		Port:     config.Postgres.Port,
		User:     config.Postgres.User,
		Password: config.Postgres.Password,
		Database: config.Postgres.Database,
		SSLMode:  config.Postgres.SSLMode,
	}, images)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()
	pgRepo := quiz.NewPostgresRepository(db)
	service := quiz.NewService(pgRepo, images)
	router := quizhttp.NewQuizRouter(service)
	if directory := os.Getenv("QUIZ_FRONTEND_DIR"); directory != "" {
		router, err = quizhttp.WithFrontend(router, directory)
		if err != nil {
			log.Fatal(err)
		}
	}

	address := ":" + config.HTTPPort
	log.Printf("quiz service started on %s", address)
	if err := http.ListenAndServe(address, router); err != nil {
		log.Fatal(err)
	}
}
