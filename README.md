# QuizGo

Проект разделён на два сервиса: `quiz` отвечает за CRUD и каталог квизов, `game` —
за игровые сессии и realtime-протокол. Они общаются только через контракт
`api/quiz/v1/quiz.proto`; game не подключается к PostgreSQL quiz-сервиса.

```text
api/quiz/v1/             общий protobuf-контракт
gen/quiz/v1/             сгенерированный Go-код
services/quiz/           CRUD: main, internal, frontend и отдельный tests/
services/game/           game: main, internal и отдельный tests/
```

Frontend quiz-сервиса использует React, TypeScript, Vite, Tailwind CSS, React Router
и TanStack Query. Квизы хранятся в PostgreSQL, изображения — в S3, игровые снимки —
в Redis.

## Запуск через Docker

Требуется запущенный Docker с Compose v2. При первой настройке скопируйте
`.env.example` в `.env` (если своего `.env` ещё нет), заполните `POSTGRES_PASSWORD`,
`JWT_SECRET` и отдельный `QUIZ_GRPC_SERVICE_TOKEN`. Для каждого значения можно
выполнить `openssl rand -hex 32`.
Храните ключ в `.env`, чтобы вход не сбрасывался при перезапуске контейнеров.

```sh
docker compose up --build -d --wait
```

Приложение: **http://localhost:3000**, Swagger: **http://localhost:3000/swagger/index.html**.
Порт интерфейса задаётся через `FRONTEND_PORT` в `.env`. Зарегистрируйте аккаунт
через интерфейс — база при первом запуске пустая.

Compose собирает оба Go-сервиса и React-приложение, запускает PostgreSQL 18, Redis и Nginx.
Nginx раздаёт сборку Vite, проксирует CRUD-маршруты в quiz-сервис, а
`/api/games` и `/ws` — в game-сервис;
прямые ссылки React Router также работают. На хост публикуется только порт
интерфейса, доступный локально. PostgreSQL и API доступны внутри Docker-сети.
`POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_SSLMODE` и `QUIZ_HTTP_PORT` из локального
окружения для контейнеров переопределяются в Compose.

Миграции применяются quiz-сервисом автоматически после готовности PostgreSQL.
Данные квизов и ссылки по ID сохраняются в томе `postgres_data` при пересоздании
контейнеров. Файлы изображений хранятся отдельно, в настроенном S3 bucket. Параметры пользователя и пароля PostgreSQL задаются при первом
создании тома: изменение `.env` само по себе пароль существующей базы не меняет.

```sh
docker compose logs -f          # логи всех сервисов
docker compose logs -f quiz     # логи CRUD API
docker compose logs -f game     # логи realtime game-сервиса
docker compose ps              # состояние контейнеров
docker compose down            # остановить и удалить контейнеры, сохранить данные
docker compose up --build -d --wait  # пересобрать после изменения исходников
```

Docker сохраняет stdout/stderr каждого сервиса через драйвер `json-file` и
автоматически ротирует логи: по умолчанию хранится до трёх файлов по 10 MiB на
контейнер. Лимиты можно изменить через `DOCKER_LOG_MAX_SIZE` и
`DOCKER_LOG_MAX_FILES` в `.env`; после изменения пересоздайте контейнеры.
Quiz предоставляет `/healthz` для liveness и `/readyz` для проверки PostgreSQL.
Game публикует те же endpoints на `127.0.0.1:${GAME_PORT:-8081}`; его readiness
проверяет Redis и gRPC health quiz-сервиса. Docker healthcheck использует readiness.
Quiz пишет access-log каждого завершённого HTTP-запроса: метод, путь, шаблон
маршрута, статус, размер ответа, длительность, IP клиента, `X-Request-ID` и
User-Agent. Тела запросов, query-параметры и заголовок авторизации не логируются.
Регулярные обращения Docker healthcheck к `/healthz` пропускаются.

Для полного сброса тестовой базы: `docker compose down -v` — эта команда
удаляет том со всеми данными PostgreSQL; объекты S3 остаются в bucket. Docker запускает готовую сборку; для разработки
с горячей перезагрузкой используйте `scripts/dev.sh` ниже.

## Изображения в S3

Создайте приватный bucket у своего S3-провайдера и заполните поля в `.env`:

```dotenv
S3_BUCKET=your-quizgo-bucket
S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
S3_URL_TTL=1h
```

Для AWS оставьте `S3_ENDPOINT` и `S3_BROWSER_ENDPOINT` пустыми. Backend также
поддерживает стандартную цепочку AWS credentials, включая IAM role вместо ключей.
Для временных credentials задайте `AWS_SESSION_TOKEN`; подписанные ссылки могут
истечь раньше `S3_URL_TTL`, вместе с credentials.

Для S3-совместимого сервиса задайте `S3_ENDPOINT=https://s3.example.com` и,
если провайдер требует, `S3_FORCE_PATH_STYLE=true`. Endpoint — адрес сервиса без
имени bucket и пути. Если backend использует внутренний адрес хранилища,
`S3_BROWSER_ENDPOINT` должен содержать адрес того же сервиса, доступный браузеру.
Например, `http://minio:9000` внутри Docker и `http://localhost:9000` для браузера.
URL подписывается сразу для внешнего адреса: менять host после подписи нельзя.

Секреты передаются только backend. Для работы с существующим bucket нужны права
`s3:PutObject`, `s3:GetObject`, `s3:DeleteObject` на `arn:aws:s3:::your-quizgo-bucket/images/*`.
Backend не создаёт bucket и не запрашивает список bucket. Публичный доступ не нужен:
API выдаёт временные подписанные GET-ссылки. Загрузка идёт через backend, отображение —
обычным `<img>` напрямую из S3; для этого сценария CORS на bucket не требуется.
При публикации приложения по HTTPS адрес S3 для браузера также должен быть HTTPS.

После настройки:

```sh
docker compose up --build -d --wait
```

Если `S3_BUCKET` пустой, API работает с квизами без изображений, а операции,
которым нужно S3, возвращают `503 image storage unavailable`.

Схема хранения: `images(id, author_id)`, `questions.image_id` — внешний ключ.
`author_id` нужен для проверки прав; байты, MIME-тип и URL в PostgreSQL не хранятся.
Ключ объекта S3 — `images/{id}`, MIME-тип передаётся в метаданных объекта при загрузке.
API возвращает `image_url` в вопросах и `{id, url}` при загрузке. Frontend использует
эти ссылки, а при истечении ссылки или восстановлении старого черновика получает
новую через `GET /api/images/{id}/url`. При сохранении отправляется только `image_id`.

### Переход с хранения файлов в PostgreSQL

Остановите предыдущую версию backend перед первым запуском новой и настройте S3.
При старте существующие бинарные изображения переносятся в `images/{id}` без
изменения ID и привязок к вопросам. Только после успешной загрузки всех файлов
миграция `0003_s3_images.sql` удаляет `content`, `content_type` и `image_url`.
При ошибке S3 перенос останавливается, исходные байты сохраняются в PostgreSQL;
после исправления конфигурации можно повторить запуск. Без S3 миграция базы со
старыми файлами останавливается с понятной ошибкой, чтобы не потерять изображения.

Старые записи, содержащие только внешние URL без байтов, требуют отдельного импорта:
миграция сохраняет такие записи и сообщает об этом. После успешного переноса
предыдущая версия backend с бинарным хранением больше не совместима со схемой.
Не меняйте bucket после переноса без копирования объектов с сохранением ключей.

Удаление неиспользуемого изображения удаляет объект S3 и его запись в базе.
Если S3 недоступен, запись остаётся для повторной попытки. Удаление квиза и отвязка
картинки не удаляют объекты автоматически. S3 и PostgreSQL не образуют общую
транзакцию: после аварии между операциями возможны объекты без записей;
при ошибке записи метаданных backend пытается удалить только что загруженный объект.

## Локальный запуск

Требуются Go 1.25+, Node.js 22.12+ или 24+, PostgreSQL. Создайте базу `quizgo`
и задайте параметры существующего PostgreSQL через окружение:

```sh
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=danilmitrosin
export POSTGRES_DB=quizgo
export POSTGRES_SSLMODE=disable
# При необходимости: export POSTGRES_PASSWORD=...
./scripts/dev.sh
```

Скрипт устанавливает frontend-зависимости при первом запуске, запускает API
на `8080` и Vite на `5173`. Откройте адрес, напечатанный Vite. Зарегистрируйте
аккаунт через интерфейс. Демо-пользователей и автоматического наполнения базы нет.
Если `JWT_SECRET` не задан, dev-скрипт создаёт случайный ключ на время запуска;
после его перезапуска потребуется войти заново. Ctrl+C останавливает процессы.
Миграции применяются API автоматически; существующие записи сохраняются.

Можно запускать сервисы отдельно:

```sh
export JWT_SECRET="$(openssl rand -hex 32)"
export QUIZ_GRPC_SERVICE_TOKEN="$(openssl rand -hex 32)"
go run ./services/quiz/cmd/quiz-service
```

Для game нужны Redis и уже запущенный quiz gRPC:

```sh
export QUIZ_GRPC_SERVICE_TOKEN="<тот же секрет, что у quiz>"
export GAME_REDIS_ADDRESS=localhost:6379
export GAME_QUIZ_GRPC_ADDRESS=localhost:9090
go run ./services/game/cmd/game-service
```

```sh
npm ci --prefix services/quiz/frontend
npm run dev --prefix services/quiz/frontend
```

При запуске через `go run` или `scripts/dev.sh` переменные S3 нужно экспортировать
в окружение процесса: `.env` автоматически читает только Docker Compose.

Если API слушают другие порты, задайте `QUIZ_HTTP_PORT` для CRUD backend,
`QUIZ_API_PROXY` и `QUIZ_GAME_PROXY` для Vite (пример в
`services/quiz/frontend/.env.example`). CRUD-запросы, `/api/games` и `/ws` идут
через Vite proxy, CORS для локальной разработки не нужен.
`VITE_*` переменные с секретами не используются.

## Готовая сборка на одном origin

```sh
npm ci --prefix services/quiz/frontend
npm run build --prefix services/quiz/frontend
export QUIZ_FRONTEND_DIR=services/quiz/frontend/dist
# Задайте постоянные JWT_SECRET, QUIZ_GRPC_SERVICE_TOKEN и параметры PostgreSQL.
go run ./services/quiz/cmd/quiz-service
```

API раздаёт статические файлы и поддерживает прямое открытие React Router-маршрутов,
например `/quizzes/1/edit`. `/api`, `/auth`, `/swagger` сохраняют свою маршрутизацию.
Внешний HTTP-сервер может завершать TLS и проксировать запросы к Go-сервису.

## Внутренний gRPC API

CRUD-сервис одновременно слушает HTTP (`QUIZ_HTTP_PORT`, по умолчанию `8080`) и
закрытый gRPC (`QUIZ_GRPC_PORT`, по умолчанию `9090`). Контракт realtime-сервиса:
`api/quiz/v1/quiz.proto`. RPC `GetPlayableQuiz` возвращает консистентный снимок
квиза с revision, порядком вопросов и ответов, лимитами времени и правильными
ответами. Подписанные S3 URL в снимок не входят: передаётся стабильный `image_id`.

Вызов требует две независимые учётные записи в metadata:

```text
x-quizgo-service-token: <QUIZ_GRPC_SERVICE_TOKEN>
authorization: Bearer <пользовательский JWT>
```

Service token должен быть отдельным случайным секретом не короче 32 байт. CRUD
сам проверяет пользовательский JWT и право собственности; `user_id` от realtime
не принимается. Невалидный или пустой квиз возвращает `FailedPrecondition`, чужой —
`PermissionDenied`. Realtime должен получить снимок один раз при создании игры и
сохранить его у себя, чтобы редактирование исходного квиза не влияло на сессию.

Без TLS gRPC допустим только в доверенной локальной сети разработки. Для production
задайте одновременно `QUIZ_GRPC_TLS_CERT_FILE`, `QUIZ_GRPC_TLS_KEY_FILE`,
`QUIZ_GRPC_TLS_CLIENT_CA_FILE` и URI SAN клиентского сертификата realtime в
`QUIZ_GRPC_ALLOWED_CLIENT_URI`. Тогда CRUD требует валидный клиентский сертификат,
точное совпадение URI SAN и service token. Health service доступен без пользовательского
JWT, но не раскрывает содержимое квизов. В Docker значения `*_FILE` являются путями
внутри backend-контейнера; сертификаты нужно смонтировать read-only средствами
конкретного deployment.

Контракт проверяется и генерируется через Buf:

```sh
go run github.com/bufbuild/buf/cmd/buf@v1.61.0 lint
go run github.com/bufbuild/buf/cmd/buf@v1.61.0 generate
```

JWT теперь проверяет точный алгоритм HS256, issuer, audience, тип access-токена,
срок действия и subject. `JWT_SECRET` остаётся только в CRUD; realtime пересылает
пользовательский токен как непрозрачное значение и не получает возможность выпускать
токены самостоятельно. Токены старого формата после обновления не принимаются;
пользователям потребуется войти заново.

## HTTPS через Docker Compose

Compose запускает Caddy перед frontend-контейнером. Caddy получает и обновляет
публичный TLS-сертификат, перенаправляет HTTP на HTTPS и проксирует весь трафик
во frontend nginx. Порт frontend `3000` остаётся доступен только на loopback
самого сервера и не должен открываться наружу.

Перед запуском:

1. Создайте DNS-запись `A` (и `AAAA`, если используется IPv6), указывающую домен
   на публичный адрес сервера.
2. Разрешите входящий TCP-трафик на порты `80` и `443` в firewall сервера и
   панели облачного провайдера.
3. Скопируйте `.env.example` в `.env`, замените секреты и задайте `DOMAIN` только
   именем хоста, например `quiz.example.com`, без `https://` и завершающего `/`.
4. Убедитесь, что другой reverse proxy не занимает порты `80` и `443`.

Запуск и проверка:

```sh
docker compose up -d --build
docker compose ps
docker compose logs -f caddy
```

После получения сертификата приложение доступно по `https://<DOMAIN>`. Данные
Caddy хранятся в volumes `caddy_data` и `caddy_config`; не удаляйте их при обычном
обновлении контейнеров.

## Возможности

- Регистрация, вход, повторный вход по истечении 15-минутной сессии, выход.
- Мои квизы, поиск, сортировка, создание и удаление с подтверждением.
- Название, вопросы, 2–8 вариантов ответа, один правильный ответ, время 5–120 секунд.
- Перестановка вопросов и ответов мышью, касанием и клавиатурой.
- Загрузка JPG/PNG до 5 MiB и 20 мегапикселей, замена и отвязка картинки.
- Явное транзакционное сохранение, Ctrl/Cmd+S, восстановление черновика в той же вкладке.
- Проверка `revision`: при конфликте черновик остаётся локально, замена данными сервера
  требует подтверждения. Черновики удаляются при выходе из аккаунта.
- Предпросмотр черновика, публичная страница сохранённого квиза, боковая панель вопроса.
- Копирование ссылки. Правильность ответов доступна только автору через отдельный API.
- Адаптивные экраны, доступные диалоги и меню, состояния загрузки и ошибок.

Пустой квиз допустим. Каждый сохранённый вопрос должен иметь текст, 2–8 заполненных
ответов и ровно один правильный. Максимум 100 вопросов, 50 символов в названии,
200 символов в вопросе/ответе. Порядок элементов сохраняется без смены существующих ID.

Game skeleton уже умеет создать сессию через `POST /api/games`: он передаёт JWT
пользователя в quiz gRPC, получает проверенный снимок и сохраняет его в Redis с TTL.
WebSocket endpoint `/ws` пока возвращает `501`; игровой цикл, подключение игроков и
рассылка событий остаются следующим этапом. Раздел «Открытия» обозначен как будущая
возможность. Изображения, отвязанные от вопросов, автоматически не удаляются:
для неиспользуемых загрузок предусмотрен `DELETE /api/images/{id}`.

## Проверки

```sh
npm run test --prefix services/quiz/frontend
npm run lint --prefix services/quiz/frontend
npm run build --prefix services/quiz/frontend
go test ./services/quiz/cmd/... ./services/quiz/internal/... ./services/quiz/docs/... \
  ./services/quiz/tests/... ./services/game/... ./gen/...
```

Интеграционный HTTP-тест с реальным PostgreSQL создаёт отдельную случайную схему
и удаляет только её. Без переменной ниже этот тест пропускается:

```sh
QUIZ_TEST_POSTGRES_DSN='postgres://user:password@localhost:5432/testdb?sslmode=disable' \
  go test ./services/quiz/tests/http -run TestEditorPostgresLifecycle -count=1 -v
```

Проверяются авторизация, CRUD, конфликты, откат транзакции, сохранение ID при перестановке,
загрузка и доступ к изображениям, отсутствие правильных ответов в публичных DTO,
каскадное удаление и повторное применение миграций. Frontend-тесты проверяют черновики,
переключение вопросов, восстановление, выбор правильного ответа, сохранение и ошибки сети.

Swagger: `/swagger/index.html`. Контракты и изменения API — в [services/quiz/docs/README.md](services/quiz/docs/README.md).
Каркас игрового сервиса описан в [services/game/README.md](services/game/README.md).
