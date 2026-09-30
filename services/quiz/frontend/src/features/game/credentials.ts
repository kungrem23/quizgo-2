export interface HostGameCredentials {
  role: 'host';
  gameId: string;
  code: string;
  hostTicket: string;
  quizId?: number;
}

export interface PlayerGameCredentials {
  role: 'player';
  gameId: string;
  code: string;
  playerId: string;
  nickname: string;
  playerTicket: string;
}

export type GameCredentials = HostGameCredentials | PlayerGameCredentials;

type GameCredentialRole = GameCredentials['role'];
type StorageAccess = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

function nonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.trim() !== '';
}

function key(role: GameCredentialRole, gameId: string): string {
  return `quizgo:game:${role}:${gameId}`;
}

function browserStorage(): StorageAccess | null {
  try {
    return globalThis.sessionStorage;
  } catch {
    return null;
  }
}

function validBase(value: unknown, role: GameCredentialRole, gameId: string): boolean {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false;
  const item = value as Record<string, unknown>;
  return item.role === role && item.gameId === gameId && nonEmptyString(item.code);
}

function parseHost(value: unknown, gameId: string): HostGameCredentials | null {
  if (!validBase(value, 'host', gameId)) return null;
  const item = value as Record<string, unknown>;
  if (
    !nonEmptyString(item.hostTicket) ||
    (item.quizId !== undefined &&
      (typeof item.quizId !== 'number' || !Number.isSafeInteger(item.quizId) || item.quizId < 1))
  ) {
    return null;
  }
  return {
    role: 'host',
    gameId,
    code: item.code as string,
    hostTicket: item.hostTicket,
    quizId: item.quizId as number | undefined,
  };
}

function parsePlayer(value: unknown, gameId: string): PlayerGameCredentials | null {
  if (!validBase(value, 'player', gameId)) return null;
  const item = value as Record<string, unknown>;
  if (
    !nonEmptyString(item.playerId) ||
    !nonEmptyString(item.nickname) ||
    !nonEmptyString(item.playerTicket)
  ) {
    return null;
  }
  return {
    role: 'player',
    gameId,
    code: item.code as string,
    playerId: item.playerId,
    nickname: item.nickname,
    playerTicket: item.playerTicket,
  };
}

export function saveGameCredentials(
  value: GameCredentials,
  storage: StorageAccess | null = browserStorage(),
): boolean {
  if (!storage) return false;
  try {
    storage.setItem(key(value.role, value.gameId), JSON.stringify(value));
    return true;
  } catch {
    return false;
  }
}

export function loadHostCredentials(
  gameId: string,
  storage: StorageAccess | null = browserStorage(),
): HostGameCredentials | null {
  if (!storage || !gameId) return null;
  try {
    return parseHost(JSON.parse(storage.getItem(key('host', gameId)) || 'null'), gameId);
  } catch {
    return null;
  }
}

export function loadPlayerCredentials(
  gameId: string,
  storage: StorageAccess | null = browserStorage(),
): PlayerGameCredentials | null {
  if (!storage || !gameId) return null;
  try {
    return parsePlayer(JSON.parse(storage.getItem(key('player', gameId)) || 'null'), gameId);
  } catch {
    return null;
  }
}

export function removeGameCredentials(
  role: GameCredentialRole,
  gameId: string,
  storage: StorageAccess | null = browserStorage(),
): void {
  if (!storage) return;
  try {
    storage.removeItem(key(role, gameId));
  } catch {
    /* The active in-memory session can finish when browser storage is unavailable. */
  }
}
