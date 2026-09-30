import { useMutation } from '@tanstack/react-query';
import { Play, Radio, Users } from 'lucide-react';
import { useNavigate } from 'react-router';
import { Button, ErrorBox } from '../../shared/ui/ui';
import { pluralQuestions } from '../quizzes/presentation';
import { hostLaunchErrorMessage, launchHostGame } from './launch';

export function HostLaunchCard({
  quizId,
  questionCount,
}: {
  quizId: number;
  questionCount: number;
}) {
  const navigate = useNavigate();
  const launch = useMutation({
    mutationFn: () => launchHostGame(quizId),
    onSuccess: (game) => {
      void navigate(`/games/${encodeURIComponent(game.id)}/host`);
    },
  });

  return (
    <section className="realtime-launch-card" aria-labelledby="realtime-launch-title">
      <div className="realtime-launch-copy">
        <span className="realtime-launch-icon">
          <Radio size={22} />
        </span>
        <div>
          <h2 id="realtime-launch-title">Синхронная игра в реальном времени</h2>
          <p>Участники подключатся со смартфонов по коду или QR-ссылке.</p>
        </div>
      </div>
      <div className="realtime-launch-meta">
        <span>
          <Users size={16} />
          До 100 игроков
        </span>
        <span>{pluralQuestions(questionCount)}</span>
      </div>
      {launch.error && <ErrorBox message={hostLaunchErrorMessage(launch.error)} />}
      {questionCount === 0 && (
        <p className="realtime-launch-warning">Добавьте хотя бы один вопрос перед запуском.</p>
      )}
      <div className="realtime-launch-actions">
        <Button
          busy={launch.isPending}
          disabled={questionCount === 0}
          onClick={() => launch.mutate()}
        >
          <Play size={17} fill="currentColor" />
          Запустить игру
        </Button>
        <span>Редактирование квиза не повлияет на уже созданную игровую сессию.</span>
      </div>
    </section>
  );
}
