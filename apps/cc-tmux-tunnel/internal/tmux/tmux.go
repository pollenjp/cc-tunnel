package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// NewSession creates a new detached tmux session with the given window size.
func NewSession(name string, width, height int, command string) error {
	args := []string{
		"new-session", "-d", "-s", name,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	}
	if command != "" {
		args = append(args, command)
	}
	cmd := exec.Command("tmux", args...)
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	return cmd.Run()
}

// ResizeWindow resizes the window of a running tmux session.
func ResizeWindow(name string, width, height int) error {
	return exec.Command("tmux", "resize-window", "-t", name,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	).Run()
}

// SendKeys sends keystrokes to a tmux session.
// Each key is passed as a separate argument to tmux send-keys.
// Special key names (Enter, Escape, C-c, etc.) are interpreted by tmux.
func SendKeys(name string, keys []string) error {
	args := append([]string{"send-keys", "-t", name}, keys...)
	return exec.Command("tmux", args...).Run()
}

// CapturePaneOutput captures the visible content of a tmux pane.
func CapturePaneOutput(name string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", name, "-p", "-J", "-S", "-").Output()
	if err != nil {
		return "", fmt.Errorf("capture-pane: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// SendKeysToPane sends keystrokes to a specific pane in a tmux session.
func SendKeysToPane(sessionName string, paneIndex int, keys []string) error {
	target := fmt.Sprintf("%s:0.%d", sessionName, paneIndex)
	args := append([]string{"send-keys", "-t", target}, keys...)
	return exec.Command("tmux", args...).Run()
}

// CapturePaneOutputByPane captures the content of a specific pane in a tmux session.
func CapturePaneOutputByPane(sessionName string, paneIndex int) (string, error) {
	target := fmt.Sprintf("%s:0.%d", sessionName, paneIndex)
	out, err := exec.Command("tmux", "capture-pane", "-t", target, "-p", "-J", "-S", "-").Output()
	if err != nil {
		return "", fmt.Errorf("capture-pane %s: %w", target, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// ListPanes returns the number of panes in a tmux session's first window.
func ListPanes(sessionName string) (int, error) {
	out, err := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_index}").Output()
	if err != nil {
		return 0, fmt.Errorf("list-panes %s: %w", sessionName, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0, nil
	}
	return len(lines), nil
}

// RunScript executes a shell script and waits for it to complete.
func RunScript(scriptPath string) error {
	cmd := exec.Command("bash", scriptPath)
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	return cmd.Run()
}

// KillSession kills a tmux session.
func KillSession(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

// HasSession checks if a tmux session exists.
func HasSession(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// ListSessions returns a list of tmux session names.
func ListSessions() ([]string, error) {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		// tmux returns error when no sessions exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}
