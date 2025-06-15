-- In internal/database/migrations/000001_create_jobs_table.up.sql

CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY,
    type VARCHAR(255) NOT NULL,
    payload JSONB,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    execute_at TIMESTAMPTZ, -- This can be NULL
    attempts INTEGER DEFAULT 0,
    error_message TEXT,
    result JSONB
);