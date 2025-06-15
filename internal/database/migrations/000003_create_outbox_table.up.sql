-- In internal/database/migrations/000003_create_outbox_table.up.sql

CREATE TABLE IF NOT EXISTS job_queue_outbox (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed ON job_queue_outbox (created_at) WHERE processed_at IS NULL;