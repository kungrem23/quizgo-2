import { describe, expect, it } from 'vitest';
import {
  gameReducer,
  initialGameState,
  type GameClientState,
} from '../../../src/features/game/reducer';
import { parseServerMessage } from '../../../src/features/game/wire';

function snapshot(sequence = 3) {
  return parseServerMessage({
    type: 'state',
    sequence,
    payload: {
      game_id: 'game-1',
      code: 'ABC123',
      quiz_title: 'Quiz',
      phase: 'lobby',
      players: [{ id: 'p1', nickname: 'Alice', score: 0 }],
      current_question_index: -1,
    },
  });
}

describe('authoritative game reducer', () => {
  it('upserts events with the same sequence as the state snapshot', () => {
    let state = gameReducer(initialGameState, { type: 'message', message: snapshot() });
    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'player_joined',
        sequence: 3,
        payload: { id: 'p1', nickname: 'Alice', score: 0 },
      }),
    });
    expect(state.snapshot?.players).toHaveLength(1);
    expect(state.lastSequence).toBe(3);
  });

  it('tracks unique players who answered and resets them for a new question', () => {
    let state: GameClientState = gameReducer(initialGameState, {
      type: 'message',
      message: snapshot(),
    });
    const answered = parseServerMessage({
      type: 'player_answered',
      sequence: 4,
      payload: { player_id: 'p1' },
    });
    state = gameReducer(state, { type: 'message', message: answered });
    state = gameReducer(state, { type: 'message', message: answered });
    expect(state.answeredPlayerIds).toEqual(['p1']);

    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'question_opened',
        sequence: 5,
        payload: {
          game_id: 'game-1',
          code: 'ABC123',
          quiz_title: 'Quiz',
          phase: 'question_open',
          players: [{ id: 'p1', nickname: 'Alice', score: 0 }],
          current_question_index: 0,
          current_question: {
            id: 1,
            text: 'Question?',
            time_limit_seconds: 20,
            answers: [
              { id: 1, text: 'A' },
              { id: 2, text: 'B' },
            ],
          },
          question_closes_at: '2026-09-30T12:00:00Z',
        },
      }),
    });
    expect(state.answeredPlayerIds).toEqual([]);
  });

  it('ignores stale mutations after a newer state snapshot', () => {
    const state = gameReducer(initialGameState, { type: 'message', message: snapshot(8) });
    const stale = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'player_joined',
        sequence: 7,
        payload: { id: 'p2', nickname: 'Bob', score: 0 },
      }),
    });
    expect(stale).toBe(state);
  });

  it('does not invent an answered count when reconnect state is already in a question', () => {
    const state = gameReducer(initialGameState, {
      type: 'message',
      message: parseServerMessage({
        type: 'state',
        sequence: 9,
        payload: {
          game_id: 'game-1',
          code: 'ABC123',
          quiz_title: 'Quiz',
          phase: 'question_open',
          players: [{ id: 'p1', nickname: 'Alice', score: 0 }],
          current_question_index: 1,
          current_question: {
            id: 2,
            text: 'Question?',
            time_limit_seconds: 20,
            answers: [
              { id: 3, text: 'A' },
              { id: 4, text: 'B' },
            ],
          },
          question_closes_at: '2026-09-30T12:00:00Z',
        },
      }),
    });
    expect(state.answeredPlayerIds).toBeNull();
  });
});
