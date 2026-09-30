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
    expect(state.observedQuestionIds).toEqual([1]);
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

  it('applies player joins and explicit leaves to the lobby snapshot', () => {
    let state = gameReducer(initialGameState, { type: 'message', message: snapshot() });
    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'player_joined',
        sequence: 4,
        payload: { id: 'p2', nickname: 'Bob', score: 0 },
      }),
    });
    expect(state.snapshot?.players.map((player) => player.id)).toEqual(['p1', 'p2']);
    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'player_left',
        sequence: 5,
        payload: { id: 'p1', nickname: 'Alice', score: 0 },
      }),
    });
    expect(state.snapshot?.players.map((player) => player.id)).toEqual(['p2']);
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
    expect(state.observedQuestionIds).toEqual([]);
  });

  it('keeps a browser-session answer selection for the post-question result', () => {
    const selected = gameReducer(initialGameState, {
      type: 'answer_selected',
      questionId: 10,
      answerId: 12,
    });
    expect(selected.selectedAnswerIds).toEqual({ 10: 12 });

    const unchanged = gameReducer(selected, {
      type: 'answer_selected',
      questionId: 10,
      answerId: 11,
    });
    expect(unchanged).toBe(selected);
  });

  it('follows countdown, question_open and question_closed server transitions', () => {
    let state = gameReducer(initialGameState, { type: 'message', message: snapshot(1) });
    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'countdown_started',
        sequence: 2,
        payload: {
          game_id: 'game-1',
          code: 'ABC123',
          quiz_title: 'Quiz',
          phase: 'countdown',
          players: [{ id: 'p1', nickname: 'Alice', score: 0 }],
          current_question_index: 0,
          countdown_ends_at: '2026-09-30T12:00:03Z',
        },
      }),
    });
    expect(state.snapshot?.phase).toBe('countdown');
    expect(state.snapshot?.current_question).toBeUndefined();

    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'question_opened',
        sequence: 3,
        payload: {
          game_id: 'game-1',
          code: 'ABC123',
          quiz_title: 'Quiz',
          phase: 'question_open',
          players: [{ id: 'p1', nickname: 'Alice', score: 0 }],
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
          question_closes_at: '2026-09-30T12:00:23Z',
        },
      }),
    });
    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'player_answered',
        sequence: 4,
        payload: { player_id: 'p1' },
      }),
    });
    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'answer_accepted',
        sequence: 4,
        payload: { question_id: 10 },
      }),
    });
    expect(state.snapshot?.phase).toBe('question_open');
    expect(state.answeredPlayerIds).toEqual(['p1']);
    expect(state.acceptedQuestionIds).toEqual([10]);

    state = gameReducer(state, {
      type: 'message',
      message: parseServerMessage({
        type: 'question_closed',
        sequence: 5,
        payload: {
          phase: 'scoreboard',
          question_id: 10,
          correct_answer_ids: [11],
          players: [{ id: 'p1', nickname: 'Alice', score: 900 }],
        },
      }),
    });
    expect(state.snapshot?.phase).toBe('scoreboard');
    expect(state.snapshot?.question_closes_at).toBeUndefined();
    expect(state.snapshot?.current_question?.id).toBe(10);
    expect(state.snapshot?.correct_answer_ids).toEqual([11]);
    expect(state.lastQuestionClosed?.question_id).toBe(10);
    expect(state.observedQuestionIds).toEqual([10]);
  });
});
