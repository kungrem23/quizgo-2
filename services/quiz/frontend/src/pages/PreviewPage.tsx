import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router';
import { ArrowLeft } from 'lucide-react';
import { api, errorMessage, keys } from '../shared/api/client';
import { useAuth } from '../features/auth/AuthProvider';
import { QuizOverview } from '../features/quiz-preview/QuizOverview';
import { Button, ErrorBox, Loading } from '../shared/ui/ui';
import { Footer, SiteHeader } from '../shared/ui/SiteHeader';
export function PreviewPage() {
  const id = Number(useParams().quizId);
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const query = useQuery({
    queryKey: keys.quiz(id),
    queryFn: ({ signal }) => api.quiz(id, false, signal),
    enabled: Number.isInteger(id) && id > 0,
  });
  const isAuthor = !!user && query.data?.author_id === user.id;
  const authorQuery = useQuery({
    queryKey: keys.author(user?.id || 0, id),
    queryFn: ({ signal }) => api.quiz(id, true, signal),
    enabled: isAuthor,
  });
  const author = useQuery({
    queryKey: ['user', query.data?.author_id],
    queryFn: () => api.user(query.data!.author_id),
    enabled: !!query.data,
  });
  return (
    <div className="app-shell">
      <SiteHeader />
      <main className="preview-main">
        <Link to="/quizzes" className="back-link">
          <ArrowLeft size={16} />К моим квизам
        </Link>
        {!Number.isInteger(id) || id < 1 ? (
          <ErrorBox message="Некорректная ссылка на квиз." />
        ) : query.isPending ? (
          <Loading label="Открываем квиз…" />
        ) : query.error ? (
          <ErrorBox message={errorMessage(query.error)}>
            <Button variant="secondary" onClick={() => void query.refetch()}>
              Повторить
            </Button>
          </ErrorBox>
        ) : (
          <>
            {authorQuery.error && <ErrorBox message={errorMessage(authorQuery.error)} />}
            <QuizOverview
              quiz={authorQuery.data || query.data}
              authorName={author.data?.username || '…'}
              isAuthor={isAuthor}
              selectedId={params.has('question') ? Number(params.get('question')) : null}
              onSelect={(value) =>
                setParams((p) => {
                  if (value === null) p.delete('question');
                  else p.set('question', String(value));
                  return p;
                })
              }
              onEdit={(qid) => navigate(`/quizzes/${id}/edit${qid ? `?question=${qid}` : ''}`)}
            />
          </>
        )}
      </main>
      <Footer />
    </div>
  );
}
