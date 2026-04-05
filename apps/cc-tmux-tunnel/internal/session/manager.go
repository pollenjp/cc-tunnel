package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tmux-tunnel/internal/tmux"
)

type Session struct {
	ID        string    `json:"id"`
	TmuxName  string    `json:"tmux_name"`
	CreatedAt time.Time `json:"created_at"`
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

const (
	DefaultWidth  = 100
	DefaultHeight = 50
)

type CreateOptions struct {
	Width  int
	Height int
}

func (m *Manager) Create(opts CreateOptions) (*Session, error) {
	id := generateID()
	tmuxName := "claude-" + id

	width := opts.Width
	if width <= 0 {
		width = DefaultWidth
	}
	height := opts.Height
	if height <= 0 {
		height = DefaultHeight
	}

	if err := tmux.NewSession(tmuxName, width, height, ""); err != nil {
		return nil, fmt.Errorf("create tmux session: %w", err)
	}

	// Start claude code in the tmux session
	if err := tmux.SendKeys(tmuxName, []string{"claude", "Enter"}); err != nil {
		_ = tmux.KillSession(tmuxName)
		return nil, fmt.Errorf("start claude: %w", err)
	}

	s := &Session{
		ID:        id,
		TmuxName:  tmuxName,
		CreatedAt: time.Now(),
	}

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	return s, nil
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

func (m *Manager) Resize(id string, width, height int) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	return tmux.ResizeWindow(s.TmuxName, width, height)
}

func (m *Manager) SendKeys(id string, keys []string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	return tmux.SendKeys(s.TmuxName, keys)
}

func (m *Manager) GetOutput(id string) (string, error) {
	s, ok := m.Get(id)
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}
	return tmux.CapturePaneOutput(s.TmuxName)
}

func (m *Manager) Delete(id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	if err := tmux.KillSession(s.TmuxName); err != nil {
		return fmt.Errorf("kill tmux session: %w", err)
	}

	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()

	return nil
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
