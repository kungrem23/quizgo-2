import type {
  CreatedGame,
  QuizContent,
  QuizSummary,
  SaveContent,
  UploadedImage,
  User,
} from './types';
let authorization = '';
export function setAuthorization(token: string) {
  authorization = token ? `Bearer ${token.replace(/^Bearer\s+/i, '')}` : '';
}
export class ApiError extends Error {
  status: number;
  fields: Record<string, string>;
  constructor(status: number, message: string, fields: Record<string, string> = {}) {
    super(message);
    this.status = status;
    this.fields = fields;
  }
}
export async function request<T>(
  path: string,
  options: RequestInit = {},
  authenticated = true,
): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body && !(options.body instanceof FormData))
    headers.set('Content-Type', 'application/json');
  if (authorization && authenticated) headers.set('Authorization', authorization);
  let response: Response;
  try {
    response = await fetch(path, { ...options, headers });
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error;
    throw new ApiError(
      0,
      'Не удалось связаться с сервером. Проверьте соединение и попробуйте ещё раз.',
    );
  }
  const text = await response.text();
  let data: unknown;
  try {
    data = text ? JSON.parse(text) : undefined;
  } catch {
    data = undefined;
  }
  if (!response.ok) {
    const value = data as { error?: string; fields?: Record<string, string> } | undefined;
    const message = value?.error || 'Ошибка сервера';
    if (response.status === 401 && authenticated && !message.startsWith('you cant'))
      window.dispatchEvent(new Event('quizgo:session-expired'));
    throw new ApiError(response.status, message, value?.fields);
  }
  return data as T;
}
export function errorMessage(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Что-то пошло не так. Попробуйте ещё раз.';
  if (error.status === 0) return error.message;
  if (error.status === 403 || error.message.startsWith('you cant'))
    return 'Редактировать этот квиз может только его автор.';
  if (error.status === 401) return 'Войдите в аккаунт, чтобы продолжить.';
  if (error.status === 404) return 'Квиз не найден. Возможно, он был удалён.';
  if (error.status === 409)
    return 'Квиз изменён в другой вкладке. Ваш черновик сохранён. Откройте актуальную версию, прежде чем продолжить.';
  if (error.status === 503 && error.message === 'image storage unavailable')
    return 'Хранилище изображений недоступно. Попробуйте позже.';
  if (error.status === 413) return 'Размер изображения не должен превышать 5 МБ.';
  if (error.status === 400) return 'Проверьте заполнение полей и попробуйте ещё раз.';
  return 'Сервер временно недоступен. Ваши изменения остались в редакторе.';
}
const json = (value: unknown) => JSON.stringify(value);
export const api = {
  login: (username: string, password: string) =>
    request<{ token: string }>(
      '/auth/login',
      { method: 'POST', body: json({ username, password }) },
      false,
    ),
  register: (username: string, password: string) =>
    request<void>('/auth/register', { method: 'POST', body: json({ username, password }) }, false),
  me: () => request<User>('/api/me'),
  myQuizzes: (signal?: AbortSignal) =>
    request<QuizSummary[] | null>('/api/me/quizzes', { signal }).then((v) => v ?? []),
  createQuiz: (title: string) =>
    request<{ id: number }>('/api/quizzes', { method: 'POST', body: json({ title }) }),
  createGame: (quizId: number) =>
    request<CreatedGame>('/api/games', { method: 'POST', body: json({ quiz_id: quizId }) }),
  quiz: (id: number, author = false, signal?: AbortSignal) =>
    request<QuizContent>(`/api/quizzes/${id}/${author ? 'author' : 'content'}`, { signal }, author),
  user: (id: number) => request<User>(`/api/users/${id}`, {}, false),
  saveQuiz: (id: number, content: SaveContent) =>
    request<QuizContent>(`/api/quizzes/${id}/content`, { method: 'PUT', body: json(content) }),
  deleteQuiz: (id: number) => request<void>(`/api/quizzes/${id}`, { method: 'DELETE' }),
  imageURL: (id: string, signal?: AbortSignal) =>
    request<UploadedImage>(`/api/images/${encodeURIComponent(id)}/url`, { signal }, false),
  uploadImage: (file: File) => {
    const body = new FormData();
    body.append('file', file);
    return request<UploadedImage>('/api/images', { method: 'POST', body });
  },
};
export const keys = {
  quizzes: (userId: number) => ['private', userId, 'quizzes'] as const,
  author: (userId: number, id: number) => ['private', userId, 'quiz', id] as const,
  quiz: (id: number) => ['quiz', id] as const,
};
