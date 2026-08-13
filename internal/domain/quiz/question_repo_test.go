package quiz

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/kungrem23/quizgo/internal/http/middleware"
)

// ============== CreateNewQuestionAsAuthor ===============

func TestCreateNewQuestionAsAuthor_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId      = 5
		textContent = "qwerty"
		userId      = 7
	)
	mock.ExpectBegin()
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM quizzes
	WHERE quizzes.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).
			AddRow(userId))
	query2 := regexp.QuoteMeta(`SELECT COALESCE(MAX(position), 0) + 1
	FROM questions
	WHERE quiz_id = $1;`)
	mock.ExpectQuery(query2).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"position"}).
			AddRow(1))
	query3 := regexp.QuoteMeta(`INSERT INTO questions
	(text_content, position, quiz_id)
	VALUES ($1, $2, $3)`)
	mock.ExpectExec(query3).
		WithArgs(textContent, 1, quizId).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err := repo.CreateNewQuestionAsAuthor(
		context.Background(),
		textContent,
		quizId,
		userId,
	)
	if err != nil {
		t.Fatalf(" error got: %v | expect: %v", err, nil)
	}
}

func TestCreateNewQuestionAsAuthor_Forbidden(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId      = 5
		textContent = "qwerty"
		userId      = 7
		authorId    = 8
	)
	mock.ExpectBegin()
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM quizzes
	WHERE quizzes.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).
			AddRow(authorId))
	mock.ExpectRollback()
	err := repo.CreateNewQuestionAsAuthor(
		context.Background(),
		textContent,
		quizId,
		userId,
	)
	if !errors.Is(err, middleware.ErrInsufficientRights) {
		t.Fatalf(" error got: %v | expect: %v", err, middleware.ErrInsufficientRights)
	}
}

func TestCreateNewQuestionAsAuthor_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId      = 5
		textContent = "qwerty"
		userId      = 7
	)
	mock.ExpectBegin()
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM quizzes
	WHERE quizzes.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}))
	mock.ExpectRollback()
	err := repo.CreateNewQuestionAsAuthor(
		context.Background(),
		textContent,
		quizId,
		userId,
	)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf(" error got: %v | expect: %v", err, sql.ErrNoRows)
	}
}

// ================ DeleteQuestionAsAuthor ================

func TestDeleteQuestionAsAuthor_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		textContent = "qwerty"
		position    = 3
		quizId      = 9
		userId      = 7
		imageId     = "gfdsa"
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).
			AddRow(userId))

	// GetQuestion
	queryGetQuestion := regexp.QuoteMeta(`SELECT id, text_content, position, image_id, quiz_id FROM questions
	WHERE id=$1`)
	mock.ExpectQuery(queryGetQuestion).
		WithArgs(questionId).
		WillReturnRows(sqlmock.
			NewRows([]string{"id", "text_content",
				"position", "image_id", "quiz_id"}).
			AddRow(questionId, textContent, position, imageId, quizId))

	mock.ExpectBegin()
	query2 := regexp.QuoteMeta(`DELETE FROM questions WHERE id = $1;`)
	mock.ExpectExec(query2).
		WithArgs(questionId).
		WillReturnResult(sqlmock.NewResult(0, 1))
	query3 := regexp.QuoteMeta(`UPDATE questions
	SET position = position - 1
	WHERE position > $1 AND quiz_id = $2;`)
	mock.ExpectExec(query3).
		WithArgs(position, quizId).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	err := repo.DeleteQuestionAsAuthor(context.Background(), questionId, userId)
	if err != nil {
		t.Fatalf("DeleteQuestionAsAuthor error got: %v | expect: %v", err, nil)
	}
}

func TestDeleteQuestionAsAuthor_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		textContent = "qwerty"
		position    = 3
		quizId      = 9
		userId      = 7
		imageId     = "gfdsa"
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}))

	err := repo.DeleteQuestionAsAuthor(context.Background(), questionId, userId)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteQuestionAsAuthor error got: %v | expect: %v", err, sql.ErrNoRows)
	}
}

func TestDeleteQuestionAsAuthor_Forbidden(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		textContent = "qwerty"
		position    = 3
		quizId      = 9
		userId      = 7
		imageId     = "gfdsa"
		authorId    = 10
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).
			AddRow(authorId))

	err := repo.DeleteQuestionAsAuthor(context.Background(), questionId, userId)
	if !errors.Is(err, middleware.ErrInsufficientRights) {
		t.Fatalf("DeleteQuestionAsAuthor error got: %v | expect: %v", err, middleware.ErrInsufficientRights)
	}
}

// ================ ChangeQuestionPosition ================

func TestChangeQuestionPosition_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		textContent = "qwerty"
		position    = 3
		imageId     = "gfdsa"
		quizId      = 9
		newPosition = 6
		userId      = 4
	)
	queryAuthor := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`)
	mock.ExpectQuery(queryAuthor).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).AddRow(userId))
	// GetQuestion
	queryGetQuestion := regexp.QuoteMeta(`SELECT id, text_content, position, image_id, quiz_id FROM questions
	WHERE id=$1`)
	mock.ExpectQuery(queryGetQuestion).
		WithArgs(questionId).
		WillReturnRows(sqlmock.
			NewRows([]string{"id", "text_content",
				"position", "image_id", "quiz_id"}).
			AddRow(questionId, textContent, position, imageId, quizId))

	mock.ExpectBegin()
	queryCount := regexp.QuoteMeta(`SELECT COUNT(*)
	FROM questions
	WHERE quiz_id = $1`)
	mock.ExpectQuery(queryCount).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(newPosition))
	query1 := regexp.QuoteMeta(`UPDATE questions
			SET position = position - 1
			WHERE quiz_id = $1 AND position > $2 AND position <= $3;`)
	mock.ExpectExec(query1).
		WithArgs(quizId, position, newPosition).
		WillReturnResult(sqlmock.NewResult(0, 4))
	query2 := regexp.QuoteMeta(`UPDATE questions
	SET position = $1
	WHERE id = $2`)
	mock.ExpectExec(query2).
		WithArgs(newPosition, questionId).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err := repo.ChangeQuestionPosition(
		context.Background(), questionId, newPosition, userId,
	)
	if err != nil {
		t.Fatalf("ChangeQuestionPosition error got: %v | expect: %v",
			err, nil)
	}
}

func TestChangeQuestionPosition_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		textContent = "qwerty"
		position    = 3
		imageId     = "gfdsa"
		quizId      = 9
		newPosition = 6
		userId      = 4
	)
	queryAuthor := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`)
	mock.ExpectQuery(queryAuthor).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}))

	err := repo.ChangeQuestionPosition(
		context.Background(), questionId, newPosition, userId,
	)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("ChangeQuestionPosition error got: %v | expect: %v",
			err, nil)
	}
}

func TestChangeQuestionPosition_PositionsMatches(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		textContent = "qwerty"
		position    = 3
		imageId     = "gfdsa"
		quizId      = 9
		newPosition = 3
		userId      = 4
	)
	queryAuthor := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`)
	mock.ExpectQuery(queryAuthor).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).AddRow(userId))
	// GetQuestion
	queryGetQuestion := regexp.QuoteMeta(`SELECT id, text_content, position, image_id, quiz_id FROM questions
	WHERE id=$1`)
	mock.ExpectQuery(queryGetQuestion).
		WithArgs(questionId).
		WillReturnRows(sqlmock.
			NewRows([]string{"id", "text_content",
				"position", "image_id", "quiz_id"}).
			AddRow(questionId, textContent, position, imageId, quizId))
	mock.ExpectBegin()
	queryCount := regexp.QuoteMeta(`SELECT COUNT(*)
	FROM questions
	WHERE quiz_id = $1`)
	mock.ExpectQuery(queryCount).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(position))
	mock.ExpectCommit()

	err := repo.ChangeQuestionPosition(
		context.Background(), questionId, newPosition, userId,
	)
	if err != nil {
		t.Fatalf("ChangeQuestionPosition error got: %v | expect: %v",
			err, nil)
	}
}

func TestChangeQuestionPosition_InvalidPosition(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		position    = 3
		quizId      = 9
		newPosition = 7
		userId      = 4
	)
	queryAuthor := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`)
	mock.ExpectQuery(queryAuthor).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).AddRow(userId))
	queryGetQuestion := regexp.QuoteMeta(`SELECT id, text_content, position, image_id, quiz_id FROM questions
	WHERE id=$1`)
	mock.ExpectQuery(queryGetQuestion).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "image_id", "quiz_id"}).
			AddRow(questionId, "question", position, "image", quizId))
	mock.ExpectBegin()
	queryCount := regexp.QuoteMeta(`SELECT COUNT(*)
	FROM questions
	WHERE quiz_id = $1`)
	mock.ExpectQuery(queryCount).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(6))
	mock.ExpectRollback()

	err := repo.ChangeQuestionPosition(context.Background(), questionId, newPosition, userId)
	if !errors.Is(err, ErrInvalidQuestionPosition) {
		t.Fatalf("ChangeQuestionPosition error got: %v | want: %v", err, ErrInvalidQuestionPosition)
	}
}

func TestChangeQuestionPosition_Forbidden(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		authorId    = 4
		otherUser   = 7
		newPosition = 6
	)
	queryAuthor := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`)
	mock.ExpectQuery(queryAuthor).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).AddRow(authorId))

	err := repo.ChangeQuestionPosition(context.Background(), questionId, newPosition, otherUser)
	if !errors.Is(err, middleware.ErrInsufficientRights) {
		t.Fatalf("ChangeQuestionPosition error got: %v | want: %v", err, middleware.ErrInsufficientRights)
	}
}

// ===================== GetQuestion ======================

func TestGetQuestion_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		textContent = "qwerty"
		position    = 3
		imageId     = "gfdsa"
		quizId      = 9
	)
	query := regexp.QuoteMeta(`SELECT id, text_content, position, image_id, quiz_id FROM questions
	WHERE id=$1`)
	mock.ExpectQuery(query).
		WithArgs(questionId).
		WillReturnRows(sqlmock.
			NewRows([]string{"id", "text_content",
				"position", "image_id", "quiz_id"}).
			AddRow(questionId, textContent, position, imageId, quizId))
	want := Question{Id: questionId, TextContent: textContent,
		Position: position, ImageId: imageId, QuizId: quizId}
	got, err := repo.GetQuestion(context.Background(), questionId)
	if err != nil {
		t.Fatalf("GetQuestion error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetQuestion got: %#v | expect: %#v", got, want)
	}
}

func TestGetQuestion_NotFoud(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId  = 5
		textContent = "qwerty"
		position    = 3
		imageId     = "gfdsa"
		quizId      = 9
	)
	query := regexp.QuoteMeta(`SELECT id, text_content, position, image_id, quiz_id FROM questions
	WHERE id=$1`)
	mock.ExpectQuery(query).
		WithArgs(questionId).
		WillReturnRows(sqlmock.
			NewRows([]string{"id", "text_content",
				"position", "image_id", "quiz_id"}))
	_, err := repo.GetQuestion(context.Background(), questionId)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetQuestion error got: %v | expect: %v", err, sql.ErrNoRows)
	}
}

// =================== GetAllQuestions ====================

func TestGetAllQuestions_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		questionId1  = 5
		textContent1 = "qwerty"
		position1    = 3
		imageId1     = "gfdsa"
		quizId1      = 9

		questionId2  = 5
		textContent2 = "qwerty"
		position2    = 3
		imageId2     = "gfdsa"
		quizId2      = 9
	)
	query := regexp.QuoteMeta(`SELECT id, text_content, position, image_id, quiz_id FROM questions`)
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "image_id", "quiz_id"}).
			AddRow(questionId1, textContent1, position1, imageId1, quizId1).
			AddRow(questionId1, textContent1, position1, imageId1, quizId1))
	want := []Question{
		{
			Id:          questionId1,
			TextContent: textContent1,
			Position:    position1,
			ImageId:     imageId1,
			QuizId:      quizId1,
		},
		{
			Id:          questionId2,
			TextContent: textContent2,
			Position:    position2,
			ImageId:     imageId2,
			QuizId:      quizId2,
		},
	}
	got, err := repo.GetAllQuestions(context.Background())
	if err != nil {
		t.Fatalf("GetAllQuestions error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllQuestions got: %#v | expect: %#v", got, want)
	}
}

func TestGetAllQuestions_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	// const ()
	query := regexp.QuoteMeta(`SELECT id, text_content, position, image_id, quiz_id FROM questions`)
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "image_id", "quiz_id"}))
	var want []Question
	got, err := repo.GetAllQuestions(context.Background())
	if err != nil {
		t.Fatalf("GetAllQuestions error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllQuestions got: %#v | expect: %#v", got, want)
	}

}

// func TestCreateNewQuestionAsAuthor(t *testing.T) {
// 	repo, mock, cleanup := newPostgresRepoMock(t)
// 	defer cleanup()
// 	const ()
// 	query := regexp.QuoteMeta()
// 	mock.ExpectQuery(query).WithArgs()
// 	err := repo.
// 	if err != nil {
// 		t.Fatalf(" error got: %v | expect: %v", err, )
// 	}
// }
