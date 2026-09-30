import { useMemo, useState } from 'react';
import { Crown, LogOut, Medal, RotateCcw, Trophy } from 'lucide-react';
import { useNavigate } from 'react-router';
import { Button, ErrorBox } from '../../shared/ui/ui';
import { removeGameCredentials } from './credentials';
import { hostLaunchErrorMessage, launchHostGame } from './launch';
import type { GamePlayer, GameRole, GameStateSnapshot } from './types';

interface RankedPlayer {
  player: GamePlayer;
  place: number;
}

function formatScore(score: number): string {
  return new Intl.NumberFormat('ru-RU').format(score);
}

function initial(nickname: string): string {
  return Array.from(nickname.trim())[0]?.toLocaleUpperCase('ru') || '?';
}

function playerPlace(players: GamePlayer[], playerID: string | undefined): RankedPlayer | null {
  const index = players.findIndex((player) => player.id === playerID);
  return index < 0 ? null : { player: players[index], place: index + 1 };
}

function PodiumPlace({ entry }: { entry: RankedPlayer }) {
  return (
    <article className={`final-podium-place final-podium-place-${entry.place}`}>
      {entry.place === 1 && <Crown className="final-podium-crown" aria-hidden="true" />}
      <span className="final-podium-avatar">{initial(entry.player.nickname)}</span>
      <strong title={entry.player.nickname}>{entry.player.nickname}</strong>
      <span>{formatScore(entry.player.score)} очков</span>
      <div className="final-podium-step" aria-label={`${entry.place} место`}>
        {entry.place}
      </div>
    </article>
  );
}

function HostFinalResults({
  snapshot,
  quizId,
  onExit,
}: {
  snapshot: GameStateSnapshot;
  quizId?: number;
  onExit: () => void;
}) {
  const navigate = useNavigate();
  const [replaying, setReplaying] = useState(false);
  const [replayError, setReplayError] = useState('');
  const ranked = useMemo(
    () => snapshot.players.map((player, index) => ({ player, place: index + 1 })),
    [snapshot.players],
  );
  const podium = [ranked[1], ranked[0], ranked[2]].filter(
    (entry): entry is RankedPlayer => entry !== undefined,
  );
  const remaining = ranked.slice(3);

  async function replay() {
    if (quizId === undefined) return;
    setReplaying(true);
    setReplayError('');
    try {
      const created = await launchHostGame(quizId);
      void navigate(`/games/${encodeURIComponent(created.id)}/host`, { replace: true });
    } catch (error) {
      setReplayError(hostLaunchErrorMessage(error));
    } finally {
      setReplaying(false);
    }
  }

  return (
    <main className="final-host-page">
      <header className="final-host-header">
        <span className="final-results-brand">
          QuizGo<span>.</span>
        </span>
        <h1>Финальные результаты</h1>
      </header>

      <div className="final-host-main">
        {podium.length > 0 ? (
          <section
            className={`final-podium final-podium-count-${podium.length}`}
            aria-label="Пьедестал"
          >
            {podium.map((entry) => (
              <PodiumPlace key={entry.player.id} entry={entry} />
            ))}
          </section>
        ) : (
          <section className="final-results-empty">
            <Trophy size={38} />
            <h2>Игра завершена</h2>
            <p>В итоговой таблице нет игроков.</p>
          </section>
        )}

        {remaining.length > 0 && (
          <section className="final-leaderboard" aria-labelledby="final-leaderboard-title">
            <h2 id="final-leaderboard-title">Итоговая таблица</h2>
            <ol start={4}>
              {remaining.map((entry) => (
                <li key={entry.player.id}>
                  <span>{entry.place}</span>
                  <strong>{entry.player.nickname}</strong>
                  <span>{formatScore(entry.player.score)} очков</span>
                </li>
              ))}
            </ol>
          </section>
        )}

        {replayError && <ErrorBox message={replayError} />}
        <div className="final-host-actions">
          {quizId !== undefined && (
            <Button busy={replaying} onClick={() => void replay()}>
              <RotateCcw size={17} />
              Сыграть ещё раз
            </Button>
          )}
          <Button variant="secondary" disabled={replaying} onClick={onExit}>
            <LogOut size={17} />
            Вернуться к квизам
          </Button>
        </div>
      </div>
    </main>
  );
}

function PlayerFinalResult({
  snapshot,
  playerId,
  onExit,
}: {
  snapshot: GameStateSnapshot;
  playerId?: string;
  onExit: () => void;
}) {
  const result = playerPlace(snapshot.players, playerId);

  return (
    <main className="final-player-page">
      <span className="final-results-brand">
        QuizGo<span>.</span>
      </span>
      <section className="final-player-card">
        <span className="final-player-trophy">
          {result?.place === 1 ? <Trophy size={44} /> : <Medal size={44} />}
        </span>
        <span className="eyebrow">ИГРА ЗАВЕРШЕНА</span>
        <h1>{result?.place === 1 ? 'Великолепная игра!' : 'Спасибо за игру!'}</h1>
        <p className="final-player-quiz">{snapshot.quiz_title}</p>
        {result ? (
          <>
            <strong className="final-player-name">{result.player.nickname}</strong>
            <p className="final-player-place">Вы заняли {result.place} место</p>
            <div className="final-player-score">
              <span>Итоговый счёт</span>
              <strong>{formatScore(result.player.score)}</strong>
            </div>
            <p className="final-player-total">
              Участников в итоговой таблице: {snapshot.players.length}
            </p>
          </>
        ) : (
          <ErrorBox message="Ваш итоговый результат отсутствует в состоянии game-service." />
        )}
        <Button onClick={onExit}>
          <LogOut size={17} />
          Выйти из игры
        </Button>
      </section>
    </main>
  );
}

export function FinalResultsScreen({
  role,
  snapshot,
  playerId,
  quizId,
}: {
  role: GameRole;
  snapshot: GameStateSnapshot;
  playerId?: string;
  quizId?: number;
}) {
  const navigate = useNavigate();

  function exit() {
    removeGameCredentials(role, snapshot.game_id);
    void navigate(role === 'host' ? '/quizzes' : '/join', { replace: true });
  }

  return role === 'host' ? (
    <HostFinalResults snapshot={snapshot} quizId={quizId} onExit={exit} />
  ) : (
    <PlayerFinalResult snapshot={snapshot} playerId={playerId} onExit={exit} />
  );
}
