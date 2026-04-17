-- +goose Up
CREATE TABLE conversations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL DEFAULT '',
    model       TEXT NOT NULL DEFAULT 'claude-sonnet-4-6',
    system_prompt TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_conversations_updated_at ON conversations(updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS conversations;
