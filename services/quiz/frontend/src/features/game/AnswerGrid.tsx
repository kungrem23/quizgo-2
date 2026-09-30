import { Check, LoaderCircle } from 'lucide-react';
import type { GameAnswer } from './types';

const symbols = ['▲', '◆', '●', '■', '✦', '⬟', '✚', '★'];

function answerSymbol(index: number): string {
  if (index < symbols.length) return symbols[index];
  return String.fromCharCode(65 + ((index - symbols.length) % 26));
}

function AnswerContent({
  answer,
  index,
  pending,
  accepted,
}: {
  answer: GameAnswer;
  index: number;
  pending: boolean;
  accepted: boolean;
}) {
  return (
    <>
      <span className="game-answer-symbol">{answerSymbol(index)}</span>
      <span className="game-answer-text">{answer.text}</span>
      {pending && <LoaderCircle className="spin game-answer-status" size={22} />}
      {accepted && <Check className="game-answer-status" size={24} />}
    </>
  );
}

export function AnswerGrid({
  answers,
  interactive = false,
  selectedAnswerId = null,
  pending = false,
  accepted = false,
  disabled = false,
  onSelect,
}: {
  answers: GameAnswer[];
  interactive?: boolean;
  selectedAnswerId?: number | null;
  pending?: boolean;
  accepted?: boolean;
  disabled?: boolean;
  onSelect?: (answerId: number) => void;
}) {
  const compact = answers.length >= 5;
  return (
    <div
      className={`game-answer-grid ${compact ? 'is-compact' : ''}`}
      data-answer-count={answers.length}
    >
      {answers.map((answer, index) => {
        const selected = selectedAnswerId === answer.id;
        const className = `game-answer-card answer-tone-${index % symbols.length}${selected ? ' is-selected' : ''}${selected && accepted ? ' is-accepted' : ''}`;
        return interactive ? (
          <button
            key={answer.id}
            type="button"
            className={className}
            aria-pressed={selected}
            disabled={disabled}
            onClick={() => onSelect?.(answer.id)}
          >
            <AnswerContent
              answer={answer}
              index={index}
              pending={selected && pending}
              accepted={selected && accepted}
            />
          </button>
        ) : (
          <div key={answer.id} className={className}>
            <AnswerContent answer={answer} index={index} pending={false} accepted={false} />
          </div>
        );
      })}
    </div>
  );
}
