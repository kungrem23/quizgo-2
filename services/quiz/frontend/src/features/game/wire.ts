import type {
  AuthenticatedPayload,
  CommandAcceptedPayload,
  ErrorPayload,
  GameAnswer,
  GameErrorCode,
  GamePhase,
  GamePlayer,
  GameQuestion,
  GameRole,
  GameServerMessage,
  GameStateSnapshot,
  JoinedPayload,
  QuestionClosedPayload,
} from './types';

const phases = new Set<GamePhase>([
  'lobby',
  'countdown',
  'question_open',
  'question_closed',
  'scoreboard',
  'finished',
]);

const roles = new Set<GameRole>(['host', 'player']);

const errorCodes = new Set<GameErrorCode>([
  'unauthorized',
  'game_not_found',
  'nickname_taken',
  'invalid_payload',
  'unknown_message_type',
  'rate_limited',
  'room_owner_changed',
  'invalid_phase',
  'already_answered',
  'question_closed',
  'room_not_local',
  'internal_error',
]);

export class GameProtocolError extends Error {}

function record(value: unknown, name: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new GameProtocolError(`${name} must be an object`);
  }
  return value as Record<string, unknown>;
}

function string(value: unknown, name: string): string {
  if (typeof value !== 'string') throw new GameProtocolError(`${name} must be a string`);
  return value;
}

function integer(value: unknown, name: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) {
    throw new GameProtocolError(`${name} must be an integer`);
  }
  return value;
}

function positiveInteger(value: unknown, name: string): number {
  const parsed = integer(value, name);
  if (parsed < 1) throw new GameProtocolError(`${name} must be positive`);
  return parsed;
}

function array<T>(value: unknown, name: string, parse: (item: unknown) => T): T[] {
  if (!Array.isArray(value)) throw new GameProtocolError(`${name} must be an array`);
  return value.map(parse);
}

function optionalString(value: unknown, name: string): string | undefined {
  return value === undefined ? undefined : string(value, name);
}

function parsePhase(value: unknown): GamePhase {
  const parsed = string(value, 'phase') as GamePhase;
  if (!phases.has(parsed)) throw new GameProtocolError(`unknown game phase: ${parsed}`);
  return parsed;
}

function parseRole(value: unknown): GameRole {
  const parsed = string(value, 'role') as GameRole;
  if (!roles.has(parsed)) throw new GameProtocolError(`unknown game role: ${parsed}`);
  return parsed;
}

function parsePlayer(value: unknown): GamePlayer {
  const item = record(value, 'player');
  return {
    id: string(item.id, 'player.id'),
    nickname: string(item.nickname, 'player.nickname'),
    score: integer(item.score, 'player.score'),
  };
}

function parseAnswer(value: unknown): GameAnswer {
  const item = record(value, 'answer');
  return { id: positiveInteger(item.id, 'answer.id'), text: string(item.text, 'answer.text') };
}

function parseQuestion(value: unknown): GameQuestion {
  const item = record(value, 'question');
  return {
    id: positiveInteger(item.id, 'question.id'),
    text: string(item.text, 'question.text'),
    image_id: optionalString(item.image_id, 'question.image_id'),
    time_limit_seconds: positiveInteger(item.time_limit_seconds, 'question.time_limit_seconds'),
    answers: array(item.answers, 'question.answers', parseAnswer),
  };
}

export function parseGameState(value: unknown): GameStateSnapshot {
  const item = record(value, 'state');
  const currentQuestion = item.current_question;
  const correctAnswerIDs = item.correct_answer_ids;
  return {
    game_id: string(item.game_id, 'state.game_id'),
    code: string(item.code, 'state.code'),
    quiz_title: string(item.quiz_title, 'state.quiz_title'),
    phase: parsePhase(item.phase),
    players: array(item.players, 'state.players', parsePlayer),
    current_question_index: integer(item.current_question_index, 'state.current_question_index'),
    current_question: currentQuestion === undefined ? undefined : parseQuestion(currentQuestion),
    countdown_ends_at: optionalString(item.countdown_ends_at, 'state.countdown_ends_at'),
    question_closes_at: optionalString(item.question_closes_at, 'state.question_closes_at'),
    correct_answer_ids:
      correctAnswerIDs === undefined
        ? undefined
        : array(correctAnswerIDs, 'state.correct_answer_ids', (id) =>
            positiveInteger(id, 'correct_answer_id'),
          ),
  };
}

function parseJoined(value: unknown): JoinedPayload {
  const item = record(value, 'joined payload');
  return {
    game_id: string(item.game_id, 'joined.game_id'),
    player_id: string(item.player_id, 'joined.player_id'),
    nickname: string(item.nickname, 'joined.nickname'),
    ticket: string(item.ticket, 'joined.ticket'),
  };
}

function parseAuthenticated(value: unknown): AuthenticatedPayload {
  const item = record(value, 'authenticated payload');
  return {
    game_id: string(item.game_id, 'authenticated.game_id'),
    participant_id: string(item.participant_id, 'authenticated.participant_id'),
    role: parseRole(item.role),
  };
}

function parseCommandAccepted(value: unknown): CommandAcceptedPayload {
  const item = record(value, 'command_accepted payload');
  if (typeof item.duplicate !== 'boolean') {
    throw new GameProtocolError('command_accepted.duplicate must be a boolean');
  }
  return { duplicate: item.duplicate };
}

function parseError(value: unknown): ErrorPayload {
  const item = record(value, 'error payload');
  const code = string(item.code, 'error.code') as GameErrorCode;
  if (!errorCodes.has(code)) throw new GameProtocolError(`unknown game error code: ${code}`);
  return { code };
}

function parseQuestionClosed(value: unknown): QuestionClosedPayload {
  const item = record(value, 'question_closed payload');
  return {
    phase: parsePhase(item.phase),
    question_id: positiveInteger(item.question_id, 'question_closed.question_id'),
    correct_answer_ids: array(item.correct_answer_ids, 'question_closed.correct_answer_ids', (id) =>
      positiveInteger(id, 'correct_answer_id'),
    ),
    players: array(item.players, 'question_closed.players', parsePlayer),
  };
}

export function parseServerMessage(raw: string | unknown): GameServerMessage {
  let decoded: unknown = raw;
  if (typeof raw === 'string') {
    try {
      decoded = JSON.parse(raw) as unknown;
    } catch {
      throw new GameProtocolError('server message is not valid JSON');
    }
  }
  const message = record(decoded, 'server message');
  const type = string(message.type, 'message.type');
  const sequence = integer(message.sequence, 'message.sequence');
  if (sequence < 0) throw new GameProtocolError('message.sequence must not be negative');
  const requestID = optionalString(message.request_id, 'message.request_id');
  const envelope = <Type extends GameServerMessage['type'], Payload>(
    envelopeType: Type,
    payload: Payload,
  ) => ({ type: envelopeType, request_id: requestID, sequence, payload });

  switch (type) {
    case 'joined':
      return envelope(type, parseJoined(message.payload));
    case 'authenticated':
      return envelope(type, parseAuthenticated(message.payload));
    case 'command_accepted':
      return envelope(type, parseCommandAccepted(message.payload));
    case 'error':
      return envelope(type, parseError(message.payload));
    case 'state':
    case 'countdown_started':
    case 'question_opened':
    case 'game_finished':
      return envelope(type, parseGameState(message.payload));
    case 'player_joined':
    case 'player_left':
    case 'player_removed':
      return envelope(type, parsePlayer(message.payload));
    case 'answer_accepted': {
      const payload = record(message.payload, 'answer_accepted payload');
      return envelope(type, {
        question_id: positiveInteger(payload.question_id, 'answer_accepted.question_id'),
      });
    }
    case 'player_answered': {
      const payload = record(message.payload, 'player_answered payload');
      return envelope(type, {
        player_id: string(payload.player_id, 'player_answered.player_id'),
      });
    }
    case 'question_closed':
      return envelope(type, parseQuestionClosed(message.payload));
    default:
      throw new GameProtocolError(`unknown server message type: ${type}`);
  }
}
