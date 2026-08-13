package quiz

import (
	// "database/sql"
	// "github.com/kungrem23/quizgo/internal/store/models"
	"context"

	"github.com/kungrem23/quizgo/internal/http/middleware"
	// "log"
)

// type QuestionRepo struct {
// 	db *sql.DB
// }

// func NewQuestionRepo(db *sql.DB) *QuestionRepo {
// 	return &QuestionRepo{db: db}
// }

func (r *PostgresRepository) CreateNewQuestionAsAuthor(ctx context.Context, textContent string, quizId, authorId int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `SELECT quizzes.author_id
	FROM quizzes
	WHERE quizzes.id = $1`
	row := tx.QueryRowContext(ctx, query, quizId)
	var quizAuthorId int
	err = row.Scan(&quizAuthorId)
	if err != nil {
		return err
	}
	if quizAuthorId != authorId {
		return middleware.ErrInsufficientRights
	}

	queryMax := `SELECT COALESCE(MAX(position), 0) + 1
	FROM questions
	WHERE quiz_id = $1;`
	row = tx.QueryRowContext(ctx, queryMax, quizId)
	var position int
	err = row.Scan(&position)
	if err != nil {
		// log.Printf("Getting question's position errror: %v\n", err)
		return err
	}
	query = `INSERT INTO questions
	(text_content, position, quiz_id)
	VALUES ($1, $2, $3)`
	// question := NewQuestion()
	// if imageId != "" {
	// 	_, err = r.db.ExecContext(ctx, query, textContent, position, quizId, imageId)
	// } else {
	_, err = tx.ExecContext(ctx, query, textContent, position, quizId)
	// }
	// err = row.Scan(&question.Id, &question.Position, &question.TextContent, &question.QuizId, &question.ImageId)
	if err != nil {
		// log.Printf("Adding question error: %v", err)
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepository) DeleteQuestionAsAuthor(ctx context.Context, id int, authorId int) error {
	query := `SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	var quizAuthorId int
	err := row.Scan(&quizAuthorId)
	if err != nil {
		return err
	}
	if quizAuthorId != authorId {
		return middleware.ErrInsufficientRights
	}
	question, err := r.GetQuestion(ctx, id)
	if err != nil {
		return err
	}
	pos := question.Position

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query = `DELETE FROM questions WHERE id = $1;`
	_, err = tx.ExecContext(ctx, query, id)
	if err != nil {
		// log.Printf("Deleting quesiton error: %v\n", err)
		return err
	}
	query = `UPDATE questions
	SET position = position - 1
	WHERE position > $1 AND quiz_id = $2;`
	_, err = tx.ExecContext(ctx, query, pos, question.QuizId)
	if err != nil {
		// log.Printf("Changing question's positions error: %v", err)
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepository) ChangeQuestionPosition(ctx context.Context, id, new_position, authorId int) error {
	query := `SELECT quizzes.author_id
	FROM questions
	JOIN quizzes ON quizzes.id = questions.quiz_id
	WHERE questions.id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	var quizAuthorId int
	if err := row.Scan(&quizAuthorId); err != nil {
		return err
	}
	if quizAuthorId != authorId {
		return middleware.ErrInsufficientRights
	}

	question, err := r.GetQuestion(ctx, id)
	if err != nil {
		return err
	}
	old_pos := question.Position

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	countQuery := `SELECT COUNT(*)
	FROM questions
	WHERE quiz_id = $1`
	var questionCount int
	if err := tx.QueryRowContext(ctx, countQuery, question.QuizId).Scan(&questionCount); err != nil {
		return err
	}
	if new_position < 1 || new_position > questionCount {
		return ErrInvalidQuestionPosition
	}
	if new_position == old_pos {
		return tx.Commit()
	}

	query = ""
	if new_position < old_pos {
		query = `UPDATE questions
			SET position = position + 1
			WHERE quiz_id = $1 AND position < $2 AND position >= $3;`
	} else if new_position > old_pos {
		query = `UPDATE questions
			SET position = position - 1
			WHERE quiz_id = $1 AND position > $2 AND position <= $3;`
	}
	_, err = tx.ExecContext(ctx, query, question.QuizId, old_pos, new_position)
	if err != nil {
		// log.Printf("Changing question's position error: %v\n", err)
		return err
	}
	query = `UPDATE questions
	SET position = $1
	WHERE id = $2`
	// question := NewQuestion()
	_, err = tx.ExecContext(ctx, query, new_position, id)
	// err = row.Scan(&question.Id, &question.TextContent, &question.Position, &question.QuizId, &question.ImageId)
	if err != nil {
		// log.Printf("Changing question's(id = %v) position error: %v", id, err)
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepository) GetQuestion(ctx context.Context, id int) (Question, error) {
	query := `SELECT id, text_content, position, image_id, quiz_id FROM questions
	WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	var question Question
	err := row.Scan(&question.Id, &question.TextContent, &question.Position, &question.ImageId, &question.QuizId)
	// if err != nil {
	// 	log.Printf("Scanning question(id=%v) error: %v", id, err)
	// 	return question, err
	// }
	return question, err
}

func (r *PostgresRepository) GetAllQuestions(ctx context.Context) ([]Question, error) {
	query := `SELECT id, text_content, position, image_id, quiz_id FROM questions`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		// log.Printf("Get questions error: %v", err)
		return nil, err
	}
	var questions []Question
	defer rows.Close()
	for rows.Next() {
		var q Question
		err := rows.Scan(&q.Id, &q.TextContent, &q.Position, &q.ImageId, &q.QuizId)
		if err != nil {
			// log.Printf("Scanning question error: %v", err)
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

// func (r *Repository) GetQuestionsByQuizId(quiz_id int) ([]*Question, error) {
// 	query := `SELECT id, text_content, position, image_id, quiz_id FROM questions
// 	WHERE quiz_id=$1`
// 	rows, err := r.db.Query(query, quiz_id)
// 	if err != nil {
// 		log.Printf("Get questions error: %v", err)
// 		return nil, err
// 	}
// 	var questions []*Question
// 	defer rows.Close()
// 	for rows.Next() {
// 		question := NewQuestion()
// 		err := rows.Scan(&question.Id, &question.TextContent, &question.Position, &question.ImageId, &question.QuizId)
// 		if err != nil {
// 			log.Printf("Scanning question error: %v", err)
// 			return nil, err
// 		}
// 		questions = append(questions, question)
// 	}
// 	return questions, nil
// }
