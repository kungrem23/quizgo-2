import { useState, type FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { ArrowRight, Sparkles } from 'lucide-react';
import { api, errorMessage, keys } from '../../shared/api/client';
import { useAuth } from '../auth/AuthProvider';
import { Button, ErrorBox, Modal } from '../../shared/ui/ui';
import type { QuizVisibility } from '../../shared/api/types';
export function CreateQuizDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const [title, setTitle] = useState('');
  const [visibility, setVisibility] = useState<QuizVisibility>('private');
  const navigate = useNavigate();
  const client = useQueryClient();
  const { user } = useAuth();
  const mutation = useMutation({
    mutationFn: ({ name, access }: { name: string; access: QuizVisibility }) =>
      api.createQuiz(name, access),
    onSuccess: async (value) => {
      await client.invalidateQueries({ queryKey: keys.quizzes(user!.id) });
      setTitle('');
      setVisibility('private');
      onOpenChange(false);
      navigate(`/quizzes/${value.id}/edit`);
    },
  });
  function submit(e: FormEvent) {
    e.preventDefault();
    if (title.trim() && !mutation.isPending)
      mutation.mutate({ name: title.trim(), access: visibility });
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
        <label className="field-label" htmlFor="quiz-visibility">
          Доступ
        </label>
        <select
          id="quiz-visibility"
          value={visibility}
          onChange={(e) => setVisibility(e.target.value as QuizVisibility)}
          disabled={mutation.isPending}
        >
          <option value="private">Приватный — доступен только вам</option>
          <option value="public">Публичный — доступен по ссылке</option>
        </select>
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
