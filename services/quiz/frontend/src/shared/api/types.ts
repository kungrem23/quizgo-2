export interface User {
  id: number;
  username: string;
}
export interface QuizSummary {
  id: number;
  title: string;
  author_id: number;
  question_count: number;
  updated_at: string;
  revision: number;
}
export interface Answer {
  id: number;
  text_content: string;
  is_correct?: boolean;
}
export interface Question {
  id: number;
  text_content: string;
  image_id: string;
  image_url?: string;
  time_limit: number;
  answers: Answer[];
}
export interface QuizContent extends QuizSummary {
  questions: Question[];
}
export interface SaveContent {
  title: string;
  revision: number;
  questions: Omit<Question, 'image_url'>[];
}
export interface UploadedImage {
  id: string;
  url: string;
}
