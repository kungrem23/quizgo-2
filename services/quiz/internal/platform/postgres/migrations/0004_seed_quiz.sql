INSERT INTO users (username, password_hash)
VALUES (
    'username',
    '$2a$10$37ODasR3ZthfV8SVNZsrDOksc3.MilJmnZjN4mGVY154WXJnH8xrS'
)
ON CONFLICT (username) DO NOTHING;

DO $$
DECLARE
    seed_quiz_id INT;
BEGIN
    -- Migrations are applied on every service start, so serialize the lookup and
    -- insert to keep this seed idempotent even when several instances start.
    LOCK TABLE quizzes IN SHARE ROW EXCLUSIVE MODE;

    INSERT INTO quizzes (title, author_id)
    SELECT 'Удивительные факты', id
    FROM users
    WHERE username = 'username'
      AND NOT EXISTS (
          SELECT 1
          FROM quizzes
          WHERE author_id = users.id
            AND title = 'Удивительные факты'
      )
    RETURNING id INTO seed_quiz_id;

    IF seed_quiz_id IS NULL THEN
        RETURN;
    END IF;

    INSERT INTO questions (text_content, position, quiz_id, time_limit)
    VALUES
        ('Какое животное имеет три сердца?', 1, seed_quiz_id, 20),
        ('Какой из этих продуктов при правильном хранении практически не портится?', 2, seed_quiz_id, 20),
        ('Какой цвет получится, если смешать красный и синий свет?', 3, seed_quiz_id, 20),
        ('Какой из этих персонажей появился раньше остальных?', 4, seed_quiz_id, 20),
        ('У какого животного отпечатки пальцев настолько похожи на человеческие, что их сложно различить?', 5, seed_quiz_id, 20),
        ('Что из этого технически является ягодой с точки зрения ботаники?', 6, seed_quiz_id, 20),
        ('Где находится самая маленькая кость человеческого тела?', 7, seed_quiz_id, 20),
        ('Что из этого было изобретено раньше?', 8, seed_quiz_id, 20),
        ('Какой металл плавится примерно при температуре человеческого тела?', 9, seed_quiz_id, 20),
        ('Какое животное не умеет прыгать?', 10, seed_quiz_id, 20);

    INSERT INTO answers (text_content, is_correct, question_id, position)
    SELECT answer.text_content, answer.is_correct, question.id, answer.position
    FROM questions AS question
    JOIN (VALUES
        (1, 1, 'Акула', FALSE),
        (1, 2, 'Осьминог', TRUE),
        (1, 3, 'Дельфин', FALSE),
        (1, 4, 'Пингвин', FALSE),
        (2, 1, 'Рис', FALSE),
        (2, 2, 'Сахар', FALSE),
        (2, 3, 'Мёд', TRUE),
        (2, 4, 'Соль', FALSE),
        (3, 1, 'Жёлтый', FALSE),
        (3, 2, 'Пурпурный', TRUE),
        (3, 3, 'Зелёный', FALSE),
        (3, 4, 'Коричневый', FALSE),
        (4, 1, 'Марио', FALSE),
        (4, 2, 'Соник', FALSE),
        (4, 3, 'Пакман', TRUE),
        (4, 4, 'Кирби', FALSE),
        (5, 1, 'Шимпанзе', FALSE),
        (5, 2, 'Енот', FALSE),
        (5, 3, 'Коала', TRUE),
        (5, 4, 'Ленивец', FALSE),
        (6, 1, 'Клубника', FALSE),
        (6, 2, 'Малина', FALSE),
        (6, 3, 'Банан', TRUE),
        (6, 4, 'Вишня', FALSE),
        (7, 1, 'В пальце', FALSE),
        (7, 2, 'В носу', FALSE),
        (7, 3, 'В ухе', TRUE),
        (7, 4, 'В стопе', FALSE),
        (8, 1, 'Велосипед', FALSE),
        (8, 2, 'Консервная банка', TRUE),
        (8, 3, 'Спички', FALSE),
        (8, 4, 'Телефон', FALSE),
        (9, 1, 'Алюминий', FALSE),
        (9, 2, 'Медь', FALSE),
        (9, 3, 'Галлий', TRUE),
        (9, 4, 'Титан', FALSE),
        (10, 1, 'Носорог', FALSE),
        (10, 2, 'Бегемот', FALSE),
        (10, 3, 'Слон', TRUE),
        (10, 4, 'Жираф', FALSE)
    ) AS answer(question_position, position, text_content, is_correct)
      ON question.quiz_id = seed_quiz_id
     AND question.position = answer.question_position;

    -- Question and answer triggers advance the revision while seeding. The
    -- completed quiz is the initial editor version, so expose it as revision 1.
    UPDATE quizzes
    SET revision = 1,
        updated_at = clock_timestamp()
    WHERE id = seed_quiz_id;
END $$;
