import { describe, expect, it } from 'vitest';
import {
  fingerprint,
  newQuestion,
  payload,
  reducer,
  toDraft,
  validateDraft,
  type Draft,
} from './model';
import type { QuizContent } from '../../shared/api/types';
function valid(): Draft {
  const q = newQuestion();
  q.text_content = 'Столица Франции?';
  q.answers = q.answers.slice(0, 2);
  q.answers[0].text_content = 'Париж';
  q.answers[0].is_correct = true;
  q.answers[1].text_content = 'Лондон';
  return { title: 'География', revision: 1, questions: [q] };
}
describe('quiz drafts', () => {
  it('creates questions when randomUUID is unavailable', () => {
    const original = globalThis.crypto.randomUUID;
    Object.defineProperty(globalThis.crypto, 'randomUUID', {
      configurable: true,
      value: undefined,
    });
    try {
      const question = newQuestion();
      expect(question.key).toBeTruthy();
      expect(question.answers).toHaveLength(4);
      expect(new Set(question.answers.map((answer) => answer.key)).size).toBe(4);
    } finally {
      Object.defineProperty(globalThis.crypto, 'randomUUID', {
        configurable: true,
        value: original,
      });
    }
  });
  it('keeps changes when switching or reordering questions', () => {
    let v = valid();
    const key = v.questions[0].key;
    const another = newQuestion();
    v = reducer(v, { type: 'add', question: another });
    v = reducer(v, { type: 'question', key, value: { text_content: 'Изменённый вопрос' } });
    v = reducer(v, { type: 'reorder', from: key, to: another.key });
    expect(v.questions[1].text_content).toBe('Изменённый вопрос');
    expect(v.questions[1].answers[0].is_correct).toBe(true);
  });
  it('rejects missing/multiple correct answers and empty answer texts', () => {
    const v = valid();
    expect(validateDraft(v)).toBeNull();
    v.questions[0].answers[1].is_correct = true;
    expect(validateDraft(v)?.questionKey).toBe(v.questions[0].key);
    v.questions[0].answers[1].is_correct = false;
    v.questions[0].answers[0].text_content = '  ';
    expect(validateDraft(v)).not.toBeNull();
  });
  it('strips client keys and keeps existing IDs and revision in the save payload', () => {
    const v = valid();
    v.questions[0].id = 9;
    v.questions[0].answers[0].id = 12;
    const p = payload(v);
    expect(JSON.stringify(p)).not.toContain('key');
    expect(p.questions[0].id).toBe(9);
    expect(p.questions[0].answers[0].id).toBe(12);
    expect(p.revision).toBe(1);
  });
  it('detects changed correctness, order, removal, and title', () => {
    const v = valid();
    const base = fingerprint(v);
    expect(fingerprint(reducer(v, { type: 'title', value: 'Новая тема' }))).not.toBe(base);
    expect(fingerprint(reducer(v, { type: 'remove', key: v.questions[0].key }))).not.toBe(base);
    expect(
      fingerprint(
        reducer(v, {
          type: 'question',
          key: v.questions[0].key,
          value: { answers: [...v.questions[0].answers].reverse() },
        }),
      ),
    ).not.toBe(base);
  });
  it('accepts an empty quiz and rejects questions without answers', () => {
    const v = valid();
    v.questions = [];
    expect(validateDraft(v)).toBeNull();
    v.questions = [newQuestion()];
    expect(validateDraft(v)).not.toBeNull();
  });
  it('does not mutate query cache objects', () => {
    const content: QuizContent = {
      id: 1,
      author_id: 1,
      title: 'Тема',
      revision: 2,
      updated_at: '',
      question_count: 1,
      questions: [
        {
          id: 3,
          text_content: 'Вопрос',
          image_id: '',
          time_limit: 20,
          answers: [{ id: 7, text_content: 'Ответ', is_correct: true }],
        },
      ],
    };
    const v = toDraft(content);
    v.questions[0].answers[0].text_content = 'Другой';
    expect(content.questions[0].answers[0].text_content).toBe('Ответ');
  });
});

it('does not save temporary image URLs or treat link refresh as an edit', () => {
  const q = newQuestion();
  q.image_id = 'image-1';
  q.image_url = 'https://s3.test/old-signature';
  const draft = { title: 'Quiz', revision: 1, questions: [q] };
  const before = JSON.stringify(payload(draft));
  q.image_url = 'https://s3.test/fresh-signature';
  expect(JSON.stringify(payload(draft))).toBe(before);
  expect(payload(draft).questions[0]).not.toHaveProperty('image_url');
});
