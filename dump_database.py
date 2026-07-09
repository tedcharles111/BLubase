import requests

ANON_KEY = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE3ODA5NTQ4NjMsInJlZiI6Im9NVnN2MiIsInJvbGUiOiJhbm9uIn0.7E-uWH50Og-eAqsYNRBPQhXhOn4ovmhftlee2Es_0QU"
BASE = "https://blubase.onrender.com/sql/sql"
HEADERS = {"Authorization": f"Bearer {ANON_KEY}", "x-project-ref": "oMVsv2"}

# Static schema (exact copy of seed_schema.sql)
schema = """
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

CREATE TABLE IF NOT EXISTS project_oMVsv2_users (LIKE platform_users INCLUDING ALL);

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

CREATE OR REPLACE VIEW "themultiverse_view" AS SELECT * FROM "themultiverse-build";

CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    "userId" TEXT,
    title TEXT,
    message TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

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

CREATE TABLE IF NOT EXISTS community_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    slug TEXT UNIQUE,
    icon TEXT,
    is_private BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS community_apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID,
    name TEXT NOT NULL,
    description TEXT,
    author_id UUID,
    created_at TIMESTAMPTZ DEFAULT now()
);

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

CREATE TABLE IF NOT EXISTS chats (
    id TEXT PRIMARY KEY,
    participants JSONB DEFAULT '[]',
    "projectId" TEXT,
    "lastMessage" TEXT,
    "updatedAt" TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    "chatId" TEXT REFERENCES chats(id) ON DELETE CASCADE,
    "senderId" TEXT,
    text TEXT,
    timestamp BIGINT,
    "isRead" BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT,
    owner_id UUID,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workspace_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID,
    role TEXT DEFAULT 'member',
    joined_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(workspace_id, user_id)
);

CREATE TABLE IF NOT EXISTS referrals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inviter_id UUID,
    referred_id UUID,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(referred_id)
);

CREATE TABLE IF NOT EXISTS system_config (
    id TEXT PRIMARY KEY,
    config JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    user_email TEXT,
    action TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now()
);

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
"""

# Get table list
r = requests.get(BASE, headers=HEADERS, params={"query": "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name"})
tables = [t['table_name'] for t in r.json()]

# Dump data
inserts = []
for table in tables:
    if table in ["themultiverse_view"]:
        continue
    r = requests.get(BASE, headers=HEADERS, params={"query": f'SELECT * FROM "{table}"'})
    rows = r.json()
    if not rows:
        continue
    cols = list(rows[0].keys())
    for row in rows:
        values = []
        for c in cols:
            val = row[c]
            if val is None:
                values.append("NULL")
            elif isinstance(val, str):
                safe = val.replace("'", "''")
                values.append(f"'{safe}'")
            elif isinstance(val, bool):
                values.append(str(val).lower())
            else:
                values.append(str(val))
        values_str = ", ".join(values)
        inserts.append(f'INSERT INTO "{table}" ({", ".join(cols)}) VALUES ({values_str}) ON CONFLICT DO NOTHING;')

# Write final seed.sql
with open("seed.sql", "w") as f:
    f.write(schema + "\n" + "\n".join(inserts))

print(f"Backup completed: {len(inserts)} rows written.")
