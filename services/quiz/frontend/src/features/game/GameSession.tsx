import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import type { GameCredentials } from './credentials';
import { gameReducer, initialGameState, type GameClientState } from './reducer';
import { GameSocket, type GameSocketStatus, type PendingGameCommand } from './socket';
import type { GameCommand, GameErrorCode, GameServerMessage } from './types';

interface GameSessionValue {
  game: GameClientState;
  status: GameSocketStatus;
  error: GameErrorCode | 'connection_failed' | 'protocol_error' | null;
  sendCommand: (command: GameCommand, requestId?: string) => PendingGameCommand;
}

const GameSessionContext = createContext<GameSessionValue | null>(null);

export function GameSessionProvider({
  credentials,
  children,
}: {
  credentials: GameCredentials;
  children: ReactNode;
}) {
  const [game, dispatch] = useReducer(gameReducer, initialGameState);
  const [status, setStatus] = useState<GameSocketStatus>({ state: 'idle' });
  const [error, setError] = useState<GameSessionValue['error']>(null);
  const socketRef = useRef<GameSocket | null>(null);

  useEffect(() => {
    let active = true;
    const socket = new GameSocket({
      onMessage: (message: GameServerMessage) => {
        if (!active) return;
        if (message.type === 'error') setError(message.payload.code);
        dispatch({ type: 'message', message });
      },
      onStatus: (next) => {
        if (active) setStatus(next);
      },
      onProtocolError: () => {
        if (active) setError('protocol_error');
      },
    });
    socketRef.current = socket;
    void socket
      .connect({ gameId: credentials.gameId })
      .then(() => {
        if (!active) return;
        if (credentials.role === 'host') {
          socket.authenticateHost(credentials.gameId, credentials.hostTicket);
        } else {
          socket.authenticatePlayer(
            credentials.gameId,
            credentials.playerId,
            credentials.playerTicket,
          );
        }
      })
      .catch(() => {
        if (active) setError('connection_failed');
      });
    return () => {
      active = false;
      socketRef.current = null;
      socket.disconnect();
    };
  }, [credentials]);

  const sendCommand = useCallback((command: GameCommand, requestId?: string) => {
    const socket = socketRef.current;
    if (!socket) throw new Error('game socket is unavailable');
    return socket.sendCommand(command, requestId);
  }, []);

  const value = useMemo(
    () => ({ game, status, error, sendCommand }),
    [error, game, sendCommand, status],
  );
  return <GameSessionContext.Provider value={value}>{children}</GameSessionContext.Provider>;
}

export function useGameSession(): GameSessionValue {
  const value = useContext(GameSessionContext);
  if (!value) throw new Error('GameSessionProvider missing');
  return value;
}
