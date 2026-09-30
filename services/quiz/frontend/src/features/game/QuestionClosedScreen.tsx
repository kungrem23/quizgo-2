import { useState, type ReactNode } from 'react';
import { ArrowRight, Check, Hourglass, Trophy, X } from 'lucide-react';
import { Button, ErrorBox } from '../../shared/ui/ui';
import { AnswerGrid } from './AnswerGrid';
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

function questionLabel(snapshot: GameStateSnapshot): string {
  const current = snapshot.current_question_index + 1;
  return snapshot.total_questions
    ? `Вопрос ${current} из ${snapshot.total_questions}`
    : `Вопрос ${current}`;
}

function ResultHeader({ snapshot, actions }: { snapshot: GameStateSnapshot; actions?: ReactNode }) {
  return (
    <header className="question-result-header">
      <span className="question-result-brand">QuizGo</span>
      <span className="question-result-meta">{questionLabel(snapshot)} • Ответы приняты</span>
      {actions && <div className="question-result-header-actions">{actions}</div>}
    </header>
  );
}

function HostQuestionResult({
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
    <main className="question-result-page host-question-result">
      <ResultHeader
        snapshot={snapshot}
        actions={sendCommand && <HostFinishButton sendCommand={sendCommand} disabled={pending} />}
      />
      <div className="host-result-main">
        {snapshot.current_question && (
          <>
            <section className="question-result-prompt">
              <h1>{snapshot.current_question.text}</h1>
            </section>
            <section className="host-result-answers" aria-label="Результат вопроса">
              <AnswerGrid
                answers={snapshot.current_question.answers}
                correctAnswerIds={snapshot.correct_answer_ids ?? []}
              />
              <p>Правильный ответ выделен зелёной рамкой.</p>
            </section>
          </>
        )}

        {sendCommand && (
          <Button className="host-result-next" busy={pending} onClick={() => void next()}>
            Следующий вопрос
            <ArrowRight size={18} />
          </Button>
        )}

        <section className="host-result-leaderboard" aria-label="Таблица лидеров">
          <h2>Таблица лидеров</h2>
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
        </section>
        {error && <ErrorBox message={error} />}
      </div>
    </main>
  );
}

function QuestionResultStatus({
  outcome,
  selectedAnswer,
  correctAnswers,
  score,
}: {
  outcome: PlayerOutcome;
  selectedAnswer?: GameAnswer;
  correctAnswers: GameAnswer[];
  score?: number;
}) {
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
    <section className="player-result-card">
      <span className="player-result-icon">{content.icon}</span>
      <h1>{content.title}</h1>
      <p className="player-result-detail">{content.detail}</p>
      <dl className="player-result-answers">
        <div>
          <dt>Ваш ответ</dt>
          <dd>{outcome === 'timeout' ? 'Не выбран' : answerText(selectedAnswer)}</dd>
        </div>
        <div>
          <dt>Правильный ответ</dt>
          <dd className={correctAnswers.length > 0 ? 'correct' : 'incorrect'}>
            {correctAnswers.length > 0
              ? correctAnswers.map((answer) => answer.text).join(', ')
              : 'Недоступен'}
          </dd>
        </div>
      </dl>
      <div className="player-result-score">
        <span>Текущий счёт</span>
        <strong>{score === undefined ? 'Недоступен' : formatScore(score)}</strong>
      </div>
      <p className="player-result-waiting">Ждём, пока ведущий запустит следующий вопрос.</p>
    </section>
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
  const correctAnswers =
    question?.answers.filter((answer) => correctAnswerIDs.includes(answer.id)) ?? [];
  const player = snapshot.players.find((item) => item.id === playerId);

  return (
    <main className={`question-result-page player-result-page is-${outcome}`}>
      <ResultHeader snapshot={snapshot} />
      <div className="player-result-main">
        {question && (
          <section className="question-result-prompt">
            <h2>{question.text}</h2>
          </section>
        )}
        <QuestionResultStatus
          outcome={outcome}
          selectedAnswer={selectedAnswer}
          correctAnswers={correctAnswers}
          score={player?.score}
        />
      </div>
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
    return <HostQuestionResult snapshot={snapshot} sendCommand={sendCommand} />;
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
