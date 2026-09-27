# Game service

Компилируемый каркас realtime-сервиса отделён от CRUD физически и по зависимостям:

- `cmd` — composition root и graceful shutdown;
- `internal/domain` — снимок квиза, сессия, фазы, команды и события;
- `internal/application` — сценарий создания игры и порты зависимостей;
- `internal/client/quizgrpc` — единственная граница чтения quiz-сервиса;
- `internal/store/redis` — собственное хранилище игровых снимков и кодов входа;
- `internal/transport/http` — health/readiness и `POST /api/games`;
- `internal/transport/ws` — DTO будущего WebSocket-протокола.
- `tests` — отдельное дерево black-box тестов слоёв сервиса.

Создание игры принимает `{"quiz_id": 1}` и пользовательский `Authorization: Bearer ...`.
JWT не разбирается game-сервисом: он передаётся quiz gRPC вместе с отдельным service
token. Quiz проверяет пользователя, владение и готовность квиза, после чего game
сохраняет независимый снимок в Redis. Правильные ответы не возвращаются HTTP-клиенту.

`GET /healthz` проверяет процесс, `GET /readyz` — Redis и gRPC health quiz-сервиса.
`GET /ws` пока явно отвечает `501 Not Implemented`; realtime-цикл будет добавлен поверх
domain/application без прямого доступа к PostgreSQL или внутренним пакетам quiz.
