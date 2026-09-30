// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { saveGameCredentials, loadPlayerCredentials } from '../../../src/features/game/credentials';
import { FinalResultsScreen } from '../../../src/features/game/FinalResultsScreen';
import type { GameStateSnapshot } from '../../../src/features/game/types';

const finished: GameStateSnapshot = {
  game_id: 'game-1',
  code: 'ABC123',
  quiz_title: 'Физика',
  phase: 'finished',
  players: [
    { id: 'player-1', nickname: 'Max', score: 9450 },
    { id: 'player-2', nickname: 'Alice', score: 8120 },
    { id: 'player-3', nickname: 'Bob', score: 7690 },
    { id: 'player-4', nickname: 'Dana', score: 6100 },
  ],
  current_question_index: 2,
};

function renderRoute(element: React.ReactNode, path: string) {
  const router = createMemoryRouter(
    [
      { path, element },
      { path: '/join', element: <div>Join page</div> },
      { path: '/quizzes', element: <div>Quizzes page</div> },
    ],
    { initialEntries: [path] },
  );
  return { router, ...render(<RouterProvider router={router} />) };
}

describe('final game results', () => {
  beforeEach(() => sessionStorage.clear());
  afterEach(cleanup);

  it('shows the host podium and remaining authoritative leaderboard', () => {
    renderRoute(
      <FinalResultsScreen role="host" snapshot={finished} />,
      '/games/game-1/host',
    );

    expect(screen.getByRole('heading', { name: 'Финальные результаты' })).toBeTruthy();
    expect(screen.getByLabelText('1 место')).toBeTruthy();
    expect(screen.getByLabelText('2 место')).toBeTruthy();
    expect(screen.getByLabelText('3 место')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Итоговая таблица' })).toBeTruthy();
    expect(screen.getByText('Dana')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Сыграть ещё раз' })).toBeNull();
  });

  it('offers a replay only when a persisted quiz id was loaded', () => {
    renderRoute(
      <FinalResultsScreen role="host" snapshot={finished} quizId={42} />,
      '/games/game-1/host',
    );

    expect(screen.getByRole('button', { name: 'Сыграть ещё раз' })).toBeTruthy();
  });

  it('shows only available player totals and exits without mutating final state', async () => {
    saveGameCredentials({
      role: 'player',
      gameId: 'game-1',
      code: 'ABC123',
      playerId: 'player-2',
      nickname: 'Alice',
      playerTicket: 'ticket',
    });
    const { router } = renderRoute(
      <FinalResultsScreen role="player" snapshot={finished} playerId="player-2" />,
      '/games/game-1/player',
    );

    expect(screen.getByRole('heading', { name: 'Спасибо за игру!' })).toBeTruthy();
    expect(screen.getByText('Вы заняли 2 место')).toBeTruthy();
    expect(
      screen.getByText((_, element) => element?.textContent === '8\u00a0120'),
    ).toBeTruthy();
    expect(screen.getByText('Участников в итоговой таблице: 4')).toBeTruthy();
    expect(screen.queryByText(/Правильных ответов/i)).toBeNull();
    expect(screen.queryByText(/Лучшая серия/i)).toBeNull();

    await userEvent.click(screen.getByRole('button', { name: 'Выйти из игры' }));
    expect(router.state.location.pathname).toBe('/join');
    expect(loadPlayerCredentials('game-1')).toBeNull();
  });
});
