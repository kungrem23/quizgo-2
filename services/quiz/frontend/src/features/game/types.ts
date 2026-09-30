export type GameRole = 'host' | 'player';

export type GamePhase =
  'lobby' | 'countdown' | 'question_open' | 'question_closed' | 'scoreboard' | 'finished';

export interface GamePlayer {
  id: string;
  nickname: string;
  score: number;
}

export interface GameAnswer {
  id: number;
  text: string;
}

export interface GameQuestion {
  id: number;
  text: string;
  image_id?: string;
  time_limit_seconds: number;
  answers: GameAnswer[];
}

/** Public state returned by application.StateView in game-service. */
export interface GameStateSnapshot {
  game_id: string;
  code: string;
  quiz_title: string;
  phase: GamePhase;
  players: GamePlayer[];
  current_question_index: number;
  current_question?: GameQuestion;
  countdown_ends_at?: string;
  question_closes_at?: string;
  correct_answer_ids?: number[];
}

export interface JoinedPayload {
  game_id: string;
  player_id: string;
  nickname: string;
  ticket: string;
}

export interface AuthenticatedPayload {
  game_id: string;
  participant_id: string;
  role: GameRole;
}

export interface CommandAcceptedPayload {
  duplicate: boolean;
}

export type GameErrorCode =
  | 'unauthorized'
  | 'game_not_found'
  | 'nickname_taken'
  | 'invalid_payload'
  | 'unknown_message_type'
  | 'rate_limited'
  | 'room_owner_changed'
  | 'invalid_phase'
  | 'already_answered'
  | 'question_closed'
  | 'room_not_local'
  | 'internal_error';

export interface ErrorPayload {
  code: GameErrorCode;
}

export interface QuestionClosedPayload {
  phase: GamePhase;
  question_id: number;
  correct_answer_ids: number[];
  players: GamePlayer[];
}

interface ServerEnvelope<Type extends string, Payload> {
  type: Type;
  request_id?: string;
  sequence: number;
  payload: Payload;
}

export type GameServerMessage =
  | ServerEnvelope<'joined', JoinedPayload>
  | ServerEnvelope<'authenticated', AuthenticatedPayload>
  | ServerEnvelope<'command_accepted', CommandAcceptedPayload>
  | ServerEnvelope<'error', ErrorPayload>
  | ServerEnvelope<'state', GameStateSnapshot>
  | ServerEnvelope<'player_joined', GamePlayer>
  | ServerEnvelope<'countdown_started', GameStateSnapshot>
  | ServerEnvelope<'question_opened', GameStateSnapshot>
  | ServerEnvelope<'answer_accepted', { question_id: number }>
  | ServerEnvelope<'player_answered', { player_id: string }>
  | ServerEnvelope<'question_closed', QuestionClosedPayload>
  | ServerEnvelope<'player_left', GamePlayer>
  | ServerEnvelope<'player_removed', GamePlayer>
  | ServerEnvelope<'game_finished', GameStateSnapshot>;

export type GameHandshake =
  | { type: 'join'; payload: { code: string; nickname: string } }
  | { type: 'host_auth'; payload: { game_id: string; ticket: string } }
  | {
      type: 'player_auth';
      payload: { game_id: string; participant_id: string; ticket: string };
    };

export type GameCommand =
  | { type: 'start' }
  | { type: 'next' }
  | { type: 'finish' }
  | { type: 'leave' }
  | { type: 'remove_player'; payload: { player_id: string } }
  | { type: 'answer'; payload: { answer_id: number } };

export interface ClientEnvelope {
  type: GameHandshake['type'] | GameCommand['type'];
  request_id: string;
  payload?: Record<string, string | number>;
}
