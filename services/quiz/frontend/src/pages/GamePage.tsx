import { useMemo, type ReactNode } from 'react';
import { Link, useParams } from 'react-router';
import {
  loadHostCredentials,
  loadPlayerCredentials,
  type GameCredentials,
} from '../features/game/credentials';
import { GameSessionProvider, useGameSession } from '../features/game/GameSession';
import type { GameRole } from '../features/game/types';
import { ErrorBox, Loading } from '../shared/ui/ui';

function MissingCredentials({ role }: { role: GameRole }) {
  return (
    <main className="game-foundation-page">
      <div className="game-foundation-card">
        <ErrorBox
          message={
            role === 'host'
              ? 'В этой вкладке нет host ticket для игры.'
              : 'В этой вкладке нет player ticket для игры.'
          }
        />
        <Link className="button button-primary" to={role === 'host' ? '/quizzes' : '/join'}>
          {role === 'host' ? 'К моим квизам' : 'Ввести код игры'}
        </Link>
      </div>
    </main>
  );
}

function FoundationGameView({ role }: { role: GameRole }) {
  const { game, status, error } = useGameSession();
  if (!game.snapshot && !error) return <Loading label="Подключаемся к игре…" />;
  return (
    <main className="game-foundation-page">
      <div className="game-foundation-card">
        {error && <ErrorBox message={`Realtime-соединение: ${error}`} />}
        {game.snapshot && (
          <>
            <span className="eyebrow">{role === 'host' ? 'ВЕДУЩИЙ' : 'ИГРОК'}</span>
            <h1>{game.snapshot.quiz_title}</h1>
            <p>
              Фаза: <strong>{game.snapshot.phase}</strong>
            </p>
            <p>Статус соединения: {status.state}</p>
            <p>Участников: {game.snapshot.players.length}</p>
          </>
        )}
        <p>Полноценный игровой интерфейс будет подключён поверх этого состояния.</p>
      </div>
    </main>
  );
}

function GameCredentialGuard({ role, children }: { role: GameRole; children: ReactNode }) {
  const gameId = useParams().gameId || '';
  const credentials = useMemo<GameCredentials | null>(
    () => (role === 'host' ? loadHostCredentials(gameId) : loadPlayerCredentials(gameId)),
    [gameId, role],
  );
  if (!gameId || !credentials) return <MissingCredentials role={role} />;
  return <GameSessionProvider credentials={credentials}>{children}</GameSessionProvider>;
}

export function HostGamePage() {
  return (
    <GameCredentialGuard role="host">
      <FoundationGameView role="host" />
    </GameCredentialGuard>
  );
}

export function PlayerGamePage() {
  return (
    <GameCredentialGuard role="player">
      <FoundationGameView role="player" />
    </GameCredentialGuard>
  );
}
