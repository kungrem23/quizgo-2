CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(40) NOT NULL UNIQUE,
    password_hash VARCHAR(100) NOT NULL
);

CREATE TABLE IF NOT EXISTS quizzes (
    id SERIAL PRIMARY KEY,
    title VARCHAR(50) NOT NULL,
    author_id INT NOT NULL,
    FOREIGN KEY (author_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS images(
    id VARCHAR(40) PRIMARY KEY,
    image_url VARCHAR(200) NOT NULL
);

CREATE TABLE IF NOT EXISTS questions(
    id SERIAL PRIMARY KEY,
    text_content VARCHAR(200) NOT NULL,
    position INT NOT NULL,
    quiz_id INT NOT NULL,
    FOREIGN KEY (quiz_id) REFERENCES quizzes(id) ON DELETE CASCADE,
    image_id VARCHAR(40),
    FOREIGN KEY (image_id) REFERENCES images(id),
    UNIQUE (quiz_id, position)
);

CREATE TABLE IF NOT EXISTS answers(
    id SERIAL PRIMARY KEY,
    text_content VARCHAR(200) NOT NULL,
    is_correct BOOLEAN DEFAULT FALSE,
    question_id INT NOT NULL,
    FOREIGN KEY(question_id) REFERENCES questions(id)
);