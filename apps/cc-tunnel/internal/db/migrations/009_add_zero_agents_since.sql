-- +goose Up
-- zero_agents_since records the timestamp at which container-manager last
-- reported zero running cc-remote-agent containers on the VM. When the value
-- has aged past the configured threshold, the VM reaper deletes the VM.
-- NULL means "currently has at least one running agent (or not yet observed)".
ALTER TABLE vm_instances ADD COLUMN zero_agents_since TIMESTAMPTZ;

CREATE INDEX idx_vm_instances_zero_agents_since
    ON vm_instances(zero_agents_since)
    WHERE status = 'running' AND zero_agents_since IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_vm_instances_zero_agents_since;
ALTER TABLE vm_instances DROP COLUMN zero_agents_since;
