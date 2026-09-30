import { describe, expect, it } from 'vitest';
import { GameProtocolError, parseServerMessage } from '../../../src/features/game/wire';

const state = {
  game_id: 'game-1',
  code: 'ABC123',
  quiz_title: 'Физика',
  total_questions: 3,
  phase: 'question_open',
  players: [{ id: 'player-1', nickname: 'Alice', score: 500 }],
  current_question_index: 0,
  current_question: {
    id: 10,
    text: 'Вопрос?',
    image_id: 'image-1',
    time_limit_seconds: 20,
    answers: [
      { id: 11, text: 'Да' },
      { id: 12, text: 'Нет' },
    ],
  },
  question_closes_at: '2026-09-30T12:00:00Z',
};

describe('game wire parser', () => {
  it('parses the public state without exposing correct answer flags', () => {
    const message = parseServerMessage(
      JSON.stringify({
        type: 'state',
        sequence: 4,
        server_time: '2026-09-30T11:59:59.123456789Z',
        payload: state,
      }),
      1234,
    );
    expect(message.type).toBe('state');
    if (message.type !== 'state') throw new Error('unexpected message');
    expect(message.payload.current_question?.answers).toEqual([
      { id: 11, text: 'Да' },
      { id: 12, text: 'Нет' },
    ]);
    expect(message.payload.question_closes_at).toBe('2026-09-30T12:00:00Z');
    expect(message.payload.total_questions).toBe(3);
    expect(message.server_time).toBe('2026-09-30T11:59:59.123456789Z');
    expect(message.received_at_ms).toBe(1234);
  });

  it('parses question_closed separately from a full state snapshot', () => {
    const message = parseServerMessage({
      type: 'question_closed',
      sequence: 7,
      payload: {
        phase: 'scoreboard',
        question_id: 10,
        correct_answer_ids: [11],
        players: state.players,
      },
    });
    expect(message).toMatchObject({
      type: 'question_closed',
      sequence: 7,
      payload: { phase: 'scoreboard', correct_answer_ids: [11] },
    });
  });

  it('rejects unknown events and malformed payloads', () => {
    expect(() => parseServerMessage({ type: 'timer_tick', sequence: 1, payload: {} })).toThrow(
      GameProtocolError,
    );
    expect(() =>
      parseServerMessage({ type: 'player_answered', sequence: 2, payload: { player_id: 42 } }),
    ).toThrow('player_answered.player_id must be a string');
  });
});
