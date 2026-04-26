-- +goose Up
CREATE TABLE session_endpoints (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    vm_instance_id  UUID        NOT NULL REFERENCES vm_instances(id),
    container_name  TEXT        NOT NULL,
    port            INTEGER     NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'provisioning'
                    CHECK (status IN ('provisioning', 'running', 'terminated')),
    last_activity   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (conversation_id)
);

CREATE INDEX idx_session_endpoints_last_activity
    ON session_endpoints(last_activity)
    WHERE status = 'running';

CREATE INDEX idx_session_endpoints_vm
    ON session_endpoints(vm_instance_id)
    WHERE status = 'running';

-- +goose Down
DROP TABLE IF EXISTS session_endpoints;
