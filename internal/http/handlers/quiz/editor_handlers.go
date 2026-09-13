package quizhandler

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/kungrem23/quizgo/internal/domain/quiz"
	"github.com/kungrem23/quizgo/internal/http/middleware"
	"github.com/kungrem23/quizgo/internal/http/middleware/respond"
)

type editorService interface {
	GetUser(context.Context, int) (quiz.User, error)
	GetQuizContent(context.Context, int) (quiz.QuizContent, error)
	ListQuizSummaries(context.Context, int) ([]quiz.QuizSummary, error)
	SaveQuizContent(context.Context, int, int, quiz.SaveQuizContent) (quiz.QuizContent, error)
	DeleteQuizAsAuthor(context.Context, int, int) error
	UploadImage(context.Context, int, string, []byte) (quiz.UploadedImage, error)
	ReadImage(context.Context, string) (quiz.UploadedImage, error)
	DeleteUploadedImage(context.Context, string, int) error
}
type EditorHandler struct{ service editorService }

func NewEditorHandler(service editorService) *EditorHandler { return &EditorHandler{service: service} }

// PublicContent intentionally has no is_correct field, including nested answers.
type PublicContent struct {
	quiz.QuizSummary
	Questions []PublicContentQuestion `json:"questions"`
}
type PublicContentQuestion struct {
	ID          int                   `json:"id"`
	TextContent string                `json:"text_content"`
	ImageID     string                `json:"image_id"`
	ImageURL    string                `json:"image_url,omitempty"`
	TimeLimit   int                   `json:"time_limit"`
	Answers     []PublicContentAnswer `json:"answers"`
}
type PublicContentAnswer struct {
	ID          int    `json:"id"`
	TextContent string `json:"text_content"`
}

func publicContent(v quiz.QuizContent) PublicContent {
	result := PublicContent{QuizSummary: v.QuizSummary, Questions: []PublicContentQuestion{}}
	for _, q := range v.Questions {
		p := PublicContentQuestion{ID: q.ID, TextContent: q.TextContent, ImageID: q.ImageID, ImageURL: q.ImageURL, TimeLimit: q.TimeLimit, Answers: []PublicContentAnswer{}}
		for _, a := range q.Answers {
			p.Answers = append(p.Answers, PublicContentAnswer{ID: a.ID, TextContent: a.TextContent})
		}
		result.Questions = append(result.Questions, p)
	}
	return result
}
func editorError(w http.ResponseWriter, err error) {
	status, message := http.StatusInternalServerError, "server error"
	switch {
	case errors.Is(err, sql.ErrNoRows):
		status, message = 404, "not found"
	case errors.Is(err, middleware.ErrInsufficientRights):
		status, message = 403, "forbidden"
	case errors.Is(err, quiz.ErrContentInvalid):
		status, message = 400, "invalid quiz content"
	case errors.Is(err, quiz.ErrContentConflict):
		status, message = 409, "quiz has changed"
	case errors.Is(err, quiz.ErrImageStorage):
		status, message = 503, "image storage unavailable"
	case errors.Is(err, quiz.ErrImageInUse):
		status, message = 409, "image is in use"
	}
	respond.WriteJSON(w, status, respond.ErrorResponse{Error: message})
}
func editorID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		respond.WriteJSON(w, 400, respond.ErrorResponse{Error: "invalid id"})
		return 0, false
	}
	return id, true
}

// Me returns the authenticated user.
// @Summary Текущий пользователь
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResponse
// @Failure 401 {object} respond.ErrorResponse
// @Router /me [get]
func (h *EditorHandler) Me(w http.ResponseWriter, r *http.Request) {
	id, _ := middleware.UserIDFromContext(r.Context())
	v, err := h.service.GetUser(r.Context(), id)
	if err != nil {
		editorError(w, err)
		return
	}
	respond.WriteJSON(w, 200, UserResponse{ID: v.Id, Username: v.Username})
}

// MyQuizzes returns compact summaries.
// @Summary Мои квизы с количеством вопросов и датой изменения
// @Tags quizzes
// @Produce json
// @Security BearerAuth
// @Success 200 {array} quiz.QuizSummary
// @Failure 401 {object} respond.ErrorResponse
// @Router /me/quizzes [get]
func (h *EditorHandler) MyQuizzes(w http.ResponseWriter, r *http.Request) {
	id, _ := middleware.UserIDFromContext(r.Context())
	v, err := h.service.ListQuizSummaries(r.Context(), id)
	if err != nil {
		editorError(w, err)
		return
	}
	respond.WriteJSON(w, 200, v)
}

// GetContent returns saved public content.
// @Summary Содержимое квиза для просмотра
// @Description Порядок массивов соответствует порядку вопросов и ответов. is_correct отсутствует. image_url — временная подписанная ссылка S3.
// @Tags quizzes
// @Produce json
// @Param id path int true "ID квиза"
// @Success 200 {object} PublicContent
// @Failure 404 {object} respond.ErrorResponse
// @Router /quizzes/{id}/content [get]
func (h *EditorHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := editorID(w, r)
	if !ok {
		return
	}
	v, err := h.service.GetQuizContent(r.Context(), id)
	if err != nil {
		editorError(w, err)
		return
	}
	respond.WriteJSON(w, 200, publicContent(v))
}

// GetAuthorContent returns the editor representation.
// @Summary Содержимое квиза для автора, включая правильные ответы
// @Tags quizzes
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID квиза"
// @Success 200 {object} quiz.QuizContent
// @Failure 403 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Router /quizzes/{id}/author [get]
func (h *EditorHandler) GetAuthorContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := editorID(w, r)
	if !ok {
		return
	}
	v, err := h.service.GetQuizContent(r.Context(), id)
	if err != nil {
		editorError(w, err)
		return
	}
	userID, _ := middleware.UserIDFromContext(r.Context())
	if userID != v.AuthorID {
		editorError(w, middleware.ErrInsufficientRights)
		return
	}
	respond.WriteJSON(w, 200, v)
}

// SaveContent atomically updates all content while retaining existing IDs.
// @Summary Сохранить квиз целиком
// @Description Требуется актуальная revision, иначе 409. ID=0 создаёт элемент; отсутствующие в массивах элементы удаляются. Порядок задаётся массивами. Пустой квиз допустим. До 100 вопросов, в каждом 2–8 ответов и ровно один правильный. Название до 50 символов, тексты до 200. time_limit: 5,10,15,20,30,45,60,90,120 секунд.
// @Tags quizzes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID квиза"
// @Param request body quiz.SaveQuizContent true "Содержимое и версия"
// @Success 200 {object} quiz.QuizContent
// @Failure 400 {object} respond.ErrorResponse
// @Failure 403 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Router /quizzes/{id}/content [put]
func (h *EditorHandler) SaveContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := editorID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var v quiz.SaveQuizContent
	if err := decodeJSON(r, &v); err != nil {
		respond.WriteJSON(w, 400, respond.ErrorResponse{Error: "bad request"})
		return
	}
	userID, _ := middleware.UserIDFromContext(r.Context())
	result, err := h.service.SaveQuizContent(r.Context(), id, userID, v)
	if err != nil {
		editorError(w, err)
		return
	}
	respond.WriteJSON(w, 200, result)
}

// DeleteQuiz removes a quiz and all its questions and answers.
// @Summary Удалить квиз
// @Tags quizzes
// @Security BearerAuth
// @Param id path int true "ID квиза"
// @Success 204
// @Failure 403 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Router /quizzes/{id} [delete]
func (h *EditorHandler) DeleteQuiz(w http.ResponseWriter, r *http.Request) {
	id, ok := editorID(w, r)
	if !ok {
		return
	}
	userID, _ := middleware.UserIDFromContext(r.Context())
	if err := h.service.DeleteQuizAsAuthor(r.Context(), id, userID); err != nil {
		editorError(w, err)
		return
	}
	w.WriteHeader(204)
}

// UploadImage stores a validated JPEG or PNG in S3; only the ID and owner enter PostgreSQL.
// @Summary Загрузить изображение вопроса
// @Description JPG/PNG до 5 MiB, до 20 мегапикселей. Поле multipart: file. Возвращает ID и временную ссылку S3.
// @Tags images
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "Изображение"
// @Success 201 {object} quiz.UploadedImage
// @Failure 400 {object} respond.ErrorResponse
// @Failure 413 {object} respond.ErrorResponse
// @Failure 503 {object} respond.ErrorResponse
// @Router /images [post]
func (h *EditorHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, (5<<20)+(64<<10))
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		respond.WriteJSON(w, 413, respond.ErrorResponse{Error: "image too large or invalid multipart"})
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, err := r.FormFile("file")
	if err != nil {
		respond.WriteJSON(w, 400, respond.ErrorResponse{Error: "file required"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (5<<20)+1))
	if err != nil || len(data) > 5<<20 {
		respond.WriteJSON(w, 413, respond.ErrorResponse{Error: "image too large"})
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 20_000_000 {
		respond.WriteJSON(w, 400, respond.ErrorResponse{Error: "invalid image"})
		return
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		respond.WriteJSON(w, 400, respond.ErrorResponse{Error: "invalid image"})
		return
	}
	userID, _ := middleware.UserIDFromContext(r.Context())
	v, err := h.service.UploadImage(r.Context(), userID, "image/"+format, data)
	if err != nil {
		editorError(w, err)
		return
	}
	respond.WriteJSON(w, 201, v)
}

// ReadImage redirects old image links to a fresh S3 URL.
// @Summary Перейти к изображению в S3
// @Tags images

// @Param id path string true "ID изображения"
// @Success 307 "Перенаправление на временную ссылку S3"
// @Header 307 {string} Location "Ссылка S3"
// @Failure 404 {object} respond.ErrorResponse
// @Router /images/{id} [get]
func (h *EditorHandler) ReadImage(w http.ResponseWriter, r *http.Request) {
	if _, err := uuid.Parse(r.PathValue("id")); err != nil {
		respond.WriteJSON(w, 400, respond.ErrorResponse{Error: "invalid id"})
		return
	}
	v, err := h.service.ReadImage(r.Context(), r.PathValue("id"))
	if err != nil {
		editorError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, v.URL, http.StatusTemporaryRedirect)
}

// ImageURL returns a fresh link for restored drafts and expired signed URLs.
// @Summary Получить свежую ссылку на изображение
// @Tags images
// @Produce json
// @Param id path string true "ID изображения"
// @Success 200 {object} quiz.UploadedImage
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 503 {object} respond.ErrorResponse
// @Router /images/{id}/url [get]
func (h *EditorHandler) ImageURL(w http.ResponseWriter, r *http.Request) {
	if _, err := uuid.Parse(r.PathValue("id")); err != nil {
		respond.WriteJSON(w, 400, respond.ErrorResponse{Error: "invalid id"})
		return
	}
	v, err := h.service.ReadImage(r.Context(), r.PathValue("id"))
	if err != nil {
		editorError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respond.WriteJSON(w, 200, v)
}

// DeleteImage removes an unattached upload owned by the current user.
// @Summary Удалить неиспользуемое изображение
// @Tags images
// @Security BearerAuth
// @Param id path string true "ID изображения"
// @Success 204
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Router /images/{id} [delete]
func (h *EditorHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	if err := h.service.DeleteUploadedImage(r.Context(), r.PathValue("id"), userID); err != nil {
		editorError(w, err)
		return
	}
	w.WriteHeader(204)
}
