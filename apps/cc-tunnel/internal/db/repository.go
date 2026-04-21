package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Conversation ---

type Conversation struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Model        string    `json:"model"`
	SystemPrompt *string   `json:"system_prompt,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateConversation(ctx context.Context, title, model string, systemPrompt *string) (*Conversation, error) {
	const q = `
		INSERT INTO conversations (title, model, system_prompt)
		VALUES ($1, $2, $3)
		RETURNING id, title, model, system_prompt, status, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, q, title, model, systemPrompt)
	c := &Conversation{}
	if err := row.Scan(&c.ID, &c.Title, &c.Model, &c.SystemPrompt, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *Repository) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	const q = `
		SELECT id, title, model, system_prompt, status, created_at, updated_at
		FROM conversations WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, q, id)
	c := &Conversation{}
	if err := row.Scan(&c.ID, &c.Title, &c.Model, &c.SystemPrompt, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *Repository) ListConversations(ctx context.Context) ([]*Conversation, error) {
	const q = `
		SELECT id, title, model, system_prompt, status, created_at, updated_at
		FROM conversations ORDER BY updated_at DESC
	`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Conversation
	for rows.Next() {
		c := &Conversation{}
		if err := rows.Scan(&c.ID, &c.Title, &c.Model, &c.SystemPrompt, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *Repository) DeleteConversation(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, id)
	return err
}

func (r *Repository) UpdateConversationUpdatedAt(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE conversations SET updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *Repository) UpdateConversationTitle(ctx context.Context, id string, title string) error {
	_, err := r.pool.Exec(ctx, `UPDATE conversations SET title = $1, updated_at = NOW() WHERE id = $2`, title, id)
	return err
}

func (r *Repository) UpdateConversationStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE conversations SET status = $1 WHERE id = $2`, status, id)
	return err
}

// --- Message ---

type Message struct {
	ID             string                 `json:"id"`
	ConversationID string                 `json:"conversation_id"`
	Role           string                 `json:"role"`
	MessageData    map[string]interface{} `json:"message_data"`
	Status         string                 `json:"status"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

func (r *Repository) CreateMessage(ctx context.Context, conversationID, role string, messageData map[string]interface{}) (*Message, error) {
	if messageData == nil {
		messageData = map[string]interface{}{}
	}
	dataBytes, err := json.Marshal(messageData)
	if err != nil {
		return nil, err
	}

	const q = `
		INSERT INTO messages (conversation_id, role, message_data)
		VALUES ($1, $2, $3)
		RETURNING id, conversation_id, role, message_data, created_at
	`
	row := r.pool.QueryRow(ctx, q, conversationID, role, dataBytes)
	m := &Message{}
	var dataRaw []byte
	if err := row.Scan(&m.ID, &m.ConversationID, &m.Role, &dataRaw, &m.CreatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dataRaw, &m.MessageData); err != nil {
		return nil, err
	}
	return m, nil
}

func (r *Repository) ListMessages(ctx context.Context, conversationID string) ([]*Message, error) {
	const q = `
		SELECT id, conversation_id, role, message_data, status, created_at, updated_at
		FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, q, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Message
	for rows.Next() {
		m := &Message{}
		var dataRaw []byte
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &dataRaw, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(dataRaw, &m.MessageData); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *Repository) CreateStreamingMessage(ctx context.Context, conversationID, role string, messageData map[string]interface{}) (*Message, error) {
	if messageData == nil {
		messageData = map[string]interface{}{}
	}
	dataBytes, err := json.Marshal(messageData)
	if err != nil {
		return nil, err
	}
	const q = `
		INSERT INTO messages (conversation_id, role, message_data, status)
		VALUES ($1, $2, $3, 'streaming')
		RETURNING id, conversation_id, role, message_data, status, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, q, conversationID, role, dataBytes)
	m := &Message{}
	var dataRaw []byte
	if err := row.Scan(&m.ID, &m.ConversationID, &m.Role, &dataRaw, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dataRaw, &m.MessageData); err != nil {
		return nil, err
	}
	return m, nil
}

func (r *Repository) UpdateMessageContentBlocks(ctx context.Context, messageID string, contentBlocks []map[string]interface{}) error {
	dataBytes, err := json.Marshal(contentBlocks)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE messages SET message_data = message_data || jsonb_build_object('content_blocks', $1::jsonb), updated_at = NOW() WHERE id = $2`,
		dataBytes, messageID)
	return err
}

func (r *Repository) UpdateMessageStatus(ctx context.Context, messageID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE messages SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, messageID)
	return err
}

func (r *Repository) MergeMessageData(ctx context.Context, messageID string, extra map[string]interface{}) error {
	dataBytes, err := json.Marshal(extra)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE messages SET message_data = message_data || $1::jsonb, updated_at = NOW() WHERE id = $2`,
		dataBytes, messageID)
	return err
}
