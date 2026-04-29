-- 007_create_credentials.sql
-- +goose Up
CREATE TABLE credentials (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT        NOT NULL UNIQUE,
    encrypted_data  BYTEA       NOT NULL,
    nonce           BYTEA       NOT NULL,
    key_version     INTEGER     NOT NULL DEFAULT 1,
    is_valid        BOOLEAN     NOT NULL DEFAULT TRUE,
    last_validated  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_credentials_username ON credentials(username);
CREATE INDEX idx_credentials_is_valid ON credentials(is_valid) WHERE is_valid = TRUE;

-- +goose Down
DROP TABLE IF EXISTS credentials;
