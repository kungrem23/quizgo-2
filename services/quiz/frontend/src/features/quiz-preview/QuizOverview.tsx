import { useState, type ReactNode } from 'react';
import {
  BookOpen,
  Check,
  ChevronDown,
  ChevronRight,
  Clock3,
  Copy,
  Pencil,
  Share2,
  Sparkles,
} from 'lucide-react';
import type { QuizContent } from '../../shared/api/types';
import { Button, Modal, useToast } from '../../shared/ui/ui';
import { QuizImage } from '../../shared/ui/QuizImage';
import { pluralQuestions, quizTheme } from '../quizzes/presentation';
export const answerSymbols = ['▲', '◆', '●', '■', '✦', '⬟', '✚', '★'];
export function QuizOverview({
  quiz,
  authorName,
  isAuthor,
  onEdit,
  selectedId,
  onSelect,
  draft = false,
  children,
}: {
  quiz: QuizContent;
  authorName: string;
  isAuthor: boolean;
  onEdit: (questionId?: number) => void;
  selectedId: number | null;
  onSelect: (id: number | null) => void;
  draft?: boolean;
  children?: ReactNode;
}) {
  const [showAll, setShowAll] = useState(false);
  const [shareFallback, setShareFallback] = useState(false);
  const toast = useToast();
  const { name, Icon } = quizTheme(quiz.id, quiz.title);
  const questions = quiz.questions || [];
  const selected = questions.find((q) => q.id === selectedId);
  const selectedIndex = questions.findIndex((q) => q.id === selectedId);
  const url = `${window.location.origin}/quizzes/${quiz.id}`;
  async function share() {
    try {
      await navigator.clipboard.writeText(url);
      toast('Ссылка на квиз скопирована');
    } catch {
      setShareFallback(true);
    }
  }
  return (
    <>
      <div className={`overview-hero theme-${name}`}>
        <div className="hero-decoration" aria-hidden="true">
          <span className="hero-orbit" />
          <span className="hero-orbit hero-orbit-two" />
          <span className="hero-spark">✦</span>
          <span className="hero-spark hero-spark-two">✦</span>
          <span className="hero-symbol">?</span>
        </div>
        <span className="overview-icon">
          <Icon size={40} strokeWidth={1.5} />
        </span>
        <span className="eyebrow">
          {draft ? 'ПРЕДПРОСМОТР ЧЕРНОВИКА' : 'ВРЕМЯ ДЛЯ НОВЫХ ОТКРЫТИЙ'}
        </span>
        <h1>{quiz.title || 'Квиз без названия'}</h1>
        <p>
          {pluralQuestions(questions.length)}
          <span>·</span>автор {authorName}
        </p>
        <div className="hero-actions">
          {isAuthor && (
            <Button onClick={() => onEdit()}>
              <Pencil size={17} />
              Редактировать
            </Button>
          )}
          {!draft && (
            <Button variant="secondary" onClick={() => void share()}>
              <Share2 size={17} />
              Поделиться
            </Button>
          )}
        </div>
      </div>
      {children}
      <section className="overview-questions">
        <div className="section-heading">
          <h2>
            Что внутри<span>{questions.length}</span>
          </h2>
          <span>
            <BookOpen size={15} />
            Вопросы квиза
          </span>
        </div>
        {questions.length === 0 ? (
          <div className="empty-questions">
            <Sparkles size={30} />
            <h3>Первый вопрос ещё впереди</h3>
            <p>
              {isAuthor
                ? 'Откройте редактор и начните свою историю.'
                : 'Автор пока не добавил вопросы.'}
            </p>
            {isAuthor && (
              <Button variant="secondary" onClick={() => onEdit()}>
                Добавить вопросы
              </Button>
            )}
          </div>
        ) : (
          <div className="question-list">
            {(showAll ? questions : questions.slice(0, 5)).map((q, i) => (
              <button className="overview-question" key={q.id} onClick={() => onSelect(q.id)}>
                <span className="question-number">{String(i + 1).padStart(2, '0')}</span>
                <span className="overview-question-text">{q.text_content || 'Новый вопрос'}</span>
                <span className="question-meta">
                  {q.answers.length} отв.<span>·</span>
                  {q.time_limit} сек.
                </span>
                <ChevronRight size={17} />
              </button>
            ))}
          </div>
        )}
        {questions.length > 5 && (
          <button className="show-all" onClick={() => setShowAll(!showAll)}>
            {showAll ? 'Свернуть список' : `Показать все ${pluralQuestions(questions.length)}`}
            <ChevronDown size={16} className={showAll ? 'rotate-180' : ''} />
          </button>
        )}
      </section>
      <Modal
        open={!!selected}
        onOpenChange={(v) => {
          if (!v) onSelect(null);
        }}
        title={`Вопрос ${selectedIndex + 1}`}
        className="question-drawer"
      >
        {selected && (
          <>
            <div className="drawer-scroll">
              <h3>{selected.text_content || 'Новый вопрос'}</h3>
              {selected.image_id && (
                <QuizImage
                  className="question-image"
                  imageId={selected.image_id}
                  url={selected.image_url}
                  alt="Иллюстрация к вопросу"
                />
              )}
              <div className="preview-answers">
                {selected.answers.map((a, i) => (
                  <div
                    key={a.id || i}
                    className={`preview-answer ${isAuthor && a.is_correct ? 'answer-is-correct' : ''}`}
                  >
                    <span className={`answer-symbol answer-color-${i % 4}`}>
                      {answerSymbols[i]}
                    </span>
                    <span>{a.text_content || `Ответ ${i + 1}`}</span>
                    {isAuthor && a.is_correct && (
                      <Check className="correct-check" size={19} aria-label="Правильный ответ" />
                    )}
                  </div>
                ))}
              </div>
              {isAuthor && (
                <p className="author-hint">
                  <Check size={14} />
                  Правильный ответ виден только вам
                </p>
              )}
            </div>
            <div className="drawer-footer">
              <span>
                <Clock3 size={16} />
                {selected.time_limit} секунд
              </span>
              {isAuthor && (
                <Button
                  variant="secondary"
                  onClick={() => {
                    onSelect(null);
                    onEdit(selected.id);
                  }}
                >
                  <Pencil size={16} />
                  Изменить
                </Button>
              )}
            </div>
          </>
        )}
      </Modal>
      <Modal
        open={shareFallback}
        onOpenChange={setShareFallback}
        title="Поделиться квизом"
        description="Скопируйте ссылку и отправьте друзьям."
      >
        <div className="input-icon">
          <Copy size={18} />
          <input
            value={url}
            readOnly
            aria-label="Ссылка на квиз"
            onFocus={(e) => e.target.select()}
          />
        </div>
      </Modal>
    </>
  );
}
