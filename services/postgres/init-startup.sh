#!/bin/sh
# Wait for PostgreSQL to be ready
until pg_isready -U postgres; do sleep 1; done
# Create the platform_users table if it doesn't exist
psql -U postgres -d blubase -c "
CREATE TABLE IF NOT EXISTS platform_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE,
    password_hash TEXT,
    phone TEXT,
    suspended BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);
"
# Ensure the dev user exists
HASH='$2b$12$W9sXNtY05zqheApAdMl44OzxPUu8J30yWOZfEzSCqtOWib/r7Rfm2'
psql -U postgres -d blubase -c "INSERT INTO platform_users (email, password_hash) VALUES ('dev@blubase.dev', '\$HASH') ON CONFLICT (email) DO NOTHING;"
