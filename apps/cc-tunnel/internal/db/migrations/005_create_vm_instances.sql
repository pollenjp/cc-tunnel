-- +goose Up
CREATE TABLE vm_instances (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    gce_instance_name   TEXT        NOT NULL UNIQUE,
    zone                TEXT        NOT NULL,
    internal_ip         TEXT        NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'provisioning'
                        CHECK (status IN ('provisioning', 'running', 'terminated')),
    active_containers   INTEGER     NOT NULL DEFAULT 0,
    idle_since          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vm_instances_status
    ON vm_instances(status)
    WHERE status = 'running';

-- +goose Down
DROP TABLE IF EXISTS vm_instances;
