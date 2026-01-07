-- Short URL Service Database Schema
-- Version: 1.0.0

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- URLs table for storing short URL mappings
CREATE TABLE IF NOT EXISTS urls (
    id           BIGSERIAL PRIMARY KEY,
    short_code   VARCHAR(11) UNIQUE NOT NULL,
    url_hash     VARCHAR(64) UNIQUE NOT NULL,  -- SHA256 hex for deduplication
    original_url TEXT NOT NULL,
    click_count  BIGINT DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    expires_at   TIMESTAMPTZ,
    is_active    BOOLEAN DEFAULT TRUE
);

-- Indexes for performance optimization
-- Note: short_code and url_hash already have implicit indexes from UNIQUE constraints
CREATE INDEX idx_urls_created_at ON urls(created_at DESC);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger for auto-updating updated_at
CREATE TRIGGER update_urls_updated_at
    BEFORE UPDATE ON urls
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Optional: URL access logs for analytics
CREATE TABLE IF NOT EXISTS url_access_logs (
    id          BIGSERIAL PRIMARY KEY,
    url_id      BIGINT REFERENCES urls(id) ON DELETE CASCADE,
    accessed_at TIMESTAMPTZ DEFAULT NOW(),
    ip_address  INET,
    user_agent  TEXT,
    referer     TEXT
);

CREATE INDEX idx_url_access_logs_url_id ON url_access_logs(url_id);
CREATE INDEX idx_url_access_logs_accessed_at ON url_access_logs(accessed_at DESC);

-- Comment on tables
COMMENT ON TABLE urls IS 'Stores short URL to original URL mappings';
COMMENT ON TABLE url_access_logs IS 'Stores access logs for analytics purposes';

