// @vitest-environment jsdom
import { act, cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CountdownScreen } from '../../../src/features/game/CountdownScreen';
import { gameReducer, initialGameState } from '../../../src/features/game/reducer';
import type { GameSocketStatus } from '../../../src/features/game/socket';
import { parseServerMessage } from '../../../src/features/game/wire';

const connected: GameSocketStatus = { state: 'open' };

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('host and player countdown', () => {
  it('uses the same future server deadline, counts down, and waits at zero for question_opened', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-30T12:00:00.223Z'));
    const nativeParse = Date.parse;
    vi.spyOn(Date, 'parse').mockImplementation((value) => {
      if (/\.\d{4,}(?:Z|[+-]\d{2}:\d{2})$/.test(value)) return Number.NaN;
      return nativeParse(value);
    });
    const countdownStarted = parseServerMessage({
      type: 'countdown_started',
      sequence: 2,
      payload: {
        game_id: 'game-1',
        code: 'ABC123',
        quiz_title: 'Quiz',
        phase: 'countdown',
        players: [{ id: 'player-1', nickname: 'Alice', score: 0 }],
        current_question_index: 0,
        countdown_ends_at: '2026-09-30T12:00:03.123456789Z',
      },
    });
    const hostState = gameReducer(initialGameState, {
      type: 'message',
      message: countdownStarted,
    });
    const playerState = gameReducer(initialGameState, {
      type: 'message',
      message: countdownStarted,
    });

    expect(hostState.snapshot?.phase).toBe('countdown');
    expect(hostState.snapshot?.countdown_ends_at).toBe(playerState.snapshot?.countdown_ends_at);

    const view = render(
      <>
        <section aria-label="Host countdown">
          <CountdownScreen snapshot={hostState.snapshot!} status={connected} />
        </section>
        <section aria-label="Player countdown">
          <CountdownScreen snapshot={playerState.snapshot!} status={connected} />
        </section>
      </>,
    );
    const host = within(screen.getByRole('region', { name: 'Host countdown' }));
    const player = within(screen.getByRole('region', { name: 'Player countdown' }));

    expect(host.getByRole('timer').textContent).toBe('3');
    expect(player.getByRole('timer').textContent).toBe('3');
    expect(vi.getTimerCount()).toBe(2);

    act(() => vi.advanceTimersByTime(1_000));
    expect(host.getByRole('timer').textContent).toBe('2');
    expect(player.getByRole('timer').textContent).toBe('2');
    expect(vi.getTimerCount()).toBe(2);

    act(() => vi.advanceTimersByTime(2_000));
    expect(host.getByRole('timer').textContent).toBe('0');
    expect(player.getByRole('timer').textContent).toBe('0');
    expect(host.getByText('Ожидаем вопрос от сервера…')).toBeTruthy();
    expect(player.getByText('Ожидаем вопрос от сервера…')).toBeTruthy();
    expect(hostState.snapshot?.phase).toBe('countdown');
    expect(playerState.snapshot?.phase).toBe('countdown');
    expect(vi.getTimerCount()).toBe(0);

    const questionOpened = parseServerMessage({
      type: 'question_opened',
      sequence: 3,
      payload: {
        game_id: 'game-1',
        code: 'ABC123',
        quiz_title: 'Quiz',
        phase: 'question_open',
        players: [{ id: 'player-1', nickname: 'Alice', score: 0 }],
        current_question_index: 0,
        current_question: {
          id: 10,
          text: 'Question?',
          time_limit_seconds: 20,
          answers: [
            { id: 11, text: 'A' },
            { id: 12, text: 'B' },
          ],
        },
        question_closes_at: '2026-09-30T12:00:23.123456789Z',
      },
    });
    expect(
      gameReducer(hostState, { type: 'message', message: questionOpened }).snapshot?.phase,
    ).toBe('question_open');
    expect(
      gameReducer(playerState, { type: 'message', message: questionOpened }).snapshot?.phase,
    ).toBe('question_open');

    view.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
