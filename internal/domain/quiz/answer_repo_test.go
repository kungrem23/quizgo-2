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

// ============= CreateNewAnswerAsAuthor ================

func TestCreateNewAnswerAsAuthor_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		textContent = "qwerty"
		isCorrect   = true
		questionId  = 7
		userId      = 9
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON questions.quiz_id = quizzes.id
	WHERE questions.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).
			AddRow(userId))

	query2 := regexp.QuoteMeta(`INSERT INTO answers
	(text_content, is_correct, question_id)
	VALUES ($1, $2, $3)`)
	mock.ExpectExec(query2).
		WithArgs(textContent, isCorrect, questionId).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CreateNewAnswerAsAuthor(context.Background(),
		textContent, isCorrect,
		questionId, userId)

	if err != nil {
		t.Fatalf("CreateNewAnswerAsAuthor error got: %v | expect: %v",
			err, nil)
	}
}

func TestCreateNewAnswerAsAuthor_Forbidden(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		textContent = "qwerty"
		isCorrect   = true
		questionId  = 7
		userId      = 9
		author_id   = 8
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON questions.quiz_id = quizzes.id
	WHERE questions.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).
			AddRow(author_id))
	err := repo.CreateNewAnswerAsAuthor(context.Background(),
		textContent, isCorrect,
		questionId, userId)
	if !errors.Is(err, middleware.ErrInsufficientRights) {
		t.Fatalf("CreateNewAnswerAsAuthor error got: %v | want: %v",
			err, middleware.ErrInsufficientRights)
	}
}

func TestCreateNewAnswerAsAuthor_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		textContent = "qwerty"
		isCorrect   = true
		questionId  = 7
		userId      = 9
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON questions.quiz_id = quizzes.id
	WHERE questions.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}))
	err := repo.CreateNewAnswerAsAuthor(context.Background(),
		textContent, isCorrect,
		questionId, userId)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("CreateNewAnswerAsAuthor error got: %v | want: %v",
			err, sql.ErrNoRows)
	}
}

// func TestCreateNewAnswerAsAuthor_(t *testing.T) {
// 	repo, mock, cleanup := newPostgresRepoMock(t)
// 	defer cleanup()
// 	const (
// 		textContent = "qwerty"
// 		isCorrect   = true
// 		questionId  = 7
// 		userId      = 9
// 	)
// }

// ============= DeleteAnswerAsAuthor ================

func TestDeleteAnswerAsAuthor_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId = 12
		userId   = 5
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM answers
	JOIN questions ON questions.id = answers.question_id
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE answers.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(answerId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).
			AddRow(userId))
	query2 := regexp.QuoteMeta(`DELETE FROM answers WHERE id = $1;`)
	mock.ExpectExec(query2).
		WithArgs(answerId).
		WillReturnResult(sqlmock.NewResult(0, 1))
	err := repo.DeleteAnswerAsAuthor(context.Background(), answerId, userId)
	if err != nil {
		t.Fatalf("DeleteAnswerAsAuthor error got: %v | expect: %v",
			err, nil)
	}
}

func TestDeleteAnswerAsAuthor_Forbidden(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId = 12
		userId   = 5
		authorId = 7
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM answers
	JOIN questions ON questions.id = answers.question_id
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE answers.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(answerId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}).
			AddRow(authorId))
	err := repo.DeleteAnswerAsAuthor(
		context.Background(),
		answerId, userId)
	if !errors.Is(err, middleware.ErrInsufficientRights) {
		t.Fatalf("DeleteAnswerAsAuthor error got: %v | expect: %v",
			err, middleware.ErrInsufficientRights)
	}
}

func TestDeleteAnswerAsAuthor_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId = 12
		userId   = 5
	)
	query1 := regexp.QuoteMeta(`SELECT quizzes.author_id
	FROM answers
	JOIN questions ON questions.id = answers.question_id
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE answers.id = $1`)
	mock.ExpectQuery(query1).
		WithArgs(answerId).
		WillReturnRows(sqlmock.NewRows([]string{"author_id"}))
	err := repo.DeleteAnswerAsAuthor(
		context.Background(),
		answerId, userId)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteAnswerAsAuthor error got: %v | expect: %v",
			err, sql.ErrNoRows)
	}
	// query2 := regexp.QuoteMeta(`DELETE FROM answers WHERE id = $1;`)
	// mock.ExpectExec(query2).
	// 	WithArgs(answerId).
	// 	WillReturnResult(sqlmock.NewResult(0, 0))

}

// func TestDeleteAnswerAsAuthor_(t *testing.T) {
// 	repo, mock, cleanup := newPostgresRepoMock(t)
// 	defer cleanup()
// 	const (
// 		answerId = 12
// 		userId = 5
// 	)

// }

// ================== GetAnswer ======================

func TestGetAnswer_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId    = 12
		textContent = "qwerty"
		isCorrect   = true
		questionId  = 5
	)
	query1 := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id 
	FROM answers WHERE id=$1`)
	mock.ExpectQuery(query1).
		WithArgs(answerId).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId, textContent, isCorrect, questionId))
	want := Answer{Id: answerId, TextContent: textContent,
		IsCorrect: isCorrect, QuestionId: questionId}
	got, err := repo.GetAnswer(context.Background(), answerId)
	if err != nil {
		t.Fatalf("GetAnswer error got: %v | expect: %v", err, nil)
	}
	if got != want {
		t.Fatalf("GetAnswer got: %#v | expect: %#v", got, want)
	}
}

func TestGetAnswer_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId    = 12
		textContent = "qwerty"
		isCorrect   = true
		questionId  = 5
	)
	query1 := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id 
	FROM answers WHERE id=$1`)
	mock.ExpectQuery(query1).
		WithArgs(answerId).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "text_content", "is_correct", "question_id"}))
	_, err := repo.GetAnswer(context.Background(), answerId)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetAnswer error got: %v | expect: %v", err, sql.ErrNoRows)
	}
}

func TestGetAnswerAsAuthor_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId   = 12
		questionId = 5
		userId     = 4
	)
	query := regexp.QuoteMeta(`SELECT answers.id, answers.text_content, answers.is_correct, answers.question_id, quizzes.author_id
	FROM answers
	JOIN questions ON questions.id = answers.question_id
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE answers.id = $1`)
	mock.ExpectQuery(query).
		WithArgs(answerId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id", "author_id"}).
			AddRow(answerId, "Paris", true, questionId, userId))

	want := Answer{Id: answerId, TextContent: "Paris", IsCorrect: true, QuestionId: questionId}
	got, err := repo.GetAnswerAsAuthor(context.Background(), answerId, userId)
	if err != nil {
		t.Fatalf("GetAnswerAsAuthor error got: %v | want: nil", err)
	}
	if got != want {
		t.Fatalf("GetAnswerAsAuthor got: %#v | want: %#v", got, want)
	}
}

func TestGetAnswerAsAuthor_Forbidden(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId    = 12
		authorId    = 4
		otherUserId = 7
	)
	query := regexp.QuoteMeta(`SELECT answers.id, answers.text_content, answers.is_correct, answers.question_id, quizzes.author_id
	FROM answers
	JOIN questions ON questions.id = answers.question_id
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE answers.id = $1`)
	mock.ExpectQuery(query).
		WithArgs(answerId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id", "author_id"}).
			AddRow(answerId, "Paris", true, 5, authorId))

	_, err := repo.GetAnswerAsAuthor(context.Background(), answerId, otherUserId)
	if !errors.Is(err, middleware.ErrInsufficientRights) {
		t.Fatalf("GetAnswerAsAuthor error got: %v | want: %v", err, middleware.ErrInsufficientRights)
	}
}

// func TestGetAnswer_(t *testing.T) {
// 	repo, mock, cleanup := newPostgresRepoMock(t)
// 	defer cleanup()
// 	const (
// 		answerId = 12
// 	)

// }

// ================ GetAllAnswers ====================

func TestGetAllAnswers_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId1    = 12
		textContent1 = "qwerty"
		isCorrect1   = true
		questionId1  = 5

		answerId2    = 13
		textContent2 = "asdfg"
		isCorrect2   = false
		questionId2  = 6
	)
	query1 := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id 
	FROM answers`)
	mock.ExpectQuery(query1).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId1, textContent1, isCorrect1, questionId1).
			AddRow(answerId2, textContent2, isCorrect2, questionId2))
	want := []Answer{
		{
			Id:          answerId1,
			TextContent: textContent1,
			IsCorrect:   isCorrect1,
			QuestionId:  questionId1,
		},
		{
			Id:          answerId2,
			TextContent: textContent2,
			IsCorrect:   isCorrect2,
			QuestionId:  questionId2,
		},
	}

	got, err := repo.GetAllAnswers(context.Background())
	if err != nil {
		t.Fatalf("GetAllAnswers error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllAnswers got: %#v | expect: %#v", got, want)
	}
}

func TestGetAllAnswers_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()

	query1 := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id 
	FROM answers`)
	mock.ExpectQuery(query1).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "text_content", "is_correct", "question_id"}))
	var want []Answer
	got, err := repo.GetAllAnswers(context.Background())
	if err != nil {
		t.Fatalf("GetAllAnswers error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllAnswers got: %#v | expect: %#v", got, want)
	}
}

// ============ GetAnswersByQuestionId ===============

func TestGetAnswersByQuestionId_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		answerId1    = 12
		textContent1 = "qwerty"
		isCorrect1   = true
		questionId1  = 5

		answerId2    = 13
		textContent2 = "asdfg"
		isCorrect2   = false
		questionId2  = 5
	)
	query1 := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id 
	FROM answers
	WHERE question_id=$1`)
	mock.ExpectQuery(query1).
		WithArgs(questionId1).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId1, textContent1, isCorrect1, questionId1).
			AddRow(answerId2, textContent2, isCorrect2, questionId2))
	want := []Answer{
		{
			Id:          answerId1,
			TextContent: textContent1,
			IsCorrect:   isCorrect1,
			QuestionId:  questionId1,
		},
		{
			Id:          answerId2,
			TextContent: textContent2,
			IsCorrect:   isCorrect2,
			QuestionId:  questionId2,
		},
	}

	got, err := repo.GetAnswersByQuestionId(context.Background(), questionId1)
	if err != nil {
		t.Fatalf("GetAnswersByQuestionId error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAnswersByQuestionId got: %#v | expect: %#v", got, want)
	}
}

func TestGetAnswersByQuestionId_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()

	const (
		questionId = 5
	)

	query1 := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id 
	FROM answers
	WHERE question_id=$1`)
	mock.ExpectQuery(query1).
		WithArgs(questionId).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "text_content", "is_correct", "question_id"}))
	var want []Answer
	got, err := repo.GetAnswersByQuestionId(context.Background(), questionId)
	if err != nil {
		t.Fatalf("GetAnswersByQuestionId error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAnswersByQuestionId got: %#v | expect: %#v", got, want)
	}
}
