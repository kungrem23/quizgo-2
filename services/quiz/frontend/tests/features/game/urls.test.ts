// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { gameJoinURL, quizURL } from '../../../src/features/game/urls';

describe('public game URLs', () => {
  afterEach(() => vi.unstubAllEnvs());

  it('uses the configured game domain for player join links', () => {
    vi.stubEnv('VITE_GAME_ORIGIN', 'https://game.example.com');

    expect(gameJoinURL('ABC123')).toBe('https://game.example.com/join/ABC123');
  });

  it('uses the configured quiz domain when leaving the player site', () => {
    vi.stubEnv('VITE_QUIZ_ORIGIN', 'https://quiz.example.com');

    expect(quizURL('/quizzes')).toBe('https://quiz.example.com/quizzes');
  });
});
