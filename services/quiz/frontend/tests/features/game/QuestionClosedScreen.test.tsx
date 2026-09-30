// @vitest-environment jsdom
import { act, cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { QuestionClosedScreen } from '../../../src/features/game/QuestionClosedScreen';
import type { GameStateSnapshot } from '../../../src/features/game/types';

const snapshot: GameStateSnapshot = {
  game_id: 'game-1',
  code: 'ABC123',
  quiz_title: 'Quiz',
  phase: 'scoreboard',
  players: [{ id: 'player-1', nickname: 'Alice', score: 100 }],
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
  correct_answer_ids: [11],
};

afterEach(cleanup);

describe('question closed transition', () => {
  it('lets only the host send next and keeps the command pending until its acknowledgement', async () => {
    let acknowledge!: (value: { duplicate: boolean }) => void;
    const acknowledged = new Promise<{ duplicate: boolean }>((resolve) => {
      acknowledge = resolve;
    });
    const sendCommand = vi.fn(() => ({ requestId: 'next-1', acknowledged }));
    const user = userEvent.setup();

    render(<QuestionClosedScreen role="host" snapshot={snapshot} sendCommand={sendCommand} />);
    const button = screen.getByRole('button', { name: 'Следующий вопрос' });
    await user.click(button);

    expect(sendCommand).toHaveBeenCalledTimes(1);
    expect(sendCommand).toHaveBeenCalledWith({ type: 'next' });
    expect((button as HTMLButtonElement).disabled).toBe(true);
    expect(button.getAttribute('aria-busy')).toBe('true');

    await act(async () => acknowledge({ duplicate: false }));
    expect((button as HTMLButtonElement).disabled).toBe(false);
  });

  it('keeps a player waiting without exposing the host command', () => {
    render(<QuestionClosedScreen role="player" snapshot={snapshot} />);

    expect(screen.getByText('Ждём, пока ведущий запустит следующий вопрос.')).toBeTruthy();
    expect(
      screen.getByText('Результат ответа недоступен после переподключения.'),
    ).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Следующий вопрос' })).toBeNull();
  });

  it('shows a correct accepted answer and the current total score without fake deltas', () => {
    render(
      <QuestionClosedScreen
        role="player"
        snapshot={snapshot}
        playerId="player-1"
        selectedAnswerId={11}
        answerAccepted
        questionObserved
      />,
    );

    expect(screen.getByRole('heading', { name: 'Верно!' })).toBeTruthy();
    expect(screen.getAllByText('A')).toHaveLength(2);
    expect(screen.getByText('100')).toBeTruthy();
    expect(screen.queryByText(/\+100/)).toBeNull();
  });

  it('distinguishes an incorrect answer from a real timeout', () => {
    const view = render(
      <QuestionClosedScreen
        role="player"
        snapshot={snapshot}
        playerId="player-1"
        selectedAnswerId={12}
        answerAccepted
        questionObserved
      />,
    );
    expect(screen.getByRole('heading', { name: 'В этот раз не угадали' })).toBeTruthy();
    expect(screen.getByText('B')).toBeTruthy();
    expect(screen.getByText('A')).toBeTruthy();

    view.rerender(
      <QuestionClosedScreen
        role="player"
        snapshot={snapshot}
        playerId="player-1"
        questionObserved
      />,
    );
    expect(screen.getByRole('heading', { name: 'Время вышло!' })).toBeTruthy();
    expect(screen.getByText('Не выбран')).toBeTruthy();
  });
});
