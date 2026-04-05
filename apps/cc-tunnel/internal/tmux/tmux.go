package tmux

import (
	"fmt"
	"os/exec"
	"strings"
)

// NewSession creates a new detached tmux session and runs a command in it.
func NewSession(name string, command string) error {
	args := []string{"new-session", "-d", "-s", name, "-x", "200", "-y", "50"}
	if command != "" {
		args = append(args, command)
	}
	return exec.Command("tmux", args...).Run()
}

// SendKeys sends keystrokes to a tmux session.
func SendKeys(name string, keys string) error {
	return exec.Command("tmux", "send-keys", "-t", name, keys, "Enter").Run()
}

// CapturePaneOutput captures the visible content of a tmux pane.
func CapturePaneOutput(name string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", name, "-p", "-S", "-").Output()
	if err != nil {
		return "", fmt.Errorf("capture-pane: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
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
