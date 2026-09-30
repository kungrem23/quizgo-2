// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { HostLobby } from '../../../src/features/game/HostLobby';

const mocks = vi.hoisted(() => ({
  sendCommand: vi.fn(),
  session: {
    game: {
      snapshot: {
        game_id: 'game-1',
        code: 'ABC123',
        quiz_title: 'Физика',
        phase: 'lobby',
        players: [] as Array<{ id: string; nickname: string; score: number }>,
        current_question_index: -1,
      },
      lastSequence: 1,
      answeredPlayerIds: [] as string[],
      acceptedQuestionIds: [] as number[],
      lastQuestionClosed: null,
    },
    status: { state: 'open' },
    error: null,
  },
}));

vi.mock('../../../src/features/game/GameSession', () => ({
  useGameSession: () => ({ ...mocks.session, sendCommand: mocks.sendCommand }),
}));

vi.mock('../../../src/features/game/JoinQRCode', () => ({
  JoinQRCode: ({ value }: { value: string }) => <div data-testid="qr">{value}</div>,
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

describe('host lobby', () => {
  afterEach(cleanup);

  beforeEach(() => {
    mocks.sendCommand.mockReset();
    mocks.session.game.snapshot.players = [];
  });

  it('renders the empty lobby with start disabled', () => {
    render(<HostLobby />);
    expect(screen.getByText('Ждём участников…')).toBeTruthy();
    expect(
      (screen.getByRole('button', { name: 'Начать игру' }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect(screen.getByTestId('qr').textContent).toContain('/join/ABC123');
  });

  it('locks lobby commands while start is pending', async () => {
    mocks.session.game.snapshot.players = [{ id: 'p1', nickname: 'Alice', score: 0 }];
    const acknowledgement = deferred<{ duplicate: boolean }>();
    mocks.sendCommand.mockReturnValue({
      requestId: 'start-1',
      acknowledged: acknowledgement.promise,
    });
    render(<HostLobby />);

    fireEvent.click(screen.getByRole('button', { name: 'Начать игру' }));
    expect(mocks.sendCommand).toHaveBeenCalledWith({ type: 'start' });
    expect(
      (screen.getByRole('button', { name: 'Начать игру' }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect(
      (screen.getByRole('button', { name: 'Удалить игрока Alice' }) as HTMLButtonElement).disabled,
    ).toBe(true);

    acknowledgement.resolve({ duplicate: false });
    await waitFor(() =>
      expect(
        (screen.getByRole('button', { name: 'Начать игру' }) as HTMLButtonElement).disabled,
      ).toBe(false),
    );
  });

  it('sends remove_player and disables other controls until acknowledgement', async () => {
    mocks.session.game.snapshot.players = [{ id: 'p1', nickname: 'Alice', score: 0 }];
    const acknowledgement = deferred<{ duplicate: boolean }>();
    mocks.sendCommand.mockReturnValue({
      requestId: 'remove-1',
      acknowledged: acknowledgement.promise,
    });
    render(<HostLobby />);

    fireEvent.click(screen.getByRole('button', { name: 'Удалить игрока Alice' }));
    expect(mocks.sendCommand).toHaveBeenCalledWith({
      type: 'remove_player',
      payload: { player_id: 'p1' },
    });
    expect(
      (screen.getByRole('button', { name: 'Начать игру' }) as HTMLButtonElement).disabled,
    ).toBe(true);

    acknowledgement.resolve({ duplicate: false });
    await waitFor(() =>
      expect(
        (screen.getByRole('button', { name: 'Начать игру' }) as HTMLButtonElement).disabled,
      ).toBe(false),
    );
  });
});
