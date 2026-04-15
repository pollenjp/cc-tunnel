package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tmux-tunnel/internal/tmux"
)

const (
	SessionTypeClaudeCode        = "claude_code"
	SessionTypeMultiAgentShogun  = "multi_agent_shogun"
	ShogunStartupScript          = "multi-agent-shogun/shutsujin_departure.sh"
	ShogunSessionName            = "shogun"
	MultiagentSessionName        = "multiagent"
	MultiagentPaneCount          = 9
)

type Session struct {
	ID                 string    `json:"id"`
	Type               string    `json:"type"`
	TmuxName           string    `json:"tmux_name"`
	CreatedAt          time.Time `json:"created_at"`
	MultiagentTmuxName string    `json:"multiagent_tmux_name,omitempty"`
	PaneCount          int       `json:"pane_count"`
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
	Width    int
	Height   int
	Type     string
	TmuxName string
}

func (m *Manager) Create(opts CreateOptions) (*Session, error) {
	sessionType := opts.Type
	if sessionType == "" {
		sessionType = SessionTypeClaudeCode
	}

	switch sessionType {
	case SessionTypeClaudeCode:
		return m.createClaudeCode(opts)
	case SessionTypeMultiAgentShogun:
		return m.createMultiAgentShogun()
	default:
		return nil, fmt.Errorf("unknown session type: %s", sessionType)
	}
}

func (m *Manager) createClaudeCode(opts CreateOptions) (*Session, error) {
	id := generateID()

	width := opts.Width
	if width <= 0 {
		width = DefaultWidth
	}
	height := opts.Height
	if height <= 0 {
		height = DefaultHeight
	}

	var tmuxName string
	if opts.TmuxName != "" {
		// Adopt existing tmux session
		if !tmux.HasSession(opts.TmuxName) {
			return nil, fmt.Errorf("tmux session not found: %s", opts.TmuxName)
		}
		tmuxName = opts.TmuxName
	} else {
		// Create new session
		tmuxName = "claude-" + id
		if err := tmux.NewSession(tmuxName, width, height, ""); err != nil {
			return nil, fmt.Errorf("create tmux session: %w", err)
		}
		if err := tmux.SendKeys(tmuxName, []string{"claude", "Enter"}); err != nil {
			_ = tmux.KillSession(tmuxName)
			return nil, fmt.Errorf("start claude: %w", err)
		}
	}

	s := &Session{
		ID:        id,
		Type:      SessionTypeClaudeCode,
		TmuxName:  tmuxName,
		CreatedAt: time.Now(),
		PaneCount: 1,
	}

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	return s, nil
}

func (m *Manager) createMultiAgentShogun() (*Session, error) {
	// Check if already managed
	m.mu.RLock()
	for _, s := range m.sessions {
		if s.Type == SessionTypeMultiAgentShogun {
			m.mu.RUnlock()
			return s, nil
		}
	}
	m.mu.RUnlock()

	hasShogun := tmux.HasSession(ShogunSessionName)
	hasMultiagent := tmux.HasSession(MultiagentSessionName)

	if !hasShogun || !hasMultiagent {
		// Run startup script
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		scriptPath := filepath.Join(home, ShogunStartupScript)
		if err := tmux.RunScript(scriptPath); err != nil {
			return nil, fmt.Errorf("run shogun startup script: %w", err)
		}

		// Wait for sessions to appear (up to 30 seconds)
		for i := 0; i < 30; i++ {
			hasShogun = tmux.HasSession(ShogunSessionName)
			hasMultiagent = tmux.HasSession(MultiagentSessionName)
			if hasShogun && hasMultiagent {
				break
			}
			time.Sleep(1 * time.Second)
		}

		if !tmux.HasSession(ShogunSessionName) {
			return nil, fmt.Errorf("shogun session not found after startup")
		}
		if !tmux.HasSession(MultiagentSessionName) {
			return nil, fmt.Errorf("multiagent session not found after startup")
		}
	}

	id := generateID()
	s := &Session{
		ID:                 id,
		Type:               SessionTypeMultiAgentShogun,
		TmuxName:           ShogunSessionName,
		CreatedAt:          time.Now(),
		MultiagentTmuxName: MultiagentSessionName,
		PaneCount:          1 + MultiagentPaneCount, // shogun(1) + multiagent(9)
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

// Resize updates the tmux window size.
//
// For multi_agent_shogun sessions the target is chosen by paneIndex:
//
//   - paneIndex == nil  → resize both the shogun and multiagent tmux
//     sessions, and apply colWidths/rowHeights to the multiagent grid.
//   - paneIndex == 0    → resize only the shogun tmux session; the
//     multiagent session is left untouched so that switching the frontend
//     to single-view shogun doesn't disrupt the agent grid layout.
//   - paneIndex 1–9     → resize only the multiagent tmux session;
//     colWidths/rowHeights still apply when provided.
//
// colWidths/rowHeights (each exactly 3 entries when set) drive per-column /
// per-row pane sizes of the multiagent 3x3 grid.
func (m *Manager) Resize(id string, width, height int, paneIndex *int, colWidths, rowHeights []int) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	// For non-multiagent sessions paneIndex is a no-op.
	if s.Type != SessionTypeMultiAgentShogun {
		return tmux.ResizeWindow(s.TmuxName, width, height)
	}

	resizeShogun := paneIndex == nil || *paneIndex == 0
	resizeMultiagent := paneIndex == nil || (*paneIndex >= 1 && *paneIndex <= MultiagentPaneCount)

	if resizeShogun {
		if err := tmux.ResizeWindow(s.TmuxName, width, height); err != nil {
			return fmt.Errorf("resize %s: %w", s.TmuxName, err)
		}
	}
	if resizeMultiagent && s.MultiagentTmuxName != "" {
		if err := tmux.ResizeWindow(s.MultiagentTmuxName, width, height); err != nil {
			return fmt.Errorf("resize %s: %w", s.MultiagentTmuxName, err)
		}
		if err := resizeMultiagentGrid(s.MultiagentTmuxName, colWidths, rowHeights); err != nil {
			return err
		}
	}
	return nil
}

// resizeMultiagentGrid applies per-column and per-row pane sizes to the
// multiagent session. Pane layout (column-major, 0-based):
//
//	col 0 -> panes 0, 1, 2
//	col 1 -> panes 3, 4, 5
//	col 2 -> panes 6, 7, 8
//
// Only one pane per column/row is used to drive tmux's layout — the rest
// follow by the layout constraints.
func resizeMultiagentGrid(sessionName string, colWidths, rowHeights []int) error {
	if len(colWidths) == 3 {
		for c, w := range colWidths {
			if w <= 0 {
				continue
			}
			// First pane of each column: 0, 3, 6.
			if err := tmux.ResizePane(sessionName, c*3, w, 0); err != nil {
				return fmt.Errorf("resize col %d: %w", c, err)
			}
		}
	}
	if len(rowHeights) == 3 {
		for r, h := range rowHeights {
			if h <= 0 {
				continue
			}
			// First pane of each row (in column 0): 0, 1, 2.
			if err := tmux.ResizePane(sessionName, r, 0, h); err != nil {
				return fmt.Errorf("resize row %d: %w", r, err)
			}
		}
	}
	return nil
}

func (m *Manager) SendKeys(id string, keys []string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	return tmux.SendKeys(s.TmuxName, keys)
}

// SendKeysToPane sends keys to a specific pane.
// For claude_code: only paneIndex 0 is valid.
// For multi_agent_shogun: paneIndex 0 = shogun, 1-9 = multiagent panes 0-8.
func (m *Manager) SendKeysToPane(id string, paneIndex int, keys []string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	sessionName, tmuxPaneIndex, err := m.resolvePaneTarget(s, paneIndex)
	if err != nil {
		return err
	}

	return tmux.SendKeysToPane(sessionName, tmuxPaneIndex, keys)
}

func (m *Manager) GetOutput(id string) (string, error) {
	s, ok := m.Get(id)
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}
	return tmux.CapturePaneOutput(s.TmuxName)
}

// GetPaneOutput gets output from a specific pane.
// For claude_code: only paneIndex 0 is valid.
// For multi_agent_shogun: paneIndex 0 = shogun, 1-9 = multiagent panes 0-8.
func (m *Manager) GetPaneOutput(id string, paneIndex int) (string, error) {
	s, ok := m.Get(id)
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}

	sessionName, tmuxPaneIndex, err := m.resolvePaneTarget(s, paneIndex)
	if err != nil {
		return "", err
	}

	return tmux.CapturePaneOutputByPane(sessionName, tmuxPaneIndex)
}

// GetAllPaneOutputs returns output from all panes.
func (m *Manager) GetAllPaneOutputs(id string) (map[int]string, error) {
	s, ok := m.Get(id)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	result := make(map[int]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for i := 0; i < s.PaneCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sessionName, tmuxPaneIndex, err := m.resolvePaneTarget(s, idx)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			output, err := tmux.CapturePaneOutputByPane(sessionName, tmuxPaneIndex)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			result[idx] = output
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

// resolvePaneTarget maps a logical paneIndex to a tmux session name and pane index.
func (m *Manager) resolvePaneTarget(s *Session, paneIndex int) (sessionName string, tmuxPaneIndex int, err error) {
	if paneIndex < 0 || paneIndex >= s.PaneCount {
		return "", 0, fmt.Errorf("pane index %d out of range [0, %d)", paneIndex, s.PaneCount)
	}

	switch s.Type {
	case SessionTypeClaudeCode:
		return s.TmuxName, 0, nil
	case SessionTypeMultiAgentShogun:
		if paneIndex == 0 {
			return s.TmuxName, 0, nil // shogun pane
		}
		return s.MultiagentTmuxName, paneIndex - 1, nil // multiagent pane
	default:
		return "", 0, fmt.Errorf("unknown session type: %s", s.Type)
	}
}

func (m *Manager) Delete(id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	if err := tmux.KillSession(s.TmuxName); err != nil {
		return fmt.Errorf("kill tmux session %s: %w", s.TmuxName, err)
	}

	// For multi_agent_shogun, also kill the multiagent session
	if s.Type == SessionTypeMultiAgentShogun && s.MultiagentTmuxName != "" {
		if err := tmux.KillSession(s.MultiagentTmuxName); err != nil {
			// Log but don't fail - primary session already killed
			fmt.Fprintf(os.Stderr, "warning: failed to kill multiagent session %s: %v\n", s.MultiagentTmuxName, err)
		}
	}

	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()

	return nil
}

// Close kills all managed tmux sessions and clears the session map.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []error
	for id, s := range m.sessions {
		if err := tmux.KillSession(s.TmuxName); err != nil {
			errs = append(errs, fmt.Errorf("kill session %s: %w", id, err))
		}
		if s.Type == SessionTypeMultiAgentShogun && s.MultiagentTmuxName != "" {
			if err := tmux.KillSession(s.MultiagentTmuxName); err != nil {
				errs = append(errs, fmt.Errorf("kill multiagent session %s: %w", id, err))
			}
		}
	}
	m.sessions = make(map[string]*Session)
	return errors.Join(errs...)
}

// DiscoveredSession represents an unmanaged tmux session that can be adopted.
type DiscoveredSession struct {
	Type      string   `json:"type"`
	TmuxNames []string `json:"tmux_names"`
}

// DiscoverSessions lists tmux sessions that are not currently managed.
func (m *Manager) DiscoverSessions() ([]DiscoveredSession, error) {
	allSessions, err := tmux.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("list tmux sessions: %w", err)
	}

	m.mu.RLock()
	managed := make(map[string]bool)
	hasManagedShogun := false
	for _, s := range m.sessions {
		managed[s.TmuxName] = true
		if s.MultiagentTmuxName != "" {
			managed[s.MultiagentTmuxName] = true
		}
		if s.Type == SessionTypeMultiAgentShogun {
			hasManagedShogun = true
		}
	}
	m.mu.RUnlock()

	var discovered []DiscoveredSession

	hasShogun := false
	hasMultiagent := false
	for _, name := range allSessions {
		if name == ShogunSessionName {
			hasShogun = true
		}
		if name == MultiagentSessionName {
			hasMultiagent = true
		}
	}

	// Discover multi-agent-shogun
	if hasShogun && hasMultiagent && !hasManagedShogun {
		discovered = append(discovered, DiscoveredSession{
			Type:      SessionTypeMultiAgentShogun,
			TmuxNames: []string{ShogunSessionName, MultiagentSessionName},
		})
	}

	// Discover unmanaged claude-* sessions
	for _, name := range allSessions {
		if strings.HasPrefix(name, "claude-") && !managed[name] {
			discovered = append(discovered, DiscoveredSession{
				Type:      SessionTypeClaudeCode,
				TmuxNames: []string{name},
			})
		}
	}

	return discovered, nil
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
