// @vitest-environment jsdom
import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { loadPlayerCredentials, saveGameCredentials } from '../../../src/features/game/credentials';
import { HostGamePage, PlayerGamePage } from '../../../src/pages/GamePage';
import { JoinPage } from '../../../src/pages/JoinPage';

vi.mock('../../../src/features/game/JoinQRCode', () => ({
  JoinQRCode: () => <div data-testid="qr-code" />,
}));

class TestWebSocket extends EventTarget {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: TestWebSocket[] = [];

  readonly sent: Array<Record<string, unknown>> = [];
  readyState = TestWebSocket.CONNECTING;

  constructor(readonly url: string) {
    super();
    TestWebSocket.instances.push(this);
    queueMicrotask(() => {
      if (this.readyState !== TestWebSocket.CONNECTING) return;
      this.readyState = TestWebSocket.OPEN;
      this.dispatchEvent(new Event('open'));
    });
  }

  send(value: string) {
    this.sent.push(JSON.parse(value) as Record<string, unknown>);
  }

  close() {
    this.readyState = TestWebSocket.CLOSED;
  }

  receive(message: Record<string, unknown>) {
    this.dispatchEvent(new MessageEvent('message', { data: JSON.stringify(message) }));
  }
}

function snapshot(phase: 'lobby' | 'countdown') {
  return {
    game_id: 'game-1',
    code: 'ABC123',
    quiz_title: 'Физика',
    phase,
    players: [{ id: 'player-1', nickname: 'Alice', score: 0 }],
    current_question_index: phase === 'countdown' ? 0 : -1,
    ...(phase === 'countdown' ? { countdown_ends_at: '2026-09-30T17:00:03Z' } : {}),
  };
}

function openQuestionSnapshot() {
  return {
    game_id: 'game-1',
    code: 'ABC123',
    quiz_title: 'Физика',
    phase: 'question_open',
    players: [{ id: 'player-1', nickname: 'Alice', score: 0 }],
    current_question_index: 0,
    current_question: {
      id: 10,
      text: 'Какой ответ правильный?',
      time_limit_seconds: 30,
      answers: [
        { id: 11, text: 'Первый вариант' },
        { id: 12, text: 'Второй вариант' },
      ],
    },
    question_closes_at: '2099-09-30T17:00:30Z',
  };
}

describe('player join route flow', () => {
  beforeEach(() => {
    sessionStorage.clear();
    TestWebSocket.instances = [];
    vi.stubGlobal('WebSocket', TestWebSocket);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('joins, persists credentials, reauthenticates and leaves lobby after host start', async () => {
    const router = createMemoryRouter(
      [
        { path: '/join', element: <JoinPage /> },
        { path: '/join/:code', element: <JoinPage /> },
        { path: '/games/:gameId/player', element: <PlayerGamePage /> },
      ],
      { initialEntries: ['/join/ABC123'] },
    );
    const user = userEvent.setup();
    const playerView = render(<RouterProvider router={router} />);

    await user.type(screen.getByLabelText('Никнейм'), 'Alice');
    await user.click(screen.getByRole('button', { name: 'Войти в игру' }));

    await waitFor(() => expect(TestWebSocket.instances).toHaveLength(1));
    const joinSocket = TestWebSocket.instances[0];
    await waitFor(() =>
      expect(joinSocket.sent).toEqual([
        expect.objectContaining({
          type: 'join',
          payload: { code: 'ABC123', nickname: 'Alice' },
        }),
      ]),
    );

    act(() => {
      joinSocket.receive({
        type: 'joined',
        request_id: 'join-1',
        sequence: 0,
        payload: {
          game_id: 'game-1',
          player_id: 'player-1',
          nickname: 'Alice',
          ticket: 'player-ticket',
        },
      });
    });

    await waitFor(() => expect(router.state.location.pathname).toBe('/games/game-1/player'));
    expect(loadPlayerCredentials('game-1')).toEqual({
      role: 'player',
      gameId: 'game-1',
      code: 'ABC123',
      playerId: 'player-1',
      nickname: 'Alice',
      playerTicket: 'player-ticket',
    });
    expect(joinSocket.readyState).toBe(TestWebSocket.CLOSED);

    await waitFor(() => expect(TestWebSocket.instances).toHaveLength(2));
    const gameSocket = TestWebSocket.instances[1];
    await waitFor(() =>
      expect(gameSocket.sent).toEqual([
        expect.objectContaining({
          type: 'player_auth',
          payload: {
            game_id: 'game-1',
            participant_id: 'player-1',
            ticket: 'player-ticket',
          },
        }),
      ]),
    );

    act(() => {
      gameSocket.receive({
        type: 'authenticated',
        request_id: 'auth-1',
        sequence: 1,
        payload: { game_id: 'game-1', participant_id: 'player-1', role: 'player' },
      });
      gameSocket.receive({ type: 'state', sequence: 1, payload: snapshot('lobby') });
    });

    expect(await screen.findByRole('heading', { name: 'Alice' })).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Ждём старта игры' })).toBeTruthy();
    expect(screen.getByText('В комнате уже 1 участник')).toBeTruthy();

    expect(
      saveGameCredentials({
        role: 'host',
        gameId: 'game-1',
        code: 'ABC123',
        hostTicket: 'host-ticket',
      }),
    ).toBe(true);
    const hostRouter = createMemoryRouter(
      [{ path: '/games/:gameId/host', element: <HostGamePage /> }],
      { initialEntries: ['/games/game-1/host'] },
    );
    const hostView = render(<RouterProvider router={hostRouter} />);
    await waitFor(() => expect(TestWebSocket.instances).toHaveLength(3));
    const hostSocket = TestWebSocket.instances[2];
    await waitFor(() =>
      expect(hostSocket.sent).toEqual([
        expect.objectContaining({
          type: 'host_auth',
          payload: { game_id: 'game-1', ticket: 'host-ticket' },
        }),
      ]),
    );
    act(() => {
      hostSocket.receive({
        type: 'authenticated',
        request_id: 'host-auth-1',
        sequence: 1,
        payload: { game_id: 'game-1', participant_id: 'host', role: 'host' },
      });
      hostSocket.receive({ type: 'state', sequence: 1, payload: snapshot('lobby') });
    });

    await user.click(
      await within(hostView.container).findByRole('button', { name: 'Начать игру' }),
    );
    const startMessage = hostSocket.sent[1];
    expect(startMessage).toEqual(expect.objectContaining({ type: 'start' }));

    act(() => {
      gameSocket.receive({
        type: 'countdown_started',
        sequence: 2,
        payload: snapshot('countdown'),
      });
      hostSocket.receive({
        type: 'countdown_started',
        sequence: 2,
        payload: snapshot('countdown'),
      });
      hostSocket.receive({
        type: 'command_accepted',
        request_id: startMessage.request_id,
        sequence: 2,
        payload: { duplicate: false },
      });
    });

    expect(await within(playerView.container).findByRole('timer')).toBeTruthy();
    expect(
      within(playerView.container).queryByRole('heading', { name: 'Ждём старта игры' }),
    ).toBeNull();
    expect(router.state.location.pathname).toBe('/games/game-1/player');

    act(() => {
      gameSocket.receive({
        type: 'question_opened',
        sequence: 3,
        payload: openQuestionSnapshot(),
      });
      hostSocket.receive({
        type: 'question_opened',
        sequence: 3,
        payload: openQuestionSnapshot(),
      });
    });
    expect(
      await within(playerView.container).findByRole('heading', {
        name: 'Какой ответ правильный?',
      }),
    ).toBeTruthy();
    expect(within(playerView.container).getAllByRole('button')).toHaveLength(2);

    act(() => {
      const closed = {
        type: 'question_closed',
        sequence: 4,
        payload: {
          phase: 'scoreboard',
          question_id: 10,
          correct_answer_ids: [11],
          players: [{ id: 'player-1', nickname: 'Alice', score: 0 }],
        },
      };
      gameSocket.receive(closed);
      hostSocket.receive(closed);
    });
    expect(await within(playerView.container).findByText('scoreboard')).toBeTruthy();
    expect(
      within(playerView.container).queryByRole('heading', { name: 'Какой ответ правильный?' }),
    ).toBeNull();
  });

  it.each([
    ['game_not_found', 'Игра с таким кодом не найдена.'],
    ['invalid_phase', 'Игра уже началась, присоединиться к ней нельзя.'],
    ['nickname_taken', 'Этот никнейм уже занят в комнате.'],
  ] as const)('shows the real %s join error without navigating', async (code, message) => {
    const router = createMemoryRouter(
      [
        { path: '/join', element: <JoinPage /> },
        { path: '/join/:code', element: <JoinPage /> },
      ],
      { initialEntries: ['/join/ABC123'] },
    );
    const user = userEvent.setup();
    render(<RouterProvider router={router} />);

    await user.type(screen.getByLabelText(/Никнейм/), 'Alice');
    await user.click(screen.getByRole('button', { name: 'Войти в игру' }));
    await waitFor(() => expect(TestWebSocket.instances).toHaveLength(1));
    const joinSocket = TestWebSocket.instances[0];
    await waitFor(() => expect(joinSocket.sent[0]?.type).toBe('join'));

    act(() => {
      joinSocket.receive({
        type: 'error',
        request_id: 'join-1',
        sequence: 0,
        payload: { code },
      });
    });

    expect(await screen.findByText(message)).toBeTruthy();
    expect(router.state.location.pathname).toBe('/join/ABC123');
    expect(loadPlayerCredentials('game-1')).toBeNull();
  });

  it('leaves explicitly, removes credentials and cleans up the game socket', async () => {
    expect(
      saveGameCredentials({
        role: 'player',
        gameId: 'game-1',
        code: 'ABC123',
        playerId: 'player-1',
        nickname: 'Alice',
        playerTicket: 'player-ticket',
      }),
    ).toBe(true);
    const router = createMemoryRouter(
      [
        { path: '/join/:code', element: <JoinPage /> },
        { path: '/games/:gameId/player', element: <PlayerGamePage /> },
      ],
      { initialEntries: ['/games/game-1/player'] },
    );
    const user = userEvent.setup();
    render(<RouterProvider router={router} />);

    await waitFor(() => expect(TestWebSocket.instances).toHaveLength(1));
    const gameSocket = TestWebSocket.instances[0];
    await waitFor(() => expect(gameSocket.sent[0]?.type).toBe('player_auth'));
    act(() => {
      gameSocket.receive({ type: 'state', sequence: 1, payload: snapshot('lobby') });
    });
    await user.click(await screen.findByRole('button', { name: 'Покинуть игру' }));

    const leaveMessage = gameSocket.sent[1];
    expect(leaveMessage).toEqual(expect.objectContaining({ type: 'leave' }));
    act(() => {
      gameSocket.receive({
        type: 'player_left',
        sequence: 2,
        payload: { id: 'player-1', nickname: 'Alice', score: 0 },
      });
      gameSocket.receive({
        type: 'command_accepted',
        request_id: leaveMessage.request_id,
        sequence: 2,
        payload: { duplicate: false },
      });
    });

    await waitFor(() => expect(router.state.location.pathname).toBe('/join/ABC123'));
    expect(loadPlayerCredentials('game-1')).toBeNull();
    await waitFor(() => expect(gameSocket.readyState).toBe(TestWebSocket.CLOSED));
    expect((screen.getByLabelText('Код игры') as HTMLInputElement).value).toBe('ABC123');
  });
});
