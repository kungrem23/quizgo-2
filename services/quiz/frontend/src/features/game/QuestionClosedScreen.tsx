import { useState } from 'react';
import { ArrowRight, Check, Hourglass, Trophy, X } from 'lucide-react';
import { Button, ErrorBox } from '../../shared/ui/ui';
import { GameCommandError, type PendingGameCommand } from './socket';
import type { GameAnswer, GameCommand, GameRole, GameStateSnapshot } from './types';
import { HostFinishButton } from './HostFinishButton';

type SendCommand = (command: GameCommand, requestId?: string) => PendingGameCommand;
type PlayerOutcome = 'correct' | 'incorrect' | 'timeout' | 'unknown';

function nextError(error: unknown): string {
  if (error instanceof GameCommandError && error.code === 'invalid_phase') {
    return 'Состояние игры уже изменилось.';
  }
  return 'Не удалось перейти к следующему вопросу. Проверьте соединение и попробуйте ещё раз.';
}

function formatScore(score: number): string {
  return new Intl.NumberFormat('ru-RU').format(score);
}

function answerText(answer: GameAnswer | undefined): string {
  return answer?.text ?? 'Недоступен';
}

function HostLeaderboard({
  snapshot,
  sendCommand,
}: {
  snapshot: GameStateSnapshot;
  sendCommand?: SendCommand;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const leaders = snapshot.players.slice(0, 5);
  const remainingPlayers = Math.max(0, snapshot.players.length - leaders.length);

  async function next() {
    if (!sendCommand) return;
    setPending(true);
    setError('');
    try {
      await sendCommand({ type: 'next' }).acknowledged;
    } catch (caught) {
      setError(nextError(caught));
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="game-rating-page">
      <header className="game-rating-header">
        <span className="game-rating-brand">
          QuizGo<span>.</span>
        </span>
        <div className="game-rating-title">
          <span>Вопрос {snapshot.current_question_index + 1} завершён</span>
          <h1>Таблица лидеров</h1>
        </div>
        <div className="game-rating-actions">
          {sendCommand && <HostFinishButton sendCommand={sendCommand} disabled={pending} />}
          <Button busy={pending} onClick={() => void next()}>
            Следующий вопрос
            <ArrowRight size={17} />
          </Button>
        </div>
      </header>

      <section className="game-rating-main" aria-label="Таблица лидеров">
        {leaders.length > 0 ? (
          <ol className="game-rating-list">
            {leaders.map((player, index) => {
              const rank = snapshot.players.findIndex((item) => item.score === player.score) + 1;
              return (
                <li className={index === 0 ? 'is-leader' : ''} key={player.id}>
                  <span className="game-rating-rank">{rank}</span>
                  <span className="game-rating-player">
                    {index === 0 && <Trophy size={17} aria-hidden="true" />}
                    <strong>{player.nickname}</strong>
                  </span>
                  <span className="game-rating-score">{formatScore(player.score)} очков</span>
                </li>
              );
            })}
          </ol>
        ) : (
          <p className="game-rating-empty">В игре пока нет участников.</p>
        )}
        {remainingPlayers > 0 && (
          <p className="game-rating-remaining">
            Ещё {remainingPlayers} {remainingPlayers === 1 ? 'игрок' : 'игроков'} в таблице
          </p>
        )}
        {error && <ErrorBox message={error} />}
      </section>
    </main>
  );
}

function PlayerQuestionResult({
  snapshot,
  playerId,
  selectedAnswerId,
  answerAccepted,
  questionObserved,
}: {
  snapshot: GameStateSnapshot;
  playerId?: string;
  selectedAnswerId?: number;
  answerAccepted: boolean;
  questionObserved: boolean;
}) {
  const question = snapshot.current_question;
  const correctAnswerIDs = snapshot.correct_answer_ids ?? [];
  const outcome: PlayerOutcome =
    selectedAnswerId !== undefined && answerAccepted
      ? correctAnswerIDs.includes(selectedAnswerId)
        ? 'correct'
        : 'incorrect'
      : selectedAnswerId === undefined && questionObserved
        ? 'timeout'
        : 'unknown';
  const selectedAnswer = question?.answers.find((answer) => answer.id === selectedAnswerId);
  const correctAnswers = question?.answers.filter((answer) => correctAnswerIDs.includes(answer.id));
  const player = snapshot.players.find((item) => item.id === playerId);
  const content = {
    correct: {
      title: 'Верно!',
      detail: 'Ответ принят и совпал с правильным.',
      icon: <Check size={40} strokeWidth={3} />,
    },
    incorrect: {
      title: 'В этот раз не угадали',
      detail: 'Ответ принят, но выбран другой вариант.',
      icon: <X size={38} strokeWidth={2.5} />,
    },
    timeout: {
      title: 'Время вышло!',
      detail: 'Вы не успели выбрать вариант.',
      icon: <Hourglass size={36} />,
    },
    unknown: {
      title: 'Вопрос завершён',
      detail: 'Результат ответа недоступен после переподключения.',
      icon: <Hourglass size={36} />,
    },
  }[outcome];

  return (
    <main className={`player-result-page is-${outcome}`}>
      <section className="player-result-card">
        <span className="player-result-icon">{content.icon}</span>
        <span className="eyebrow">ВОПРОС {snapshot.current_question_index + 1} ЗАВЕРШЁН</span>
        <h1>{content.title}</h1>
        <p className="player-result-detail">{content.detail}</p>

        <dl className="player-result-answers">
          <div>
            <dt>Ваш ответ</dt>
            <dd>{outcome === 'timeout' ? 'Не выбран' : answerText(selectedAnswer)}</dd>
          </div>
          <div>
            <dt>Правильный ответ</dt>
            <dd>
              {correctAnswers && correctAnswers.length > 0
                ? correctAnswers.map((answer) => answer.text).join(', ')
                : 'Недоступен'}
            </dd>
          </div>
        </dl>

        <div className="player-result-score">
          <span>Текущий счёт</span>
          <strong>{player ? formatScore(player.score) : 'Недоступен'}</strong>
        </div>
        <p className="player-result-waiting">Ждём, пока ведущий запустит следующий вопрос.</p>
      </section>
    </main>
  );
}

export function QuestionClosedScreen({
  role,
  snapshot,
  sendCommand,
  playerId,
  selectedAnswerId,
  answerAccepted = false,
  questionObserved = false,
}: {
  role: GameRole;
  snapshot: GameStateSnapshot;
  sendCommand?: SendCommand;
  playerId?: string;
  selectedAnswerId?: number;
  answerAccepted?: boolean;
  questionObserved?: boolean;
}) {
  if (role === 'host') {
    return <HostLeaderboard snapshot={snapshot} sendCommand={sendCommand} />;
  }
  return (
    <PlayerQuestionResult
      snapshot={snapshot}
      playerId={playerId}
      selectedAnswerId={selectedAnswerId}
      answerAccepted={answerAccepted}
      questionObserved={questionObserved}
    />
  );
}
