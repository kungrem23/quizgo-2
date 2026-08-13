package quiz

import (
	"context"
	"database/sql"
	// "regexp"
	"errors"

	"github.com/kungrem23/quizgo/internal/utils"
)

type Service struct {
	repo Repository
}

func NewService(r Repository) *Service {
	return &Service{repo: r}
}

// ==============QUIZ===============

func (s *Service) GetQuiz(ctx context.Context, id int) (Quiz, error) {
	return s.repo.GetQuiz(ctx, id)
}

func (s *Service) CreateQuiz(ctx context.Context, title string, authorId int) error {
	return s.repo.CreateQuiz(ctx, title, authorId)
}

func (s *Service) ListQuizzes(ctx context.Context) ([]Quiz, error) {
	return s.repo.ListQuizzes(ctx)
}

func (s *Service) ListQuizzesByAuthor(ctx context.Context, authorID int) ([]Quiz, error) {
	return s.repo.ListQuizzesByAuthor(ctx, authorID)
}

// ==============QUESTION===============

func (s *Service) CreateQuestionAsAuthor(ctx context.Context, textContent string, quizId, authorId int) error {
	return s.repo.CreateNewQuestionAsAuthor(ctx, textContent, quizId, authorId)
}

func (s *Service) DeleteQuestionAsAuthor(ctx context.Context, id int, userId int) error {
	return s.repo.DeleteQuestionAsAuthor(ctx, id, userId)
}

func (s *Service) ChangeQuestionPosition(ctx context.Context, id, new_position, authorId int) error {
	return s.repo.ChangeQuestionPosition(ctx, id, new_position, authorId)
}

func (s *Service) GetQuestion(ctx context.Context, id int) (Question, error) {
	return s.repo.GetQuestion(ctx, id)
}

// ????????????
func (s *Service) ListQuestions(ctx context.Context) ([]Question, error) {
	return s.repo.GetAllQuestions(ctx)
}

// ==============ANSWER===============

func (s *Service) CreateAnswerAsAuthor(ctx context.Context, textContent string, isCorrect bool, questionId, userId int) error {
	return s.repo.CreateNewAnswerAsAuthor(ctx, textContent, isCorrect, questionId, userId)
}

func (s *Service) DeleteAnswerAsAuthor(ctx context.Context, id int, userId int) error {
	return s.repo.DeleteAnswerAsAuthor(ctx, id, userId)
}

func (s *Service) GetAnswer(ctx context.Context, id int) (Answer, error) {
	return s.repo.GetAnswer(ctx, id)
}

func (s *Service) GetAnswerAsAuthor(ctx context.Context, id, userId int) (Answer, error) {
	return s.repo.GetAnswerAsAuthor(ctx, id, userId)
}

func (s *Service) ListAnswersByQuestionId(ctx context.Context, questionId int) ([]Answer, error) {
	return s.repo.GetAnswersByQuestionId(ctx, questionId)
}

// ==============IMAGE===============

func (s *Service) CreateImage(ctx context.Context, imageURL string) error {
	return s.repo.CreateNewImage(ctx, imageURL)
}

func (s *Service) DeleteImage(ctx context.Context, id string) error {
	return s.repo.DeleteImage(ctx, id)
}

func (s *Service) GetImage(ctx context.Context, id string) (Image, error) {
	return s.repo.GetImage(ctx, id)
}

// ==============USER===============

func (s *Service) CreateUser(ctx context.Context, username string, passwordHash string) error {
	return s.repo.CreateNewUser(ctx, username, passwordHash)
}

func (s *Service) DeleteUser(ctx context.Context, id int) error {
	return s.repo.DeleteUser(ctx, id)
}

func (s *Service) GetUser(ctx context.Context, id int) (User, error) {
	return s.repo.GetUser(ctx, id)
}

func (s *Service) GetUserByUsername(ctx context.Context, username string) (User, error) {
	return s.repo.GetUserByUsername(ctx, username)
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.repo.GetAllUsers(ctx)
}

// ===========AUTHORIZATION===============

var ErrInvalidUsername = errors.New("invalid username")
var ErrInvalidPassword = errors.New("invalid password")

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	user, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrInvalidUsername
		} else {
			return "", err
		}
	}

	if !utils.CheckPasswordHash(password, user.PasswordHash) {
		return "", ErrInvalidPassword
	}
	return utils.GenerateJWT(user.Id)
}

var ErrTakenUsername = errors.New("user with this username already exists")

func (s *Service) Register(ctx context.Context, username, password string) error {
	_, err := s.GetUserByUsername(ctx, username)
	if err == nil {
		return ErrTakenUsername
	}
	if err == sql.ErrNoRows {
		passwordHash, err := utils.HashPassword(password)
		if err != nil {
			return err
		}
		err = s.CreateUser(ctx, username, passwordHash)
		return err
	}
	return err
}
