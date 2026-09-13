package quiz

import (
	"context"
	"database/sql"

	"github.com/kungrem23/quizgo/internal/http/middleware"
	"github.com/lib/pq"
)

type contentDB interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readContent(ctx context.Context, db contentDB, id int) (QuizContent, error) {
	v := QuizContent{Questions: []ContentQuestion{}}
	err := db.QueryRowContext(ctx, `SELECT id, title, author_id, updated_at, revision FROM quizzes WHERE id=$1`, id).Scan(&v.ID, &v.Title, &v.AuthorID, &v.UpdatedAt, &v.Revision)
	if err != nil {
		return v, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id, text_content, COALESCE(image_id, ''), time_limit FROM questions WHERE quiz_id=$1 ORDER BY position, id`, id)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		q := ContentQuestion{Answers: []ContentAnswer{}}
		if err = rows.Scan(&q.ID, &q.TextContent, &q.ImageID, &q.TimeLimit); err != nil {
			rows.Close()
			return v, err
		}
		v.Questions = append(v.Questions, q)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	v.QuestionCount = len(v.Questions)
	indices := map[int]int{}
	for i, q := range v.Questions {
		indices[q.ID] = i
	}
	rows, err = db.QueryContext(ctx, `SELECT a.id, a.text_content, a.is_correct, a.question_id FROM answers a JOIN questions q ON q.id=a.question_id WHERE q.quiz_id=$1 ORDER BY a.position, a.id`, id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var a ContentAnswer
		var qid int
		if err = rows.Scan(&a.ID, &a.TextContent, &a.IsCorrect, &qid); err != nil {
			return v, err
		}
		if index, ok := indices[qid]; ok {
			v.Questions[index].Answers = append(v.Questions[index].Answers, a)
		}
	}
	return v, rows.Err()
}

func (r *PostgresRepository) GetQuizContent(ctx context.Context, id int) (QuizContent, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return QuizContent{}, err
	}
	defer tx.Rollback()
	v, err := readContent(ctx, tx, id)
	if err != nil {
		return v, err
	}
	return v, tx.Commit()
}

func (r *PostgresRepository) ListQuizSummaries(ctx context.Context, userID int) ([]QuizSummary, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT q.id, q.title, q.author_id, q.updated_at, q.revision, COUNT(questions.id) FROM quizzes q LEFT JOIN questions ON questions.quiz_id=q.id WHERE q.author_id=$1 GROUP BY q.id ORDER BY q.updated_at DESC, q.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []QuizSummary{}
	for rows.Next() {
		var v QuizSummary
		if err := rows.Scan(&v.ID, &v.Title, &v.AuthorID, &v.UpdatedAt, &v.Revision, &v.QuestionCount); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func lockQuiz(ctx context.Context, tx *sql.Tx, id, userID int) (int64, error) {
	var authorID int
	var revision int64
	err := tx.QueryRowContext(ctx, `SELECT author_id, revision FROM quizzes WHERE id=$1 FOR UPDATE`, id).Scan(&authorID, &revision)
	if err != nil {
		return 0, err
	}
	if authorID != userID {
		return 0, middleware.ErrInsufficientRights
	}
	return revision, nil
}

func expectOne(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrContentInvalid
	}
	return nil
}

func (r *PostgresRepository) SaveQuizContent(ctx context.Context, id, userID int, input SaveQuizContent) (QuizContent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return QuizContent{}, err
	}
	defer tx.Rollback()
	revision, err := lockQuiz(ctx, tx, id, userID)
	if err != nil {
		return QuizContent{}, err
	}
	if revision != input.Revision {
		return QuizContent{}, ErrContentConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE quizzes SET title=$1, updated_at=clock_timestamp(), revision=revision+1 WHERE id=$2`, input.Title, id); err != nil {
		return QuizContent{}, err
	}
	retained := []int{}
	for qi, q := range input.Questions {
		if q.ImageID != "" {
			var exists bool
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM images WHERE id=$1 AND author_id=$2)`, q.ImageID, userID).Scan(&exists)
			if err != nil {
				return QuizContent{}, err
			}
			if !exists {
				return QuizContent{}, ErrContentInvalid
			}
		}
		if q.ID == 0 {
			err = tx.QueryRowContext(ctx, `INSERT INTO questions (quiz_id, position, text_content, image_id, time_limit) VALUES ($1,$2,$3,NULLIF($4,''),$5) RETURNING id`, id, qi+1, q.TextContent, q.ImageID, q.TimeLimit).Scan(&q.ID)
		} else {
			err = expectOne(tx.ExecContext(ctx, `UPDATE questions SET position=$1, text_content=$2, image_id=NULLIF($3,''), time_limit=$4 WHERE id=$5 AND quiz_id=$6`, qi+1, q.TextContent, q.ImageID, q.TimeLimit, q.ID, id))
		}
		if err != nil {
			return QuizContent{}, err
		}
		retained = append(retained, q.ID)
		answerIDs := []int{}
		for ai, a := range q.Answers {
			if a.ID == 0 {
				err = tx.QueryRowContext(ctx, `INSERT INTO answers (question_id, position, text_content, is_correct) VALUES ($1,$2,$3,$4) RETURNING id`, q.ID, ai+1, a.TextContent, a.IsCorrect).Scan(&a.ID)
			} else {
				err = expectOne(tx.ExecContext(ctx, `UPDATE answers SET position=$1, text_content=$2, is_correct=$3 WHERE id=$4 AND question_id=$5`, ai+1, a.TextContent, a.IsCorrect, a.ID, q.ID))
			}
			if err != nil {
				return QuizContent{}, err
			}
			answerIDs = append(answerIDs, a.ID)
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM answers WHERE question_id=$1 AND NOT(id=ANY($2))`, q.ID, pq.Array(answerIDs)); err != nil {
			return QuizContent{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM questions WHERE quiz_id=$1 AND NOT(id=ANY($2))`, id, pq.Array(retained)); err != nil {
		return QuizContent{}, err
	}
	v, err := readContent(ctx, tx, id)
	if err != nil {
		return v, err
	}
	return v, tx.Commit()
}

func (r *PostgresRepository) DeleteQuizAsAuthor(ctx context.Context, id, userID int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = lockQuiz(ctx, tx, id, userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM quizzes WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}
