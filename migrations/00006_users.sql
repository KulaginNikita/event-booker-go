-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE CHECK (length(trim(username)) > 0),
    password_hash TEXT NOT NULL CHECK (length(password_hash) > 0),
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO users (username, password_hash, role)
VALUES
    ('admin', '$2a$10$7pEsC5lYWcqJUe26x3v/sO47ZvgFnSpH/RQHW6frVW0MiwT1ZUxGO', 'admin'),
    ('user', '$2a$10$c7KKYzWPqbV.rxpXDbNSBOCKKAR6LkwlTeFYpA0unu7RlqgrubCca', 'user')
ON CONFLICT (username) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
