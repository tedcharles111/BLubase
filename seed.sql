-- Minimal seed for first boot (all tables are created by the schema later)
-- Include only the essential data that was previously corrupted

-- Dev user (UUID explicitly cast to prevent byte‑array issues)
INSERT INTO platform_users (id, email, password_hash, username, is_admin, status, tier)
VALUES ('25a01526-ca8a-47a3-b8c1-2648c43451b9',
        'dev@blubase.dev',
        '$2b$12$sXbAWT.k9RSwNKkA19sbcOcx.weSs5DcwGdoswHCT5MRaos43E3XO',
        'dev', true, 'active', 'free')
ON CONFLICT (email) DO NOTHING;

-- System config (Mistral keys)
INSERT INTO system_config (id, config) VALUES
('mistral_keys', '["bnXRKcksJSDzVBl6lWYM6LVeL7XKFZ0e", "7Y9YTSfAfqL4MEHL6YHPH5BEOlONfVU2", "qHABo8b8mSk52hTjfYZvRGY6Z5u5dkXj"]')
ON CONFLICT (id) DO NOTHING;
