package quizhandler

import "github.com/kungrem23/quizgo/internal/domain/quiz"

type QuizResponse struct {
	ID        int                 `json:"id"`
	Title     string              `json:"title"`
	AuthorID  int                 `json:"author_id"`
	Questions []*QuestionResponse `json:"questions"`
}

type QuestionResponse struct {
	ID          int              `json:"id"`
	QuizID      int              `json:"quiz_id"`
	Position    int              `json:"position"`
	TextContent string           `json:"text_content"`
	ImageID     string           `json:"image_id"`
	Answers     []AnswerResponse `json:"answers"`
}

type AnswerResponse struct {
	ID          int    `json:"id"`
	TextContent string `json:"text_content"`
	IsCorrect   bool   `json:"is_correct"`
	QuestionID  int    `json:"question_id"`
}

type UserResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func newQuizResponse(value quiz.Quiz) QuizResponse {
	result := QuizResponse{ID: value.Id, Title: value.Title, AuthorID: value.AuthorId}
	if value.Questions != nil {
		result.Questions = make([]*QuestionResponse, len(value.Questions))
		for i, question := range value.Questions {
			if question != nil {
				response := newQuestionResponse(*question)
				result.Questions[i] = &response
			}
		}
	}
	return result
}

func newQuizResponses(values []quiz.Quiz) []QuizResponse {
	if values == nil {
		return nil
	}
	result := make([]QuizResponse, len(values))
	for i := range values {
		result[i] = newQuizResponse(values[i])
	}
	return result
}

func newQuestionResponse(value quiz.Question) QuestionResponse {
	return QuestionResponse{
		ID:          value.Id,
		QuizID:      value.QuizId,
		Position:    value.Position,
		TextContent: value.TextContent,
		ImageID:     value.ImageId,
		Answers:     newAnswerResponses(value.Answers),
	}
}

func newAnswerResponse(value quiz.Answer) AnswerResponse {
	return AnswerResponse{
		ID:          value.Id,
		TextContent: value.TextContent,
		IsCorrect:   value.IsCorrect,
		QuestionID:  value.QuestionId,
	}
}

func newAnswerResponses(values []quiz.Answer) []AnswerResponse {
	if values == nil {
		return nil
	}
	result := make([]AnswerResponse, len(values))
	for i := range values {
		result[i] = newAnswerResponse(values[i])
	}
	return result
}
