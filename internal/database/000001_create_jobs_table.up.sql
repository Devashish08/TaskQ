-- Use IF NOT EXISTS to prevent errors on subsequent runs
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY,
    type VARCHAR(255) NOT NULL,
    payload JSONB,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts INTEGER DEFAULT 0,
    error_message TEXT,
    result JSONB
);