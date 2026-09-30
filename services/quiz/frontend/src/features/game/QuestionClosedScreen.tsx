import { useState } from 'react';
import { ArrowRight, CheckCircle2, Users } from 'lucide-react';
import { Button, ErrorBox } from '../../shared/ui/ui';
import { GameCommandError, type PendingGameCommand } from './socket';
import type { GameCommand, GameRole, GameStateSnapshot } from './types';

type SendCommand = (command: GameCommand, requestId?: string) => PendingGameCommand;

function nextError(error: unknown): string {
  if (error instanceof GameCommandError && error.code === 'invalid_phase') {
    return 'Состояние игры уже изменилось.';
  }
  return 'Не удалось перейти к следующему вопросу. Проверьте соединение и попробуйте ещё раз.';
}

export function QuestionClosedScreen({
  role,
  snapshot,
  sendCommand,
}: {
  role: GameRole;
  snapshot: GameStateSnapshot;
  sendCommand?: SendCommand;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');

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
    <main className="question-closed-page">
      <section className="question-closed-card">
        <span className="question-closed-icon">
          <CheckCircle2 size={34} />
        </span>
        <span className="eyebrow">ВОПРОС {snapshot.current_question_index + 1} ЗАВЕРШЁН</span>
        <h1>Ответы приняты</h1>
        <p>
          {role === 'host'
            ? 'Game-service закрыл вопрос. Можно перейти к следующему.'
            : 'Ждём, пока ведущий запустит следующий вопрос.'}
        </p>
        <span className="question-closed-players">
          <Users size={16} />
          {snapshot.players.length} игроков в игре
        </span>
        {error && <ErrorBox message={error} />}
        {role === 'host' && (
          <Button busy={pending} onClick={() => void next()}>
            Следующий вопрос
            <ArrowRight size={17} />
          </Button>
        )}
      </section>
    </main>
  );
}
