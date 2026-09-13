import { useState } from 'react';
import { Link, useSearchParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import * as Dropdown from '@radix-ui/react-dropdown-menu';
import {
  ArrowUpRight,
  Plus,
  Search,
  SlidersHorizontal,
  MoreHorizontal,
  Pencil,
  Trash2,
  Eye,
  Sparkles,
  X,
} from 'lucide-react';
import { api, errorMessage, keys } from '../shared/api/client';
import type { QuizSummary } from '../shared/api/types';
import { useAuth } from '../features/auth/AuthProvider';
import { CreateQuizDialog } from '../features/quizzes/CreateQuizDialog';
import { pluralQuestions, quizTheme, relativeDate } from '../features/quizzes/presentation';
import { Button, ErrorBox, Modal, useToast } from '../shared/ui/ui';
import { Footer, SiteHeader } from '../shared/ui/SiteHeader';
export function QuizzesPage() {
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const client = useQueryClient();
  const toast = useToast();
  const [deleting, setDeleting] = useState<QuizSummary | null>(null);
  const query = useQuery({
    queryKey: keys.quizzes(user!.id),
    queryFn: ({ signal }) => api.myQuizzes(signal),
  });
  const search = params.get('q') || '';
  const sort = params.get('sort') || 'recent';
  const all = query.data || [];
  const filtered = all
    .filter((q) => q.title.toLocaleLowerCase('ru').includes(search.toLocaleLowerCase('ru')))
    .sort((a, b) =>
      sort === 'title'
        ? a.title.localeCompare(b.title, 'ru')
        : sort === 'questions'
          ? b.question_count - a.question_count || a.title.localeCompare(b.title, 'ru')
          : Date.parse(b.updated_at) - Date.parse(a.updated_at),
    );
  function param(key: string, value: string, replace = false) {
    setParams(
      (p) => {
        if (value) p.set(key, value);
        else p.delete(key);
        return p;
      },
      { replace },
    );
  }
  const deletion = useMutation({
    mutationFn: api.deleteQuiz,
    onSuccess: async (_, id) => {
      client.removeQueries({ queryKey: keys.author(user!.id, id) });
      client.removeQueries({ queryKey: keys.quiz(id) });
      await client.invalidateQueries({ queryKey: keys.quizzes(user!.id) });
      setDeleting(null);
      toast('Квиз удалён');
    },
  });
  return (
    <div className="app-shell">
      <SiteHeader />
      <main className="quizzes-main">
        <div className="page-heading">
          <div>
            <span className="eyebrow">
              <span />
              ВАША КОЛЛЕКЦИЯ ИДЕЙ
            </span>
            <h1>
              Мои квизы<span className="heading-count">{all.length}</span>
            </h1>
            <p>Создавайте, делитесь и открывайте новое вместе.</p>
          </div>
          <Button onClick={() => param('create', '1')}>
            <Plus size={19} />
            Создать квиз
          </Button>
        </div>
        <div className="library-toolbar">
          <div className="input-icon search-field">
            <Search size={19} />
            <input
              aria-label="Поиск квизов"
              placeholder="Найти тот самый квиз…"
              value={search}
              onChange={(e) => param('q', e.target.value, true)}
            />
            {search && (
              <button
                className="icon-button"
                aria-label="Очистить поиск"
                onClick={() => param('q', '', true)}
              >
                <X size={16} />
              </button>
            )}
          </div>
          <div className="sort-control">
            <SlidersHorizontal size={16} />
            <select
              aria-label="Сортировка квизов"
              value={sort}
              onChange={(e) => param('sort', e.target.value)}
            >
              <option value="recent">Сначала новые</option>
              <option value="title">По названию</option>
              <option value="questions">По числу вопросов</option>
            </select>
          </div>
        </div>
        {query.isPending ? (
          <div className="quiz-grid" aria-label="Загрузка квизов" aria-busy="true">
            {Array.from({ length: 6 }, (_, i) => (
              <div key={i} className="quiz-skeleton" />
            ))}
          </div>
        ) : query.error ? (
          <ErrorBox message={errorMessage(query.error)}>
            <Button variant="secondary" onClick={() => void query.refetch()}>
              Попробовать снова
            </Button>
          </ErrorBox>
        ) : all.length === 0 ? (
          <div className="empty-library">
            <div className="empty-art">
              <Sparkles size={42} />
              <span>✦</span>
              <span>?</span>
            </div>
            <span className="eyebrow">МЕСТО ДЛЯ ВАШИХ ОТКРЫТИЙ</span>
            <h2>Какой будет ваша первая тема?</h2>
            <p>
              Любимое кино, далёкие страны или основы Go.
              <br />
              Начните с идеи — всё остальное сложится.
            </p>
            <Button onClick={() => param('create', '1')}>
              <Plus size={18} />
              Создать первый квиз
            </Button>
            <div className="topic-chips">
              <span>🌍 Вокруг света</span>
              <span>💻 Немного кода</span>
              <span>🎬 Киномания</span>
            </div>
          </div>
        ) : filtered.length === 0 ? (
          <div className="empty-search">
            <Search size={32} />
            <h2>Такого квиза пока нет</h2>
            <p>Попробуйте другое название или создайте новый.</p>
            <Button variant="secondary" onClick={() => param('q', '', true)}>
              Сбросить поиск
            </Button>
          </div>
        ) : (
          <div className="quiz-grid">
            {filtered.map((q) => {
              const { name, Icon, symbol } = quizTheme(q.id, q.title);
              return (
                <article className={`quiz-card theme-${name}`} key={q.id}>
                  <span className="card-watermark" aria-hidden="true">
                    {symbol}
                  </span>
                  <div className="card-top">
                    <span className="quiz-icon">
                      <Icon size={34} strokeWidth={1.6} />
                    </span>
                    <Dropdown.Root>
                      <Dropdown.Trigger
                        className="card-menu"
                        aria-label={`Действия с квизом ${q.title}`}
                      >
                        <MoreHorizontal size={21} />
                      </Dropdown.Trigger>
                      <Dropdown.Portal>
                        <Dropdown.Content className="dropdown" align="end" sideOffset={6}>
                          <Dropdown.Item asChild>
                            <Link className="dropdown-item" to={`/quizzes/${q.id}`}>
                              <Eye size={16} />
                              Просмотреть
                            </Link>
                          </Dropdown.Item>
                          <Dropdown.Item asChild>
                            <Link className="dropdown-item" to={`/quizzes/${q.id}/edit`}>
                              <Pencil size={16} />
                              Редактировать
                            </Link>
                          </Dropdown.Item>
                          <Dropdown.Separator className="dropdown-separator" />
                          <Dropdown.Item
                            className="dropdown-item text-danger"
                            onSelect={() => {
                              deletion.reset();
                              setDeleting(q);
                            }}
                          >
                            <Trash2 size={16} />
                            Удалить
                          </Dropdown.Item>
                        </Dropdown.Content>
                      </Dropdown.Portal>
                    </Dropdown.Root>
                  </div>
                  <Link className="card-title" to={`/quizzes/${q.id}`}>
                    {q.title}
                  </Link>
                  <span className="card-count">{pluralQuestions(q.question_count)}</span>
                  <div className="card-footer">
                    <span>Обновлён {relativeDate(q.updated_at)}</span>
                    <ArrowUpRight size={17} />
                  </div>
                </article>
              );
            })}
            <button className="create-card" onClick={() => param('create', '1')}>
              <span className="create-card-plus">
                <Plus size={26} />
              </span>
              <strong>Новая идея?</strong>
              <span>
                Превратите её в квиз
                <ArrowUpRight size={14} />
              </span>
            </button>
          </div>
        )}
        {all.length > 0 && (
          <div className="library-tip">
            <Sparkles size={18} />
            <span>Самые интересные открытия начинаются с хорошего вопроса.</span>
          </div>
        )}
      </main>
      <Footer />
      <CreateQuizDialog
        open={params.get('create') === '1'}
        onOpenChange={(v) => param('create', v ? '1' : '')}
      />
      <Modal
        open={!!deleting}
        onOpenChange={(v) => {
          if (!v && !deletion.isPending) setDeleting(null);
        }}
        title="Удалить квиз?"
        description={`«${deleting?.title || ''}» и все его вопросы будут удалены. Это действие нельзя отменить.`}
      >
        {deletion.error && <ErrorBox message={errorMessage(deletion.error)} />}
        <div className="modal-actions">
          <Button
            variant="secondary"
            onClick={() => setDeleting(null)}
            disabled={deletion.isPending}
          >
            Оставить
          </Button>
          <Button
            variant="danger"
            busy={deletion.isPending}
            onClick={() => deleting && deletion.mutate(deleting.id)}
          >
            Удалить квиз
          </Button>
        </div>
      </Modal>
    </div>
  );
}
