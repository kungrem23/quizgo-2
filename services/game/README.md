# Game service

Game — отдельный realtime-сервис. Он получает неизменяемый снимок опубликованного
квиза из quiz-сервиса по gRPC, хранит игровое состояние в Redis и общается с
ведущим и игроками по WebSocket.

## Устройство

- `cmd/game-service` — сборка зависимостей, HTTP-сервер и graceful shutdown;
- `internal/domain` — жизненный цикл игры, ответы и подсчёт очков;
- `internal/application` — создание игры и последовательный actor-loop комнаты;
- `internal/client/quizgrpc` — единственная граница чтения quiz-сервиса;
- `internal/store/redis` — снимки игр и индекс шестизначных кодов входа;
- `internal/transport/http` — health/readiness и создание игры;
- `internal/transport/ws` — handshake, команды и события WebSocket;
- `tests` — отдельное дерево black-box тестов всех слоёв.

Одна комната обрабатывает команды строго последовательно. Каждое изменение сначала
записывается в Redis и только после успешной записи становится текущим состоянием и
рассылается клиентам. Поэтому одновременные ответы не изменяют `Game` конкурентно.

## Создание игры

```http
POST /api/games
Authorization: Bearer <quiz access token>
Content-Type: application/json

{"quiz_id": 1}
```

Ответ `201 Created`:

```json
{
  "id": "game UUID",
  "code": "ABC123",
  "phase": "lobby",
  "quiz_revision": 4,
  "host_ticket": "secret returned only here"
}
```

JWT не разбирается game-сервисом: он передаётся quiz gRPC вместе с отдельным
service token. Quiz проверяет пользователя, владение и готовность квиза. Правильные
ответы и secret ticket не возвращаются в публичном состоянии. В Redis сохраняются
только SHA-256-хеши ticket'ов.

## WebSocket-протокол

Соединение открывается на `GET /ws`. Все сообщения имеют envelope:

```json
{"type":"...","request_id":"client-id","sequence":3,"payload":{}}
```

`request_id` задаёт клиент и получает обратно в `command_accepted` или `error`.
`sequence` задаёт сервер и увеличивает при каждом изменении состояния игры.

Первое сообщение обязательно выполняет один из handshake-сценариев:

```json
{"type":"join","payload":{"code":"ABC123","nickname":"Alice"}}
{"type":"host_auth","payload":{"game_id":"...","ticket":"..."}}
{"type":"player_auth","payload":{"game_id":"...","participant_id":"...","ticket":"..."}}
```

`join` возвращает `joined` с `game_id`, `player_id` и одноразово показанным
`ticket`. Два остальных сценария используются для подключения ведущего и
переподключения игрока; при успехе приходит `authenticated`. После handshake
сервер отправляет актуальное событие `state`.

Команды после handshake:

```json
{"type":"start","request_id":"1"}
{"type":"answer","request_id":"2","payload":{"answer_id":17}}
{"type":"next","request_id":"3"}
```

- `start` — ведущий открывает первый вопрос; без игроков старт запрещён;
- `answer` — игрок отвечает один раз до дедлайна вопроса;
- `next` — ведущий открывает следующий вопрос или завершает игру.

Основные события: `player_joined`, `question_opened`, `answer_accepted`,
`player_answered`, `question_closed`, `game_finished`. До `question_closed`
клиенты получают варианты без признака правильности. В момент закрытия сервер
показывает `correct_answer_ids` и таблицу результатов. Вопрос закрывается серверным
таймером, поэтому клиентский таймер не является источником истины.

## Ограничение текущего этапа

Actor-loop комнат находится в памяти процесса. Redis позволяет восстановить игру
при переподключении после рестарта, но несколько реплик game-сервиса пока нельзя
запускать без sticky routing или распределённого владельца комнаты: разные реплики
могут создать два loop для одной игры. Текущая конфигурация рассчитана на одну
реплику; распределённая координация — отдельный следующий этап.

`GET /healthz` проверяет процесс, `GET /readyz` — Redis и gRPC health
quiz-сервиса. Размер WebSocket-сообщения ограничен 64 KiB, handshake — 10 секундами,
а запись одного сообщения — 5 секундами. Медленный клиент отключается, если не
успевает читать очередь событий.

Проверка сервиса:

```sh
go test -race ./services/game/...
```
