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
		RETURNING id, title, model, system_prompt, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, q, title, model, systemPrompt)
	c := &Conversation{}
	if err := row.Scan(&c.ID, &c.Title, &c.Model, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *Repository) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	const q = `
		SELECT id, title, model, system_prompt, created_at, updated_at
		FROM conversations WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, q, id)
	c := &Conversation{}
	if err := row.Scan(&c.ID, &c.Title, &c.Model, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *Repository) ListConversations(ctx context.Context) ([]*Conversation, error) {
	const q = `
		SELECT id, title, model, system_prompt, created_at, updated_at
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
		if err := rows.Scan(&c.ID, &c.Title, &c.Model, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt); err != nil {
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

// --- Message ---

type Message struct {
	ID             string                 `json:"id"`
	ConversationID string                 `json:"conversation_id"`
	Role           string                 `json:"role"`
	MessageData    map[string]interface{} `json:"message_data"`
	CreatedAt      time.Time              `json:"created_at"`
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
		SELECT id, conversation_id, role, message_data, created_at
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
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &dataRaw, &m.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(dataRaw, &m.MessageData); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
