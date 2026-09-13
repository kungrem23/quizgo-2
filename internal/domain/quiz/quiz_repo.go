package quiz

import (
	"context"
	"database/sql"
	// "fmt"
	// "log"
	// "github.com/kungrem23/quizgo/internal/store/models"
)

// type QuizRepo struct {
// 	db *sql.DB
// }

// func NewQuizRepo(db *sql.DB) *Repository {
// 	return &QuizRepo{db: db}
// }

func (r *PostgresRepository) CreateQuiz(ctx context.Context, title string, userId int) (Quiz, error) {
	q := Quiz{Questions: []*Question{}}
	err := r.db.QueryRowContext(ctx, `INSERT INTO quizzes (title, author_id) VALUES ($1, $2) RETURNING id, title, author_id`, title, userId).Scan(&q.Id, &q.Title, &q.AuthorId)
	return q, err
}

func (r *PostgresRepository) GetQuizAnswers(ctx context.Context, quizId int, questionMap map[int]*Question) error {
	query := `SELECT id, text_content, is_correct, question_id
	FROM answers
	WHERE question_id IN 
	(SELECT id FROM questions WHERE quiz_id=$1) ORDER BY position, id`
	rows, err := r.db.QueryContext(ctx, query, quizId)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a Answer
		err = rows.Scan(&a.Id, &a.TextContent, &a.IsCorrect, &a.QuestionId)
		if err != nil {
			return err
		}
		if question, ok := questionMap[a.QuestionId]; ok {
			question.Answers = append(question.Answers, a)
		}
	}
	return nil
}

func (r *PostgresRepository) GetQuizQuestions(ctx context.Context, quizId int) ([]*Question, error) {
	query := `SELECT id, text_content, position, quiz_id, image_id 
	FROM questions 
	WHERE quiz_id=$1 
	ORDER BY position`
	rows, err := r.db.QueryContext(ctx, query, quizId)
	if err != nil {
		// log.Printf("Scanning quiz(id=%v) error: %v", quizId, err)
		return nil, err
	}
	defer rows.Close()
	questions := []*Question{}
	questionMap := make(map[int]*Question)
	for rows.Next() {
		qu := &Question{}
		var imageID sql.NullString
		err := rows.Scan(&qu.Id, &qu.TextContent, &qu.Position, &qu.QuizId, &imageID)
		qu.ImageId = imageID.String
		if err != nil {
			return nil, err
		}
		questions = append(questions, qu)
		questionMap[qu.Id] = qu
	}
	err = r.GetQuizAnswers(ctx, quizId, questionMap)
	if err != nil {
		return nil, err
	}
	return questions, nil
}

func (r *PostgresRepository) GetQuiz(ctx context.Context, id int) (Quiz, error) {
	var q Quiz
	query := `SELECT id, title, author_id FROM quizzes WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(&q.Id, &q.Title, &q.AuthorId)
	if err != nil {
		// log.Printf("Scanning quiz(id=%v) error: %v", id, err)
		return q, err
	}
	questions, err := r.GetQuizQuestions(ctx, id)
	if err != nil {
		return q, err
	}
	q.Questions = questions
	return q, nil
}

func (r *PostgresRepository) ListQuizzes(ctx context.Context) ([]Quiz, error) {
	quizzes := []Quiz{}
	query := `SELECT id, title, author_id FROM quizzes`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var q Quiz
		err := rows.Scan(&q.Id, &q.Title, &q.AuthorId)
		if err != nil {
			return nil, err
		}
		// questions, err := r.GetQuizQuestions(ctx, q.Id)
		// if err != nil {
		// 	return nil, err
		// }
		// q.Questions = questions
		quizzes = append(quizzes, q)
	}
	for i := range quizzes {
		questions, err := r.GetQuizQuestions(ctx, quizzes[i].Id)
		if err != nil {
			return nil, err
		}
		quizzes[i].Questions = questions
	}
	return quizzes, nil
}

func (r *PostgresRepository) ListQuizzesByAuthor(ctx context.Context, authorID int) ([]Quiz, error) {
	quizzes := []Quiz{}
	query := `SELECT id, title, author_id FROM quizzes WHERE author_id = $1`
	rows, err := r.db.QueryContext(ctx, query, authorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// for rows.Next() {
	// 	var q Quiz
	// 	err := rows.Scan(&q.Id, &q.Title, &q.AuthorId)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	questions, err := r.GetQuizQuestions(ctx, q.Id)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	q.Questions = questions
	// 	quizzes = append(quizzes, q)
	// }
	for rows.Next() {
		var q Quiz
		err := rows.Scan(&q.Id, &q.Title, &q.AuthorId)
		if err != nil {
			return nil, err
		}
		// questions, err := r.GetQuizQuestions(ctx, q.Id)
		// if err != nil {
		// 	return nil, err
		// }
		// q.Questions = questions
		quizzes = append(quizzes, q)
	}
	for i := range quizzes {
		questions, err := r.GetQuizQuestions(ctx, quizzes[i].Id)
		if err != nil {
			return nil, err
		}
		quizzes[i].Questions = questions
	}

	return quizzes, nil
}

// func (r *QuizRepo) GetQuizzesByAuthorId(authorId int) ([]*models.Quiz, error) {
// 	query := `SELECT id, title, author_id FROM quizzes WHERE author_id=$1`
// 	rows, err := r.db.Query(query, authorId)
// 	if err != nil {
// 		log.Printf("Get quizzes error: %v", err)
// 		return nil, err
// 	}
// 	defer rows.Close()
// 	var quizzes []*models.Quiz
// 	for rows.Next() {
// 		quiz := models.NewQuiz()
// 		err = rows.Scan(&quiz.Id, &quiz.Title, &quiz.AuthorId)
// 		if err != nil {
// 			log.Printf("Scanning quiz error: %v", err)
// 			return nil, err
// 		}
// 		quizzes = append(quizzes, quiz)
// 	}
// 	return quizzes, nil
// }

// func (r *QuizRepo) GetAllQuizzes() ([]*models.Quiz, error) {
// 	query := `SELECT id, title, author_id FROM quizzes`
// 	rows, err := r.db.Query(query)
// 	if err != nil {
// 		log.Printf("Get quizzes error: %v", err)
// 		return nil, err
// 	}
// 	defer rows.Close()
// 	var quizzes []*models.Quiz
// 	for rows.Next() {
// 		quiz := models.NewQuiz()
// 		err = rows.Scan(&quiz.Id, &quiz.Title, &quiz.AuthorId)
// 		if err != nil {
// 			log.Printf("Scanning quiz error: %v", err)
// 			return nil, err
// 		}
// 		quizzes = append(quizzes, quiz)
// 	}
// 	return quizzes, nil
// }
