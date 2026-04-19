-- +goose Up
ALTER TABLE messages ADD COLUMN status TEXT NOT NULL DEFAULT 'completed'
  CHECK (status IN ('streaming', 'completed', 'error'));
ALTER TABLE messages ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS status;
ALTER TABLE messages DROP COLUMN IF EXISTS updated_at;
