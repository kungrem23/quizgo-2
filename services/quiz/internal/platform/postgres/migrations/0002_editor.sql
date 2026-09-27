ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE questions ADD COLUMN IF NOT EXISTS time_limit INT NOT NULL DEFAULT 20;
ALTER TABLE answers ADD COLUMN IF NOT EXISTS position INT NOT NULL DEFAULT 0;
ALTER TABLE images ADD COLUMN IF NOT EXISTS author_id INT REFERENCES users(id);
ALTER TABLE images ADD COLUMN IF NOT EXISTS content_type TEXT;
ALTER TABLE images ADD COLUMN IF NOT EXISTS content BYTEA;

-- Deferred uniqueness lets positions be swapped safely inside a transaction.
ALTER TABLE questions DROP CONSTRAINT IF EXISTS questions_quiz_id_position_key;
ALTER TABLE questions ADD CONSTRAINT questions_quiz_id_position_key
    UNIQUE (quiz_id, position) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE answers DROP CONSTRAINT IF EXISTS answers_question_id_fkey;
ALTER TABLE answers ADD CONSTRAINT answers_question_id_fkey
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS quizzes_author_idx ON quizzes(author_id);
CREATE INDEX IF NOT EXISTS answers_question_idx ON answers(question_id);

-- Legacy per-question/per-answer endpoints also invalidate editor revisions.
CREATE OR REPLACE FUNCTION touch_question_quiz() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE quizzes SET updated_at = clock_timestamp(), revision = revision + 1
    WHERE id = CASE WHEN TG_OP = 'DELETE' THEN OLD.quiz_id ELSE NEW.quiz_id END;
    RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS question_touches_quiz ON questions;
CREATE TRIGGER question_touches_quiz AFTER INSERT OR UPDATE OR DELETE ON questions
    FOR EACH ROW EXECUTE FUNCTION touch_question_quiz();
CREATE OR REPLACE FUNCTION touch_answer_quiz() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE quizzes SET updated_at = clock_timestamp(), revision = revision + 1
    WHERE id = (SELECT quiz_id FROM questions WHERE id =
      CASE WHEN TG_OP = 'DELETE' THEN OLD.question_id ELSE NEW.question_id END);
    RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS answer_touches_quiz ON answers;
CREATE TRIGGER answer_touches_quiz AFTER INSERT OR UPDATE OR DELETE ON answers
    FOR EACH ROW EXECUTE FUNCTION touch_answer_quiz();
