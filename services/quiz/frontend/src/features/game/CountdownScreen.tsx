import type { GameSocketStatus } from './socket';
import type { GameStateSnapshot } from './types';
import { useDeadline } from './deadline';

export function CountdownScreen({
  snapshot,
  status,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
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
