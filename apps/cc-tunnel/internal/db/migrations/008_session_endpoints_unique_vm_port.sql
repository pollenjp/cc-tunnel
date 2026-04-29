-- +goose Up
ALTER TABLE session_endpoints
    ADD CONSTRAINT session_endpoints_vm_port_unique UNIQUE (vm_instance_id, port);

-- +goose Down
ALTER TABLE session_endpoints
    DROP CONSTRAINT IF EXISTS session_endpoints_vm_port_unique;
