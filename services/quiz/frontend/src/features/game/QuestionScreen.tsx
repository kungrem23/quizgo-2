import { useRef, useState, type CSSProperties } from 'react';
import { Clock3, Wifi } from 'lucide-react';
import { QuizImage } from '../../shared/ui/QuizImage';
import { AnswerGrid } from './AnswerGrid';
import { deadlineProgress, useDeadline, type DeadlineState } from './deadline';
import { GameCommandError, type GameSocketStatus, type PendingGameCommand } from './socket';
import type { GameCommand, GameStateSnapshot } from './types';

type SendCommand = (command: GameCommand, requestId?: string) => PendingGameCommand;

function createAnswerRequestID(questionID: number): string {
  const suffix =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
  return `answer-${questionID}-${suffix}`;
}

function QuestionHeader({
  snapshot,
  status,
  remainingSeconds,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
  remainingSeconds: number;
}) {
  return (
    <header className="game-question-header">
      <span className="game-question-brand">
        QuizGo<span>.</span>
      </span>
      <span className="game-question-number">Вопрос {snapshot.current_question_index + 1}</span>
      <strong>{snapshot.quiz_title}</strong>
      <span className={`game-question-online ${status.state === 'open' ? 'is-connected' : ''}`}>
        <Wifi size={14} />
        {status.state === 'open' ? 'В сети' : 'Нет соединения'}
      </span>
      <span className="game-question-time">
        <Clock3 size={15} />
        {remainingSeconds} с
      </span>
    </header>
  );
}

function QuestionCard({ snapshot }: { snapshot: GameStateSnapshot }) {
  const question = snapshot.current_question!;
  return (
    <section className={`game-question-card ${question.image_id ? 'has-image' : ''}`}>
      {question.image_id && (
        <QuizImage
          className="game-question-image"
          imageId={question.image_id}
          alt="Иллюстрация к вопросу"
        />
      )}
      <h1>{question.text}</h1>
    </section>
  );
}

function QuestionShell({
  snapshot,
  status,
  deadline,
  children,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
  deadline: DeadlineState;
  children: React.ReactNode;
}) {
  const question = snapshot.current_question!;
  const progress = deadlineProgress(deadline.remainingMs, question.time_limit_seconds);
  const timerStyle = { '--question-time-progress': `${progress}%` } as CSSProperties;

  return (
    <main className="game-question-page" style={timerStyle}>
      <div className="game-question-progress" aria-hidden="true">
        <span />
      </div>
      <QuestionHeader
        snapshot={snapshot}
        status={status}
        remainingSeconds={deadline.remainingSeconds}
      />
      <div className="game-question-main">
        <QuestionCard snapshot={snapshot} />
        {children}
        {deadline.expired && (
          <div className="game-question-expired" role="status">
            Время вышло. Ожидаем перехода от game-service…
          </div>
        )}
      </div>
    </main>
  );
}

export function HostQuestionScreen({
  snapshot,
  status,
  answeredPlayerIds,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
  answeredPlayerIds: string[] | null;
}) {
  const question = snapshot.current_question!;
  const deadline = useDeadline(snapshot.question_closes_at);
  return (
    <QuestionShell snapshot={snapshot} status={status} deadline={deadline}>
      <div className="host-answer-progress" aria-live="polite">
        <strong>{answeredPlayerIds === null ? '—' : answeredPlayerIds.length}</strong>
        <span>из {snapshot.players.length} игроков ответили</span>
        {answeredPlayerIds === null && <small>Счётчик недоступен после переподключения</small>}
      </div>
      <AnswerGrid answers={question.answers} />
    </QuestionShell>
  );
}

function answerError(error: unknown): string {
  if (!(error instanceof GameCommandError)) {
    return 'Не удалось подтвердить ответ. Не закрывайте страницу.';
  }
  if (error.code === 'already_answered') return 'Ответ на этот вопрос уже был принят.';
  if (error.code === 'question_closed' || error.code === 'invalid_phase') {
    return 'Вопрос уже закрыт. Ожидаем следующий этап.';
  }
  return `Game-service отклонил ответ: ${error.code}.`;
}

export function PlayerQuestionScreen({
  snapshot,
  status,
  acceptedQuestionIds,
  selectedAnswerId: recordedAnswerId,
  onAnswerSelected,
  sendCommand,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
  acceptedQuestionIds: number[];
  selectedAnswerId?: number | null;
  onAnswerSelected?: (questionId: number, answerId: number) => void;
  sendCommand: SendCommand;
}) {
  const question = snapshot.current_question!;
  const deadline = useDeadline(snapshot.question_closes_at);
  const [localAnswerId, setLocalAnswerId] = useState<number | null>(null);
  const [pending, setPending] = useState(false);
  const [submissionError, setSubmissionError] = useState('');
  const lockedRef = useRef(false);
  const requestIDRef = useRef<string | null>(null);
  const selectedAnswerId = recordedAnswerId ?? localAnswerId;
  const accepted = acceptedQuestionIds.includes(question.id);
  const locked = lockedRef.current || selectedAnswerId !== null || accepted || deadline.expired;

  async function answer(answerID: number) {
    if (lockedRef.current || accepted || deadline.expired) return;
    lockedRef.current = true;
    setLocalAnswerId(answerID);
    onAnswerSelected?.(question.id, answerID);
    setPending(true);
    setSubmissionError('');
    const requestID = requestIDRef.current ?? createAnswerRequestID(question.id);
    requestIDRef.current = requestID;
    try {
      await sendCommand({ type: 'answer', payload: { answer_id: answerID } }, requestID)
        .acknowledged;
    } catch (error) {
      setSubmissionError(answerError(error));
    } finally {
      setPending(false);
    }
  }

  return (
    <QuestionShell snapshot={snapshot} status={status} deadline={deadline}>
      <p className="player-answer-instruction">
        {accepted
          ? 'Ответ принят. Ожидаем завершения вопроса.'
          : selectedAnswerId !== null
            ? pending
              ? 'Отправляем ответ…'
              : 'Ожидаем подтверждения game-service…'
            : deadline.expired
              ? 'Время для ответа закончилось.'
              : 'Выберите один вариант — изменить ответ нельзя'}
      </p>
      <AnswerGrid
        answers={question.answers}
        interactive
        selectedAnswerId={selectedAnswerId}
        pending={pending}
        accepted={accepted}
        disabled={locked || status.state !== 'open'}
        onSelect={(answerID) => void answer(answerID)}
      />
      {submissionError && (
        <div className="player-answer-error" role="alert">
          {submissionError}
        </div>
      )}
    </QuestionShell>
  );
}
