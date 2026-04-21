-- +goose Up
ALTER TABLE conversations
    ADD COLUMN status TEXT NOT NULL DEFAULT 'idle'
    CHECK (status IN ('idle', 'running', 'completed'));

-- +goose Down
ALTER TABLE conversations DROP COLUMN status;
