import { useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { Check, Wifi } from 'lucide-react';
import { QuizImage } from '../../shared/ui/QuizImage';
import { AnswerGrid } from './AnswerGrid';
import { deadlineProgress, useDeadline, type DeadlineState } from './deadline';
import { GameCommandError, type GameSocketStatus, type PendingGameCommand } from './socket';
import type { GameCommand, GameStateSnapshot } from './types';
import { HostFinishButton } from './HostFinishButton';

type SendCommand = (command: GameCommand, requestId?: string) => PendingGameCommand;

function createAnswerRequestID(questionID: number): string {
  const suffix =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
  return `answer-${questionID}-${suffix}`;
}

function questionLabel(snapshot: GameStateSnapshot): string {
  const current = snapshot.current_question_index + 1;
  return snapshot.total_questions
    ? `Вопрос ${current} из ${snapshot.total_questions}`
    : `Вопрос ${current}`;
}

function QuestionProgress({
  deadline,
  snapshot,
}: {
  deadline: DeadlineState;
  snapshot: GameStateSnapshot;
}) {
  const question = snapshot.current_question!;
  const progress = deadlineProgress(deadline.remainingMs, question.time_limit_seconds);
  const style = { '--question-time-progress': `${progress}%` } as CSSProperties;
  return (
    <div className="game-question-progress" style={style} aria-hidden="true">
      <span />
    </div>
  );
}

function QuestionMedia({ snapshot }: { snapshot: GameStateSnapshot }) {
  const imageID = snapshot.current_question?.image_id;
  if (!imageID) return null;
  return (
    <div className="game-question-media">
      <QuizImage className="game-question-image" imageId={imageID} alt="Иллюстрация к вопросу" />
    </div>
  );
}

function DeadlineNotice({ deadline }: { deadline: DeadlineState }) {
  return deadline.expired ? (
    <div className="game-question-expired" role="status">
      Время вышло. Ожидаем перехода от game-service…
    </div>
  ) : null;
}

function HostQuestionHeader({
  snapshot,
  status,
  answeredPlayerIds,
  sendCommand,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
  answeredPlayerIds: string[] | null;
  sendCommand: SendCommand;
}) {
  return (
    <header className="host-question-header">
      <span className="game-question-brand">QuizGo</span>
      <span className="game-question-number">{questionLabel(snapshot)}</span>
      <span className="host-question-answered">
        <i aria-hidden="true" />
        Ответили: <strong>
          {answeredPlayerIds === null ? '—' : answeredPlayerIds.length}
        </strong> из {snapshot.players.length}
      </span>
      <span className={`game-question-online ${status.state === 'open' ? 'is-connected' : ''}`}>
        <Wifi size={14} />
        {status.state === 'open' ? 'В сети' : 'Нет соединения'}
      </span>
      <HostFinishButton
        className="question-finish"
        sendCommand={sendCommand}
        disabled={status.state !== 'open'}
      />
    </header>
  );
}

export function HostQuestionScreen({
  snapshot,
  status,
  answeredPlayerIds,
  sendCommand,
  serverTimeOffsetMs = 0,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
  answeredPlayerIds: string[] | null;
  sendCommand: SendCommand;
  serverTimeOffsetMs?: number;
}) {
  const question = snapshot.current_question!;
  const deadline = useDeadline(snapshot.question_closes_at, serverTimeOffsetMs);
  return (
    <main className="game-question-page is-host">
      <QuestionProgress deadline={deadline} snapshot={snapshot} />
      <HostQuestionHeader
        snapshot={snapshot}
        status={status}
        answeredPlayerIds={answeredPlayerIds}
        sendCommand={sendCommand}
      />
      <div className="host-question-main">
        <section className="host-question-prompt">
          <span className="host-question-timer" role="timer">
            {deadline.remainingSeconds}
          </span>
          <h1>{question.text}</h1>
        </section>
        <QuestionMedia snapshot={snapshot} />
        {answeredPlayerIds === null && (
          <p className="host-answer-unavailable">
            Счётчик ответов недоступен после переподключения
          </p>
        )}
        <AnswerGrid answers={question.answers} />
        <DeadlineNotice deadline={deadline} />
      </div>
    </main>
  );
}

function PlayerQuestionLayout({
  snapshot,
  deadline,
  instruction,
  children,
}: {
  snapshot: GameStateSnapshot;
  deadline: DeadlineState;
  instruction: ReactNode;
  children: ReactNode;
}) {
  const question = snapshot.current_question!;
  return (
    <main className="game-question-page is-player">
      <QuestionProgress deadline={deadline} snapshot={snapshot} />
      <div className="player-question-main">
        <div className="player-question-meta">
          <span>{questionLabel(snapshot)}</span>
          <strong role="timer">{deadline.remainingSeconds} с</strong>
        </div>
        <h1>{question.text}</h1>
        <QuestionMedia snapshot={snapshot} />
        <p className="player-answer-instruction">{instruction}</p>
        {children}
        <DeadlineNotice deadline={deadline} />
      </div>
    </main>
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
  serverTimeOffsetMs = 0,
}: {
  snapshot: GameStateSnapshot;
  status: GameSocketStatus;
  acceptedQuestionIds: number[];
  selectedAnswerId?: number | null;
  onAnswerSelected?: (questionId: number, answerId: number) => void;
  sendCommand: SendCommand;
  serverTimeOffsetMs?: number;
}) {
  const question = snapshot.current_question!;
  const deadline = useDeadline(snapshot.question_closes_at, serverTimeOffsetMs);
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

  const instruction = accepted ? (
    <span className="player-answer-accepted">
      <Check size={15} /> Ответ принят. Ожидаем завершения вопроса.
    </span>
  ) : selectedAnswerId !== null ? (
    pending ? (
      'Отправляем ответ…'
    ) : (
      'Ожидаем подтверждения game-service…'
    )
  ) : deadline.expired ? (
    'Время для ответа закончилось.'
  ) : (
    'Выберите один вариант — изменить ответ нельзя'
  );

  return (
    <PlayerQuestionLayout snapshot={snapshot} deadline={deadline} instruction={instruction}>
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
    </PlayerQuestionLayout>
  );
}
