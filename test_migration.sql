CREATE TABLE IF NOT EXISTS migrated_notes (id serial PRIMARY KEY, content text); INSERT INTO migrated_notes (content) VALUES ('Hello from migration');
