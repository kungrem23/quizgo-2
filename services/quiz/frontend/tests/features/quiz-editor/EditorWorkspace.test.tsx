// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { EditorWorkspace } from '../../../src/features/quiz-editor/EditorWorkspace';
import { ToastProvider } from '../../../src/shared/ui/ui';
import type { QuizContent } from '../../../src/shared/api/types';
vi.mock('../../../src/features/auth/AuthProvider', () => ({
  useAuth: () => ({ user: { id: 42, username: 'alice' } }),
}));
const fixture = (): QuizContent => ({
  id: 1,
  author_id: 42,
  title: 'География',
  revision: 1,
  updated_at: new Date().toISOString(),
  question_count: 2,
  questions: [1, 2].map((id) => ({
    id,
    text_content: `Вопрос ${id}`,
    image_id: '',
    time_limit: 20,
    answers: [
      { id: id * 10, text_content: 'Париж', is_correct: true },
      { id: id * 10 + 1, text_content: 'Лондон', is_correct: false },
    ],
  })),
});
function mount(content = fixture()) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  function ContentHost() {
    const { data } = useQuery({
      queryKey: ['private', 42, 'quiz', 1],
      queryFn: async () => content,
      initialData: content,
      staleTime: Infinity,
    });
    return <EditorWorkspace content={data} />;
  }
  const router = createMemoryRouter([{ path: '/quizzes/1/edit', element: <ContentHost /> }], {
    initialEntries: ['/quizzes/1/edit'],
  });
  render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <RouterProvider router={router} />
      </ToastProvider>
    </QueryClientProvider>,
  );
  return { client, router };
}
beforeEach(() => sessionStorage.clear());
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
describe('editor workflow', () => {
  it('retains text across question switches and refresh recovery', async () => {
    const user = userEvent.setup();
    mount();
    const field = screen.getByLabelText('Ваш вопрос', { exact: false });
    await user.clear(field);
    await user.type(field, 'Сохранённый локально вопрос');
    await user.click(screen.getByRole('button', { name: 'Вопрос 2: Вопрос 2' }));
    expect(
      (screen.getByLabelText('Ваш вопрос', { exact: false }) as HTMLTextAreaElement).value,
    ).toBe('Вопрос 2');
    await user.click(screen.getByRole('button', { name: 'Вопрос 1: Сохранённый локально вопрос' }));
    expect(
      (screen.getByLabelText('Ваш вопрос', { exact: false }) as HTMLTextAreaElement).value,
    ).toBe('Сохранённый локально вопрос');
    cleanup();
    mount();
    expect(
      (screen.getByLabelText('Ваш вопрос', { exact: false }) as HTMLTextAreaElement).value,
    ).toBe('Сохранённый локально вопрос');
  });
  it('saves edited content and correct answer, then clears the local draft', async () => {
    const user = userEvent.setup();
    const data = fixture();
    const fetchMock = vi.fn(async (_url: unknown, options?: RequestInit) => {
      const input = JSON.parse(String(options?.body));
      return new Response(JSON.stringify({ ...data, ...input, revision: 12 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);
    mount();
    await user.click(screen.getByRole('radio', { name: 'Ответ 2 — правильный' }));
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const body = JSON.parse(String(fetchMock.mock.calls[0][1]?.body));
    expect(body.questions[0].answers[1].is_correct).toBe(true);
    expect(body.questions[0].answers[0].is_correct).toBe(false);
    expect(body.questions[0].id).toBe(1);
    await waitFor(() => expect(sessionStorage.getItem('quizgo:draft:42:1')).toBeNull());
    expect(screen.getByText('Все изменения сохранены')).toBeTruthy();
    expect(
      (screen.getByRole('radio', { name: 'Ответ 2 — правильный' }) as HTMLInputElement).checked,
    ).toBe(true);
  });
  it('keeps the draft on a network failure', async () => {
    const user = userEvent.setup();
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('offline')));
    mount();
    await user.type(screen.getByLabelText('Название квиза'), ' 2');
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    await screen.findByRole('alert');
    expect(sessionStorage.getItem('quizgo:draft:42:1')).toContain('География 2');
    expect((screen.getByLabelText('Название квиза') as HTMLInputElement).value).toBe('География 2');
  });
  it('validates an incomplete new question before sending a request', async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    mount();
    await user.click(screen.getByRole('button', { name: 'Добавить вопрос' }));
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(screen.getByRole('alert').textContent).toContain('Вопрос 3');
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
