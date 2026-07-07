CREATE USER blubase WITH PASSWORD 'blubase';
CREATE DATABASE blubase OWNER blubase;
\c blubase
CREATE TABLE IF NOT EXISTS platform_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE,
    password_hash TEXT,
    phone TEXT,
    suspended BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);
INSERT INTO platform_users (email, password_hash)
VALUES ('dev@blubase.dev', '$2a$10$Eq5Q9R7b0Wy1rYWa4nV3WuJ3K3B7bHHQX5c7pq/0oGkZGj7Qjq4qK');
