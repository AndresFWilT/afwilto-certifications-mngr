CREATE TABLE categories (
    id               uuid PRIMARY KEY,
    path             text NOT NULL,
    normalized_path  text NOT NULL UNIQUE,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT categories_path_len CHECK (char_length(path) BETWEEN 1 AND 120)
);
