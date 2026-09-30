import type { GameSocketStatus, PendingGameCommand } from './socket';
import type { GameCommand, GameRole, GameStateSnapshot } from './types';
import { useDeadline } from './deadline';
import { HostFinishButton } from './HostFinishButton';

export function CountdownScreen({
  snapshot,
  status,
  role,
  sendCommand,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
  role?: GameRole;
  sendCommand?: (command: GameCommand, requestId?: string) => PendingGameCommand;
}) {
  const deadline = useDeadline(snapshot.countdown_ends_at);
  const questionNumber = snapshot.current_question_index + 1;

  return (
    <main className="game-countdown-page">
      <div className="countdown-orbit countdown-orbit-outer" />
      <div className="countdown-orbit countdown-orbit-inner" />
      <span className={`countdown-connection ${status.state === 'open' ? 'is-connected' : ''}`}>
        {status.state === 'open' ? 'В сети' : 'Нет соединения'}
      </span>
      {role === 'host' && sendCommand && (
        <HostFinishButton
          className="countdown-finish"
          sendCommand={sendCommand}
          disabled={status.state !== 'open'}
        />
      )}
      <div className={`countdown-value ${deadline.expired ? 'is-expired' : ''}`} role="timer">
        {deadline.remainingSeconds}
      </div>
      <h1>
        {deadline.expired
          ? 'Ожидаем вопрос от сервера…'
          : `Приготовьтесь к вопросу ${questionNumber}!`}
      </h1>
      <p>
        {deadline.expired
          ? 'Отсчёт завершён. Состояние изменится после server event.'
          : 'Быстрый правильный ответ приносит больше очков'}
      </p>
    </main>
  );
}
