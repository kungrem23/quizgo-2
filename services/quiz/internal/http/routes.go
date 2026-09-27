package http

import (
	"context"
	stdhttp "net/http"
	"time"

	_ "github.com/kungrem23/quizgo/services/quiz/docs"
	"github.com/kungrem23/quizgo/services/quiz/internal/domain"
	"github.com/kungrem23/quizgo/services/quiz/internal/http/handlers/auth"
	quizhandler "github.com/kungrem23/quizgo/services/quiz/internal/http/handlers/quiz"
	"github.com/kungrem23/quizgo/services/quiz/internal/http/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func NewQuizRouter(service *quiz.Service) stdhttp.Handler {
	return NewQuizRouterWithReadiness(service, nil)
}

func NewQuizRouterWithReadiness(service *quiz.Service, ready func(context.Context) error) stdhttp.Handler {
	router := stdhttp.NewServeMux()

	router.HandleFunc("GET /healthz", func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	})
	router.HandleFunc("GET /readyz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if ready == nil {
			w.WriteHeader(stdhttp.StatusOK)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			stdhttp.Error(w, "not ready", stdhttp.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(stdhttp.StatusOK)
	})
	router.HandleFunc("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("doc.json")))

	authHandler := auth.NewAuthHandler(service)
	quizHandler := quizhandler.NewQuizHandler(service)
	questionHandler := quizhandler.NewQuestionHandler(service)
	answerHandler := quizhandler.NewAnswerHandler(service)
	userHandler := quizhandler.NewUserHandler(service)
	editor := quizhandler.NewEditorHandler(service)
	router.Handle("GET /api/me", authenticated(service, editor.Me))
	router.Handle("GET /api/me/quizzes", authenticated(service, editor.MyQuizzes))
	router.HandleFunc("GET /api/quizzes/{id}/content", editor.GetContent)
	router.Handle("GET /api/quizzes/{id}/author", authenticated(service, editor.GetAuthorContent))
	router.Handle("PUT /api/quizzes/{id}/content", authenticated(service, editor.SaveContent))
	router.Handle("DELETE /api/quizzes/{id}", authenticated(service, editor.DeleteQuiz))
	router.Handle("POST /api/images", authenticated(service, editor.UploadImage))
	router.HandleFunc("GET /api/images/{id}", editor.ReadImage)
	router.HandleFunc("GET /api/images/{id}/url", editor.ImageURL)
	router.Handle("DELETE /api/images/{id}", authenticated(service, editor.DeleteImage))

	router.HandleFunc("POST /auth/login", authHandler.Login)
	router.HandleFunc("POST /auth/register", authHandler.Register)

	router.Handle("POST /api/quizzes", authenticated(service, quizHandler.CreateQuiz))
	router.HandleFunc("GET /api/quizzes", quizHandler.ListQuizzes)
	router.HandleFunc("GET /api/quizzes/{id}", quizHandler.GetQuiz)
	router.HandleFunc("GET /api/users/{authorId}/quizzes", quizHandler.ListQuizzesByAuthor)

	router.Handle("POST /api/questions", authenticated(service, questionHandler.CreateQuestion))
	router.HandleFunc("GET /api/questions/{id}", questionHandler.GetQuestion)
	router.Handle("DELETE /api/questions/{id}", authenticated(service, questionHandler.DeleteQuestion))
	router.Handle("PATCH /api/questions/{id}/position", authenticated(service, questionHandler.ChangeQuestionPosition))

	router.Handle("POST /api/answers", authenticated(service, answerHandler.CreateAnswer))
	router.HandleFunc("GET /api/answers/{id}", answerHandler.GetAnswer)
	router.Handle("GET /api/answers/{id}/author", authenticated(service, answerHandler.GetAnswerAsAuthor))
	router.Handle("DELETE /api/answers/{id}", authenticated(service, answerHandler.DeleteAnswer))

	router.HandleFunc("GET /api/users/{id}", userHandler.GetUser)

	return router
}

func authenticated(verifier middleware.AccessTokenVerifier, handler stdhttp.HandlerFunc) stdhttp.Handler {
	return middleware.Auth(verifier, handler)
}
