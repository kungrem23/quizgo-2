package quizhandler

import "github.com/kungrem23/quizgo/internal/domain/quiz"

type QuizResponse struct {
	ID        int                 `json:"id"`
	Title     string              `json:"title"`
	AuthorID  int                 `json:"author_id"`
	Questions []*QuestionResponse `json:"questions"`
}

type QuestionResponse struct {
	ID          int                    `json:"id"`
	QuizID      int                    `json:"quiz_id"`
	Position    int                    `json:"position"`
	TextContent string                 `json:"text_content"`
	ImageID     string                 `json:"image_id"`
	Answers     []PublicAnswerResponse `json:"answers"`
}

type PublicAnswerResponse struct {
	ID          int    `json:"id"`
	TextContent string `json:"text_content"`
	QuestionID  int    `json:"question_id"`
}

type AuthorAnswerResponse struct {
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
