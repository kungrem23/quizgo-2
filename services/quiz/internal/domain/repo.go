package quiz

import (
	"context"
	"database/sql"
)

type Repository interface {
	GetQuizContent(context.Context, int) (QuizContent, error)
	ListQuizSummaries(context.Context, int) ([]QuizSummary, error)
	SaveQuizContent(context.Context, int, int, SaveQuizContent) (QuizContent, error)
	DeleteQuizAsAuthor(context.Context, int, int) error
	CreateImageRecord(context.Context, string, int) error
	ImageExists(context.Context, string) error
	DeleteUploadedImage(context.Context, string, int, func(context.Context, string) error) error
	// ========QUIZ======
	GetQuiz(ctx context.Context, id int) (Quiz, error)
	ListQuizzes(ctx context.Context) ([]Quiz, error)
	ListQuizzesByAuthor(ctx context.Context, authorID int) ([]Quiz, error)
	CreateQuiz(ctx context.Context, title string, authorId int) (Quiz, error)

	// =========QUESTION=========
	CreateNewQuestionAsAuthor(ctx context.Context, textContent string, quizId, authorId int) error
	DeleteQuestionAsAuthor(ctx context.Context, id int, userId int) error
	ChangeQuestionPosition(ctx context.Context, id int, new_position, authorId int) error
	GetQuestion(ctx context.Context, id int) (Question, error)
	GetAllQuestions(ctx context.Context) ([]Question, error)

	// ==========ANSWER===========

	CreateNewAnswerAsAuthor(ctx context.Context, textContent string, isCorrect bool, questionId, userId int) error
	DeleteAnswerAsAuthor(ctx context.Context, id int, userId int) error
	GetAnswer(ctx context.Context, id int) (Answer, error)
	GetAnswerAsAuthor(ctx context.Context, id, userId int) (Answer, error)
	GetAnswersByQuestionId(ctx context.Context, questionId int) ([]Answer, error)

	// ==========USER=============

	CreateNewUser(ctx context.Context, username string, passwordHash string) error
	DeleteUser(ctx context.Context, id int) error
	GetUser(ctx context.Context, id int) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetAllUsers(ctx context.Context) ([]User, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}
