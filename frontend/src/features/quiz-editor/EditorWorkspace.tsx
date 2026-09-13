import { useCallback, useEffect, useReducer, useRef, useState } from 'react';
import { useBlocker, useSearchParams } from 'react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Check, Eye, GripVertical, List, Plus, Save, Sparkles, X } from 'lucide-react';
import { api, ApiError, errorMessage, keys } from '../../shared/api/client';
import type { QuizContent } from '../../shared/api/types';
import { Button, ErrorBox, Modal, useToast } from '../../shared/ui/ui';
import { SiteHeader } from '../../shared/ui/SiteHeader';
import { useAuth } from '../auth/AuthProvider';
import { QuizOverview } from '../quiz-preview/QuizOverview';
import { QuestionForm } from './QuestionForm';
import {
  draftStorageKey,
  fingerprint,
  newQuestion,
  payload,
  readDraft,
  reducer,
  toDraft,
  validateDraft,
  type DraftQuestion,
} from './model';
function SidebarQuestion({
  question,
  index,
  selected,
  onSelect,
  disabled,
}: {
  question: DraftQuestion;
  index: number;
  selected: boolean;
  onSelect: () => void;
  disabled: boolean;
}) {
  const sortable = useSortable({ id: question.key, disabled });
  return (
    <div
      ref={sortable.setNodeRef}
      style={{
        transform: CSS.Transform.toString(sortable.transform),
        transition: sortable.transition,
      }}
      className={`sidebar-question ${selected ? 'selected' : ''} ${sortable.isDragging ? 'dragging' : ''}`}
    >
      <button
        className="drag-handle"
        disabled={disabled}
        {...sortable.attributes}
        {...sortable.listeners}
        aria-label={`Переместить вопрос ${index + 1}`}
      >
        <GripVertical size={15} />
      </button>
      <button
        className="sidebar-select"
        aria-label={`Вопрос ${index + 1}: ${question.text_content || 'Новый вопрос'}`}
        disabled={disabled}
        onClick={onSelect}
        aria-current={selected ? 'true' : undefined}
      >
        <span>{index + 1}</span>
        <span>{question.text_content || 'Новый вопрос'}</span>
      </button>
    </div>
  );
}
export function EditorWorkspace({ content }: { content: QuizContent }) {
  const { user } = useAuth();
  const client = useQueryClient();
  const toast = useToast();
  const [params, setParams] = useSearchParams();
  const storageKey = draftStorageKey(user!.id, content.id);
  const [state, dispatch] = useReducer(
    reducer,
    content,
    (value) => readDraft(storageKey) || toDraft(value),
  );
  const [baseline, setBaseline] = useState(() => fingerprint(toDraft(content)));
  const requested = params.get('question');
  const [selected, setSelected] = useState(
    () =>
      state.questions.find((q) => String(q.id) === requested || q.key === requested)?.key ||
      state.questions[0]?.key ||
      '',
  );
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [preview, setPreview] = useState(false);
  const [previewQuestion, setPreviewQuestion] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [discard, setDiscard] = useState(false);
  const [error, setError] = useState('');
  const [uploading, setUploading] = useState(false);
  const [storageError, setStorageError] = useState(false);
  const dirty = fingerprint(state) !== baseline;
  const dirtyRef = useRef(dirty);
  dirtyRef.current = dirty;
  const conflict = state.revision !== content.revision;
  const index = Math.max(
    0,
    state.questions.findIndex((q) => q.key === selected),
  );
  const current = state.questions[index];
  const mutation = useMutation({
    mutationFn: () => api.saveQuiz(content.id, payload(state)),
    onError: (error) => {
      if (error instanceof ApiError && error.status === 409)
        void client.invalidateQueries({ queryKey: keys.author(user!.id, content.id) });
    },
    onSuccess: async (value) => {
      const next = toDraft(value);
      dispatch({ type: 'replace', value: next });
      setBaseline(fingerprint(next));
      setSelected(next.questions[index]?.key || next.questions[0]?.key || '');
      setError('');
      client.setQueryData(keys.author(user!.id, content.id), value);
      await Promise.all([
        client.invalidateQueries({ queryKey: keys.quizzes(user!.id) }),
        client.invalidateQueries({ queryKey: keys.quiz(content.id) }),
      ]);
      toast('Все изменения сохранены');
    },
  });
  const busy = mutation.isPending || uploading;
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      (dirty || busy) && currentLocation.pathname !== nextLocation.pathname,
  );
  useEffect(() => {
    const before = (e: BeforeUnloadEvent) => {
      if (dirtyRef.current) {
        e.preventDefault();
        e.returnValue = '';
      }
    };
    window.addEventListener('beforeunload', before);
    return () => window.removeEventListener('beforeunload', before);
  }, []);
  useEffect(() => {
    try {
      if (dirty) sessionStorage.setItem(storageKey, JSON.stringify(state));
      else sessionStorage.removeItem(storageKey);
      setStorageError(false);
    } catch {
      setStorageError(true);
    }
  }, [state, dirty, storageKey]);
  useEffect(() => {
    if (!dirty && content.revision !== state.revision) {
      const next = toDraft(content);
      dispatch({ type: 'replace', value: next });
      setBaseline(fingerprint(next));
    }
  }, [content, dirty, state.revision]);
  useEffect(() => {
    if (requested) {
      const q = state.questions.find((q) => String(q.id) === requested || q.key === requested);
      if (q) setSelected(q.key);
    }
  }, [requested, state.questions]);
  function select(key: string) {
    setSelected(key);
    setSidebarOpen(false);
    const q = state.questions.find((q) => q.key === key);
    setParams(
      (p) => {
        p.set('question', q?.id ? String(q.id) : key);
        return p;
      },
      { replace: true },
    );
  }
  function add() {
    if (state.questions.length >= 100) return;
    const q = newQuestion();
    dispatch({ type: 'add', question: q });
    setSelected(q.key);
    setParams(
      (p) => {
        p.set('question', q.key);
        return p;
      },
      { replace: true },
    );
    setSidebarOpen(false);
  }
  function save() {
    if (busy) return;
    const issue = validateDraft(state);
    if (issue) {
      setError(issue.message);
      if (issue.questionKey) select(issue.questionKey);
      return;
    }
    setError('');
    mutation.mutate();
  }
  const saveRef = useRef(save);
  saveRef.current = save;
  useEffect(() => {
    const keydown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
        e.preventDefault();
        if (dirtyRef.current) saveRef.current();
      }
    };
    window.addEventListener('keydown', keydown);
    return () => window.removeEventListener('keydown', keydown);
  }, []);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const onUpload = useCallback((value: boolean) => setUploading(value), []);
  const previewContent: QuizContent = {
    ...content,
    ...payload(state),
    question_count: state.questions.length,
    questions: payload(state).questions.map((q, i) => ({ ...q, id: q.id || -(i + 1) })),
  };
  return (
    <div className="editor-shell">
      <SiteHeader editor>
        <div className="editor-title-wrap">
          <input
            className="editor-title-input"
            aria-label="Название квиза"
            value={state.title}
            onChange={(e) => dispatch({ type: 'title', value: e.target.value })}
            maxLength={50}
            disabled={busy}
          />
        </div>
        <div className="editor-actions">
          <span className={`save-status ${dirty ? 'has-changes' : ''}`} role="status">
            {busy ? (
              'Сохраняем…'
            ) : dirty ? (
              <>
                <span />
                Есть изменения
              </>
            ) : (
              <>
                <Check size={14} />
                Сохранено
              </>
            )}
          </span>
          <Button variant="secondary" onClick={() => setPreview(true)} disabled={busy}>
            <Eye size={16} />
            <span>Просмотр</span>
          </Button>
          <Button busy={mutation.isPending} disabled={!dirty || busy || conflict} onClick={save}>
            <Save size={16} />
            <span>Сохранить</span>
          </Button>
        </div>
      </SiteHeader>
      <div className="editor-mobile-toolbar">
        <Button variant="secondary" onClick={() => setSidebarOpen(!sidebarOpen)}>
          <List size={16} />
          Вопросы ({state.questions.length})
        </Button>
        <span>{current ? `Вопрос ${index + 1} из ${state.questions.length}` : 'Новый квиз'}</span>
      </div>
      <div className="editor-body">
        {sidebarOpen && (
          <button
            className="sidebar-backdrop"
            aria-label="Закрыть список вопросов"
            onClick={() => setSidebarOpen(false)}
          />
        )}
        <aside className={`editor-sidebar ${sidebarOpen ? 'sidebar-open' : ''}`}>
          <div className="sidebar-heading">
            <h2>
              Вопросы<span>{state.questions.length}</span>
            </h2>
            <button
              className="icon-button sidebar-close"
              aria-label="Закрыть список"
              onClick={() => setSidebarOpen(false)}
            >
              <X size={18} />
            </button>
          </div>
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            onDragEnd={({ active, over }) => {
              if (over) dispatch({ type: 'reorder', from: String(active.id), to: String(over.id) });
            }}
          >
            <SortableContext
              items={state.questions.map((q) => q.key)}
              strategy={verticalListSortingStrategy}
            >
              <div className="sidebar-questions">
                {state.questions.map((q, i) => (
                  <SidebarQuestion
                    key={q.key}
                    question={q}
                    index={i}
                    selected={current?.key === q.key}
                    disabled={busy}
                    onSelect={() => select(q.key)}
                  />
                ))}
              </div>
            </SortableContext>
          </DndContext>
          <Button
            className="add-question"
            variant="secondary"
            onClick={add}
            disabled={busy || state.questions.length >= 100}
          >
            <Plus size={18} />
            Добавить вопрос
          </Button>
          <div className="sidebar-note">
            <Sparkles size={18} />
            <p>
              Хороший вопрос —<br />
              начало большого открытия.
            </p>
          </div>
        </aside>
        <main className="editor-main">
          {(error || mutation.error) && (
            <ErrorBox message={error || errorMessage(mutation.error)} />
          )}{' '}
          {storageError && (
            <ErrorBox message="Браузер не смог сохранить черновик. Сохраните квиз перед закрытием вкладки." />
          )}
          {conflict && dirty && (
            <ErrorBox message="На сервере есть новая версия квиза. Ваши изменения сохранены в черновике.">
              <Button variant="secondary" onClick={() => setDiscard(true)}>
                Загрузить актуальную версию
              </Button>
            </ErrorBox>
          )}
          {current ? (
            <QuestionForm
              key={current.key}
              question={current}
              index={index}
              disabled={busy}
              onChange={(value) => dispatch({ type: 'question', key: current.key, value })}
              onDelete={() => setDeleting(true)}
              onUploadState={onUpload}
            />
          ) : (
            <div className="editor-empty">
              <div className="empty-art">
                <Sparkles size={42} />
              </div>
              <span className="eyebrow">С ЧЕГО НАЧНЁМ?</span>
              <h1>
                Один вопрос.
                <br />
                Тысяча открытий.
              </h1>
              <p>
                Добавьте первый вопрос, придумайте варианты
                <br />
                ответа — и ваш квиз оживёт.
              </p>
              <Button onClick={add}>
                <Plus size={18} />
                Добавить первый вопрос
              </Button>
            </div>
          )}
          <div className="editor-bottom-note">
            <span>Ваши идеи заслуживают хороших вопросов.</span>
            <span>⌘ / Ctrl + S — сохранить</span>
          </div>
        </main>
      </div>
      <Modal
        open={preview}
        onOpenChange={setPreview}
        title="Предпросмотр"
        description="Так выглядит текущий черновик. Чтобы поделиться изменениями, сохраните квиз."
        className="preview-modal"
      >
        <QuizOverview
          quiz={previewContent}
          authorName={user!.username}
          isAuthor
          draft
          selectedId={previewQuestion}
          onSelect={setPreviewQuestion}
          onEdit={(qid) => {
            setPreview(false);
            if (qid) {
              const i = previewContent.questions.findIndex((q) => q.id === qid);
              if (i >= 0) select(state.questions[i].key);
            }
          }}
        />
      </Modal>
      <Modal
        open={deleting}
        onOpenChange={setDeleting}
        title="Убрать этот вопрос?"
        description="Вопрос и его ответы будут удалены из черновика. Изменение вступит в силу после сохранения."
      >
        <div className="modal-actions">
          <Button variant="secondary" onClick={() => setDeleting(false)}>
            Оставить
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              if (current) {
                dispatch({ type: 'remove', key: current.key });
                setSelected(
                  state.questions[index + 1]?.key || state.questions[index - 1]?.key || '',
                );
                setParams(
                  (p) => {
                    p.delete('question');
                    return p;
                  },
                  { replace: true },
                );
              }
              setDeleting(false);
            }}
          >
            Убрать вопрос
          </Button>
        </div>
      </Modal>
      <Modal
        open={blocker.state === 'blocked'}
        onOpenChange={(v) => {
          if (!v && blocker.state === 'blocked') blocker.reset();
        }}
        title="Есть несохранённые изменения"
        description={
          busy
            ? 'Дождитесь завершения сохранения или загрузки.'
            : 'Можно остаться и сохранить квиз. Если уйти, черновик останется в этой вкладке до выхода из аккаунта.'
        }
      >
        <div className="modal-actions">
          <Button
            variant="secondary"
            onClick={() => {
              if (blocker.state === 'blocked') blocker.reset();
            }}
          >
            Остаться
          </Button>
          <Button
            disabled={busy}
            onClick={() => {
              if (blocker.state === 'blocked') blocker.proceed();
            }}
          >
            Уйти со страницы
          </Button>
        </div>
      </Modal>
      <Modal
        open={discard}
        onOpenChange={setDiscard}
        title="Загрузить версию с сервера?"
        description="Локальные правки этого квиза будут заменены актуальной сохранённой версией."
      >
        <div className="modal-actions">
          <Button variant="secondary" onClick={() => setDiscard(false)}>
            Оставить черновик
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              const next = toDraft(content);
              dispatch({ type: 'replace', value: next });
              setBaseline(fingerprint(next));
              setSelected(next.questions[0]?.key || '');
              setError('');
              mutation.reset();
              setDiscard(false);
            }}
          >
            Заменить черновик
          </Button>
        </div>
      </Modal>
    </div>
  );
}
