package quizserver

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	quizv1 "github.com/kungrem23/quizgo/gen/quiz/v1"
	"github.com/kungrem23/quizgo/services/quiz/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type QuizService interface {
	GetPlayableQuiz(context.Context, int, int) (quiz.QuizContent, error)
}

type Server struct {
	quizv1.UnimplementedQuizCatalogServiceServer
	service QuizService
}

func New(service QuizService) *Server {
	return &Server{service: service}
}

func (s *Server) GetPlayableQuiz(ctx context.Context, request *quizv1.GetPlayableQuizRequest) (*quizv1.GetPlayableQuizResponse, error) {
	if request.GetQuizId() < 1 || request.GetQuizId() > int64(math.MaxInt) {
		return nil, status.Error(codes.InvalidArgument, "invalid quiz id")
	}
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	content, err := s.service.GetPlayableQuiz(ctx, int(request.GetQuizId()), principal.UserID)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
		case errors.Is(err, context.Canceled):
			return nil, status.Error(codes.Canceled, "request canceled")
		case errors.Is(err, sql.ErrNoRows):
			return nil, status.Error(codes.NotFound, "quiz not found")
		case errors.Is(err, quiz.ErrNotQuizOwner):
			return nil, status.Error(codes.PermissionDenied, "quiz belongs to another user")
		case errors.Is(err, quiz.ErrQuizNotPlayable):
			return nil, status.Error(codes.FailedPrecondition, "quiz is not playable")
		default:
			return nil, status.Error(codes.Internal, "internal error")
		}
	}
	return &quizv1.GetPlayableQuizResponse{Quiz: mapPlayableQuiz(content)}, nil
}

func mapPlayableQuiz(content quiz.QuizContent) *quizv1.PlayableQuiz {
	result := &quizv1.PlayableQuiz{
		Id: int64(content.ID), Revision: content.Revision, Title: content.Title, OwnerUserId: int64(content.AuthorID),
		Questions: make([]*quizv1.PlayableQuestion, 0, len(content.Questions)),
	}
	for _, question := range content.Questions {
		mapped := &quizv1.PlayableQuestion{
			Id: int64(question.ID), Text: question.TextContent, ImageId: question.ImageID,
			TimeLimitSeconds: uint32(question.TimeLimit),
			Answers:          make([]*quizv1.PlayableAnswer, 0, len(question.Answers)),
		}
		for _, answer := range question.Answers {
			mapped.Answers = append(mapped.Answers, &quizv1.PlayableAnswer{
				Id: int64(answer.ID), Text: answer.TextContent, IsCorrect: answer.IsCorrect,
			})
		}
		result.Questions = append(result.Questions, mapped)
	}
	return result
}
