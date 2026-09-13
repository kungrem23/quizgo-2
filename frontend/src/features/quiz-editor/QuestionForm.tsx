import { useRef, useState } from 'react';
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
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import {
  Check,
  Clock3,
  GripVertical,
  ImagePlus,
  Plus,
  Trash2,
  X,
  LoaderCircle,
} from 'lucide-react';
import { api, errorMessage } from '../../shared/api/client';
import { Button, ErrorBox } from '../../shared/ui/ui';
import { QuizImage } from '../../shared/ui/QuizImage';
import { answerSymbols } from '../quiz-preview/QuizOverview';
import { newAnswer, type DraftAnswer, type DraftQuestion } from './model';
function AnswerRow({
  answer,
  index,
  disabled,
  canDelete,
  onChange,
  onCorrect,
  onDelete,
}: {
  answer: DraftAnswer;
  index: number;
  disabled: boolean;
  canDelete: boolean;
  onChange: (v: string) => void;
  onCorrect: () => void;
  onDelete: () => void;
}) {
  const sortable = useSortable({ id: answer.key, disabled });
  return (
    <div
      ref={sortable.setNodeRef}
      style={{
        transform: CSS.Transform.toString(sortable.transform),
        transition: sortable.transition,
      }}
      className={`answer-row ${answer.is_correct ? 'answer-row-correct' : ''} ${sortable.isDragging ? 'dragging' : ''}`}
    >
      <button
        type="button"
        className="drag-handle"
        {...sortable.attributes}
        {...sortable.listeners}
        disabled={disabled}
        aria-label={`Переместить ответ ${index + 1}`}
      >
        <GripVertical size={17} />
      </button>
      <span className={`answer-symbol answer-color-${index % 4}`}>{answerSymbols[index]}</span>
      <input
        aria-label={`Ответ ${index + 1}`}
        value={answer.text_content}
        onChange={(e) => onChange(e.target.value)}
        maxLength={200}
        placeholder={`Вариант ответа ${index + 1}`}
        disabled={disabled}
      />
      <label className="correct-choice" title="Отметить правильным">
        <input
          type="radio"
          name="correct-answer"
          aria-label={`Ответ ${index + 1} — правильный`}
          checked={answer.is_correct}
          onChange={onCorrect}
          disabled={disabled}
        />
        {answer.is_correct && (
          <span>
            <Check size={12} />
            Верный
          </span>
        )}
      </label>
      <button
        type="button"
        className="icon-button delete-answer"
        aria-label={`Удалить ответ ${index + 1}`}
        onClick={onDelete}
        disabled={disabled || !canDelete}
      >
        <Trash2 size={16} />
      </button>
    </div>
  );
}
export function QuestionForm({
  question: q,
  index,
  disabled,
  onChange,
  onDelete,
  onUploadState,
}: {
  question: DraftQuestion;
  index: number;
  disabled: boolean;
  onChange: (value: Partial<DraftQuestion>) => void;
  onDelete: () => void;
  onUploadState: (value: boolean) => void;
}) {
  const fileRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [imageError, setImageError] = useState('');
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  async function upload(file?: File) {
    if (!file) return;
    setImageError('');
    if (!['image/jpeg', 'image/png'].includes(file.type)) {
      setImageError('Выберите изображение JPG или PNG.');
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setImageError('Максимальный размер изображения — 5 МБ.');
      return;
    }
    setUploading(true);
    onUploadState(true);
    try {
      const image = await api.uploadImage(file);
      onChange({ image_id: image.id, image_url: image.url });
    } catch (error) {
      setImageError(errorMessage(error));
    } finally {
      setUploading(false);
      onUploadState(false);
      if (fileRef.current) fileRef.current.value = '';
    }
  }
  return (
    <section className="question-form">
      <div className="question-form-heading">
        <div>
          <span className="eyebrow">СОБИРАЕМ ИСТОРИЮ ПО ВОПРОСАМ</span>
          <h2>Вопрос {index + 1}</h2>
        </div>
        <button
          className="icon-button delete-question"
          aria-label="Удалить вопрос"
          onClick={onDelete}
          disabled={disabled || uploading}
        >
          <Trash2 size={19} />
        </button>
      </div>
      <fieldset disabled={disabled} className="question-fields">
        <label className="field-label" htmlFor="question-text">
          Ваш вопрос<span aria-hidden="true">{q.text_content.length}/200</span>
        </label>
        <textarea
          id="question-text"
          value={q.text_content}
          onChange={(e) => onChange({ text_content: e.target.value })}
          placeholder="О чём спросим на этот раз?"
          maxLength={200}
          rows={2}
        />
        <label className="field-label image-label">
          Изображение<span>необязательно</span>
        </label>
        <input
          type="file"
          ref={fileRef}
          hidden
          accept="image/jpeg,image/png"
          aria-label="Выбрать изображение"
          disabled={uploading || disabled}
          onChange={(e) => void upload(e.target.files?.[0])}
        />
        {q.image_id ? (
          <div className="editor-image">
            <QuizImage imageId={q.image_id} url={q.image_url} alt="Изображение вопроса" />
            <button
              type="button"
              className="icon-button"
              aria-label="Убрать изображение"
              disabled={uploading || disabled}
              onClick={() => onChange({ image_id: '', image_url: undefined })}
            >
              <X size={18} />
            </button>
            <Button variant="secondary" busy={uploading} onClick={() => fileRef.current?.click()}>
              Заменить
            </Button>
          </div>
        ) : (
          <button
            type="button"
            className="image-upload"
            disabled={uploading}
            onClick={() => fileRef.current?.click()}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              if (!disabled && !uploading) void upload(e.dataTransfer.files[0]);
            }}
          >
            {uploading ? <LoaderCircle className="spin" size={25} /> : <ImagePlus size={25} />}
            <strong>{uploading ? 'Загружаем изображение…' : 'Добавить изображение'}</strong>
            <span>Перетащите файл сюда или нажмите · JPG, PNG до 5 МБ</span>
          </button>
        )}
        {imageError && <ErrorBox message={imageError} />}
        <div className="answers-heading">
          <label className="field-label">Варианты ответа</label>
          <span>Отметьте один правильный</span>
        </div>
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={({ active, over }) => {
            if (over && active.id !== over.id)
              onChange({
                answers: arrayMove(
                  q.answers,
                  q.answers.findIndex((a) => a.key === active.id),
                  q.answers.findIndex((a) => a.key === over.id),
                ),
              });
          }}
        >
          <SortableContext
            items={q.answers.map((a) => a.key)}
            strategy={verticalListSortingStrategy}
          >
            <div className="answers-editor">
              {q.answers.map((a, i) => (
                <AnswerRow
                  key={a.key}
                  answer={a}
                  index={i}
                  disabled={disabled}
                  canDelete={q.answers.length > 2}
                  onChange={(value) =>
                    onChange({
                      answers: q.answers.map((item) =>
                        item.key === a.key ? { ...item, text_content: value } : item,
                      ),
                    })
                  }
                  onCorrect={() =>
                    onChange({
                      answers: q.answers.map((item) => ({
                        ...item,
                        is_correct: item.key === a.key,
                      })),
                    })
                  }
                  onDelete={() =>
                    onChange({ answers: q.answers.filter((item) => item.key !== a.key) })
                  }
                />
              ))}
            </div>
          </SortableContext>
        </DndContext>
        <div className="question-form-bottom">
          <Button
            variant="secondary"
            disabled={q.answers.length >= 8}
            onClick={() => onChange({ answers: [...q.answers, newAnswer()] })}
          >
            <Plus size={16} />
            Добавить ответ
          </Button>
          <label className="time-control">
            <Clock3 size={17} />
            <span>Время на ответ</span>
            <select
              value={q.time_limit}
              onChange={(e) => onChange({ time_limit: Number(e.target.value) })}
              aria-label="Время на ответ"
            >
              {[5, 10, 15, 20, 30, 45, 60, 90, 120].map((t) => (
                <option key={t} value={t}>
                  {t} секунд
                </option>
              ))}
            </select>
          </label>
        </div>
      </fieldset>
    </section>
  );
}
