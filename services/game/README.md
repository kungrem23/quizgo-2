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
Для аутентифицированных команд непустой `request_id` также является ключом
идемпотентности в пределах участника и типа команды. Успешный повтор не применяет
изменение и возвращает `command_accepted` с `payload.duplicate=true`; первая
обработка возвращает `false`. Последние 1024 успешные команды сохраняются вместе
с игрой в Redis, поэтому защита работает после reconnect и рестарта процесса.
Пустой `request_id` поддерживается для старых клиентов, но не дедуплицируется;
максимальная длина непустого значения — 128 байт.
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
сервер отправляет актуальное событие `state`. Новая успешная аутентификация
атомарно заменяет предыдущее соединение того же участника; старое соединение
закрывается и больше не может выполнять команды.

Команды после handshake:

```json
{"type":"start","request_id":"1"}
{"type":"answer","request_id":"2","payload":{"answer_id":17}}
{"type":"next","request_id":"3"}
{"type":"finish","request_id":"4"}
{"type":"remove_player","request_id":"5","payload":{"player_id":"..."}}
{"type":"leave","request_id":"6"}
```

- `start` — ведущий запускает трёхсекундный countdown перед первым вопросом; без игроков старт запрещён;
- `answer` — игрок отвечает один раз до дедлайна вопроса;
- `next` — ведущий из scoreboard запускает countdown следующего вопроса;
- `finish` — ведущий завершает незавершённую игру, в том числе закрывает lobby;
- `remove_player` — ведущий удаляет игрока из lobby;
- `leave` — игрок удаляет из lobby только самого себя и закрывает своё соединение.

Основные события: `player_joined`, `countdown_started`, `question_opened`,
`answer_accepted`, `player_answered`, `question_closed`, `player_left`,
`player_removed`, `game_finished`.
`countdown_started` содержит `countdown_ends_at`, но ещё не раскрывает вопрос.
После истечения времени не последнего вопроса фаза становится `scoreboard`, а
совместимое событие `question_closed` показывает `correct_answer_ids` и таблицу
результатов. Полное событие `state` при переподключении содержит дедлайн текущей
фазы, а в scoreboard — правильные ответы. Countdown и вопрос завершаются серверными
таймерами, поэтому клиентский таймер не является источником истины. После последнего
вопроса сервер автоматически сохраняет фазу `finished` и вслед за итоговым
`question_closed` рассылает `game_finished`; команда `next` для этого не нужна.
Ticket удалённого или вышедшего игрока сразу перестаёт проходить повторную
аутентификацию.

После handshake сервер принимает от ведущего только `start`, `next`, `finish` и
`remove_player`, а от игрока — только `answer` и `leave`. Неизвестный тип получает
ошибку `unknown_message_type`, превышение частоты — `rate_limited`, некорректный
или содержащий неизвестные поля payload — `invalid_payload`. После пяти
последовательных нарушений протокола соединение закрывается с policy violation;
успешная команда сбрасывает счётчик.

В Redis вместе с фазой сохраняются абсолютные `countdown_ends_at`,
`question_opened_at` и `question_closes_at`. После рестарта actor комнаты лениво
создаётся при первом join/reconnect, восстанавливает один таймер до ближайшего
deadline и до отправки `state` применяет все уже просроченные timed-переходы.
Переходы привязаны к сохранённым deadline, а не ко времени reconnect: поэтому
долго просроченный countdown открывает вопрос в исходный момент и при необходимости
сразу закрывает его в scoreboard или `finished`. Lobby и scoreboard таймеров не
создают.

## Distributed room ownership

Actor-loop по-прежнему находится только в памяти одной реплики, но перед его
запуском реплика атомарно получает в Redis lease комнаты. Первая обратившаяся к
неактивной комнате реплика становится owner. В lease сохраняются уникальный
`instance_id`, внутренний URL реплики, случайный `lease_id` и возрастающий fencing
token. Owner продлевает lease каждые 5 секунд; TTL lease по умолчанию равен 15
секундам. Если Redis недоступен, actor работает только до локально известного
безопасного срока lease и затем останавливается.

Каждая запись игрового snapshot выполняется одним Lua-скриптом, который сначала
сверяет `lease_id` и fencing token. Поэтому actor со старым или истёкшим lease не
может перезаписать состояние нового owner. Освобождение lease также compare-and-delete:
старый процесс не может удалить lease новой реплики. После потери owner другая
реплика получает lease, читает последний подтверждённый snapshot и восстанавливает
таймеры по сохранённым абсолютным deadline. Graceful shutdown сначала прекращает
actor-loop и продление lease, освобождает принадлежащие процессу lease, а затем
останавливает HTTP-сервер.

Для предмаршрутизации WebSocket рекомендуется указывать комнату в URL, не меняя
формат сообщений:

```text
/ws?code=ABC123       # join
/ws?game_id=<UUID>    # host_auth и player_auth/reconnect
```

Если комната принадлежит другой реплике, принявший HTTP Upgrade сервис прозрачно
проксирует соединение на `GAME_INTERNAL_URL` owner. Внутренний запрос привязан к
конкретному `lease_id`, поэтому после смены owner старый маршрут отклоняется и
клиент должен переподключиться. Старый URL `/ws` остаётся рабочим без изменений:
он захватывает свободную комнату локально, но при попадании на чужого owner закрывает
handshake с retryable WebSocket status 1013. Sticky sessions не требуются для URL
с routing hint.

Каждая реплика должна иметь уникальный `GAME_INSTANCE_ID` и собственный
`GAME_INTERNAL_URL`, доступный другим репликам. Стандартный `compose.yaml` описывает
одну реплику; для нескольких контейнеров им нужны отдельные DNS-имена/URL, поэтому
его нельзя просто масштабировать с одним общим `GAME_INTERNAL_URL`.

Ownership рассчитан на один логический Redis primary. Это lease с fencing, а не
consensus-протокол: при failover Redis без гарантированно сохранённых последних
записей возможна потеря уже подтверждённого lease или snapshot. Активные WebSocket
не мигрируют между процессами — при падении owner они разрываются, а reconnect
становится возможен после истечения lease (не более его TTL плюс сетевой retry).

`GET /healthz` проверяет процесс, `GET /readyz` — Redis и gRPC health
quiz-сервиса. По умолчанию размер входящего WebSocket-сообщения ограничен 16 KiB,
handshake — 10 секундами, обработка команды и запись сообщения — 5 секундами.
Сервер отправляет WebSocket ping каждые 30 секунд и закрывает соединение, если
pong не получен за 10 секунд. Медленный клиент отключается, если не успевает
читать очередь событий. Входящие команды ограничиваются отдельными token bucket:
игрок — 5 команд/с с burst 10, ведущий — 10 команд/с с burst 20. Burst сохраняет
возможность немедленного идемпотентного retry.

Лимиты задаются переменными `GAME_WS_MAX_MESSAGE_BYTES`,
`GAME_WS_HANDSHAKE_TIMEOUT`, `GAME_WS_COMMAND_TIMEOUT`, `GAME_WS_WRITE_TIMEOUT`,
`GAME_WS_PING_INTERVAL`, `GAME_WS_PONG_TIMEOUT`, `GAME_WS_PLAYER_COMMAND_RATE`,
`GAME_WS_PLAYER_COMMAND_BURST`, `GAME_WS_HOST_COMMAND_RATE`,
`GAME_WS_HOST_COMMAND_BURST` и `GAME_WS_INVALID_MESSAGE_LIMIT`; значения по
умолчанию приведены в `.env.example`.

Ownership настраивается через `GAME_INSTANCE_ID`, `GAME_INTERNAL_URL`,
`GAME_ROOM_LEASE_TTL`, `GAME_ROOM_LEASE_RENEW_INTERVAL` и
`GAME_ROOM_LEASE_SAFETY_MARGIN`. Renew interval должен быть короче TTL минус safety
margin. Значения по умолчанию: 15 секунд, 5 секунд и 1 секунда соответственно.

Проверка сервиса:

```sh
go test -race ./services/game/...
```

Redis-интеграционный тест lease/fencing можно дополнительно запустить на
изолированной базе:

```sh
GAME_TEST_REDIS_ADDRESS=127.0.0.1:6379 go test \
  ./services/game/tests/redis ./services/game/tests/ws \
  -run 'TestRoomLeaseExpiryAndFencing|TestRoomRouterTwoReplicasWithRedis' -count=1
```
