// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from 'vitest';
import {
  loadHostCredentials,
  loadPlayerCredentials,
  removeGameCredentials,
  saveGameCredentials,
} from '../../../src/features/game/credentials';

describe('game credential storage', () => {
  beforeEach(() => sessionStorage.clear());

  it('keeps host and player tickets in role-scoped session storage', () => {
    expect(
      saveGameCredentials({
        role: 'host',
        gameId: 'game-1',
        code: 'ABC123',
        hostTicket: 'host-secret',
        quizId: 42,
      }),
    ).toBe(true);
    expect(
      saveGameCredentials({
        role: 'player',
        gameId: 'game-1',
        code: 'ABC123',
        playerId: 'player-1',
        nickname: 'Alice',
        playerTicket: 'player-secret',
      }),
    ).toBe(true);
    expect(loadHostCredentials('game-1')).toMatchObject({ hostTicket: 'host-secret', quizId: 42 });
    expect(loadPlayerCredentials('game-1')).toMatchObject({
      playerId: 'player-1',
      playerTicket: 'player-secret',
    });
  });

  it('rejects malformed or mismatched credentials', () => {
    sessionStorage.setItem(
      'quizgo:game:host:game-1',
      JSON.stringify({ role: 'host', gameId: 'another-game', code: 'ABC123' }),
    );
    expect(loadHostCredentials('game-1')).toBeNull();
    sessionStorage.setItem(
      'quizgo:game:player:game-1',
      JSON.stringify({
        role: 'player',
        gameId: 'game-1',
        code: 'ABC123',
        playerId: 'player-1',
        nickname: 'Alice',
        playerTicket: '',
      }),
    );
    expect(loadPlayerCredentials('game-1')).toBeNull();
  });

  it('removes only the selected role credentials', () => {
    saveGameCredentials({
      role: 'host',
      gameId: 'game-1',
      code: 'ABC123',
      hostTicket: 'secret',
    });
    removeGameCredentials('host', 'game-1');
    expect(loadHostCredentials('game-1')).toBeNull();
  });
});
