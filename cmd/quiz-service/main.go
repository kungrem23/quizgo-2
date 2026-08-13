package quizservice

import (
	"log"
	"net/http"

	"github.com/kungrem23/quizgo/internal/domain/quiz"
	quizhttp "github.com/kungrem23/quizgo/internal/http"
	"github.com/kungrem23/quizgo/internal/platform/postgres"
)

func main() {
	db := postgres.NewDBConnection()
	defer db.Close()
	pgRepo := quiz.NewPostgresRepository(db)
	service := quiz.NewService(pgRepo)
	router := quizhttp.NewQuizRouter(service)

	log.Println("server started on port :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
