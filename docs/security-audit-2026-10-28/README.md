# Аудит безопасности QuizGo — 28 октября 2026

Проверено текущее рабочее дерево поверх `26d4db50208bf559244a257a53375eb60f6a1305`, включая незакоммиченные изменения видимости квизов. Исправления прикладного кода в рамках аудита не вносились. Добавлены только этот отчёт и материалы проверок.

**Результат: 12 замечаний — 4 с высоким приоритетом P1 и 8 с приоритетом P2.** Это инженерная оценка приоритета исправления, а не CVSS. Условные инфраструктурные риски и результаты статического анализа явно отделены от воспроизведённых ошибок. Оснований утверждать наличие удалённого захвата произвольного аккаунта, SQL injection или RCE в прикладном коде не найдено.

## Область и ограничения

Проверены Go-сервисы quiz/game, HTTP- и gRPC-маршруты, JWT и refresh-сессии, права на квизы/вопросы/ответы/изображения, WebSocket, actor loop, Redis leases/fencing, SQL-репозитории и миграции, S3, React-клиент, Dockerfiles, Compose, Caddy/nginx, скрипты запуска, Go/npm-зависимости и доступная Git-история.

Модель угроз: анонимный интернет-клиент; зарегистрированный пользователь со своими квизами; участник, знающий код комнаты; атакующий сайт в браузере пользователя; нарушитель с доступом во внутреннюю сеть контейнеров. Последняя модель требует отдельной предпосылки и не равнозначна доступности Redis/PostgreSQL из интернета.

Ограничения проверки:

- Docker daemon недоступен. Контейнеры, их реальные версии, CVE базовых образов, эффективные сетевые ACL и настройки запущенного production не проверены.
- Интеграционные тесты, требующие `QUIZ_TEST_POSTGRES_DSN`, `GAME_TEST_REDIS_ADDRESS`, `QUIZGO_E2E_HTTP_URL`/`QUIZGO_E2E_GAME_URL`, без этих переменных пропускаются. Успешный `go test` не означает прохождение этих сценариев.
- Не проводились запросы к production, нагрузочное исчерпание ресурсов, проверка реального S3 bucket/IAM, облачного аккаунта, резервных копий и внешнего CI.
- Локальный `.env` проверялся без вывода значений секретов. Поиск секретов в Git использовал ограниченный набор сигнатур и не доказывает отсутствие секретов произвольного формата.
- Воспроизведение login CSRF проверяет HTTP-обработчик; полный сценарий навигации в браузере отдельно не запускался.

## Реестр замечаний

| ID | Приоритет | Проблема | Подтверждение / предпосылка |
| --- | --- | --- | --- |
| QG-01 | P1 | В production-миграции создаётся аккаунт `username` / `password` | Пароль проверен по seed-хешу локальным тестом |
| QG-02 | P1 | Go 1.25 вышел из поддержки; локально используется 1.25.1 с известными уязвимостями | Toolchain, Dockerfiles, govulncheck |
| QG-03 | P1 | Нет ограничения попыток login/register и стоимости bcrypt | Код HTTP-маршрутов, auth-сервиса и reverse proxy |
| QG-04 | P1 | Нет общих квот на комнаты, соединения и сохраняемые данные | Код; неверная host-аутентификация сохраняет созданный actor |
| QG-05 | P2 | Гостевые входы позволяют занять все 100 мест комнаты | 100 последовательных Join/Disconnect воспроизведены |
| QG-06 | P2 | Login принимает cross-site `text/plain` запрос и устанавливает cookie | Локальный HTTP-тест |
| QG-07 | P2 | Старый CRUD обходит ограничения размеров структуры квиза | Код сервиса, репозиториев и миграций |
| QG-08 | P2 | Redis без аутентификации/ACL в общей сети Compose | Конфигурация; требуется доступ к внутренней сети |
| QG-09 | P2 | Приложение использует PostgreSQL superuser | Конфигурация стандартного образа; усиливает последствия компрометации |
| QG-10 | P2 | Межсервисные соединения по умолчанию не шифруются | Конфигурация; требуется доступ к каналу/сети |
| QG-11 | P2 | В Go-зависимостях остаются известные уязвимости | govulncheck; достижимость и условия описаны ниже |
| QG-12 | P2 | Уязвимые npm-инструменты сборки и тестов | npm audit; не доказана эксплуатация в production-бандле |

## QG-01. Предсказуемый аккаунт в обязательной миграции

Код: [0004_seed_quiz.sql](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/platform/postgres/migrations/0004_seed_quiz.sql:1), [ApplyMigrations](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/platform/postgres/postgres.go:57).

Миграция без условия окружения вставляет пользователя `username` с фиксированным bcrypt-хешем. Локальная проверка `CheckPasswordHash("password", seedHash)` успешна. Все SQL-миграции исполняются при каждом старте сервиса; удаление этого пользователя само по себе не устраняет причину. `ON CONFLICT DO NOTHING` сохраняет уже существующий аккаунт, поэтому сменённый ранее пароль не перезаписывается.

**Последствия:** любой знающий пару получает доступ к встроенному аккаунту и принадлежащим ему квизам, может менять/удалять их и использовать доступные обычному пользователю ресурсы. Администраторских полномочий у этой записи в исследованном коде нет; доступ к чужим приватным квизам из этого не следует.

**Исправление:** вынести демонстрационные данные в отдельный явно включаемый dev/seed-процесс; исключить создание фиксированного аккаунта при обычном старте. В существующих установках удалить либо сменить пароль этой записи и отозвать её refresh-сессии. Учесть остаточный срок уже выпущенных access-токенов.

**Критерий проверки:** чистая production-БД после запуска не содержит такого аккаунта, а рестарт не создаёт его заново.

## QG-02. Устаревшая и неподдерживаемая версия Go

Код: [go.mod](/Users/danilmitrosin/Documents/goprojects/quizgo-2/go.mod:3), [game Dockerfile](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/Dockerfile:2), [quiz Dockerfile](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/Dockerfile:2).

Локальный компилятор — `go1.25.1`; обе сборочные стадии используют ветку `golang:1.25-alpine`. На дату аудита поддерживаются ветки 1.26 и 1.27; официальные исправленные релизы — 1.26.10 и 1.27.3. Источники: [история и политика релизов Go](https://go.dev/doc/devel/release).

Для двух production entrypoint на Linux/amd64 с анализируемым Go 1.25.1 govulncheck сообщил **69 уникальных GO-ID**, из них **48 с путями к уязвимым символам**. Это количество предупреждений статического анализа, а не 48 доказанных атак. Часть путей относится к конфигурации TLS, шаблонам с фиксированным содержимым или неиспользуемым режимам библиотек. Сводка с модулями и примерами путей находится в `go-vulnerabilities.json`.

Примеры, требующие внимания:

- [GO-2026-4341](https://pkg.go.dev/vuln/GO-2026-4341): рост памяти при разборе параметров URL; `RoomRouter` вызывает `URL.Query()` для внешнего `/ws`. Размер URI ограничивается proxy/HTTP-сервером, поэтому предупреждение не означает неограниченную аллокацию в штатной конфигурации.
- [GO-2026-6608](https://pkg.go.dev/vuln/GO-2026-6608): обход внутренних memory limits multipart; путь ведёт в `UploadImage`. Общий `MaxBytesReader` около 5 MiB сохраняет внешний предел тела и уменьшает последствия.
- [GO-2026-6611](https://pkg.go.dev/vuln/GO-2026-6611), [GO-2026-6617](https://pkg.go.dev/vuln/GO-2026-6617): HTTP/2 CPU DoS и crash. Значимы для gRPC; внешний HTTP завершается в Caddy, а gRPC в Compose не публикуется наружу.

**Исправление:** обновить сборочную ветку и фактический toolchain как минимум до поддерживаемого исправленного релиза, пересобрать оба сервиса и проверить результат на новой версии. Директива `go` задаёт минимум, а плавающий Docker tag не доказывает версию уже развёрнутого бинарника.

**Критерий проверки:** повторный symbol scan production entrypoint и проверка `go version -m` фактически собранных бинарников. Необходимо отдельно устранить QG-11.

## QG-03. Неограниченный перебор паролей и расход CPU на аутентификацию

Код: [auth routes](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/http/routes.go:61), [Login](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/http/handlers/auth/auth.go:70), [Register](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/http/handlers/auth/auth.go:198), [authenticate](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/domain/service.go:270), [nginx.conf](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/frontend/nginx.conf:14).

Ни приложение, ни приложенные Caddy/nginx-конфигурации не ограничивают частоту login/register. Проверка существующего аккаунта и регистрация нового требуют bcrypt; параллельные запросы могут занимать CPU сервиса без ограниченного worker pool. Лимит 4 KiB на запрос ограничивает размер, но не количество и вычислительную стоимость.

**Последствия:** онлайн-перебор/credential stuffing и анонимный CPU DoS. Зарегистрированные имена также публичны через профиль; для отсутствующего имени bcrypt не выполняется, что дополнительно создаёт временной канал.

**Исправление:** лимиты по IP и имени аккаунта, общий предел параллельных bcrypt-операций, постепенная задержка/временная блокировка с защитой от блокировки чужих аккаунтов. В нескольких репликах ограничения должны согласовываться. Источник IP следует определять через доверенный proxy, а не произвольный клиентский `X-Forwarded-For`.

**Критерий проверки:** серия запросов с разных соединений не обходит лимит; существуют отдельные тесты для одного аккаунта с разных IP и для множества аккаунтов с одного IP.

## QG-04. Нет общих квот ресурсов; actor создаётся до успешной аутентификации

Код: [CreateGame](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/application/service.go:53), [prepareRoom](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/application/hub.go:352), [room.run](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/application/hub.go:477), [RoomRouter](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/transport/ws/router.go:40), [WebSocket handler](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/transport/ws/handler.go:130), [UploadImage](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/http/handlers/quiz/editor_handlers.go:250).

Нет квот числа игр на пользователя, общего числа загруженных actors/соединений, числа квизов и суммарного объёма изображений. Регистрация открыта. Каждый `CreateGame` копирует snapshot квиза в Redis; лимита активных игр нет. WS command bucket создаётся после handshake и относится только к текущему соединению.

`roomForID` запускает actor до проверки host/player ticket. Локальный тест подтвердил: ошибочный host ticket отклоняется, но actor остаётся в `Hub.rooms`. При `GET /ws?game_id=...`/`?code=...` маршрутизация выполняется даже до проверки Upgrade/Origin внутри `websocket.Accept`. Это не обходит права на игру, но даёт неаутентифицированный способ активировать существующую комнату.

Для комнаты без подписчиков нет idle-выгрузки; завершённая игра также не выгружается сразу. В distributed-режиме actor обычно останется до истечения Redis-состояния, потери lease или остановки процесса; по умолчанию TTL состояния — 24 часа, изменения продлевают его. Renew lease сам по себе TTL состояния не продлевает.

**Последствия:** один обычный аккаунт может создавать множество игр/snapshots и активировать их, расходуя Redis, память, goroutines и операции продления lease. Анонимный клиент может расходовать соединения и активировать известные комнаты. Параллельный decode изображений до 20 MP дополнительно требует существенной памяти; предел на один файл не заменяет общего бюджета.

**Исправление:** квоты игр/загрузок и байтов по пользователю, общий admission limit соединений и actors, ограниченная конкурентность image decode. Idle/terminal-выгрузку делать внутри жизненного цикла actor с сохранением нужного для восстановления состояния в Redis. Проверять корректность WS-запроса и ограничивать admission до дорогой маршрутизации.

**Критерий проверки:** отказы по квоте не создают новые Redis-записи/actors; незанятые комнаты освобождают локальные ресурсы и корректно восстанавливаются; ограничения работают между репликами. Для изменений переходов игрового состояния нужны тесты.

## QG-05. Заполнение лобби ботами без удержания соединений

Код: [handshake join](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/transport/ws/handler.go:291), [Hub.Disconnect](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/application/hub.go:250), [AddPlayer](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/domain/session.go:98).

Для join достаточно кода и произвольного уникального nickname. Disconnect снимает подписку, но сохраняет участника для reconnect. Ограничение в 100 игроков существует, а ограничение гостевых входов по источнику/admission — отсутствует. Bucket команд после handshake эту операцию не ограничивает.

**Воспроизведено:** 100 последовательных `Join` с разными именами и немедленным `Disconnect`; в сохранённой игре остаются 100 игроков, следующий настоящий игрок получает `ErrInvalidPlayer`. Знать host ticket или иметь аккаунт не требуется. Хост может удалять ботов вручную, но источник повторных входов не блокируется.

**Исправление:** rate limit join по IP и комнате плюс admission-политика: например, подтверждение хостом/блокировка повторных входов. Если вводится grace period для отключённых гостей, согласовать его с обещанным reconnect и реализовать в actor/Redis, сохранив совместимость протокола.

**Критерий проверки:** повторные подключения с одного источника не занимают все места; законный reconnect не расходует новый слот.

## QG-06. Login CSRF через text/plain форму

Код: [Login](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/http/handlers/auth/auth.go:70), [decodeAuthJSON](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/http/handlers/auth/auth.go:245), [sameOrigin](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/http/handlers/auth/auth.go:186).

Refresh/Logout проверяют Origin, Login — нет. JSON decoder не проверяет Content-Type. Поэтому обычная cross-site HTML-форма с `enctype="text/plain"` может отправить валидный JSON без CORS preflight. Для публично созданного аккаунта атакующего с паролем, начинающимся с `=`, разделитель формы становится частью пароля:

```html
<form action="https://quiz.example/auth/login" method="POST" enctype="text/plain">
  <input name='{"username":"attacker","password":"' value='audit-pass-123"}'>
  <button>Продолжить</button>
</form>
```

Получится `{"username":"attacker","password":"=audit-pass-123"}` и завершающий CRLF. Обработчик локально ответил 200 и выдал refresh-cookie при `Origin: https://attacker.example`, `Sec-Fetch-Site: cross-site`, `Content-Type: text/plain`.

**Последствия:** при успешной навигации и последующем открытии приложения жертве может быть подставлена сессия аккаунта атакующего. Если пользователь создаёт там приватный контент, владелец подставленного аккаунта сможет его прочитать. SameSite=Strict защищает отправку существующей cookie; сам по себе этот атрибут не заменяет защиту login от подстановки новой сессии. CORS не запрещает отправку HTML-форм. Это login CSRF, а не чтение cross-origin ответа или кража текущего токена жертвы.

**Исправление:** разрешать только JSON Content-Type для JSON-аутентификации; проверять Origin/Fetch Metadata на Login и Register, при необходимости использовать pre-session CSRF-token. Для browser-flow не принимать отсутствие Origin без определённой политики. Рекомендации: [OWASP CSRF Prevention](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).

**Критерий проверки:** приведённая форма и её HTTP-эквивалент отклоняются до выдачи токена/cookie; обычный same-origin вход работает.

## QG-07. Лимиты редактора можно обойти через старые CRUD-маршруты

Код: [SaveQuizContent.Validate](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/domain/editor.go:86), [CreateNewQuestionAsAuthor](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/domain/question_repo.go:19), [CreateNewAnswerAsAuthor](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/domain/answer_repo.go:19).

`PUT /api/quizzes/{id}/content` ограничивает структуру 100 вопросами и 2–8 ответами. Доступные автору `POST /api/questions` и `POST /api/answers` выполняют отдельные вставки без проверки общего количества. В SQL нет ограничений на число строк. Через них можно создать 101-й вопрос, 9-й и последующие ответы, несколько правильных ответов.

**Последствия:** автор создаёт неограниченно большой объект; чтение контента загружает его целиком до `ValidatePlayable`. Если сделать квиз публичным, повторные анонимные GET усиливают расход ресурсов. Ошибка не позволяет редактировать чужой квиз: проверка автора в этих операциях есть.

**Исправление:** вынести общие инварианты в переиспользуемую серверную абстракцию и применить ко всем путям записи; проверку количества и вставку выполнять атомарно под блокировкой квиза/вопроса, чтобы параллельные запросы не обходили лимит. Совместимые возможности черновиков следует определить отдельно. Ограничить объём чтения до загрузки огромной структуры.

**Критерий проверки:** обе формы API и конкурентные вставки соблюдают одинаковые предельные размеры; 101-й вопрос/9-й ответ отклоняются.

## QG-08. Неаутентифицированный Redis доступен всем сервисам общей сети

Код: [redis service](/Users/danilmitrosin/Documents/goprojects/quizgo-2/compose.yaml:88), [Redis client](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/store/redis/redis.go:49).

Redis запускается только с `--appendonly yes`, без пароля/ACL. Переменная `GAME_REDIS_PASSWORD` передаётся клиенту, но не настраивает пароль на сервере. Все сервисы Compose находятся в одной default network, включая frontend и Caddy.

**Условия:** нужен доступ к этой сети, например после компрометации другого контейнера; Redis не публикуется на внешний интерфейс хоста в данном Compose. Это не подтверждённая публичная Redis-инсталляция.

**Последствия:** чтение snapshots с правильными ответами, изменение баллов, ticket hashes и owner metadata, удаление/подмена игрового состояния. Lease/fencing защищают согласованность штатных writers, но не нарушителя, который может менять Redis напрямую.

**Исправление:** отдельная закрытая сеть game↔Redis; ACL-пользователь с минимальными командами/ключами, необходимыми клиенту и Lua scripts; согласованная настройка credentials на сервере и клиенте. Не давать служебной роли административные команды. Включить память/ресурсные бюджеты без нарушения восстановления игр.

**Критерий проверки:** frontend/Caddy не могут открыть Redis; запрос без credentials запрещён; штатные persistence/ownership-тесты проходят.

## QG-09. Runtime-приложение работает с superuser PostgreSQL

Код: [Compose PostgreSQL](/Users/danilmitrosin/Documents/goprojects/quizgo-2/compose.yaml:26), [quiz DB environment](/Users/danilmitrosin/Documents/goprojects/quizgo-2/compose.yaml:60).

Для инициализации образа PostgreSQL и для подключения quiz используется одна пара `POSTGRES_USER`/`POSTGRES_PASSWORD`. В стандартном образе `POSTGRES_USER` создаётся как superuser: [документация Docker](https://docs.docker.com/guides/postgresql).

**Последствия:** при компрометации приложения или утечке DB credentials нарушитель получает полные полномочия PostgreSQL, включая опасные серверные функции. В самом приложении SQL injection не найдена; это усилитель последствий отдельной компрометации.

**Исправление:** отдельный migration/owner-пользователь и runtime-роль без SUPERUSER, CREATEDB, CREATEROLE и серверных execution/file-привилегий; минимальные права на нужные таблицы/последовательности. Миграции выполнять отдельным шагом развёртывания. Для существующего volume изменение env не меняет права уже созданной роли.

**Критерий проверки:** runtime `current_user` не superuser, опасные операции запрещены, обычные операции и отдельный запуск миграций проходят.

## QG-10. Межсервисный трафик по умолчанию передаётся открыто

Код: [gRPC insecure fallback](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/client/quizgrpc/client.go:91), [TLS optional config](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/internal/config/quiz_service.go:96), [POSTGRES_SSLMODE](/Users/danilmitrosin/Documents/goprojects/quizgo-2/compose.yaml:62), [Redis options](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/game/internal/store/redis/redis.go:49).

При пустых TLS-настройках gRPC передаёт access token и service token без шифрования. В Compose PostgreSQL принудительно использует `sslmode=disable`; Redis-клиент не предоставляет TLS-конфигурацию. Внутреннее HTTP-проксирование WebSocket к owner также по умолчанию незашифрованное и переносит tickets. При этом публичный HTTPS/WSS через Caddy предусмотрен.

**Условия:** возможность перехвата/подмены внутреннего трафика; особенно значимо при разнесении сервисов по хостам. На изолированном single-host bridge риск ниже, чем на общей внешней сети.

**Исправление:** обязательный production-профиль с mTLS для gRPC, проверяемым TLS для удалённых PostgreSQL/Redis и защищённым owner proxy. Механизм gRPC mTLS уже реализован; нужно сделать его обязательным в соответствующем окружении и доставить сертификаты в контейнеры. Одни env-пути без volume/secret mounts недостаточны.

**Критерий проверки:** production-процесс отказывается стартовать с неполной/небезопасной конфигурацией; недоверенные сертификаты отклоняются.

## QG-11. Известные уязвимости Go-модулей

Код: [go.mod](/Users/danilmitrosin/Documents/goprojects/quizgo-2/go.mod:1). Полная сводка: `go-vulnerabilities.json`.

| Модуль | Текущая версия | Исправление в advisory | Применимость |
| --- | --- | --- | --- |
| `golang.org/x/net` | 0.57.0 | HTTP/2: 0.60.0; все найденные записи модуля: 0.62.0 | HTTP/2-символы достижимы через gRPC; HTML-записи — только module-level |
| `aws-sdk-go-v2/service/s3` | 1.97.1 | 1.97.3 | Статические пути есть; уязвимый EventStream-сценарий отдельно не доказан |
| `aws-sdk-go-v2/aws/protocol/eventstream` | 1.7.7 | 1.7.8 | Связан с предыдущей строкой |
| `golang.org/x/mod` | 0.37.0 | 0.40.0 | Module-level; путь runtime-атаки не найден |
| `golang.org/x/text` | 0.40.0 | 0.41.0 | `secure/precis` module-level; путь runtime-атаки не найден |
| `golang.org/x/crypto` | 0.54.0 | SSH: 0.56.0 | Проект использует bcrypt; найденные SSH/OpenPGP предупреждения не доказывают уязвимость bcrypt |

Наиболее значимы HTTP/2 DoS/crash в `x/net`, поскольку gRPC обрабатывает транспорт до прикладной bearer-аутентификации. По штатному Compose доступ к этому порту внутренний. Источники: [GO-2026-6611](https://pkg.go.dev/vuln/GO-2026-6611), [GO-2026-6617](https://pkg.go.dev/vuln/GO-2026-6617), [AWS EventStream advisory](https://github.com/aws/aws-sdk-go-v2/security/advisories/GHSA-xmrv-pmrh-hhx2).

**Исправление:** согласованно обновить зависимости до исправленных совместимых версий вместе с Go toolchain, выполнить `go mod tidy`, тесты и повторный анализ. Не применять автоматические majors без проверки. Для неиспользуемого OpenPGP нет исправленной версии; module-level предупреждение следует документировать как неприменимое к используемому bcrypt, а не объявлять уязвимым весь auth.

## QG-12. Уязвимости npm в окружении разработки/сборки

Код: [package-lock.json](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/frontend/package-lock.json:1), [frontend Dockerfile](/Users/danilmitrosin/Documents/goprojects/quizgo-2/services/quiz/frontend/Dockerfile:1). Результаты: `npm-audit.json`, `npm-audit-prod.json`.

`npm audit`: **7 уязвимых узлов dependency tree: 5 high, 2 moderate**. Это число узлов, не число уникальных CVE. Имена: `@vitest/mocker`, `brace-expansion`, `source-map-js`, `undici`, `vite`, `vitest`, `zod`. Для всех npm сообщает доступное исправление.

`npm audit --omit=dev`: **2 high** — `vite@7.3.6` и `source-map-js@1.2.1`. Они попадают в production dependency graph через сборочные зависимости, но финальный образ содержит nginx и статический `dist`, без Node runtime. Из этого результата не следует, что публичный nginx имеет Vite dev-server RCE или что браузерный код эксплуатирует parser source maps.

**Последствия:** риск для dev/test servers, CI и сборки при обработке недоверенных файлов/запросов; некоторые advisories Vite относятся только к Windows. В dev-скриптах bind ограничен loopback, что снижает сетевую экспозицию. Актуальные идентификаторы, диапазоны и ссылки сохранены в JSON-отчётах.

**Исправление:** обновить исправленные patch/minor в lockfile с проверкой совместимости; не открывать dev/test servers наружу; добавить audit-проверку в CI с учётом runtime/tooling-достижимости.

## Дополнительное укрепление (P3, не включено в 12 замечаний)

- У quiz HTTP-сервера есть `ReadHeaderTimeout` и `IdleTimeout`, но нет `ReadTimeout`/`WriteTimeout`; старые JSON-CRUD handlers не используют `MaxBytesReader`. На штатном nginx запросы буферизуются и ограничены 6 MiB, поэтому это не подтверждённый внешний бесконечный slow-body DoS. Для самостоятельного запуска Go-сервера нужны явные ограничения тела и времени, а для БД — пул и deadlines.
- В Caddy/nginx нет явных CSP, `frame-ancestors`, `X-Content-Type-Options`, HSTS/Referrer-Policy. Добавить согласованную browser policy, проверив Swagger и подключения к S3/WS. Отсутствие заголовка само по себе не доказывает XSS/CSRF; cross-site iframe также ограничивается текущими SameSite cookies.
- Нет видимого механизма удаления истёкших/revoked `refresh_sessions`; ввести retention/cleanup. Logout отзывает refresh token, а access JWT остаётся действителен до `exp` (по умолчанию 15 минут плюс leeway). Это следует явно считать принятой политикой либо реализовать отзыв.
- В Compose не заданы общие CPU/memory/pids budgets и сегментация сетей. Образы используют плавающие tags; для управляемого обновления и проверки supply chain полезны digest pinning и сканирование фактически собранных образов.
- `.env.example` содержит узнаваемые placeholders; некоторые достаточно длинны, чтобы пройти только проверку длины service token. Для production полезна fail-fast проверка известных placeholders и проверка независимости секретов.

## Что проверено и работает

- JWT ограничен HS256, проверяются issuer/audience/expiry/type, секрет требует минимум 32 байта; user ID берётся из проверенного subject.
- Refresh token — 32 случайных байта; в БД хранится SHA-256, ротация защищена транзакцией и `FOR UPDATE`, replay отзывает сессию; cookie имеет HttpOnly/Secure/SameSite=Strict.
- Запись чужих квизов/вопросов/ответов и изображений проверяет авторство; optional-auth не принимает неверный bearer как анонимного пользователя. Приватные GET проверяют видимость.
- Публичные HTTP DTO не возвращают `is_correct` и password hashes. WS не отправляет правильные ответы в открытом вопросе; они раскрываются после закрытия. Ticket hashes не попадают в публичные player views.
- Tickets создаются через crypto/rand, хранятся хешированно, сравниваются constant-time. Подключение нового участника с теми же credentials заменяет старую сессию; команды проверяют роль и актуальность подписчика.
- Actor loop сохраняет переход перед публикацией; Redis Lua проверяет lease/fence. Общая изменяемая game-state вне actor не обнаружена; `-race` прошёл в доступных тестах.
- SQL использует placeholders для пользовательских значений; небезопасной конкатенации пользовательского SQL не найдено. React экранирует строки, `dangerouslySetInnerHTML`/`eval` в исходниках не найдены.
- Загрузка ограничена JPEG/PNG, проверяются декодирование, размер файла и число пикселей; имя объекта генерируется сервером. Это не отменяет общий ресурсный риск QG-04.
- gRPC проверяет отдельный service token и JWT пользователя; реализован mTLS TLS 1.3 с проверкой client URI SAN при включённой конфигурации.
- PostgreSQL/Redis/gRPC не публикуются наружу штатным Compose; game/frontend host-порты привязаны к 127.0.0.1. Go runtime-контейнеры работают от отдельного непривилегированного пользователя.
- `.env` исключён из Git и Docker context, права локального файла — 0600. Проверены 74 доступных коммита / 732 blob-объекта по сигнатурам private keys, AWS access key IDs, GitHub/OpenAI tokens: совпадений нет. Это ограниченный signature scan, не энтропийный поиск всех возможных секретов. Проверялась также история путей `.env`/PEM/key; таких tracked-файлов не найдено.

## Проведённые проверки

| Проверка | Результат |
| --- | --- |
| `go test ./...` | Успешно, с оговоркой о skipped integrations выше |
| `go test -race ./services/game/... ./services/quiz/...` | Успешно |
| `go vet ./services/game/... ./services/quiz/...` | Успешно |
| Frontend `npm test` | 17 файлов, 68 тестов успешно |
| Frontend `npm run build` | Успешно; предупреждение о размере chunk не является security finding |
| Четыре изолированных audit-теста | Успешно подтвердили seed, login CSRF, заполнение лобби, actor при неверном ticket |
| govulncheck 1.9.0, Linux/amd64, анализируемый Go 1.25.1 | 69 GO-ID, 48 с symbol traces; статическая достижимость не означает эксплуатацию |
| `npm audit` / `--omit=dev` | 7 / 2 уязвимых узла |
| Образы/production/IAM/реальные DB integrations | Не проверены в этом окружении |

Первоначальный запуск `go run govulncheck@latest` автоматически переключил toolchain на 1.26.10. Его результат не использовался для оценки Go 1.25.1. Затем scanner установлен отдельно и запущен с `GOTOOLCHAIN=local`; конфигурация итогового анализа подтверждает Go 1.25.1. Отдельный Linux/amd64 scan использован для сохранённой сводки. База Go обновлена 27.10.2026 21:16:18 UTC.

Воспроизведение локальных сценариев без изменения исходных тестов:

```bash
python3 docs/security-audit-2026-10-28/reproduce.py
```

Скрипт создаёт временный Go overlay, использует существующие тестовые fakes, ничего не отправляет в production и не требует PostgreSQL/Redis. Это проверки текущего уязвимого поведения: после исправлений соответствующие проверки нужно заменить регрессионными тестами отказа.

## Порядок исправления и открытые вопросы

1. Убрать обязательный seed-аккаунт из production; обработать уже созданные сессии. Обновить Go и уязвимые сетевые зависимости, пересобрать/просканировать реальные образы.
2. Закрыть Login CSRF и добавить согласованные лимиты auth/admission, квоты игр/загрузок и тесты. Сохранить actor-loop, persistence для reconnect и совместимость WS.
3. Защитить гостевой join и унифицировать ограничения всех путей изменения квиза.
4. Разделить PostgreSQL runtime/migration roles, сегментировать Redis, согласовать credentials и включить TLS в production-профиле.
5. Обновить npm-lockfile, добавить CI security checks и повторить интеграционные/контейнерные проверки.

Не установлено, развёрнут ли этот Compose в production и есть ли внешний WAF/rate limiter. Такие внешние меры могут уменьшать некоторые риски, но не подтверждены кодом. Также остаются непроверенными фактические S3 bucket policy/IAM, версии image digest, права существующей DB-роли, ротация production-секретов и восстановление из резервной копии. Все 12 замечаний на момент завершения аудита остаются неисправленными.
