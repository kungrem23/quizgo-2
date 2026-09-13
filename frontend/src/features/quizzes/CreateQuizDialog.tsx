import { useState, type FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { ArrowRight, Sparkles } from 'lucide-react';
import { api, errorMessage, keys } from '../../shared/api/client';
import { useAuth } from '../auth/AuthProvider';
import { Button, ErrorBox, Modal } from '../../shared/ui/ui';
export function CreateQuizDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const [title, setTitle] = useState('');
  const navigate = useNavigate();
  const client = useQueryClient();
  const { user } = useAuth();
  const mutation = useMutation({
    mutationFn: api.createQuiz,
    onSuccess: async (value) => {
      await client.invalidateQueries({ queryKey: keys.quizzes(user!.id) });
      setTitle('');
      onOpenChange(false);
      navigate(`/quizzes/${value.id}/edit`);
    },
  });
  function submit(e: FormEvent) {
    e.preventDefault();
    if (title.trim() && !mutation.isPending) mutation.mutate(title.trim());
  }
  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!mutation.isPending) {
          mutation.reset();
          onOpenChange(v);
        }
      }}
      title="Всё начинается с идеи"
      description="Дайте квизу имя. Вопросы добавим на следующем шаге."
      className="create-modal"
    >
      <div className="modal-emblem">
        <Sparkles size={27} />
      </div>
      <form onSubmit={submit}>
        <label className="field-label" htmlFor="quiz-name">
          Название квиза
        </label>
        <input
          id="quiz-name"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Например, Вокруг света за 10 вопросов"
          maxLength={50}
          required
          disabled={mutation.isPending}
        />
        <div className="field-hint">
          Можно изменить в любое время<span>{title.length}/50</span>
        </div>
        {mutation.error && <ErrorBox message={errorMessage(mutation.error)} />}
        <div className="modal-actions">
          <Button
            variant="secondary"
            onClick={() => onOpenChange(false)}
            disabled={mutation.isPending}
          >
            Отмена
          </Button>
          <Button type="submit" busy={mutation.isPending} disabled={!title.trim()}>
            Создать квиз
            <ArrowRight size={17} />
          </Button>
        </div>
      </form>
    </Modal>
  );
}
