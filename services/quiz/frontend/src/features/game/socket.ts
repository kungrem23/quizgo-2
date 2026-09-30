import { parseServerMessage, GameProtocolError } from './wire';
import type {
  ClientEnvelope,
  CommandAcceptedPayload,
  GameCommand,
  GameErrorCode,
  GameServerMessage,
} from './types';

export type GameSocketStatus =
  | { state: 'idle' }
  | { state: 'connecting' }
  | { state: 'open' }
  | { state: 'closed'; code: number; reason: string };

export interface GameSocketOptions {
  onMessage: (message: GameServerMessage) => void;
  onStatus?: (status: GameSocketStatus) => void;
  onProtocolError?: (error: GameProtocolError) => void;
}

export interface PendingGameCommand {
  requestId: string;
  acknowledged: Promise<CommandAcceptedPayload>;
}

export class GameCommandError extends Error {
  constructor(
    public readonly code: GameErrorCode,
    public readonly requestId: string,
  ) {
    super(`game command failed: ${code}`);
  }
}

interface PendingRequest {
  resolve: (value: CommandAcceptedPayload) => void;
  reject: (reason: Error) => void;
}

function requestID(): string {
  return (
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
  );
}

function websocketURL(hint?: { code: string } | { gameId: string }): string {
  const url = new URL('/ws', window.location.origin);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  if (hint && 'code' in hint) url.searchParams.set('code', hint.code);
  if (hint && 'gameId' in hint) url.searchParams.set('game_id', hint.gameId);
  return url.toString();
}

export class GameSocket {
  private connection: WebSocket | null = null;
  private readonly pending = new Map<string, PendingRequest>();

  constructor(private readonly options: GameSocketOptions) {}

  connect(hint?: { code: string } | { gameId: string }): Promise<void> {
    this.disconnect();
    this.options.onStatus?.({ state: 'connecting' });
    return new Promise((resolve, reject) => {
      let opened = false;
      const connection = new WebSocket(websocketURL(hint));
      this.connection = connection;
      connection.addEventListener('open', () => {
        if (this.connection !== connection) return;
        opened = true;
        this.options.onStatus?.({ state: 'open' });
        resolve();
      });
      connection.addEventListener('message', (event) => {
        if (this.connection !== connection || typeof event.data !== 'string') return;
        try {
          this.receive(parseServerMessage(event.data));
        } catch (error) {
          if (error instanceof GameProtocolError) this.options.onProtocolError?.(error);
        }
      });
      connection.addEventListener('error', () => {
        if (!opened) reject(new Error('websocket connection failed'));
      });
      connection.addEventListener('close', (event) => {
        if (this.connection !== connection) return;
        this.connection = null;
        const error = new Error(`websocket closed (${event.code})`);
        this.rejectPending(error);
        this.options.onStatus?.({ state: 'closed', code: event.code, reason: event.reason });
        if (!opened) reject(error);
      });
    });
  }

  disconnect(code = 1000, reason = 'client disconnect'): void {
    const connection = this.connection;
    this.connection = null;
    if (connection && connection.readyState < WebSocket.CLOSING) connection.close(code, reason);
    this.rejectPending(new Error('websocket disconnected'));
    this.options.onStatus?.({ state: 'idle' });
  }

  join(code: string, nickname: string, id = requestID()): string {
    this.send({ type: 'join', request_id: id, payload: { code, nickname } });
    return id;
  }

  authenticateHost(gameId: string, ticket: string, id = requestID()): string {
    this.send({ type: 'host_auth', request_id: id, payload: { game_id: gameId, ticket } });
    return id;
  }

  authenticatePlayer(
    gameId: string,
    participantId: string,
    ticket: string,
    id = requestID(),
  ): string {
    this.send({
      type: 'player_auth',
      request_id: id,
      payload: { game_id: gameId, participant_id: participantId, ticket },
    });
    return id;
  }

  sendCommand(command: GameCommand, id = requestID()): PendingGameCommand {
    if (!id || id.length > 128) throw new Error('request_id must contain at most 128 characters');
    if (this.pending.has(id)) throw new Error(`request_id is already pending: ${id}`);
    let resolve!: (value: CommandAcceptedPayload) => void;
    let reject!: (reason: Error) => void;
    const acknowledged = new Promise<CommandAcceptedPayload>((onResolve, onReject) => {
      resolve = onResolve;
      reject = onReject;
    });
    this.pending.set(id, { resolve, reject });
    try {
      const envelope: ClientEnvelope = {
        type: command.type,
        request_id: id,
        ...('payload' in command ? { payload: command.payload } : {}),
      };
      this.send(envelope);
    } catch (error) {
      this.pending.delete(id);
      reject(error instanceof Error ? error : new Error('could not send game command'));
    }
    return { requestId: id, acknowledged };
  }

  private send(message: ClientEnvelope): void {
    if (!this.connection || this.connection.readyState !== WebSocket.OPEN) {
      throw new Error('websocket is not open');
    }
    this.connection.send(JSON.stringify(message));
  }

  private receive(message: GameServerMessage): void {
    if (message.type === 'command_accepted' && message.request_id) {
      const pending = this.pending.get(message.request_id);
      if (pending) {
        this.pending.delete(message.request_id);
        pending.resolve(message.payload);
      }
    } else if (message.type === 'error' && message.request_id) {
      const pending = this.pending.get(message.request_id);
      if (pending) {
        this.pending.delete(message.request_id);
        pending.reject(new GameCommandError(message.payload.code, message.request_id));
      }
    }
    this.options.onMessage(message);
  }

  private rejectPending(error: Error): void {
    for (const pending of this.pending.values()) pending.reject(error);
    this.pending.clear();
  }
}
