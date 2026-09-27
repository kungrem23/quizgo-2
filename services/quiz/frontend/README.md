# QuizGo frontend

Исходники находятся в `src`, а все frontend-тесты — отдельно в `tests` с
зеркальной структурой каталогов.

React + TypeScript + Vite + Tailwind CSS + React Router + TanStack Query.

Полный запуск вместе с Go API: [инструкция в корне](../README.md).

```sh
npm ci
npm run dev
npm run test
npm run lint
npm run build
```

`QUIZ_API_PROXY` — адрес backend для dev-proxy, по умолчанию `http://127.0.0.1:8080`.
Сборка использует относительные `/api` и `/auth`; frontend и backend должны быть
доступны на одном origin. API может раздавать `dist` через `QUIZ_FRONTEND_DIR`.

- `app`: маршруты, QueryClient и провайдеры.
- `features/auth`: сессия, повторный вход, формы.
- `features/quizzes`: создание и оформление карточек.
- `features/quiz-editor`: локальная модель, черновики, вопросы, сохранение.
- `features/quiz-preview`: публичное и авторское представления.
- `shared/api`: типизированный HTTP-клиент, DTO, ошибки и ключи кеша.
- `shared/ui`: компоненты, диалоги, меню, уведомления.
- `styles/app.css`: тема Tailwind и адаптивная стилизация по референсу.

Серверное состояние хранится в TanStack Query. Несохранённая форма — в reducer
редактора и `sessionStorage` с ключом пользователя/квиза. Авторские данные не попадают
в публичный кеш. Новый вопрос/ответ имеет временный клиентский ключ и `id: 0` в запросе;
после сохранения используются ID из ответа сервера.

Вспомогательные библиотеки: Radix UI (диалоги/меню), dnd-kit (сортировка),
Lucide (ISC, иконки и favicon), Inter (OFL, локальные latin/cyrillic шрифты).
Исходник favicon: https://github.com/lucide-icons/lucide/blob/main/icons/zap.svg.
