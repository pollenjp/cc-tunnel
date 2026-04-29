package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// --- VMInstance ---

type VMInstance struct {
	ID               string
	GCEInstanceName  string
	Zone             string
	InternalIP       string
	Status           string
	ActiveContainers int
	IdleSince        *time.Time
	CreatedAt        time.Time
}

// --- SessionEndpoint ---

type SessionEndpoint struct {
	ID             string
	ConversationID string
	VMInstanceID   string
	ContainerName  string
	Port           int
	Status         string
	LastActivity   time.Time
	CreatedAt      time.Time
}

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

// --- vm_instances ---

func (r *Repository) CreateVMInstance(ctx context.Context, gceInstanceName, zone, internalIP string) (*VMInstance, error) {
	const q = `
		INSERT INTO vm_instances (gce_instance_name, zone, internal_ip)
		VALUES ($1, $2, $3)
		RETURNING id, gce_instance_name, zone, internal_ip, status, active_containers, idle_since, created_at
	`
	row := r.pool.QueryRow(ctx, q, gceInstanceName, zone, internalIP)
	return scanVMInstance(row)
}

func (r *Repository) GetVMInstance(ctx context.Context, id string) (*VMInstance, error) {
	const q = `
		SELECT id, gce_instance_name, zone, internal_ip, status, active_containers, idle_since, created_at
		FROM vm_instances WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, q, id)
	return scanVMInstance(row)
}

func (r *Repository) GetVMInstanceByName(ctx context.Context, name string) (*VMInstance, error) {
	const q = `
		SELECT id, gce_instance_name, zone, internal_ip, status, active_containers, idle_since, created_at
		FROM vm_instances WHERE gce_instance_name = $1
	`
	row := r.pool.QueryRow(ctx, q, name)
	return scanVMInstance(row)
}

func (r *Repository) UpdateVMInstanceStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE vm_instances SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *Repository) UpdateVMInstanceIP(ctx context.Context, id, internalIP string) error {
	_, err := r.pool.Exec(ctx, `UPDATE vm_instances SET internal_ip = $1 WHERE id = $2`, internalIP, id)
	return err
}

func (r *Repository) IncrementVMActiveContainers(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE vm_instances SET active_containers = active_containers + 1, idle_since = NULL WHERE id = $1`, id)
	return err
}

func (r *Repository) DecrementVMActiveContainers(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE vm_instances SET active_containers = GREATEST(active_containers - 1, 0),
		 idle_since = CASE WHEN active_containers - 1 <= 0 THEN NOW() ELSE idle_since END
		 WHERE id = $1`, id)
	return err
}

func (r *Repository) GetAvailableVMInstance(ctx context.Context, maxContainers int) (*VMInstance, error) {
	const q = `
		SELECT id, gce_instance_name, zone, internal_ip, status, active_containers, idle_since, created_at
		FROM vm_instances
		WHERE status = 'running' AND active_containers < $1
		ORDER BY active_containers DESC
		LIMIT 1
	`
	row := r.pool.QueryRow(ctx, q, maxContainers)
	return scanVMInstance(row)
}

func (r *Repository) ListVMInstances(ctx context.Context) ([]*VMInstance, error) {
	const q = `
		SELECT id, gce_instance_name, zone, internal_ip, status, active_containers, idle_since, created_at
		FROM vm_instances ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*VMInstance
	for rows.Next() {
		vm, err := scanVMInstance(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, vm)
	}
	return result, rows.Err()
}

func (r *Repository) ListIdleVMInstances(ctx context.Context, idleThreshold time.Duration) ([]*VMInstance, error) {
	const q = `
		SELECT id, gce_instance_name, zone, internal_ip, status, active_containers, idle_since, created_at
		FROM vm_instances
		WHERE status = 'running' AND active_containers = 0 AND idle_since < NOW() - $1::interval
		ORDER BY idle_since ASC
	`
	rows, err := r.pool.Query(ctx, q, idleThreshold.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*VMInstance
	for rows.Next() {
		vm, err := scanVMInstance(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, vm)
	}
	return result, rows.Err()
}

func (r *Repository) DeleteVMInstance(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM vm_instances WHERE id = $1`, id)
	return err
}

// scanVMInstance scans a row into a VMInstance struct.
type vmScanner interface {
	Scan(dest ...any) error
}

func scanVMInstance(row vmScanner) (*VMInstance, error) {
	vm := &VMInstance{}
	if err := row.Scan(&vm.ID, &vm.GCEInstanceName, &vm.Zone, &vm.InternalIP, &vm.Status, &vm.ActiveContainers, &vm.IdleSince, &vm.CreatedAt); err != nil {
		return nil, err
	}
	return vm, nil
}

// --- session_endpoints ---

func (r *Repository) CreateSessionEndpoint(ctx context.Context, conversationID, vmInstanceID, containerName string, port int) (*SessionEndpoint, error) {
	const q = `
		INSERT INTO session_endpoints (conversation_id, vm_instance_id, container_name, port)
		VALUES ($1, $2, $3, $4)
		RETURNING id, conversation_id, vm_instance_id, container_name, port, status, last_activity, created_at
	`
	row := r.pool.QueryRow(ctx, q, conversationID, vmInstanceID, containerName, port)
	return scanSessionEndpoint(row)
}

func (r *Repository) GetSessionEndpointByConversationID(ctx context.Context, conversationID string) (*SessionEndpoint, error) {
	const q = `
		SELECT id, conversation_id, vm_instance_id, container_name, port, status, last_activity, created_at
		FROM session_endpoints WHERE conversation_id = $1
	`
	row := r.pool.QueryRow(ctx, q, conversationID)
	return scanSessionEndpoint(row)
}

func (r *Repository) UpdateSessionEndpointStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE session_endpoints SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *Repository) UpdateSessionEndpointLastActivity(ctx context.Context, conversationID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE session_endpoints SET last_activity = NOW() WHERE conversation_id = $1`, conversationID)
	return err
}

func (r *Repository) ListIdleSessionEndpoints(ctx context.Context, idleThreshold time.Duration) ([]*SessionEndpoint, error) {
	const q = `
		SELECT id, conversation_id, vm_instance_id, container_name, port, status, last_activity, created_at
		FROM session_endpoints
		WHERE status = 'running' AND last_activity < NOW() - $1::interval
		ORDER BY last_activity ASC
	`
	rows, err := r.pool.Query(ctx, q, idleThreshold.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*SessionEndpoint
	for rows.Next() {
		ep, err := scanSessionEndpoint(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, ep)
	}
	return result, rows.Err()
}

func (r *Repository) DeleteSessionEndpoint(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM session_endpoints WHERE id = $1`, id)
	return err
}

// GetMaxPortOnVM returns the maximum host port in use on the given VM among running session
// endpoints, or 0 if no running endpoints exist. The caller computes the next port as
// max(result, portRangeStart-1) + 1.
func (r *Repository) GetMaxPortOnVM(ctx context.Context, vmID string) (int, error) {
	const q = `
		SELECT COALESCE(MAX(port), 0)
		FROM session_endpoints
		WHERE vm_instance_id = $1 AND status = 'running'
	`
	var maxPort int
	if err := r.pool.QueryRow(ctx, q, vmID).Scan(&maxPort); err != nil {
		return 0, err
	}
	return maxPort, nil
}

type epScanner interface {
	Scan(dest ...any) error
}

func scanSessionEndpoint(row epScanner) (*SessionEndpoint, error) {
	ep := &SessionEndpoint{}
	if err := row.Scan(&ep.ID, &ep.ConversationID, &ep.VMInstanceID, &ep.ContainerName, &ep.Port, &ep.Status, &ep.LastActivity, &ep.CreatedAt); err != nil {
		return nil, err
	}
	return ep, nil
}
