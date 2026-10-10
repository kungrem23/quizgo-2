package quiz

import (
	"context"
	"database/sql"
	// "regexp"
	"errors"
	"time"

	"github.com/kungrem23/quizgo/services/quiz/internal/authn"
	"github.com/kungrem23/quizgo/services/quiz/internal/security"
)

type Service struct {
	repo          Repository
	images        ImageStore
	tokens        *authn.Manager
	refreshTokens *authn.RefreshManager
	refreshStore  RefreshSessionStore
}

func NewService(r Repository, stores ...ImageStore) *Service {
	s := &Service{repo: r}
	if len(stores) > 0 {
		s.images = stores[0]
	}
	return s
}

func NewServiceWithTokens(r Repository, images ImageStore, tokens *authn.Manager) *Service {
	return &Service{repo: r, images: images, tokens: tokens}
}

func NewServiceWithAuth(
	r Repository,
	images ImageStore,
	tokens *authn.Manager,
	refreshTokens *authn.RefreshManager,
	refreshStore RefreshSessionStore,
) *Service {
	return &Service{
		repo: r, images: images, tokens: tokens,
		refreshTokens: refreshTokens, refreshStore: refreshStore,
	}
}

func (s *Service) VerifyAccessToken(raw string) (int, error) {
	if s == nil || s.tokens == nil {
		return 0, authn.ErrInvalidToken
	}
	return s.tokens.VerifyAccessToken(raw)
}

// ==============QUIZ===============

func (s *Service) GetQuiz(ctx context.Context, id int) (Quiz, error) {
	return s.repo.GetQuiz(ctx, id)
}

func (s *Service) CreateQuiz(ctx context.Context, title string, authorId int) (Quiz, error) {
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

type AuthSession struct {
	AccessToken      string
	RefreshToken     string
	RefreshExpiresAt time.Time
}

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	user, err := s.authenticate(ctx, username, password)
	if err != nil {
		return "", err
	}
	if s.tokens == nil {
		return "", authn.ErrInvalidToken
	}
	return s.tokens.IssueAccessToken(user.Id)
}

func (s *Service) LoginSession(ctx context.Context, username, password string) (AuthSession, error) {
	user, err := s.authenticate(ctx, username, password)
	if err != nil {
		return AuthSession{}, err
	}
	if s.tokens == nil {
		return AuthSession{}, authn.ErrInvalidToken
	}
	accessToken, err := s.tokens.IssueAccessToken(user.Id)
	if err != nil {
		return AuthSession{}, err
	}
	result := AuthSession{AccessToken: accessToken}
	if s.refreshTokens == nil || s.refreshStore == nil {
		return result, nil
	}
	credential, err := s.refreshTokens.Issue()
	if err != nil {
		return AuthSession{}, err
	}
	now := s.refreshTokens.Now()
	err = s.refreshStore.CreateRefreshSession(ctx, RefreshSession{
		ID: credential.SessionID, UserID: user.Id, SecretHash: credential.Hash,
		CreatedAt: now, ExpiresAt: credential.ExpiresAt,
	})
	if err != nil {
		return AuthSession{}, err
	}
	result.RefreshToken = credential.Raw
	result.RefreshExpiresAt = credential.ExpiresAt
	return result, nil
}

func (s *Service) RefreshSession(ctx context.Context, raw string) (AuthSession, error) {
	if s == nil || s.tokens == nil || s.refreshTokens == nil || s.refreshStore == nil {
		return AuthSession{}, authn.ErrInvalidRefreshToken
	}
	sessionID, presentedHash, err := s.refreshTokens.Parse(raw)
	if err != nil {
		return AuthSession{}, err
	}
	next, err := s.refreshTokens.Rotate(sessionID)
	if err != nil {
		return AuthSession{}, err
	}
	userID, expiresAt, err := s.refreshStore.RotateRefreshSession(
		ctx, sessionID, presentedHash, next.Hash, s.refreshTokens.Now(),
	)
	if err != nil {
		return AuthSession{}, err
	}
	accessToken, err := s.tokens.IssueAccessToken(userID)
	if err != nil {
		return AuthSession{}, err
	}
	return AuthSession{
		AccessToken: accessToken, RefreshToken: next.Raw, RefreshExpiresAt: expiresAt,
	}, nil
}

func (s *Service) LogoutSession(ctx context.Context, raw string) error {
	if s == nil || s.refreshTokens == nil || s.refreshStore == nil {
		return authn.ErrInvalidRefreshToken
	}
	sessionID, presentedHash, err := s.refreshTokens.Parse(raw)
	if err != nil {
		return err
	}
	return s.refreshStore.RevokeRefreshSession(ctx, sessionID, presentedHash, s.refreshTokens.Now())
}

func (s *Service) authenticate(ctx context.Context, username, password string) (User, error) {
	user, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrInvalidUsername
		} else {
			return User{}, err
		}
	}

	if !security.CheckPasswordHash(password, user.PasswordHash) {
		return User{}, ErrInvalidPassword
	}
	return user, nil
}

var ErrTakenUsername = errors.New("user with this username already exists")

func (s *Service) Register(ctx context.Context, username, password string) error {
	_, err := s.GetUserByUsername(ctx, username)
	if err == nil {
		return ErrTakenUsername
	}
	if errors.Is(err, sql.ErrNoRows) {
		passwordHash, err := security.HashPassword(password)
		if err != nil {
			return err
		}
		err = s.CreateUser(ctx, username, passwordHash)
		return err
	}
	return err
}
