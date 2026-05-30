-- +goose Up
-- agent_dispatches: command queue from cc-tunnel to the long-lived claude
-- agent running inside a per-session container. The Stop hook polls this
-- table for the next pending prompt and feeds it back into the claude
-- session via {"decision":"block","reason":<prompt>}.
CREATE TABLE agent_dispatches (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id      UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    assistant_message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    prompt               TEXT NOT NULL,
    system_prompt        TEXT,
    status               TEXT NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'delivered', 'consumed', 'error')),
    delivered_at         TIMESTAMPTZ,
    consumed_at          TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_dispatches_conv_status
    ON agent_dispatches(conversation_id, status);

CREATE INDEX idx_agent_dispatches_pending
    ON agent_dispatches(created_at)
    WHERE status = 'pending';

-- agent_outputs: append-only event log emitted by Claude Code hooks
-- (SessionStart / UserPromptSubmit / Pre|PostToolUse / Stop). cc-tunnel
-- folds these into messages.message_data for the frontend.
CREATE TABLE agent_outputs (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispatch_id          UUID NOT NULL REFERENCES agent_dispatches(id) ON DELETE CASCADE,
    assistant_message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    event_seq            BIGINT NOT NULL,
    event_type           TEXT NOT NULL
                              CHECK (event_type IN (
                                  'session_start',
                                  'user_prompt_submit',
                                  'pre_tool_use',
                                  'post_tool_use',
                                  'stop',
                                  'assistant_text',
                                  'thinking',
                                  'error'
                              )),
    payload              JSONB NOT NULL DEFAULT '{}',
    status               TEXT NOT NULL DEFAULT 'partial'
                              CHECK (status IN ('partial', 'final', 'error')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (dispatch_id, event_seq)
);

CREATE INDEX idx_agent_outputs_message
    ON agent_outputs(assistant_message_id, created_at ASC);

CREATE INDEX idx_agent_outputs_dispatch
    ON agent_outputs(dispatch_id, event_seq ASC);

-- +goose Down
DROP TABLE IF EXISTS agent_outputs;
DROP TABLE IF EXISTS agent_dispatches;
