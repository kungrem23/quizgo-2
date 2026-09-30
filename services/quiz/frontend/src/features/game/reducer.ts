import type {
  GamePlayer,
  GameServerMessage,
  GameStateSnapshot,
  QuestionClosedPayload,
} from './types';
import { serverClockOffset } from './deadline';

export interface GameClientState {
  snapshot: GameStateSnapshot | null;
  lastSequence: number;
  /** Difference between the authoritative server clock and this device clock. */
  serverTimeOffsetMs: number;
  /** null means a reconnect cannot reconstruct how many players already answered. */
  answeredPlayerIds: string[] | null;
  acceptedQuestionIds: number[];
  /** Answers selected in this browser session; game-service does not return them on reconnect. */
  selectedAnswerIds: Record<number, number>;
  /** A question_opened event proves this session observed the whole unanswered question. */
  observedQuestionIds: number[];
  lastQuestionClosed: QuestionClosedPayload | null;
}

export const initialGameState: GameClientState = {
  snapshot: null,
  lastSequence: 0,
  serverTimeOffsetMs: 0,
  answeredPlayerIds: null,
  acceptedQuestionIds: [],
  selectedAnswerIds: {},
  observedQuestionIds: [],
  lastQuestionClosed: null,
};

export type GameStateAction =
  | { type: 'message'; message: GameServerMessage }
  | { type: 'answer_selected'; questionId: number; answerId: number }
  | { type: 'reset' };

function playersWith(current: GamePlayer[], player: GamePlayer): GamePlayer[] {
  const found = current.some((item) => item.id === player.id);
  const players = found
    ? current.map((item) => (item.id === player.id ? player : item))
    : [...current, player];
  return players.sort(
    (left, right) => right.score - left.score || left.nickname.localeCompare(right.nickname),
  );
}

function withoutPlayer(current: GamePlayer[], playerID: string): GamePlayer[] {
  return current.filter((player) => player.id !== playerID);
}

function appendUnique<T>(items: T[], item: T): T[] {
  return items.includes(item) ? items : [...items, item];
}

function fullState(
  state: GameClientState,
  message: Extract<
    GameServerMessage,
    { type: 'state' | 'countdown_started' | 'question_opened' | 'game_finished' }
  >,
): GameClientState {
  if (message.sequence < state.lastSequence) return state;
  const questionChanged =
    message.payload.current_question_index !== state.snapshot?.current_question_index;
  let answeredPlayerIds = state.answeredPlayerIds;
  if (message.type === 'countdown_started' || message.type === 'question_opened') {
    answeredPlayerIds = [];
  } else if (message.type === 'state') {
    answeredPlayerIds =
      message.payload.phase === 'question_open' && questionChanged
        ? null
        : (answeredPlayerIds ?? []);
  }
  const openedQuestionID =
    message.type === 'question_opened' ? message.payload.current_question?.id : undefined;
  return {
    snapshot: message.payload,
    lastSequence: Math.max(state.lastSequence, message.sequence),
    serverTimeOffsetMs: state.serverTimeOffsetMs,
    answeredPlayerIds,
    acceptedQuestionIds: state.acceptedQuestionIds,
    selectedAnswerIds: state.selectedAnswerIds,
    observedQuestionIds:
      openedQuestionID === undefined
        ? state.observedQuestionIds
        : appendUnique(state.observedQuestionIds, openedQuestionID),
    lastQuestionClosed: questionChanged ? null : state.lastQuestionClosed,
  };
}

export function gameReducer(state: GameClientState, action: GameStateAction): GameClientState {
  if (action.type === 'reset') return initialGameState;
  if (action.type === 'answer_selected') {
    if (state.selectedAnswerIds[action.questionId] !== undefined) return state;
    return {
      ...state,
      selectedAnswerIds: {
        ...state.selectedAnswerIds,
        [action.questionId]: action.answerId,
      },
    };
  }
  const message = action.message;
  const nextOffset = serverClockOffset(message.server_time, message.received_at_ms);
  if (nextOffset !== undefined && nextOffset !== state.serverTimeOffsetMs) {
    state = { ...state, serverTimeOffsetMs: nextOffset };
  }

  switch (message.type) {
    case 'state':
    case 'countdown_started':
    case 'question_opened':
    case 'game_finished':
      return fullState(state, message);
    case 'joined':
    case 'authenticated':
    case 'command_accepted':
    case 'error':
      return state;
    default:
      if (message.sequence < state.lastSequence) return state;
  }

  const sequence = Math.max(state.lastSequence, message.sequence);
  switch (message.type) {
    case 'player_joined':
      return state.snapshot
        ? {
            ...state,
            lastSequence: sequence,
            snapshot: {
              ...state.snapshot,
              players: playersWith(state.snapshot.players, message.payload),
            },
          }
        : { ...state, lastSequence: sequence };
    case 'player_left':
    case 'player_removed':
      return state.snapshot
        ? {
            ...state,
            lastSequence: sequence,
            answeredPlayerIds:
              state.answeredPlayerIds?.filter((playerID) => playerID !== message.payload.id) ??
              null,
            snapshot: {
              ...state.snapshot,
              players: withoutPlayer(state.snapshot.players, message.payload.id),
            },
          }
        : { ...state, lastSequence: sequence };
    case 'answer_accepted':
      return {
        ...state,
        lastSequence: sequence,
        acceptedQuestionIds: appendUnique(state.acceptedQuestionIds, message.payload.question_id),
      };
    case 'player_answered':
      return {
        ...state,
        lastSequence: sequence,
        answeredPlayerIds:
          state.answeredPlayerIds === null
            ? null
            : appendUnique(state.answeredPlayerIds, message.payload.player_id),
      };
    case 'question_closed':
      return state.snapshot
        ? {
            ...state,
            lastSequence: sequence,
            snapshot: {
              ...state.snapshot,
              phase: message.payload.phase,
              players: message.payload.players,
              question_closes_at: undefined,
              correct_answer_ids: message.payload.correct_answer_ids,
            },
            lastQuestionClosed: message.payload,
          }
        : { ...state, lastSequence: sequence };
  }
}
