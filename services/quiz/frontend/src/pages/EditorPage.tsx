import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router';
import { api, errorMessage, keys } from '../shared/api/client';
import { useAuth } from '../features/auth/AuthProvider';
import { EditorWorkspace } from '../features/quiz-editor/EditorWorkspace';
import { Button, ErrorBox, Loading } from '../shared/ui/ui';
import { SiteHeader } from '../shared/ui/SiteHeader';
export function EditorPage() {
  const { user } = useAuth();
  const id = Number(useParams().quizId);
  const valid = Number.isInteger(id) && id > 0;
  const query = useQuery({
    queryKey: keys.author(user!.id, id),
    queryFn: ({ signal }) => api.quiz(id, true, signal),
    enabled: valid,
    refetchOnWindowFocus: true,
  });
  if (query.data) return <EditorWorkspace key={`${user!.id}:${id}`} content={query.data} />;
  return (
    <div className="app-shell">
      <SiteHeader editor />
      <main className="preview-main">
        {!valid ? (
          <ErrorBox message="Некорректная ссылка на квиз." />
        ) : query.isPending ? (
          <Loading label="Готовим редактор…" />
        ) : (
          <ErrorBox message={errorMessage(query.error)}>
            <Button variant="secondary" onClick={() => void query.refetch()}>
              Повторить
            </Button>
            <Link to="/quizzes">Вернуться к моим квизам</Link>
          </ErrorBox>
        )}
      </main>
    </div>
  );
}
