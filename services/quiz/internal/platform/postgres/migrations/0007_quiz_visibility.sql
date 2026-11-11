ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS visibility TEXT;

-- Quizzes created before visibility existed were already reachable by a public
-- link. Preserve that behaviour while making new quizzes private by default.
UPDATE quizzes SET visibility = 'public' WHERE visibility IS NULL;

ALTER TABLE quizzes ALTER COLUMN visibility SET DEFAULT 'private';
ALTER TABLE quizzes ALTER COLUMN visibility SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'quizzes_visibility_check'
          AND conrelid = 'quizzes'::regclass
    ) THEN
        ALTER TABLE quizzes ADD CONSTRAINT quizzes_visibility_check
            CHECK (visibility IN ('private', 'public'));
    END IF;
END $$;
