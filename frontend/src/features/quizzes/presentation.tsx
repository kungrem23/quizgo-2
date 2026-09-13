import { Globe2, Code2, FlaskConical, Pi, Landmark, BookOpen, Palette, Rocket } from 'lucide-react';
const themes = [
  { name: 'violet', Icon: Globe2, symbol: '✦' },
  { name: 'blue', Icon: Code2, symbol: '{ }' },
  { name: 'green', Icon: FlaskConical, symbol: '◌' },
  { name: 'yellow', Icon: Pi, symbol: 'π' },
  { name: 'rose', Icon: Landmark, symbol: '✷' },
  { name: 'pink', Icon: BookOpen, symbol: '✿' },
  { name: 'peach', Icon: Palette, symbol: '◈' },
  { name: 'blue', Icon: Rocket, symbol: '✧' },
];
export function quizTheme(id: number, title = '') {
  const t = title.toLowerCase();
  const subjects = ['географ', ' go', 'биолог', 'математ', 'истор', 'литератур'];
  let index = subjects.findIndex((s) => t.includes(s));
  if (index < 0 && /код|программ|typescript|javascript/.test(t)) index = 1;
  return themes[index < 0 ? Math.abs(id - 1) % themes.length : index];
}
export function pluralQuestions(n: number) {
  const a = n % 10,
    b = n % 100;
  return `${n} ${b >= 11 && b <= 14 ? 'вопросов' : a === 1 ? 'вопрос' : a >= 2 && a <= 4 ? 'вопроса' : 'вопросов'}`;
}
export function relativeDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const days = Math.floor((Date.now() - date.getTime()) / 86400000);
  if (days < 1) return 'сегодня';
  if (days === 1) return 'вчера';
  return new Intl.DateTimeFormat('ru', { day: 'numeric', month: 'short' }).format(date);
}
