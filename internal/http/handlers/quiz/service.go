package quizhandler

import (
	"context"
	"github.com/kungrem23/quizgo/internal/domain/quiz"
)

type quizService interface {
	GetQuiz(ctx context.Context, id int) (quiz.Quiz, error)
	CreateQuiz(ctx context.Context, title string, authorId int) error
	ListQuizzes(ctx context.Context) ([]quiz.Quiz, error)
	ListQuizzesByAuthor(ctx context.Context, authorID int) ([]quiz.Quiz, error)
}

type questionService interface {
	CreateQuestionAsAuthor(ctx context.Context, textContent string, quizId, authorId int) error
	DeleteQuestionAsAuthor(ctx context.Context, id int, userId int) error
	ChangeQuestionPosition(ctx context.Context, id, new_position, authorId int) error
	GetQuestion(ctx context.Context, id int) (quiz.Question, error)
}

type answerService interface {
	CreateAnswerAsAuthor(ctx context.Context, textContent string, isCorrect bool, questionId, userId int) error
	DeleteAnswerAsAuthor(ctx context.Context, id int, userId int) error
	GetAnswer(ctx context.Context, id int) (quiz.Answer, error)
	ListAnswersByQuestionId(ctx context.Context, questionId int) ([]quiz.Answer, error)
}

type userService interface {
	GetUser(ctx context.Context, id int) (quiz.User, error)
}
