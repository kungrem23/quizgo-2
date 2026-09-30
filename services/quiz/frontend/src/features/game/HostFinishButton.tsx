import { useState } from 'react';
import { Square } from 'lucide-react';
import { Button, ErrorBox, Modal } from '../../shared/ui/ui';
import { GameCommandError, type PendingGameCommand } from './socket';
import type { GameCommand } from './types';

type SendCommand = (command: GameCommand, requestId?: string) => PendingGameCommand;

function finishError(error: unknown): string {
  if (error instanceof GameCommandError && error.code === 'invalid_phase') {
    return 'Игра уже завершена или перешла в другое состояние.';
  }
  return 'Не удалось завершить игру. Проверьте соединение и попробуйте ещё раз.';
}

export function HostFinishButton({
  sendCommand,
  disabled = false,
  className = '',
}: {
  sendCommand: SendCommand;
  disabled?: boolean;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');

  async function finish() {
    setPending(true);
    setError('');
    try {
      await sendCommand({ type: 'finish' }).acknowledged;
      setOpen(false);
    } catch (caught) {
      setError(finishError(caught));
    } finally {
      setPending(false);
    }
  }

  return (
    <>
      <Button
        variant="secondary"
        className={className}
        disabled={disabled}
        onClick={() => {
          setError('');
          setOpen(true);
        }}
      >
        <Square size={15} fill="currentColor" />
        Завершить игру
      </Button>
      <Modal
        open={open}
        onOpenChange={setOpen}
        title="Завершить игру досрочно?"
        description="Текущий счёт станет финальным. Продолжить эту игровую сессию будет нельзя."
        dismissible={!pending}
      >
        {error && <ErrorBox message={error} />}
        <div className="modal-actions">
          <Button variant="secondary" disabled={pending} onClick={() => setOpen(false)}>
            Продолжить игру
          </Button>
          <Button variant="danger" busy={pending} onClick={() => void finish()}>
            Завершить игру
          </Button>
        </div>
      </Modal>
    </>
  );
}
