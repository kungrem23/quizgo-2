import { useMemo, type ReactNode } from 'react';
import { Link, useParams } from 'react-router';
import {
  loadHostCredentials,
  loadPlayerCredentials,
  type GameCredentials,
} from '../features/game/credentials';
import { CountdownScreen } from '../features/game/CountdownScreen';
import { FinalResultsScreen } from '../features/game/FinalResultsScreen';
import { GameSessionProvider, useGameSession } from '../features/game/GameSession';
import { HostLobby } from '../features/game/HostLobby';
import { PlayerLobby } from '../features/game/PlayerLobby';
import { QuestionClosedScreen } from '../features/game/QuestionClosedScreen';
import { HostQuestionScreen, PlayerQuestionScreen } from '../features/game/QuestionScreen';
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

function HostGameView({ quizId }: { quizId?: number }) {
  const { game, status, error, sendCommand } = useGameSession();
  if (!game.snapshot && !error) return <Loading label="Подключаем комнату…" />;
  if (game.snapshot?.phase === 'lobby') return <HostLobby />;
  if (game.snapshot?.phase === 'countdown') {
    return (
      <CountdownScreen
        snapshot={game.snapshot}
        status={status}
        role="host"
        sendCommand={sendCommand}
        serverTimeOffsetMs={game.serverTimeOffsetMs}
      />
    );
  }
  if (game.snapshot?.phase === 'question_open' && game.snapshot.current_question) {
    return (
      <HostQuestionScreen
        snapshot={game.snapshot}
        status={status}
        answeredPlayerIds={game.answeredPlayerIds}
        sendCommand={sendCommand}
        serverTimeOffsetMs={game.serverTimeOffsetMs}
      />
    );
  }
  if (game.snapshot?.phase === 'scoreboard' || game.snapshot?.phase === 'question_closed') {
    return <QuestionClosedScreen role="host" snapshot={game.snapshot} sendCommand={sendCommand} />;
  }
  if (game.snapshot?.phase === 'finished') {
    return <FinalResultsScreen role="host" snapshot={game.snapshot} quizId={quizId} />;
  }
  return <FoundationGameView role="host" />;
}

function PlayerGameView({ nickname, playerId }: { nickname: string; playerId: string }) {
  const { game, status, error, sendCommand, recordAnswerSelection } = useGameSession();
  if (!game.snapshot && !error) return <Loading label="Подключаемся к игре…" />;
  if (game.snapshot?.phase === 'lobby') {
    return <PlayerLobby nickname={nickname} />;
  }
  if (game.snapshot?.phase === 'countdown') {
    return (
      <CountdownScreen
        snapshot={game.snapshot}
        status={status}
        role="player"
        serverTimeOffsetMs={game.serverTimeOffsetMs}
      />
    );
  }
  if (game.snapshot?.phase === 'question_open' && game.snapshot.current_question) {
    return (
      <PlayerQuestionScreen
        key={game.snapshot.current_question.id}
        snapshot={game.snapshot}
        status={status}
        acceptedQuestionIds={game.acceptedQuestionIds}
        selectedAnswerId={game.selectedAnswerIds[game.snapshot.current_question.id]}
        onAnswerSelected={recordAnswerSelection}
        sendCommand={sendCommand}
        serverTimeOffsetMs={game.serverTimeOffsetMs}
      />
    );
  }
  if (game.snapshot?.phase === 'scoreboard' || game.snapshot?.phase === 'question_closed') {
    const questionID = game.snapshot.current_question?.id;
    return (
      <QuestionClosedScreen
        role="player"
        snapshot={game.snapshot}
        playerId={playerId}
        selectedAnswerId={questionID === undefined ? undefined : game.selectedAnswerIds[questionID]}
        answerAccepted={
          questionID === undefined ? false : game.acceptedQuestionIds.includes(questionID)
        }
        questionObserved={
          questionID === undefined ? false : game.observedQuestionIds.includes(questionID)
        }
      />
    );
  }
  if (game.snapshot?.phase === 'finished') {
    return <FinalResultsScreen role="player" snapshot={game.snapshot} playerId={playerId} />;
  }
  return <FoundationGameView role="player" />;
}

function GameCredentialGuard({
  role,
  children,
}: {
  role: GameRole;
  children: ReactNode | ((credentials: GameCredentials) => ReactNode);
}) {
  const gameId = useParams().gameId || '';
  const credentials = useMemo<GameCredentials | null>(
    () => (role === 'host' ? loadHostCredentials(gameId) : loadPlayerCredentials(gameId)),
    [gameId, role],
  );
  if (!gameId || !credentials) return <MissingCredentials role={role} />;
  return (
    <GameSessionProvider credentials={credentials}>
      {typeof children === 'function' ? children(credentials) : children}
    </GameSessionProvider>
  );
}

export function HostGamePage() {
  return (
    <GameCredentialGuard role="host">
      {(credentials) =>
        credentials.role === 'host' ? <HostGameView quizId={credentials.quizId} /> : null
      }
    </GameCredentialGuard>
  );
}

export function PlayerGamePage() {
  return (
    <GameCredentialGuard role="player">
      {(credentials) =>
        credentials.role === 'player' ? (
          <PlayerGameView nickname={credentials.nickname} playerId={credentials.playerId} />
        ) : null
      }
    </GameCredentialGuard>
  );
}
