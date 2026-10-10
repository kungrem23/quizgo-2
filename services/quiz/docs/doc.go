// Package docs contains the generated Swagger documentation for the quiz CRUD API.
//
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo doc.go --dir .,../internal/http/handlers/quiz,../internal/http/middleware/respond,../internal/domain --output . --parseInternal
package docs

// @title QuizGo CRUD API
// @version 1.0
// @description HTTP API квизов, вопросов, ответов и пользователей quiz-service.
// @description Публичные ответы не содержат is_correct, а пользователи — хешей паролей.
// @BasePath /api
//
// @tag.name quizzes
// @tag.description Создание и просмотр квизов.
// @tag.name questions
// @tag.description Создание, просмотр, удаление и изменение порядка вопросов.
// @tag.name answers
// @tag.description Создание, просмотр и удаление ответов.
// @tag.name users
// @tag.description Публичные профили пользователей.
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите Bearer <JWT>. Access token можно получить через POST /auth/login с JSON {"username":"alice","password":"secret123"}. Поле token уже содержит префикс Bearer. Refresh token устанавливается в HttpOnly cookie и обновляется через POST /auth/refresh. Регистрация: POST /auth/register с теми же полями.
