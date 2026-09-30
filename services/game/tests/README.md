# Game service tests

Все тесты game-сервиса находятся отдельно от production-кода и повторяют его слои:
`application`, `domain`, `config`, `http`, `quizgrpc` и `ws`.

```sh
go test -race ./services/game/tests/...
```

## System E2E

`e2e` — внешний системный тест реального HTTP, gRPC, PostgreSQL, Redis и
WebSocket stack. Обычный `go test ./...` его компилирует и пропускает, если URL
окружения не заданы. Изолированный Docker Compose запуск, включая restart
game-service посреди открытого вопроса:

```sh
./scripts/e2e.sh
```

Сценарий создаёт автора и квиз через публичный HTTP API, создаёт игру, подключает
host и трёх игроков, проверяет reconnect обоих ролей, запрет host-команды игроку,
идемпотентный повтор `request_id`, оба countdown/вопроса, ответы, scoreboard,
`next`, восстановление actor/timer после restart и итоговый `game_finished`.

Runner использует отдельный Compose project и по завершении удаляет только его
контейнеры и volumes. Порты можно переопределить через
`QUIZGO_E2E_FRONTEND_PORT` и `QUIZGO_E2E_GAME_PORT`; для разбора упавшего запуска
можно установить `QUIZGO_E2E_KEEP_STACK=1`.

## WebSocket load baseline

Самодостаточный локальный запуск с временным Compose stack:

```sh
./scripts/ws-load.sh -rooms 10 -players-per-room 25 -duration 30s -command-rate 500
```

Или против уже запущенного окружения (если `token`, `quiz-id` и `answer-id` не
переданы, tool сам создаёт тестового автора и один вопрос на 120 секунд):

```sh
go run ./services/game/cmd/ws-load \
  -http-url http://127.0.0.1:3000 \
  -ws-url ws://127.0.0.1:8081/ws \
  -rooms 10 -players-per-room 25 -duration 30s -command-rate 500
```

После начального успешного ответа каждый virtual player повторяет тот же
`request_id`. Это остаётся валидной идемпотентной командой во всех фазах и даёт
стабильную нагрузку на WebSocket/actor path без искусственного потока
`already_answered`. Интенсивность `command-rate` — общая для всех игроков.
При стандартном player rate limit устойчивый ориентир без намеренного throttling —
не более `rooms * players-per-room * 5` команд/с; более высокое значение полезно
для отдельной проверки `rate_limited` и закрытия нарушающих соединений.

Вывод содержит время setup, число соединений, отправленные команды, ack и
незавершённые responses, send/read errors, достигнутую send/ack интенсивность,
p50/p95/p99/max round-trip latency и ошибки команд по стабильному `code`. Setup и
lifecycle events в latency не входят.
Для серверной стороны одновременно полезно снимать `/metrics`, особенно
`quizgo_game_websocket_command_duration_seconds`, Redis errors и active actors.
