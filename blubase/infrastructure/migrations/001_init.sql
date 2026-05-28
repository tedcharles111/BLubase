CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE,
    password_hash TEXT,
    phone TEXT,
    mfa_enabled BOOLEAN DEFAULT false,
    mfa_secret TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    expires_at TIMESTAMPTZ,
    UNIQUE(provider, provider_user_id)
);

CREATE TABLE IF NOT EXISTS oauth_clients (
    client_id TEXT PRIMARY KEY,
    client_secret TEXT NOT NULL,
    redirect_uris TEXT[] NOT NULL,
    project_id UUID,
    name TEXT
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    token_hash TEXT NOT NULL,
    client_id TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID,
    action TEXT,
    ip_address INET,
    timestamp TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    ref TEXT UNIQUE NOT NULL,
    owner_id UUID REFERENCES users(id),
    anon_key TEXT NOT NULL,
    service_role_key TEXT,
    bucket_name TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
