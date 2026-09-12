package quizhandler

import "github.com/kungrem23/quizgo/internal/domain/quiz"

type QuizResponse struct {
	ID        int                 `json:"id" example:"1"`
	Title     string              `json:"title" example:"География"`
	AuthorID  int                 `json:"author_id" example:"42"`
	Questions []*QuestionResponse `json:"questions"`
}

type QuestionResponse struct {
	ID          int    `json:"id" example:"1"`
	QuizID      int    `json:"quiz_id" example:"1"`
	Position    int    `json:"position" example:"1"`
	TextContent string `json:"text_content" example:"Столица Франции?"`
	ImageID     string `json:"image_id" example:"image-1"`
	// null, если ответы отсутствуют или не были загружены.
	Answers []PublicAnswerResponse `json:"answers" extensions:"x-nullable"`
}

type PublicAnswerResponse struct {
	ID          int    `json:"id" example:"1"`
	TextContent string `json:"text_content" example:"Париж"`
	QuestionID  int    `json:"question_id" example:"1"`
}

type AuthorAnswerResponse struct {
	ID          int    `json:"id" example:"1"`
	TextContent string `json:"text_content" example:"Париж"`
	IsCorrect   bool   `json:"is_correct" example:"true"`
	QuestionID  int    `json:"question_id" example:"1"`
}

type UserResponse struct {
	ID       int    `json:"id" example:"42"`
	Username string `json:"username" example:"alice"`
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
		Answers:     newPublicAnswerResponses(value.Answers),
	}
}

func newPublicAnswerResponse(value quiz.Answer) PublicAnswerResponse {
	return PublicAnswerResponse{
		ID:          value.Id,
		TextContent: value.TextContent,
		QuestionID:  value.QuestionId,
	}
}

func newAuthorAnswerResponse(value quiz.Answer) AuthorAnswerResponse {
	return AuthorAnswerResponse{
		ID:          value.Id,
		TextContent: value.TextContent,
		IsCorrect:   value.IsCorrect,
		QuestionID:  value.QuestionId,
	}
}

func newPublicAnswerResponses(values []quiz.Answer) []PublicAnswerResponse {
	if values == nil {
		return nil
	}
	result := make([]PublicAnswerResponse, len(values))
	for i := range values {
		result[i] = newPublicAnswerResponse(values[i])
	}
	return result
}
