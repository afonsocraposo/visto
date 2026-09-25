ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN google_subject TEXT;
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email <> '';
CREATE UNIQUE INDEX idx_users_google_subject ON users(google_subject) WHERE google_subject IS NOT NULL;
