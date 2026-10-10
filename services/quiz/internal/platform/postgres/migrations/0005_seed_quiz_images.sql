CREATE TABLE IF NOT EXISTS data_migrations (
    name TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$
DECLARE
    seed_user_id INT;
    seed_quiz_id INT;
    seed_quiz_revision BIGINT;
    changed_questions INT;
BEGIN
    -- Unlike schema migrations, this update must only run once: otherwise a
    -- service restart would overwrite later image changes made in the editor.
    LOCK TABLE data_migrations IN SHARE ROW EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM data_migrations WHERE name = '0005_seed_quiz_images') THEN
        RETURN;
    END IF;

    SELECT id
    INTO seed_user_id
    FROM users
    WHERE username = 'username';

    SELECT id, revision
    INTO seed_quiz_id, seed_quiz_revision
    FROM quizzes
    WHERE author_id = seed_user_id
      AND title = 'Удивительные факты'
    FOR UPDATE;

    IF seed_user_id IS NULL OR seed_quiz_id IS NULL THEN
        RAISE EXCEPTION 'Seeded user or quiz not found';
    END IF;

    INSERT INTO images (id, author_id)
    SELECT image.id, seed_user_id
    FROM (VALUES
        ('d0b66245-e945-4339-b99e-585b2f83a87d'),
        ('61627a9d-e2fb-41c1-9784-d15739671565'),
        ('5058cb76-2217-45ed-b6ae-627fa7822efd'),
        ('b22da69c-b86d-4f8d-a779-db6c50a6496e'),
        ('a3d24e86-d685-4ff1-93e0-80b6c20999b5'),
        ('18c0ba81-094a-42cf-83de-a9b4e53d13ae'),
        ('de5e1abe-2e94-4817-848c-db44e8a2126d')
    ) AS image(id)
    ON CONFLICT (id) DO NOTHING;

    IF EXISTS (
        SELECT 1
        FROM (VALUES
            ('d0b66245-e945-4339-b99e-585b2f83a87d'),
            ('61627a9d-e2fb-41c1-9784-d15739671565'),
            ('5058cb76-2217-45ed-b6ae-627fa7822efd'),
            ('b22da69c-b86d-4f8d-a779-db6c50a6496e'),
            ('a3d24e86-d685-4ff1-93e0-80b6c20999b5'),
            ('18c0ba81-094a-42cf-83de-a9b4e53d13ae'),
            ('de5e1abe-2e94-4817-848c-db44e8a2126d')
        ) AS expected_image(id)
        JOIN images ON images.id = expected_image.id
        WHERE images.author_id IS DISTINCT FROM seed_user_id
    ) THEN
        RAISE EXCEPTION 'A seeded image belongs to another user';
    END IF;

    IF (
        SELECT count(*)
        FROM questions
        WHERE quiz_id = seed_quiz_id
          AND position IN (1, 3, 4, 5, 6, 7, 10)
    ) <> 7 THEN
        RAISE EXCEPTION 'Seeded quiz questions not found';
    END IF;

    UPDATE questions AS question
    SET image_id = seed_image.image_id
    FROM (VALUES
        (1, 'd0b66245-e945-4339-b99e-585b2f83a87d'::VARCHAR(40)),
        (3, '61627a9d-e2fb-41c1-9784-d15739671565'::VARCHAR(40)),
        (4, '5058cb76-2217-45ed-b6ae-627fa7822efd'::VARCHAR(40)),
        (5, 'b22da69c-b86d-4f8d-a779-db6c50a6496e'::VARCHAR(40)),
        (6, 'a3d24e86-d685-4ff1-93e0-80b6c20999b5'::VARCHAR(40)),
        (7, '18c0ba81-094a-42cf-83de-a9b4e53d13ae'::VARCHAR(40)),
        (10, 'de5e1abe-2e94-4817-848c-db44e8a2126d'::VARCHAR(40))
    ) AS seed_image(position, image_id)
    WHERE question.quiz_id = seed_quiz_id
      AND question.position = seed_image.position
      AND question.image_id IS DISTINCT FROM seed_image.image_id;

    GET DIAGNOSTICS changed_questions = ROW_COUNT;
    IF changed_questions > 0 THEN
        -- The question trigger increments once per row. Treat this migration as
        -- one logical editor change instead of exposing seven revision bumps.
        UPDATE quizzes
        SET revision = seed_quiz_revision + 1,
            updated_at = clock_timestamp()
        WHERE id = seed_quiz_id;
    END IF;

    INSERT INTO data_migrations (name) VALUES ('0005_seed_quiz_images');
END $$;
