import { useMemo, useState } from 'react';
import { Copy, LoaderCircle, Play, Trash2, Users, Wifi } from 'lucide-react';
import { Button, ErrorBox, useToast } from '../../shared/ui/ui';
import { useGameSession } from './GameSession';
import { GameCommandError } from './socket';
import type { GamePlayer } from './types';
import { JoinQRCode } from './JoinQRCode';
import { gameJoinURL } from './urls';
import { HostFinishButton } from './HostFinishButton';

const avatarTones = ['red', 'blue', 'amber', 'purple', 'green', 'pink', 'teal'];

function commandError(error: unknown): string {
  if (!(error instanceof GameCommandError)) {
    return 'Не удалось выполнить команду. Проверьте соединение и попробуйте ещё раз.';
  }
  if (error.code === 'invalid_phase') return 'Состояние игры уже изменилось.';
  if (error.code === 'unauthorized') return 'Host ticket больше не действителен.';
  if (error.code === 'invalid_payload') return 'Игрок уже покинул комнату.';
  return `Game-service отклонил команду: ${error.code}.`;
}

function PlayerCard({
  player,
  index,
  pending,
  disabled,
  onRemove,
}: {
  player: GamePlayer;
  index: number;
  pending: boolean;
  disabled: boolean;
  onRemove: () => void;
}) {
  const initial = Array.from(player.nickname.trim())[0]?.toLocaleUpperCase('ru') || '?';
  return (
    <article className="host-player-card">
      <span className={`host-player-avatar avatar-tone-${avatarTones[index % avatarTones.length]}`}>
        {initial}
      </span>
      <span className="host-player-name" title={player.nickname}>
        {player.nickname}
      </span>
      <button
        type="button"
        className="host-player-remove"
        aria-label={`Удалить игрока ${player.nickname}`}
        title="Удалить игрока"
        disabled={disabled}
        onClick={onRemove}
      >
        {pending ? <LoaderCircle className="spin" size={17} /> : <Trash2 size={17} />}
      </button>
    </article>
  );
}

export function HostLobby() {
  const { game, status, error: sessionError, sendCommand } = useGameSession();
  const snapshot = game.snapshot!;
  const players = snapshot.players;
  const toast = useToast();
  const [pendingStart, setPendingStart] = useState(false);
  const [removing, setRemoving] = useState<Set<string>>(() => new Set());
  const [commandErrorMessage, setCommandErrorMessage] = useState('');
  const joinURL = useMemo(() => gameJoinURL(snapshot.code), [snapshot.code]);
  const anyCommandPending = pendingStart || removing.size > 0;
  const connected = status.state === 'open';

  async function copyJoinLink() {
    try {
      await navigator.clipboard.writeText(joinURL);
      toast('Ссылка для игроков скопирована');
    } catch {
      setCommandErrorMessage(
        'Не удалось скопировать ссылку. Откройте её и скопируйте из адресной строки.',
      );
    }
  }

  async function start() {
    setPendingStart(true);
    setCommandErrorMessage('');
    try {
      await sendCommand({ type: 'start' }).acknowledged;
    } catch (caught) {
      setCommandErrorMessage(commandError(caught));
    } finally {
      setPendingStart(false);
    }
  }

  async function removePlayer(playerID: string) {
    setRemoving((current) => new Set(current).add(playerID));
    setCommandErrorMessage('');
    try {
      await sendCommand({ type: 'remove_player', payload: { player_id: playerID } }).acknowledged;
    } catch (caught) {
      setCommandErrorMessage(commandError(caught));
    } finally {
      setRemoving((current) => {
        const next = new Set(current);
        next.delete(playerID);
        return next;
      });
    }
  }

  return (
    <div className="host-lobby-page">
      <header className="host-lobby-header">
        <div className="host-lobby-header-inner">
          <span className="host-lobby-brand">
            QuizGo<span>.</span>
          </span>
          <strong className="host-lobby-title">{snapshot.quiz_title}</strong>
          <HostFinishButton
            className="host-lobby-finish"
            sendCommand={sendCommand}
            disabled={!connected || anyCommandPending}
          />
          <span className={`host-connection ${connected ? 'is-connected' : ''}`}>
            <Wifi size={15} />
            {connected ? 'Подключено' : 'Нет соединения'}
          </span>
        </div>
      </header>

      <main className="host-lobby-main">
        <section className="host-join-card">
          <div className="host-join-copy">
            <span className="host-join-label">Откройте на телефоне</span>
            <div className="host-join-link-row">
              <a href={joinURL} target="_blank" rel="noreferrer">
                {joinURL}
              </a>
              <button
                type="button"
                aria-label="Скопировать ссылку"
                onClick={() => void copyJoinLink()}
              >
                <Copy size={17} />
              </button>
            </div>
            <span className="host-join-label">и введите код:</span>
            <strong className="host-room-code">{snapshot.code}</strong>
          </div>
          <JoinQRCode value={joinURL} />
        </section>

        {(commandErrorMessage || sessionError) && (
          <ErrorBox message={commandErrorMessage || `Realtime-соединение: ${sessionError}`} />
        )}

        {players.length === 0 ? (
          <section className="host-empty-lobby">
            <span className="host-waiting-pulse">
              <span />
            </span>
            <h1>Ждём участников…</h1>
            <p>Подключитесь по коду выше, чтобы начать викторину</p>
            <span>0 из 100 игроков</span>
            <Button disabled>Начать игру</Button>
          </section>
        ) : (
          <section className="host-populated-lobby">
            <div className="host-players-heading">
              <div>
                <span className="host-players-icon">
                  <Users size={20} />
                </span>
                <div>
                  <h1>Участники</h1>
                  <p>{players.length} из 100 игроков в комнате</p>
                </div>
              </div>
              <Button
                busy={pendingStart}
                disabled={!connected || anyCommandPending || players.length === 0}
                onClick={() => void start()}
              >
                <Play size={16} fill="currentColor" />
                Начать игру
              </Button>
            </div>
            <div className="host-player-grid" aria-live="polite">
              {players.map((player, index) => (
                <PlayerCard
                  key={player.id}
                  player={player}
                  index={index}
                  pending={removing.has(player.id)}
                  disabled={!connected || anyCommandPending}
                  onRemove={() => void removePlayer(player.id)}
                />
              ))}
            </div>
          </section>
        )}
      </main>
    </div>
  );
}
