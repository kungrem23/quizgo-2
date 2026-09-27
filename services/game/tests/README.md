# Game service tests

Все тесты game-сервиса находятся отдельно от production-кода и повторяют его слои:
`application`, `domain`, `config`, `http` и `quizgrpc`.

```sh
go test -race ./services/game/tests/...
```
