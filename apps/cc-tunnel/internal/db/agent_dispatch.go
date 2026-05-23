package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// AgentDispatch is a row in agent_dispatches: a command queued for the
// long-lived claude agent in a per-session container.
type AgentDispatch struct {
	ID                 string     `json:"id"`
	ConversationID     string     `json:"conversation_id"`
	AssistantMessageID string     `json:"assistant_message_id"`
	Prompt             string     `json:"prompt"`
	SystemPrompt       *string    `json:"system_prompt,omitempty"`
	Status             string     `json:"status"`
	DeliveredAt        *time.Time `json:"delivered_at,omitempty"`
	ConsumedAt         *time.Time `json:"consumed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// AgentOutput is a row in agent_outputs: an append-only hook event tied to
// a dispatch.
type AgentOutput struct {
	ID                 string                 `json:"id"`
	DispatchID         string                 `json:"dispatch_id"`
	AssistantMessageID string                 `json:"assistant_message_id"`
	EventSeq           int64                  `json:"event_seq"`
	EventType          string                 `json:"event_type"`
	Payload            map[string]interface{} `json:"payload"`
	Status             string                 `json:"status"`
	CreatedAt          time.Time              `json:"created_at"`
}

const (
	DispatchStatusPending   = "pending"
	DispatchStatusDelivered = "delivered"
	DispatchStatusConsumed  = "consumed"
	DispatchStatusError     = "error"

	OutputStatusPartial = "partial"
	OutputStatusFinal   = "final"
	OutputStatusError   = "error"

	OutputTypeSessionStart     = "session_start"
	OutputTypeUserPromptSubmit = "user_prompt_submit"
	OutputTypePreToolUse       = "pre_tool_use"
	OutputTypePostToolUse      = "post_tool_use"
	OutputTypeStop             = "stop"
	OutputTypeAssistantText    = "assistant_text"
	OutputTypeThinking         = "thinking"
	OutputTypeError            = "error"
)

func (r *Repository) CreateAgentDispatch(ctx context.Context, conversationID, assistantMessageID, prompt string, systemPrompt *string) (*AgentDispatch, error) {
	const q = `
		INSERT INTO agent_dispatches (conversation_id, assistant_message_id, prompt, system_prompt)
		VALUES ($1, $2, $3, $4)
		RETURNING id, conversation_id, assistant_message_id, prompt, system_prompt,
		          status, delivered_at, consumed_at, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, q, conversationID, assistantMessageID, prompt, systemPrompt)
	return scanAgentDispatch(row)
}

func (r *Repository) GetAgentDispatch(ctx context.Context, id string) (*AgentDispatch, error) {
	const q = `
		SELECT id, conversation_id, assistant_message_id, prompt, system_prompt,
		       status, delivered_at, consumed_at, created_at, updated_at
		FROM agent_dispatches WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, q, id)
	return scanAgentDispatch(row)
}

// ClaimNextPendingDispatch atomically picks the oldest pending dispatch for
// the given conversation and transitions it to 'delivered'. Returns
// pgx.ErrNoRows if none are pending.
//
// This is what the Stop hook calls to find the next prompt to feed back into
// the agent via {"decision":"block","reason":<prompt>}.
func (r *Repository) ClaimNextPendingDispatch(ctx context.Context, conversationID string) (*AgentDispatch, error) {
	const q = `
		UPDATE agent_dispatches
		SET status = 'delivered',
		    delivered_at = NOW(),
		    updated_at = NOW()
		WHERE id = (
		    SELECT id FROM agent_dispatches
		    WHERE conversation_id = $1 AND status = 'pending'
		    ORDER BY created_at ASC
		    LIMIT 1
		    FOR UPDATE SKIP LOCKED
		)
		RETURNING id, conversation_id, assistant_message_id, prompt, system_prompt,
		          status, delivered_at, consumed_at, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, q, conversationID)
	d, err := scanAgentDispatch(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	return d, err
}

func (r *Repository) MarkDispatchConsumed(ctx context.Context, id string) error {
	const q = `
		UPDATE agent_dispatches
		SET status = 'consumed', consumed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, q, id)
	return err
}

func (r *Repository) MarkDispatchError(ctx context.Context, id string) error {
	const q = `
		UPDATE agent_dispatches
		SET status = 'error', updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, q, id)
	return err
}

func (r *Repository) ListDispatchesByMessage(ctx context.Context, assistantMessageID string) ([]*AgentDispatch, error) {
	const q = `
		SELECT id, conversation_id, assistant_message_id, prompt, system_prompt,
		       status, delivered_at, consumed_at, created_at, updated_at
		FROM agent_dispatches
		WHERE assistant_message_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, q, assistantMessageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*AgentDispatch
	for rows.Next() {
		d, err := scanAgentDispatch(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// AppendAgentOutput inserts a new event row. event_seq is assigned by the
// caller (typically MAX(event_seq)+1 for the dispatch). The UNIQUE
// (dispatch_id, event_seq) constraint makes retries idempotent.
func (r *Repository) AppendAgentOutput(
	ctx context.Context,
	dispatchID, assistantMessageID string,
	eventSeq int64,
	eventType string,
	payload map[string]interface{},
	status string,
) (*AgentOutput, error) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	const q = `
		INSERT INTO agent_outputs (dispatch_id, assistant_message_id, event_seq, event_type, payload, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, dispatch_id, assistant_message_id, event_seq, event_type, payload, status, created_at
	`
	row := r.pool.QueryRow(ctx, q, dispatchID, assistantMessageID, eventSeq, eventType, payloadBytes, status)
	return scanAgentOutput(row)
}

// NextEventSeq returns the next event_seq to use for the dispatch. Returns 1
// when no rows exist. Caller is expected to call AppendAgentOutput
// immediately after; concurrent appends to the same dispatch are not
// expected (Claude Code fires hooks serially).
func (r *Repository) NextEventSeq(ctx context.Context, dispatchID string) (int64, error) {
	const q = `
		SELECT COALESCE(MAX(event_seq), 0) + 1
		FROM agent_outputs WHERE dispatch_id = $1
	`
	var next int64
	if err := r.pool.QueryRow(ctx, q, dispatchID).Scan(&next); err != nil {
		return 0, err
	}
	return next, nil
}

// ListAgentOutputsByMessage returns all outputs for an assistant message in
// chronological order. Used by cc-tunnel to fold hook events back into
// messages.message_data.
func (r *Repository) ListAgentOutputsByMessage(ctx context.Context, assistantMessageID string) ([]*AgentOutput, error) {
	const q = `
		SELECT id, dispatch_id, assistant_message_id, event_seq, event_type, payload, status, created_at
		FROM agent_outputs
		WHERE assistant_message_id = $1
		ORDER BY created_at ASC, event_seq ASC
	`
	rows, err := r.pool.Query(ctx, q, assistantMessageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*AgentOutput
	for rows.Next() {
		o, err := scanAgentOutput(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

type dispatchScanner interface {
	Scan(dest ...any) error
}

func scanAgentDispatch(row dispatchScanner) (*AgentDispatch, error) {
	d := &AgentDispatch{}
	if err := row.Scan(
		&d.ID, &d.ConversationID, &d.AssistantMessageID,
		&d.Prompt, &d.SystemPrompt,
		&d.Status, &d.DeliveredAt, &d.ConsumedAt,
		&d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return d, nil
}

type outputScanner interface {
	Scan(dest ...any) error
}

func scanAgentOutput(row outputScanner) (*AgentOutput, error) {
	o := &AgentOutput{}
	var payloadRaw []byte
	if err := row.Scan(
		&o.ID, &o.DispatchID, &o.AssistantMessageID,
		&o.EventSeq, &o.EventType, &payloadRaw, &o.Status,
		&o.CreatedAt,
	); err != nil {
		return nil, err
	}
	if len(payloadRaw) > 0 {
		if err := json.Unmarshal(payloadRaw, &o.Payload); err != nil {
			return nil, err
		}
	} else {
		o.Payload = map[string]interface{}{}
	}
	return o, nil
}
