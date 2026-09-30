import { describe, expect, it, vi } from 'vitest';
import { HostCredentialStorageError, launchHostGame } from '../../../src/features/game/launch';

const created = {
  id: 'game-1',
  code: 'ABC123',
  phase: 'lobby' as const,
  quiz_revision: 4,
  host_ticket: 'host-secret',
};

describe('host game launch', () => {
  it('persists the host ticket before returning the created game', async () => {
    const createGame = vi.fn(async () => created);
    const saveCredentials = vi.fn(() => true);
    await expect(launchHostGame(42, { createGame, saveCredentials })).resolves.toEqual(created);
    expect(createGame).toHaveBeenCalledWith(42);
    expect(saveCredentials).toHaveBeenCalledWith({
      role: 'host',
      gameId: 'game-1',
      code: 'ABC123',
      hostTicket: 'host-secret',
      quizId: 42,
    });
  });

  it('does not allow navigation to continue when credentials cannot be saved', async () => {
    await expect(
      launchHostGame(42, {
        createGame: vi.fn(async () => created),
        saveCredentials: vi.fn(() => false),
      }),
    ).rejects.toBeInstanceOf(HostCredentialStorageError);
  });
});
