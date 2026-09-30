import { useState } from 'react';
import { LogOut, Wifi } from 'lucide-react';
import { useNavigate } from 'react-router';
import { Button } from '../../shared/ui/ui';
import { removeGameCredentials } from './credentials';
import { useGameSession } from './GameSession';
import { GameCommandError } from './socket';

function playersLabel(count: number): string {
  const mod100 = count % 100;
  const mod10 = count % 10;
  if (mod100 >= 11 && mod100 <= 14) return `${count} участников`;
  if (mod10 === 1) return `${count} участник`;
  if (mod10 >= 2 && mod10 <= 4) return `${count} участника`;
  return `${count} участников`;
}

export function PlayerLobby({ nickname }: { nickname: string }) {
  const { game, status, error, sendCommand } = useGameSession();
  const navigate = useNavigate();
  const snapshot = game.snapshot!;
  const connected = status.state === 'open';
  const initial = Array.from(nickname.trim())[0]?.toLocaleUpperCase('ru') || '?';
  const [leaving, setLeaving] = useState(false);
  const [leaveError, setLeaveError] = useState('');

  async function leave() {
    setLeaving(true);
    setLeaveError('');
    try {
      await sendCommand({ type: 'leave' }).acknowledged;
      removeGameCredentials('player', snapshot.game_id);
      void navigate(`/join/${encodeURIComponent(snapshot.code)}`, { replace: true });
    } catch (caught) {
      setLeaveError(
        caught instanceof GameCommandError && caught.code === 'invalid_phase'
          ? 'Игра уже началась — выйти из lobby больше нельзя.'
          : 'Не удалось покинуть игру. Проверьте соединение и попробуйте ещё раз.',
      );
    } finally {
      setLeaving(false);
    }
  }

  return (
    <main className="player-lobby-page">
      <header className="player-lobby-header">
        <span className="player-lobby-brand">
          QuizGo<span>.</span>
        </span>
        <span className={`player-lobby-connection ${connected ? 'is-connected' : ''}`}>
          <Wifi size={14} />
          {connected ? 'В сети' : 'Нет соединения'}
        </span>
      </header>

      <section className="player-lobby-card">
        <span className="player-lobby-avatar">{initial}</span>
        <h1>{nickname}</h1>
        <p>Вы в игре!</p>
        <div className="player-lobby-divider" />
        <span className="player-lobby-waiting-pulse">
          <span />
        </span>
        <h2>Ждём старта игры</h2>
        <p>Ведущий запустит викторину скоро</p>
        <strong>В комнате уже {playersLabel(snapshot.players.length)}</strong>
        {error && <span className="player-lobby-error">Realtime-соединение: {error}</span>}
        {leaveError && <span className="player-lobby-error">{leaveError}</span>}
        <Button
          variant="ghost"
          className="player-lobby-leave"
          busy={leaving}
          disabled={!connected}
          onClick={() => void leave()}
        >
          <LogOut size={16} />
          Покинуть игру
        </Button>
      </section>
    </main>
  );
}
