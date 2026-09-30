// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PlayerQuestionScreen } from '../../../src/features/game/QuestionScreen';
import type { GameStateSnapshot } from '../../../src/features/game/types';
import { api } from '../../../src/shared/api/client';

const questionState: GameStateSnapshot = {
  game_id: 'game-1',
  code: 'ABC123',
  quiz_title: 'Физика',
  phase: 'question_open',
  players: [{ id: 'player-1', nickname: 'Alice', score: 0 }],
  current_question_index: 0,
  current_question: {
    id: 10,
    text: 'Какой вариант правильный?',
    time_limit_seconds: 30,
    answers: Array.from({ length: 6 }, (_, index) => ({
      id: index + 1,
      text: `Вариант ${index + 1}`,
    })),
  },
  question_closes_at: '2099-09-30T12:00:30.000Z',
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

describe('player question answer flow', () => {
  beforeEach(() => vi.clearAllMocks());

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('keeps answers available when a fast client clock is compensated by server time', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-30T12:05:00.000Z'));
    const sendCommand = vi.fn(() => ({
      requestId: 'answer-10-clock',
      acknowledged: Promise.resolve({ duplicate: false }),
    }));
    render(
      <PlayerQuestionScreen
        snapshot={{
          ...questionState,
          question_closes_at: '2026-09-30T12:00:30.000000000Z',
        }}
        status={{ state: 'open' }}
        acceptedQuestionIds={[]}
        serverTimeOffsetMs={-5 * 60_000}
        sendCommand={sendCommand}
      />,
    );

    const firstAnswer = screen.getByRole('button', { name: /Вариант 1/ });
    expect((firstAnswer as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(firstAnswer);
    expect(sendCommand).toHaveBeenCalledTimes(1);
  });

  it('renders question images in a contain media frame without fixed dimensions', async () => {
    vi.spyOn(api, 'imageURL').mockResolvedValue({
      id: 'image-1',
      url: 'https://storage.test/portrait-image',
    });
    const sendCommand = vi.fn(() => ({
      requestId: 'answer-10-image',
      acknowledged: Promise.resolve({ duplicate: false }),
    }));
    render(
      <QueryClientProvider client={new QueryClient()}>
        <PlayerQuestionScreen
          snapshot={{
            ...questionState,
            current_question: { ...questionState.current_question!, image_id: 'image-1' },
          }}
          status={{ state: 'open' }}
          acceptedQuestionIds={[]}
          sendCommand={sendCommand}
        />
      </QueryClientProvider>,
    );

    const image = await screen.findByRole('img', { name: 'Иллюстрация к вопросу' });
    expect(image.classList.contains('game-question-image')).toBe(true);
    expect(image.parentElement?.classList.contains('game-question-media')).toBe(true);
    expect(image.hasAttribute('width')).toBe(false);
    expect(image.hasAttribute('height')).toBe(false);
  });

  it('renders every dynamic answer and locks submission through answer_accepted', async () => {
    const acknowledgement = deferred<{ duplicate: boolean }>();
    const sendCommand = vi.fn((_command, requestId?: string) => ({
      requestId: requestId || '',
      acknowledged: acknowledgement.promise,
    }));
    const view = render(
      <PlayerQuestionScreen
        snapshot={questionState}
        status={{ state: 'open' }}
        acceptedQuestionIds={[]}
        sendCommand={sendCommand}
      />,
    );

    const answers = screen.getAllByRole('button');
    expect(answers).toHaveLength(6);
    expect(document.querySelector('.game-answer-grid')?.getAttribute('data-answer-count')).toBe(
      '6',
    );
    expect(document.querySelector('.game-answer-grid')?.classList.contains('is-compact')).toBe(
      true,
    );

    fireEvent.click(screen.getByRole('button', { name: /Вариант 1/ }));
    expect(sendCommand).toHaveBeenCalledTimes(1);
    expect(sendCommand).toHaveBeenCalledWith(
      { type: 'answer', payload: { answer_id: 1 } },
      expect.stringMatching(/^answer-10-/),
    );
    expect(screen.getByText('Отправляем ответ…')).toBeTruthy();
    for (const answer of screen.getAllByRole('button')) {
      expect((answer as HTMLButtonElement).disabled).toBe(true);
    }

    fireEvent.click(screen.getByRole('button', { name: /Вариант 2/ }));
    expect(sendCommand).toHaveBeenCalledTimes(1);

    view.rerender(
      <PlayerQuestionScreen
        snapshot={questionState}
        status={{ state: 'open' }}
        acceptedQuestionIds={[10]}
        sendCommand={sendCommand}
      />,
    );
    expect(screen.getByText('Ответ принят. Ожидаем завершения вопроса.')).toBeTruthy();
    expect(sendCommand).toHaveBeenCalledTimes(1);

    await act(async () => acknowledgement.resolve({ duplicate: false }));
    await waitFor(() => expect(sendCommand).toHaveBeenCalledTimes(1));
  });
});
