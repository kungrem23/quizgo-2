package http

import (
	stdhttp "net/http"

	"github.com/kungrem23/quizgo/internal/domain/quiz"
	"github.com/kungrem23/quizgo/internal/http/handlers/auth"
	quizhandler "github.com/kungrem23/quizgo/internal/http/handlers/quiz"
	"github.com/kungrem23/quizgo/internal/http/middleware"
)

func NewQuizRouter(service *quiz.Service) stdhttp.Handler {
	router := stdhttp.NewServeMux()

	authHandler := auth.NewAuthHandler(service)
	quizHandler := quizhandler.NewQuizHandler(service)
	questionHandler := quizhandler.NewQuestionHandler(service)
	answerHandler := quizhandler.NewAnswerHandler(service)
	userHandler := quizhandler.NewUserHandler(service)

	router.HandleFunc("POST /auth/login", authHandler.Login)
	router.HandleFunc("POST /auth/register", authHandler.Register)

	router.Handle("POST /api/quizzes", authenticated(quizHandler.CreateQuiz))
	router.HandleFunc("GET /api/quizzes", quizHandler.ListQuizzes)
	router.HandleFunc("GET /api/quizzes/{id}", quizHandler.GetQuiz)
	router.HandleFunc("GET /api/users/{authorId}/quizzes", quizHandler.ListQuizzesByAuthor)

	router.Handle("POST /api/questions", authenticated(questionHandler.CreateQuestion))
	router.HandleFunc("GET /api/questions/{id}", questionHandler.GetQuestion)
	router.Handle("DELETE /api/questions/{id}", authenticated(questionHandler.DeleteQuestion))
	router.Handle("PATCH /api/questions/{id}/position", authenticated(questionHandler.ChangeQuestionPosition))

	router.Handle("POST /api/answers", authenticated(answerHandler.CreateAnswer))
	router.HandleFunc("GET /api/answers/{id}", answerHandler.GetAnswer)
	router.Handle("GET /api/answers/{id}/author", authenticated(answerHandler.GetAnswerAsAuthor))
	router.Handle("DELETE /api/answers/{id}", authenticated(answerHandler.DeleteAnswer))

	router.HandleFunc("GET /api/users/{id}", userHandler.GetUser)

	return router
}

func authenticated(handler stdhttp.HandlerFunc) stdhttp.Handler {
	return middleware.Auth(handler)
}
