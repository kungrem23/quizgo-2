# Quiz service tests

Тесты вынесены из production-пакетов и сгруппированы по проверяемым границам:
`domain`, `http`, `grpc`, `postgres`, `s3`, `authn` и `config`.

Они используют внешние test packages и проверяют сервис через публичное поведение,
поэтому внутренний рефакторинг не требует доступа тестов к приватным деталям.

```sh
go test -race ./services/quiz/tests/...
```
