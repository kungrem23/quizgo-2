package quiz

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// ==================== CreateQuiz ===================

func TestCreateQuiz_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		title  = "qwerty"
		userId = 5
	)
	query := regexp.QuoteMeta(`INSERT INTO quizzes (title, author_id) VALUES ($1, $2) RETURNING id, title, author_id`)
	mock.ExpectQuery(query).WithArgs(title, userId).WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id"}).AddRow(17, title, userId))
	created, err := repo.CreateQuiz(context.Background(), title, userId)
	if created.Id != 17 {
		t.Fatalf("created ID = %d", created.Id)
	}
	if err != nil {
		t.Fatalf(" error got: %v | expect: %v", err, nil)
	}
}

// ================== GetQuizAnswers =================

func TestGetQuizAnswers_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId = 0

		questionId1   = 1
		answerId11    = 2
		textContent11 = "qwerty"
		isCorrect11   = true

		answerId12    = 3
		textContent12 = "asdfgh"
		isCorrect12   = false

		questionId2   = 6
		answerId21    = 7
		textContent21 = "hgfd"
		isCorrect21   = true

		answerId22    = 8
		textContent22 = "mnbvc"
		isCorrect22   = false
	)
	query := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	FROM answers
	WHERE question_id IN 
	(SELECT id FROM questions WHERE quiz_id=$1)`)
	mock.ExpectQuery(query).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId11, textContent11, isCorrect11, questionId1).
			AddRow(answerId12, textContent12, isCorrect12, questionId1).
			AddRow(answerId21, textContent21, isCorrect21, questionId2).
			AddRow(answerId22, textContent22, isCorrect22, questionId2))
	want := map[int]*Question{
		questionId1: &Question{
			Answers: []Answer{
				{
					Id:          answerId11,
					TextContent: textContent11,
					IsCorrect:   isCorrect11,
					QuestionId:  questionId1,
				},
				{
					Id:          answerId12,
					TextContent: textContent12,
					IsCorrect:   isCorrect12,
					QuestionId:  questionId1,
				},
			},
		},
		questionId2: &Question{
			Answers: []Answer{
				{
					Id:          answerId21,
					TextContent: textContent21,
					IsCorrect:   isCorrect21,
					QuestionId:  questionId2,
				},
				{
					Id:          answerId22,
					TextContent: textContent22,
					IsCorrect:   isCorrect22,
					QuestionId:  questionId2,
				},
			},
		},
	}

	got := map[int]*Question{
		questionId1: &Question{},
		questionId2: &Question{},
	}

	err := repo.GetQuizAnswers(context.Background(), quizId, got)
	if err != nil {
		t.Fatalf("GetQuizAnswers error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetQuizAnswers got: %#v | expect: %#v", got, want)
	}
}

func TestGetQuizAnswers_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId = 0

		questionId1   = 1
		answerId11    = 2
		textContent11 = "qwerty"
		isCorrect11   = true

		answerId12    = 3
		textContent12 = "asdfgh"
		isCorrect12   = false

		questionId2   = 6
		answerId21    = 7
		textContent21 = "hgfd"
		isCorrect21   = true

		answerId22    = 8
		textContent22 = "mnbvc"
		isCorrect22   = false
	)
	query := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	FROM answers
	WHERE question_id IN 
	(SELECT id FROM questions WHERE quiz_id=$1)`)
	mock.ExpectQuery(query).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}))
	want := map[int]*Question{
		questionId1: {},
		questionId2: {},
	}

	got := map[int]*Question{
		questionId1: {},
		questionId2: {},
	}

	err := repo.GetQuizAnswers(context.Background(), quizId, got)
	if err != nil {
		t.Fatalf("GetQuizAnswers error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetQuizAnswers got: %#v | expect: %#v", got, want)
	}
}

// map[int]*quiz.Question{1:(*quiz.Question)(0x140002a3310), 6:(*quiz.Question)(0x140002a3360)}
// map[int]*quiz.Question{1:(*quiz.Question)(0x140002a31d0), 6:(*quiz.Question)(0x140002a3270)}

// ================= GetQuizQuestions ================

func TestGetQuizQuestions_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId = 1

		questionId1   = 2
		qTextContent1 = "qwerty"
		position1     = 4
		imageId1      = "dfgh"

		answerId11     = 2
		aTextContent11 = "qwerty"
		isCorrect11    = true

		answerId12     = 3
		aTextContent12 = "asdfgh"
		isCorrect12    = false

		questionId2   = 3
		qTextContent2 = "qwerty"
		position2     = 5
		imageId2      = ""

		answerId21     = 7
		aTextContent21 = "hgfd"
		isCorrect21    = true

		answerId22     = 8
		aTextContent22 = "mnbvc"
		isCorrect22    = false
	)
	query := regexp.QuoteMeta(`SELECT id, text_content, position, quiz_id, image_id 
	FROM questions 
	WHERE quiz_id=$1 
	ORDER BY position`)
	mock.ExpectQuery(query).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
			AddRow(questionId1, qTextContent1, position1, quizId, imageId1).
			AddRow(questionId2, qTextContent2, position2, quizId, imageId2))
	queryAnswers := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	FROM answers
	WHERE question_id IN 
	(SELECT id FROM questions WHERE quiz_id=$1)`)
	mock.ExpectQuery(queryAnswers).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId11, aTextContent11, isCorrect11, questionId1).
			AddRow(answerId12, aTextContent12, isCorrect12, questionId1).
			AddRow(answerId21, aTextContent21, isCorrect21, questionId2).
			AddRow(answerId22, aTextContent22, isCorrect22, questionId2))
	want := []*Question{
		{
			Id:          questionId1,
			TextContent: qTextContent1,
			Position:    position1,
			QuizId:      quizId,
			ImageId:     imageId1,
			Answers: []Answer{
				{
					Id:          answerId11,
					TextContent: aTextContent11,
					IsCorrect:   isCorrect11,
					QuestionId:  questionId1,
				},
				{
					Id:          answerId12,
					TextContent: aTextContent12,
					IsCorrect:   isCorrect12,
					QuestionId:  questionId1,
				},
			},
		},
		{
			Id:          questionId2,
			TextContent: qTextContent2,
			Position:    position2,
			QuizId:      quizId,
			ImageId:     imageId2,
			Answers: []Answer{
				{
					Id:          answerId21,
					TextContent: aTextContent21,
					IsCorrect:   isCorrect21,
					QuestionId:  questionId2,
				},
				{
					Id:          answerId22,
					TextContent: aTextContent22,
					IsCorrect:   isCorrect22,
					QuestionId:  questionId2,
				},
			},
		},
	}
	got, err := repo.GetQuizQuestions(context.Background(), quizId)
	if err != nil {
		t.Fatalf("GetQuizQuestions error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetQuizQuestions got: %#v | expect: %#v", got, want)
	}
}

func TestGetQuizQuestions_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId = 1

		questionId1   = 2
		qTextContent1 = "qwerty"
		position1     = 4
		imageId1      = "dfgh"

		answerId11     = 2
		aTextContent11 = "qwerty"
		isCorrect11    = true

		answerId12     = 3
		aTextContent12 = "asdfgh"
		isCorrect12    = false

		questionId2   = 3
		qTextContent2 = "qwerty"
		position2     = 5
		imageId2      = ""

		answerId21     = 7
		aTextContent21 = "hgfd"
		isCorrect21    = true

		answerId22     = 8
		aTextContent22 = "mnbvc"
		isCorrect22    = false
	)
	query := regexp.QuoteMeta(`SELECT id, text_content, position, quiz_id, image_id 
	FROM questions 
	WHERE quiz_id=$1 
	ORDER BY position`)
	mock.ExpectQuery(query).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}))
	queryAnswers := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	FROM answers
	WHERE question_id IN 
	(SELECT id FROM questions WHERE quiz_id=$1)`)
	mock.ExpectQuery(queryAnswers).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}))
	want := []*Question{}
	got, err := repo.GetQuizQuestions(context.Background(), quizId)
	if err != nil {
		t.Fatalf("GetQuizQuestions error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetQuizQuestions got: %#v | expect: %#v", got, want)
	}
}

// ===================== GetQuiz =====================

func TestGetQuiz_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId   = 1
		title    = "hgfds"
		authorId = 9

		questionId1   = 2
		qTextContent1 = "qwerty"
		position1     = 4
		imageId1      = "dfgh"

		answerId11     = 2
		aTextContent11 = "qwerty"
		isCorrect11    = true

		answerId12     = 3
		aTextContent12 = "asdfgh"
		isCorrect12    = false

		questionId2   = 3
		qTextContent2 = "qwerty"
		position2     = 5
		imageId2      = ""

		answerId21     = 7
		aTextContent21 = "hgfd"
		isCorrect21    = true

		answerId22     = 8
		aTextContent22 = "mnbvc"
		isCorrect22    = false
	)
	query := regexp.QuoteMeta(`SELECT id, title, author_id FROM quizzes WHERE id=$1`)
	mock.ExpectQuery(query).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id"}).
			AddRow(quizId, title, authorId))
	queryQuestions := regexp.QuoteMeta(`SELECT id, text_content, position, quiz_id, image_id 
	FROM questions 
	WHERE quiz_id=$1 
	ORDER BY position`)
	mock.ExpectQuery(queryQuestions).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
			AddRow(questionId1, qTextContent1, position1, quizId, imageId1).
			AddRow(questionId2, qTextContent2, position2, quizId, imageId2))
	queryAnswers := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	FROM answers
	WHERE question_id IN 
	(SELECT id FROM questions WHERE quiz_id=$1)`)
	mock.ExpectQuery(queryAnswers).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId11, aTextContent11, isCorrect11, questionId1).
			AddRow(answerId12, aTextContent12, isCorrect12, questionId1).
			AddRow(answerId21, aTextContent21, isCorrect21, questionId2).
			AddRow(answerId22, aTextContent22, isCorrect22, questionId2))
	want := Quiz{
		Id:       quizId,
		Title:    title,
		AuthorId: authorId,
		Questions: []*Question{
			{
				Id:          questionId1,
				TextContent: qTextContent1,
				Position:    position1,
				QuizId:      quizId,
				ImageId:     imageId1,
				Answers: []Answer{
					{
						Id:          answerId11,
						TextContent: aTextContent11,
						IsCorrect:   isCorrect11,
						QuestionId:  questionId1,
					},
					{
						Id:          answerId12,
						TextContent: aTextContent12,
						IsCorrect:   isCorrect12,
						QuestionId:  questionId1,
					},
				},
			},
			{
				Id:          questionId2,
				TextContent: qTextContent2,
				Position:    position2,
				QuizId:      quizId,
				ImageId:     imageId2,
				Answers: []Answer{
					{
						Id:          answerId21,
						TextContent: aTextContent21,
						IsCorrect:   isCorrect21,
						QuestionId:  questionId2,
					},
					{
						Id:          answerId22,
						TextContent: aTextContent22,
						IsCorrect:   isCorrect22,
						QuestionId:  questionId2,
					},
				},
			},
		},
	}
	got, err := repo.GetQuiz(context.Background(), quizId)
	if err != nil {
		t.Fatalf("GetQuiz error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetQuiz got: %#v | expect: %#v", got, want)
	}
}

func TestGetQuiz_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId   = 1
		title    = "hgfds"
		authorId = 9

		questionId1   = 2
		qTextContent1 = "qwerty"
		position1     = 4
		imageId1      = "dfgh"

		answerId11     = 2
		aTextContent11 = "qwerty"
		isCorrect11    = true

		answerId12     = 3
		aTextContent12 = "asdfgh"
		isCorrect12    = false

		questionId2   = 3
		qTextContent2 = "qwerty"
		position2     = 5
		imageId2      = ""

		answerId21     = 7
		aTextContent21 = "hgfd"
		isCorrect21    = true

		answerId22     = 8
		aTextContent22 = "mnbvc"
		isCorrect22    = false
	)
	query := regexp.QuoteMeta(`SELECT id, title, author_id FROM quizzes WHERE id=$1`)
	mock.ExpectQuery(query).
		WithArgs(quizId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id"}))
	_, err := repo.GetQuiz(context.Background(), quizId)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetQuiz error got: %v | expect: %v", err, sql.ErrNoRows)
	}
}

// =================== ListQuizzes ===================

func TestListQuizzes_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId1   = 1
		title1    = "hgfds"
		authorId1 = 9

		questionId11   = 11
		qTextContent11 = "qwerty"
		position11     = 4
		imageId11      = "dfgh"

		answerId111     = 111
		aTextContent111 = "qwerty"
		isCorrect111    = true

		answerId112     = 112
		aTextContent112 = "asdfgh"
		isCorrect112    = false

		questionId12   = 12
		qTextContent12 = "qwerty"
		position12     = 5
		imageId12      = ""

		answerId121     = 121
		aTextContent121 = "hgfd"
		isCorrect121    = true

		answerId122     = 122
		aTextContent122 = "mnbvc"
		isCorrect122    = false

		quizId2   = 2
		title2    = "hgfds"
		authorId2 = 20

		questionId21   = 21
		qTextContent21 = "qwerty"
		position21     = 4
		imageId21      = "dfgh"

		answerId211     = 211
		aTextContent211 = "qwerty"
		isCorrect211    = true

		answerId212     = 212
		aTextContent212 = "asdfgh"
		isCorrect212    = false

		questionId22   = 22
		qTextContent22 = "qwerty"
		position22     = 5
		imageId22      = ""

		answerId221     = 221
		aTextContent221 = "hgfd"
		isCorrect221    = true

		answerId222     = 222
		aTextContent222 = "mnbvc"
		isCorrect222    = false
	)
	query := regexp.QuoteMeta(`SELECT id, title, author_id FROM quizzes`)
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id"}).
			AddRow(quizId1, title1, authorId1).
			AddRow(quizId2, title2, authorId2))
	queryQuestions := regexp.QuoteMeta(`SELECT id, text_content, position, quiz_id, image_id 
	FROM questions 
	WHERE quiz_id=$1 
	ORDER BY position`)
	mock.ExpectQuery(queryQuestions).
		WithArgs(quizId1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
			AddRow(questionId11, qTextContent11, position11, quizId1, imageId11).
			AddRow(questionId12, qTextContent12, position12, quizId1, imageId12))
	queryAnswers := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	FROM answers
	WHERE question_id IN 
	(SELECT id FROM questions WHERE quiz_id=$1)`)
	mock.ExpectQuery(queryAnswers).
		WithArgs(quizId1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId111, aTextContent111, isCorrect111, questionId11).
			AddRow(answerId112, aTextContent112, isCorrect112, questionId11).
			AddRow(answerId121, aTextContent121, isCorrect121, questionId12).
			AddRow(answerId122, aTextContent122, isCorrect122, questionId12))
	mock.ExpectQuery(queryQuestions).
		WithArgs(quizId2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
			AddRow(questionId21, qTextContent21, position21, quizId2, imageId21).
			AddRow(questionId22, qTextContent22, position22, quizId2, imageId22))
	mock.ExpectQuery(queryAnswers).
		WithArgs(quizId2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId211, aTextContent211, isCorrect211, questionId21).
			AddRow(answerId212, aTextContent212, isCorrect212, questionId21).
			AddRow(answerId221, aTextContent221, isCorrect221, questionId22).
			AddRow(answerId222, aTextContent222, isCorrect222, questionId22))
	want := []Quiz{
		{
			Id:       quizId1,
			Title:    title1,
			AuthorId: authorId1,
			Questions: []*Question{
				{
					Id:          questionId11,
					TextContent: qTextContent11,
					Position:    position11,
					QuizId:      quizId1,
					ImageId:     imageId11,
					Answers: []Answer{
						{
							Id:          answerId111,
							TextContent: aTextContent111,
							IsCorrect:   isCorrect111,
							QuestionId:  questionId11,
						},
						{
							Id:          answerId112,
							TextContent: aTextContent112,
							IsCorrect:   isCorrect112,
							QuestionId:  questionId11,
						},
					},
				},
				{
					Id:          questionId12,
					TextContent: qTextContent12,
					Position:    position12,
					QuizId:      quizId1,
					ImageId:     imageId12,
					Answers: []Answer{
						{
							Id:          answerId121,
							TextContent: aTextContent121,
							IsCorrect:   isCorrect121,
							QuestionId:  questionId12,
						},
						{
							Id:          answerId122,
							TextContent: aTextContent122,
							IsCorrect:   isCorrect122,
							QuestionId:  questionId12,
						},
					},
				},
			},
		},
		{
			Id:       quizId2,
			Title:    title2,
			AuthorId: authorId2,
			Questions: []*Question{
				{
					Id:          questionId21,
					TextContent: qTextContent21,
					Position:    position21,
					QuizId:      quizId2,
					ImageId:     imageId21,
					Answers: []Answer{
						{
							Id:          answerId211,
							TextContent: aTextContent211,
							IsCorrect:   isCorrect211,
							QuestionId:  questionId21,
						},
						{
							Id:          answerId212,
							TextContent: aTextContent212,
							IsCorrect:   isCorrect212,
							QuestionId:  questionId21,
						},
					},
				},
				{
					Id:          questionId22,
					TextContent: qTextContent22,
					Position:    position22,
					QuizId:      quizId2,
					ImageId:     imageId22,
					Answers: []Answer{
						{
							Id:          answerId221,
							TextContent: aTextContent221,
							IsCorrect:   isCorrect221,
							QuestionId:  questionId22,
						},
						{
							Id:          answerId222,
							TextContent: aTextContent222,
							IsCorrect:   isCorrect222,
							QuestionId:  questionId22,
						},
					},
				},
			},
		},
	}
	got, err := repo.ListQuizzes(context.Background())
	if err != nil {
		t.Fatalf("ListQuizzes error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListQuizzes got: %#v | expect: %#v", got, want)
	}
}

func TestListQuizzes_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId1   = 1
		title1    = "hgfds"
		authorId1 = 9

		questionId11   = 11
		qTextContent11 = "qwerty"
		position11     = 4
		imageId11      = "dfgh"

		answerId111     = 111
		aTextContent111 = "qwerty"
		isCorrect111    = true

		answerId112     = 112
		aTextContent112 = "asdfgh"
		isCorrect112    = false

		questionId12   = 12
		qTextContent12 = "qwerty"
		position12     = 5
		imageId12      = ""

		answerId121     = 121
		aTextContent121 = "hgfd"
		isCorrect121    = true

		answerId122     = 122
		aTextContent122 = "mnbvc"
		isCorrect122    = false

		quizId2   = 2
		title2    = "hgfds"
		authorId2 = 20

		questionId21   = 21
		qTextContent21 = "qwerty"
		position21     = 4
		imageId21      = "dfgh"

		answerId211     = 211
		aTextContent211 = "qwerty"
		isCorrect211    = true

		answerId212     = 212
		aTextContent212 = "asdfgh"
		isCorrect212    = false

		questionId22   = 22
		qTextContent22 = "qwerty"
		position22     = 5
		imageId22      = ""

		answerId221     = 221
		aTextContent221 = "hgfd"
		isCorrect221    = true

		answerId222     = 222
		aTextContent222 = "mnbvc"
		isCorrect222    = false
	)
	query := regexp.QuoteMeta(`SELECT id, title, author_id FROM quizzes`)
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id"}))
	// queryQuestions := regexp.QuoteMeta(`SELECT id, text_content, position, quiz_id, image_id
	// FROM questions
	// WHERE quiz_id=$1
	// ORDER BY position`)
	// mock.ExpectQuery(queryQuestions).
	// 	WithArgs(quizId1).
	// 	WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
	// 		AddRow(questionId11, qTextContent11, position11, quizId1, imageId11).
	// 		AddRow(questionId12, qTextContent12, position12, quizId1, imageId12))
	// queryAnswers := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	// FROM answers
	// WHERE question_id IN
	// (SELECT id FROM questions WHERE quiz_id=$1)`)
	// mock.ExpectQuery(queryAnswers).
	// 	WithArgs(quizId1).
	// 	WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
	// 		AddRow(answerId111, aTextContent111, isCorrect111, questionId11).
	// 		AddRow(answerId112, aTextContent112, isCorrect112, questionId11).
	// 		AddRow(answerId121, aTextContent121, isCorrect121, questionId12).
	// 		AddRow(answerId122, aTextContent122, isCorrect122, questionId12))
	// mock.ExpectQuery(queryQuestions).
	// 	WithArgs(quizId2).
	// 	WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
	// 		AddRow(questionId21, qTextContent21, position21, quizId2, imageId21).
	// 		AddRow(questionId22, qTextContent22, position22, quizId2, imageId22))
	// mock.ExpectQuery(queryAnswers).
	// 	WithArgs(quizId2).
	// 	WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
	// 		AddRow(answerId211, aTextContent211, isCorrect211, questionId21).
	// 		AddRow(answerId212, aTextContent212, isCorrect212, questionId21).
	// 		AddRow(answerId221, aTextContent221, isCorrect221, questionId22).
	// 		AddRow(answerId222, aTextContent222, isCorrect222, questionId22))
	want := []Quiz{}
	got, err := repo.ListQuizzes(context.Background())
	if err != nil {
		t.Fatalf("ListQuizzes error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListQuizzes got: %#v | expect: %#v", got, want)
	}
}

// =============== ListQuizzesByAuthor ===============

func TestListQuizzesByAuthor_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId1  = 1
		title1   = "hgfds"
		authorId = 9

		questionId11   = 11
		qTextContent11 = "qwerty"
		position11     = 4
		imageId11      = "dfgh"

		answerId111     = 111
		aTextContent111 = "qwerty"
		isCorrect111    = true

		answerId112     = 112
		aTextContent112 = "asdfgh"
		isCorrect112    = false

		questionId12   = 12
		qTextContent12 = "qwerty"
		position12     = 5
		imageId12      = ""

		answerId121     = 121
		aTextContent121 = "hgfd"
		isCorrect121    = true

		answerId122     = 122
		aTextContent122 = "mnbvc"
		isCorrect122    = false

		quizId2 = 2
		title2  = "hgfds"
		// authorId =

		questionId21   = 21
		qTextContent21 = "qwerty"
		position21     = 4
		imageId21      = "dfgh"

		answerId211     = 211
		aTextContent211 = "qwerty"
		isCorrect211    = true

		answerId212     = 212
		aTextContent212 = "asdfgh"
		isCorrect212    = false

		questionId22   = 22
		qTextContent22 = "qwerty"
		position22     = 5
		imageId22      = ""

		answerId221     = 221
		aTextContent221 = "hgfd"
		isCorrect221    = true

		answerId222     = 222
		aTextContent222 = "mnbvc"
		isCorrect222    = false
	)
	query := regexp.QuoteMeta(`SELECT id, title, author_id FROM quizzes WHERE author_id = $1`)
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id"}).
			AddRow(quizId1, title1, authorId).
			AddRow(quizId2, title2, authorId))
	queryQuestions := regexp.QuoteMeta(`SELECT id, text_content, position, quiz_id, image_id 
	FROM questions 
	WHERE quiz_id=$1 
	ORDER BY position`)
	mock.ExpectQuery(queryQuestions).
		WithArgs(quizId1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
			AddRow(questionId11, qTextContent11, position11, quizId1, imageId11).
			AddRow(questionId12, qTextContent12, position12, quizId1, imageId12))
	queryAnswers := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	FROM answers
	WHERE question_id IN 
	(SELECT id FROM questions WHERE quiz_id=$1)`)
	mock.ExpectQuery(queryAnswers).
		WithArgs(quizId1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId111, aTextContent111, isCorrect111, questionId11).
			AddRow(answerId112, aTextContent112, isCorrect112, questionId11).
			AddRow(answerId121, aTextContent121, isCorrect121, questionId12).
			AddRow(answerId122, aTextContent122, isCorrect122, questionId12))
	mock.ExpectQuery(queryQuestions).
		WithArgs(quizId2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
			AddRow(questionId21, qTextContent21, position21, quizId2, imageId21).
			AddRow(questionId22, qTextContent22, position22, quizId2, imageId22))
	mock.ExpectQuery(queryAnswers).
		WithArgs(quizId2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
			AddRow(answerId211, aTextContent211, isCorrect211, questionId21).
			AddRow(answerId212, aTextContent212, isCorrect212, questionId21).
			AddRow(answerId221, aTextContent221, isCorrect221, questionId22).
			AddRow(answerId222, aTextContent222, isCorrect222, questionId22))
	want := []Quiz{
		{
			Id:       quizId1,
			Title:    title1,
			AuthorId: authorId,
			Questions: []*Question{
				{
					Id:          questionId11,
					TextContent: qTextContent11,
					Position:    position11,
					QuizId:      quizId1,
					ImageId:     imageId11,
					Answers: []Answer{
						{
							Id:          answerId111,
							TextContent: aTextContent111,
							IsCorrect:   isCorrect111,
							QuestionId:  questionId11,
						},
						{
							Id:          answerId112,
							TextContent: aTextContent112,
							IsCorrect:   isCorrect112,
							QuestionId:  questionId11,
						},
					},
				},
				{
					Id:          questionId12,
					TextContent: qTextContent12,
					Position:    position12,
					QuizId:      quizId1,
					ImageId:     imageId12,
					Answers: []Answer{
						{
							Id:          answerId121,
							TextContent: aTextContent121,
							IsCorrect:   isCorrect121,
							QuestionId:  questionId12,
						},
						{
							Id:          answerId122,
							TextContent: aTextContent122,
							IsCorrect:   isCorrect122,
							QuestionId:  questionId12,
						},
					},
				},
			},
		},
		{
			Id:       quizId2,
			Title:    title2,
			AuthorId: authorId,
			Questions: []*Question{
				{
					Id:          questionId21,
					TextContent: qTextContent21,
					Position:    position21,
					QuizId:      quizId2,
					ImageId:     imageId21,
					Answers: []Answer{
						{
							Id:          answerId211,
							TextContent: aTextContent211,
							IsCorrect:   isCorrect211,
							QuestionId:  questionId21,
						},
						{
							Id:          answerId212,
							TextContent: aTextContent212,
							IsCorrect:   isCorrect212,
							QuestionId:  questionId21,
						},
					},
				},
				{
					Id:          questionId22,
					TextContent: qTextContent22,
					Position:    position22,
					QuizId:      quizId2,
					ImageId:     imageId22,
					Answers: []Answer{
						{
							Id:          answerId221,
							TextContent: aTextContent221,
							IsCorrect:   isCorrect221,
							QuestionId:  questionId22,
						},
						{
							Id:          answerId222,
							TextContent: aTextContent222,
							IsCorrect:   isCorrect222,
							QuestionId:  questionId22,
						},
					},
				},
			},
		},
	}
	got, err := repo.ListQuizzesByAuthor(context.Background(), authorId)
	if err != nil {
		t.Fatalf("ListQuizzesByAuthor error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListQuizzesByAuthor got: %#v | expect: %#v", got, want)
	}
}

func TestListQuizzesByAuthor_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		quizId1  = 1
		title1   = "hgfds"
		authorId = 9

		questionId11   = 11
		qTextContent11 = "qwerty"
		position11     = 4
		imageId11      = "dfgh"

		answerId111     = 111
		aTextContent111 = "qwerty"
		isCorrect111    = true

		answerId112     = 112
		aTextContent112 = "asdfgh"
		isCorrect112    = false

		questionId12   = 12
		qTextContent12 = "qwerty"
		position12     = 5
		imageId12      = ""

		answerId121     = 121
		aTextContent121 = "hgfd"
		isCorrect121    = true

		answerId122     = 122
		aTextContent122 = "mnbvc"
		isCorrect122    = false

		quizId2 = 2
		title2  = "hgfds"
		// authorId2 = 20

		questionId21   = 21
		qTextContent21 = "qwerty"
		position21     = 4
		imageId21      = "dfgh"

		answerId211     = 211
		aTextContent211 = "qwerty"
		isCorrect211    = true

		answerId212     = 212
		aTextContent212 = "asdfgh"
		isCorrect212    = false

		questionId22   = 22
		qTextContent22 = "qwerty"
		position22     = 5
		imageId22      = ""

		answerId221     = 221
		aTextContent221 = "hgfd"
		isCorrect221    = true

		answerId222     = 222
		aTextContent222 = "mnbvc"
		isCorrect222    = false
	)
	query := regexp.QuoteMeta(`SELECT id, title, author_id FROM quizzes WHERE author_id = $1`)
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id"}))
	// queryQuestions := regexp.QuoteMeta(`SELECT id, text_content, position, quiz_id, image_id
	// FROM questions
	// WHERE quiz_id=$1
	// ORDER BY position`)
	// mock.ExpectQuery(queryQuestions).
	// 	WithArgs(quizId1).
	// 	WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
	// 		AddRow(questionId11, qTextContent11, position11, quizId1, imageId11).
	// 		AddRow(questionId12, qTextContent12, position12, quizId1, imageId12))
	// queryAnswers := regexp.QuoteMeta(`SELECT id, text_content, is_correct, question_id
	// FROM answers
	// WHERE question_id IN
	// (SELECT id FROM questions WHERE quiz_id=$1)`)
	// mock.ExpectQuery(queryAnswers).
	// 	WithArgs(quizId1).
	// 	WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
	// 		AddRow(answerId111, aTextContent111, isCorrect111, questionId11).
	// 		AddRow(answerId112, aTextContent112, isCorrect112, questionId11).
	// 		AddRow(answerId121, aTextContent121, isCorrect121, questionId12).
	// 		AddRow(answerId122, aTextContent122, isCorrect122, questionId12))
	// mock.ExpectQuery(queryQuestions).
	// 	WithArgs(quizId2).
	// 	WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "position", "quiz_id", "image_id"}).
	// 		AddRow(questionId21, qTextContent21, position21, quizId2, imageId21).
	// 		AddRow(questionId22, qTextContent22, position22, quizId2, imageId22))
	// mock.ExpectQuery(queryAnswers).
	// 	WithArgs(quizId2).
	// 	WillReturnRows(sqlmock.NewRows([]string{"id", "text_content", "is_correct", "question_id"}).
	// 		AddRow(answerId211, aTextContent211, isCorrect211, questionId21).
	// 		AddRow(answerId212, aTextContent212, isCorrect212, questionId21).
	// 		AddRow(answerId221, aTextContent221, isCorrect221, questionId22).
	// 		AddRow(answerId222, aTextContent222, isCorrect222, questionId22))
	want := []Quiz{}
	got, err := repo.ListQuizzesByAuthor(context.Background(), authorId)
	if err != nil {
		t.Fatalf("ListQuizzesByAuthor error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListQuizzesByAuthor got: %#v | expect: %#v", got, want)
	}
}

// func TestCreateQuiz_(t *testing.T) {
// 	repo, mock, cleanup := newPostgresRepoMock(t)
// 	defer cleanup()
// 	const (

// 	)
// 	query := regexp.QuoteMeta()
// 	mock.ExpectQuery(query).
// 		WithArgs().
// 		WillReturnRows(sqlmock.NewRows([]string{}).
// 			AddRow())
// 	err := repo.
// 	if err != nil {
// 		t.Fatalf(" error got: %v | expect: %v", err, nil)
// 	}
// }
