package quiz

import (
	// "database/sql"
	// "github.com/kungrem23/quizgo/internal/store/models"
	"context"

	"github.com/kungrem23/quizgo/internal/http/middleware"
	// "log"
)

// type AnswerRepo struct {
// 	db *sql.DB
// }

// func NewAnswerRepo(db *sql.DB) *AnswerRepo {
// 	return &AnswerRepo{db: db}
// }

func (r *PostgresRepository) CreateNewAnswerAsAuthor(ctx context.Context, textContent string, isCorrect bool, questionId int, userId int) error {
	query := `SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON questions.quiz_id = quizzes.id
	WHERE questions.id = $1`
	row := r.db.QueryRowContext(ctx, query, questionId)
	var quizAuthorId int
	err := row.Scan(&quizAuthorId)
	if err != nil {
		return err
	}
	if quizAuthorId != userId {
		return middleware.ErrInsufficientRights
	}
	query = `INSERT INTO answers
	(text_content, is_correct, question_id)
	VALUES ($1, $2, $3)`
	_, err = r.db.ExecContext(ctx, query, textContent, isCorrect, questionId)
	// answer := NewAnswer()
	// err := row.Scan(&answer.Id, &answer.TextContent, &answer.IsCorrect, &answer.QuestionId)
	// if err != nil {
	// 	log.Printf("Adding answer error: %v\n", err)
	// 	return nil, err
	// }
	return err
}

func (r *PostgresRepository) DeleteAnswerAsAuthor(ctx context.Context, id int, userId int) error {
	query := `SELECT quizzes.author_id
	FROM answers
	JOIN questions ON questions.id = answers.question_id
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE answers.id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	var authorId int
	err := row.Scan(&authorId)
	if err != nil {
		return err
	}
	if userId != authorId {
		return middleware.ErrInsufficientRights
	}
	query = `DELETE FROM answers WHERE id = $1;`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	// if err != nil {
	// 	log.Printf("Deleting answer(id=%v) error: %v", id, err)
	// 	return err
	// }
	return nil
}

func (r *PostgresRepository) GetAnswer(ctx context.Context, id int) (Answer, error) {
	query := `SELECT id, text_content, is_correct, question_id 
	FROM answers WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	var answer Answer
	err := row.Scan(&answer.Id, &answer.TextContent, &answer.IsCorrect, &answer.QuestionId)
	// if err != nil {
	// 	log.Printf("Get answer(id=%v) error: %v", id, err)
	// 	return answer,  err
	// }
	return answer, err
}

func (r *PostgresRepository) GetAnswerAsAuthor(ctx context.Context, id, userId int) (Answer, error) {
	query := `SELECT answers.id, answers.text_content, answers.is_correct, answers.question_id, quizzes.author_id
	FROM answers
	JOIN questions ON questions.id = answers.question_id
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE answers.id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	var answer Answer
	var authorId int
	if err := row.Scan(&answer.Id, &answer.TextContent, &answer.IsCorrect, &answer.QuestionId, &authorId); err != nil {
		return Answer{}, err
	}
	if authorId != userId {
		return Answer{}, middleware.ErrInsufficientRights
	}
	return answer, nil
}

func (r *PostgresRepository) GetAllAnswers(ctx context.Context) ([]Answer, error) {
	query := `SELECT id, text_content, is_correct, question_id 
	FROM answers`
	rows, err := r.db.QueryContext(ctx, query)

	if err != nil {
		// log.Printf("Get answers error: %v", err)
		return nil, err
	}
	var answers []Answer
	defer rows.Close()
	for rows.Next() {
		var a Answer
		err := rows.Scan(&a.Id, &a.TextContent, &a.IsCorrect, &a.QuestionId)
		if err != nil {
			// log.Printf("Scanning answer error: %v", err)
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, nil
}

func (r *PostgresRepository) GetAnswersByQuestionId(ctx context.Context, questionId int) ([]Answer, error) {
	query := `SELECT id, text_content, is_correct, question_id 
	FROM answers
	WHERE question_id=$1`
	rows, err := r.db.QueryContext(ctx, query, questionId)
	if err != nil {
		// log.Printf("Get answers error: %v", err)
		return nil, err
	}
	var answers []Answer
	defer rows.Close()
	for rows.Next() {
		var a Answer
		err := rows.Scan(&a.Id, &a.TextContent, &a.IsCorrect, &a.QuestionId)
		if err != nil {
			// log.Printf("Scanning answer error: %v", err)
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, nil
}
