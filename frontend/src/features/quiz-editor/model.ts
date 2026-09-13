import type { Question, QuizContent, SaveContent } from '../../shared/api/types';
export interface DraftAnswer {
  id: number;
  key: string;
  text_content: string;
  is_correct: boolean;
}
export interface DraftQuestion extends Omit<Question, 'answers'> {
  key: string;
  answers: DraftAnswer[];
}
export interface Draft {
  title: string;
  revision: number;
  questions: DraftQuestion[];
}
export function newAnswer(): DraftAnswer {
  return { id: 0, key: crypto.randomUUID(), text_content: '', is_correct: false };
}
export function newQuestion(): DraftQuestion {
  return {
    id: 0,
    key: crypto.randomUUID(),
    text_content: '',
    image_id: '',
    time_limit: 20,
    answers: Array.from({ length: 4 }, newAnswer),
  };
}
export function toDraft(value: QuizContent): Draft {
  return {
    title: value.title,
    revision: value.revision,
    questions: (value.questions || []).map((q) => ({
      ...q,
      key: `q-${q.id}`,
      answers: (q.answers || []).map((a) => ({
        ...a,
        key: `a-${a.id}`,
        is_correct: a.is_correct === true,
      })),
    })),
  };
}
export function payload(v: Draft): SaveContent {
  return {
    title: v.title.trim(),
    revision: v.revision,
    questions: v.questions.map((q) => ({
      id: q.id,
      text_content: q.text_content.trim(),
      image_id: q.image_id,
      time_limit: q.time_limit,
      answers: q.answers.map((a) => ({
        id: a.id,
        text_content: a.text_content.trim(),
        is_correct: a.is_correct,
      })),
    })),
  };
}
export function fingerprint(v: Draft): string {
  return JSON.stringify(payload(v));
}
export function validateDraft(v: Draft): { message: string; questionKey?: string } | null {
  if (!v.title.trim() || [...v.title.trim()].length > 50)
    return { message: 'Добавьте название квиза: от 1 до 50 символов.' };
  if (v.questions.length > 100) return { message: 'В одном квизе может быть до 100 вопросов.' };
  for (let i = 0; i < v.questions.length; i++) {
    const q = v.questions[i];
    const issue = (message: string) => ({
      message: `Вопрос ${i + 1}: ${message}`,
      questionKey: q.key,
    });
    if (!q.text_content.trim() || [...q.text_content.trim()].length > 200)
      return issue('добавьте текст длиной до 200 символов.');
    if (q.answers.length < 2 || q.answers.length > 8) return issue('нужно от 2 до 8 ответов.');
    if (q.answers.some((a) => !a.text_content.trim() || [...a.text_content.trim()].length > 200))
      return issue('заполните все ответы, максимум 200 символов.');
    if (q.answers.filter((a) => a.is_correct).length !== 1)
      return issue('выберите один правильный ответ.');
  }
  return null;
}
export type Action =
  | { type: 'title'; value: string }
  | { type: 'replace'; value: Draft }
  | { type: 'add'; question: DraftQuestion }
  | { type: 'remove'; key: string }
  | { type: 'question'; key: string; value: Partial<DraftQuestion> }
  | { type: 'reorder'; from: string; to: string };
export function reducer(state: Draft, action: Action): Draft {
  switch (action.type) {
    case 'title':
      return { ...state, title: action.value };
    case 'replace':
      return action.value;
    case 'add':
      return { ...state, questions: [...state.questions, action.question] };
    case 'remove':
      return { ...state, questions: state.questions.filter((q) => q.key !== action.key) };
    case 'question':
      return {
        ...state,
        questions: state.questions.map((q) =>
          q.key === action.key ? { ...q, ...action.value } : q,
        ),
      };
    case 'reorder': {
      const from = state.questions.findIndex((q) => q.key === action.from),
        to = state.questions.findIndex((q) => q.key === action.to);
      if (from < 0 || to < 0 || from === to) return state;
      const questions = [...state.questions];
      questions.splice(to, 0, questions.splice(from, 1)[0]);
      return { ...state, questions };
    }
  }
}
export function draftStorageKey(user: number, quiz: number) {
  return `quizgo:draft:${user}:${quiz}`;
}
export function readDraft(key: string): Draft | null {
  try {
    const value = JSON.parse(sessionStorage.getItem(key) || 'null');
    if (
      !value ||
      typeof value.title !== 'string' ||
      typeof value.revision !== 'number' ||
      !Array.isArray(value.questions) ||
      value.questions.some(
        (q: DraftQuestion) =>
          typeof q.key !== 'string' ||
          typeof q.text_content !== 'string' ||
          !Array.isArray(q.answers) ||
          q.answers.some((a) => typeof a.key !== 'string' || typeof a.text_content !== 'string'),
      )
    )
      return null;
    return value;
  } catch {
    return null;
  }
}
