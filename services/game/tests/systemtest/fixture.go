// Package systemtest contains protocol-level helpers shared by system and load tests.
package systemtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

type QuizFixture struct {
	Authorization string
	QuizID        int64
	Questions     []QuestionFixture
}

type QuestionFixture struct {
	ID                int64
	CorrectAnswerID   int64
	IncorrectAnswerID int64
}

type Game struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	HostTicket string `json:"host_ticket"`
}

func (c Client) ProvisionQuiz(ctx context.Context, questionCount, timeLimitSeconds int) (QuizFixture, error) {
	if questionCount < 1 {
		return QuizFixture{}, fmt.Errorf("question count must be positive")
	}
	username := "system_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	password := "system-test-123"
	credentials := map[string]string{"username": username, "password": password}
	if err := c.doJSON(ctx, http.MethodPost, "/auth/register", "", credentials, http.StatusCreated, nil); err != nil {
		return QuizFixture{}, fmt.Errorf("register fixture author: %w", err)
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/auth/login", "", credentials, http.StatusOK, &login); err != nil {
		return QuizFixture{}, fmt.Errorf("login fixture author: %w", err)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/quizzes", login.Token, map[string]string{"title": "System test quiz"}, http.StatusCreated, &created); err != nil {
		return QuizFixture{}, fmt.Errorf("create fixture quiz: %w", err)
	}
	var author struct {
		Revision int64 `json:"revision"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/quizzes/%d/author", created.ID), login.Token, nil, http.StatusOK, &author); err != nil {
		return QuizFixture{}, fmt.Errorf("load fixture quiz revision: %w", err)
	}

	questions := make([]map[string]any, questionCount)
	for index := range questions {
		questions[index] = map[string]any{
			"id":           0,
			"text_content": fmt.Sprintf("System question %d", index+1),
			"image_id":     "",
			"time_limit":   timeLimitSeconds,
			"answers": []map[string]any{
				{"id": 0, "text_content": "Correct", "is_correct": true},
				{"id": 0, "text_content": "Incorrect", "is_correct": false},
			},
		}
	}
	var saved struct {
		Questions []struct {
			ID      int64 `json:"id"`
			Answers []struct {
				ID        int64 `json:"id"`
				IsCorrect bool  `json:"is_correct"`
			} `json:"answers"`
		} `json:"questions"`
	}
	body := map[string]any{"title": "System test quiz", "revision": author.Revision, "questions": questions}
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/quizzes/%d/content", created.ID), login.Token, body, http.StatusOK, &saved); err != nil {
		return QuizFixture{}, fmt.Errorf("save fixture quiz: %w", err)
	}
	fixture := QuizFixture{Authorization: login.Token, QuizID: created.ID, Questions: make([]QuestionFixture, len(saved.Questions))}
	for questionIndex, question := range saved.Questions {
		fixture.Questions[questionIndex].ID = question.ID
		for _, answer := range question.Answers {
			if answer.IsCorrect {
				fixture.Questions[questionIndex].CorrectAnswerID = answer.ID
			} else if fixture.Questions[questionIndex].IncorrectAnswerID == 0 {
				fixture.Questions[questionIndex].IncorrectAnswerID = answer.ID
			}
		}
		if fixture.Questions[questionIndex].CorrectAnswerID == 0 || fixture.Questions[questionIndex].IncorrectAnswerID == 0 {
			return QuizFixture{}, fmt.Errorf("saved question %d has incomplete answers", questionIndex)
		}
	}
	return fixture, nil
}

func (c Client) CreateGame(ctx context.Context, authorization string, quizID int64) (Game, error) {
	var game Game
	if err := c.doJSON(ctx, http.MethodPost, "/api/games", authorization, map[string]int64{"quiz_id": quizID}, http.StatusCreated, &game); err != nil {
		return Game{}, fmt.Errorf("create game: %w", err)
	}
	return game, nil
}

func (c Client) doJSON(ctx context.Context, method, path, authorization string, body any, expectedStatus int, target any) error {
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, requestBody)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != expectedStatus {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return fmt.Errorf("%s %s returned %d, want %d: %s", method, path, response.StatusCode, expectedStatus, strings.TrimSpace(string(data)))
	}
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}
