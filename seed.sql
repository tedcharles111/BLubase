-- platform_users
CREATE TABLE IF NOT EXISTS platform_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE,
    password_hash TEXT,
    phone TEXT,
    username TEXT,
    display_name TEXT,
    photoURL TEXT,
    avatar_url TEXT,
    status TEXT DEFAULT 'active',
    tier TEXT DEFAULT 'free',
    prompts_used_today INT DEFAULT 0,
    last_seen TIMESTAMPTZ DEFAULT now(),
    is_admin BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- project_oMVsv2_users
CREATE TABLE IF NOT EXISTS project_oMVsv2_users (LIKE platform_users INCLUDING ALL);

-- themultiverse-build
CREATE TABLE IF NOT EXISTS "themultiverse-build" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    name TEXT,
    project_name TEXT,
    description TEXT,
    files JSONB DEFAULT '{}',
    messages JSONB DEFAULT '[]',
    env_vars JSONB DEFAULT '[]',
    preview_engine TEXT DEFAULT 'codesandbox',
    is_public BOOLEAN DEFAULT false,
    is_starred BOOLEAN DEFAULT false,
    is_paused BOOLEAN DEFAULT false,
    is_template BOOLEAN DEFAULT false,
    is_marketplace BOOLEAN DEFAULT false,
    likes INTEGER DEFAULT 0,
    listing_status TEXT DEFAULT 'draft',
    price DECIMAL(10,2),
    currency TEXT DEFAULT 'USD',
    category TEXT,
    tags TEXT[],
    team_id TEXT,
    workspace_id UUID,
    deploy_url TEXT,
    forked_from UUID,
    preview_url TEXT,
    thumbnail_url TEXT,
    html TEXT,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- themultiverse_view
CREATE OR REPLACE VIEW "themultiverse_view" AS SELECT * FROM "themultiverse-build";

-- notifications
CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    "userId" TEXT,
    title TEXT,
    message TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- assets
CREATE TABLE IF NOT EXISTS assets (
    id TEXT PRIMARY KEY,
    user_id UUID,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    url TEXT NOT NULL,
    size INT,
    "createdAt" TIMESTAMPTZ DEFAULT now(),
    source TEXT DEFAULT 'supabase'
);

-- community_messages
CREATE TABLE IF NOT EXISTS community_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID,
    user_id TEXT,
    user_email TEXT,
    content TEXT,
    parent_id UUID,
    is_pinned BOOLEAN DEFAULT false,
    reactions JSONB DEFAULT '{}',
    type TEXT DEFAULT 'text',
    image_url TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- community_rooms
CREATE TABLE IF NOT EXISTS community_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    slug TEXT UNIQUE,
    icon TEXT,
    is_private BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- community_apps
CREATE TABLE IF NOT EXISTS community_apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID,
    name TEXT NOT NULL,
    description TEXT,
    author_id UUID,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- marketplace_listings
CREATE TABLE IF NOT EXISTS marketplace_listings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    project_id UUID,
    name TEXT NOT NULL,
    description TEXT,
    price DECIMAL(10,2),
    currency TEXT DEFAULT 'USD',
    status TEXT DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- chats
CREATE TABLE IF NOT EXISTS chats (
    id TEXT PRIMARY KEY,
    participants JSONB DEFAULT '[]',
    "projectId" TEXT,
    "lastMessage" TEXT,
    "updatedAt" TIMESTAMPTZ DEFAULT now()
);

-- messages
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    "chatId" TEXT REFERENCES chats(id) ON DELETE CASCADE,
    "senderId" TEXT,
    text TEXT,
    timestamp BIGINT,
    "isRead" BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- workspaces
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT,
    owner_id UUID,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- workspace_members
CREATE TABLE IF NOT EXISTS workspace_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID,
    role TEXT DEFAULT 'member',
    joined_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(workspace_id, user_id)
);

-- referrals
CREATE TABLE IF NOT EXISTS referrals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inviter_id UUID,
    referred_id UUID,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(referred_id)
);

-- system_config
CREATE TABLE IF NOT EXISTS system_config (
    id TEXT PRIMARY KEY,
    config JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- activities
CREATE TABLE IF NOT EXISTS activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    user_email TEXT,
    action TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Extra columns on platform_users (if not already present)
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS extra_prompts_balance INT DEFAULT 0;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS marketing_used_today INT DEFAULT 0;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS last_prompt_date TIMESTAMPTZ;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS referral_code TEXT UNIQUE;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS referred_by UUID;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS total_referrals INT DEFAULT 0;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS last_invite_reward_at TIMESTAMPTZ;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS badge_ambassador_expiry TIMESTAMPTZ;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS earnings NUMERIC DEFAULT 0;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS app_uploads_count INT DEFAULT 0;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS subscription_expires_at TIMESTAMPTZ;
-- Dev user (hashed password)
INSERT INTO platform_users (id, email, password_hash, username, is_admin, status, tier)
VALUES ('25a01526-ca8a-47a3-b8c1-2648c43451b9', 'dev@blubase.dev',
'$2b$12$sXbAWT.k9RSwNKkA19sbcOcx.weSs5DcwGdoswHCT5MRaos43E3XO',
'dev', true, 'active', 'free')
ON CONFLICT (email) DO NOTHING;

-- Mistral keys (these are already public in your Blubase code, so they are safe)
INSERT INTO system_config (id, config) VALUES
('mistral_keys', '["bnXRKcksJSDzVBl6lWYM6LVeL7XKFZ0e", "7Y9YTSfAfqL4MEHL6YHPH5BEOlONfVU2", "qHABo8b8mSk52hTjfYZvRGY6Z5u5dkXj"]')
ON CONFLICT (id) DO NOTHING;
