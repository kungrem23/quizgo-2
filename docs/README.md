# Swagger для CRUD API

Документация описывает 13 реализованных HTTP-операций из
`internal/http/handlers/quiz`, работающих с `internal/domain/quiz`.
Схемы ответов строятся по HTTP DTO: публичные ответы не содержат `is_correct`,
профили пользователей не содержат хеш пароля.

После запуска quiz-service с настроенным PostgreSQL:

```sh
go run ./cmd/quiz-service
```

- Swagger UI: <http://localhost:8080/swagger/index.html>
- Спецификация через HTTP: <http://localhost:8080/swagger/doc.json>
- Файлы спецификации: `docs/swagger.json` и `docs/swagger.yaml`.

Если задан `QUIZ_HTTP_PORT`, используйте его вместо `8080`.
Swagger отправляет запросы на тот же хост и порт, с которого открыта страница.

Для защищённых операций получите токен:

```sh
curl -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123"}'
```

Пользователя можно создать через `POST /auth/register` с теми же полями.
В Swagger нажмите **Authorize** и вставьте значение `token` целиком:
`Bearer <JWT>`. Изменять вопросы и ответы, а также просматривать `is_correct`
может только автор соответствующего квиза. Текущий API возвращает `401`
как при недействительном токене, так и при недостаточных правах.

После изменения аннотаций или DTO пересоздайте документацию из корня проекта:

```sh
go generate ./docs
```

Версия генератора закреплена в `docs/doc.go`; отдельно устанавливать `swag`
не требуется. Команда обновляет `docs.go`, `swagger.json` и `swagger.yaml`.
Эти файлы следует коммитить вместе с изменениями обработчиков.
Формат аннотаций описан в [документации Swag](https://github.com/swaggo/swag#declarative-comments-format).

Swagger охватывает зарегистрированные CRUD-маршруты. Пустые обработчики списков
вопросов и ответов, операции с изображениями и методы репозитория без HTTP-маршрутов
не включены. Авторизация описана выше; игровые и WebSocket API сюда не входят.
