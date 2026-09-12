package main

import (
	"log"
	"net/http"

	"github.com/kungrem23/quizgo/internal/config"
	"github.com/kungrem23/quizgo/internal/domain/quiz"
	quizhttp "github.com/kungrem23/quizgo/internal/http"
	"github.com/kungrem23/quizgo/internal/platform/postgres"
)

func main() {
	config, err := config.LoadQuizService()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	db, err := postgres.NewDBConnection(postgres.Config{
		Host:     config.Postgres.Host,
		Port:     config.Postgres.Port,
		User:     config.Postgres.User,
		Password: config.Postgres.Password,
		Database: config.Postgres.Database,
		SSLMode:  config.Postgres.SSLMode,
	})
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()
	pgRepo := quiz.NewPostgresRepository(db)
	service := quiz.NewService(pgRepo)
	router := quizhttp.NewQuizRouter(service)

	address := ":" + config.HTTPPort
	log.Printf("quiz service started on %s", address)
	if err := http.ListenAndServe(address, router); err != nil {
		log.Fatal(err)
	}
}
