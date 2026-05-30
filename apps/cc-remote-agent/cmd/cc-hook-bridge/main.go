// Package main implements `cc-hook-bridge`, a small binary invoked by
// Claude Code hooks inside the cc-remote-agent container.
//
// It is registered in ~/.claude/settings.json as the `command` for hook
// matchers. Claude Code passes hook-specific JSON via stdin; this binary
// translates the event into agent_outputs / agent_dispatches table writes
// (or, for the Stop hook, into a {"decision":"block","reason":...} stdout
// payload that injects the next queued prompt back into the same claude
// session).
//
// Subcommands map 1:1 to Claude Code hook events:
//
//	cc-hook-bridge session-start
//	cc-hook-bridge user-prompt-submit
//	cc-hook-bridge pre-tool-use
//	cc-hook-bridge post-tool-use
//	cc-hook-bridge stop
//
// Environment:
//
//	CC_HOOK_BRIDGE_DATABASE_URL   PostgreSQL connection string (required)
//	CC_HOOK_BRIDGE_CONVERSATION_ID  Conversation UUID this container serves (required)
//	CC_HOOK_BRIDGE_STATE_FILE     Path used by /agent/kick + Stop hook to
//	                              pass the active {dispatch_id, message_id}
//	                              to other hooks. Default: /tmp/cc-hook-bridge-state.json
//	CC_HOOK_BRIDGE_STOP_TIMEOUT_SEC Max seconds Stop hook waits for a new
//	                                pending dispatch before giving up.
//	                                Default: 55 (matches multi-agent-shogun)
//
// Exit codes follow the Claude Code hook contract:
//
//	0 → success (additionalContext on stdout for SessionStart, or
//	    decision JSON for Stop)
//	1 → soft error (hook continues; Claude Code logs but ignores)
//	2 → hard error (Claude Code surfaces to the user)
//
// This is a skeleton implementation: the long-lived claude process / PTY
// integration on cc-remote-agent is not yet wired, so the Stop hook's
// block emission path is exercised only by tests and an eventual /agent
// endpoint. Today the binary's job is to land the contract and the DB
// schema.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: cc-hook-bridge <session-start|user-prompt-submit|pre-tool-use|post-tool-use|stop>")
	}
	cmd := os.Args[1]

	cfg, err := loadConfig()
	if err != nil {
		fatal(fmt.Sprintf("config: %v", err))
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal(fmt.Sprintf("db connect: %v", err))
	}
	defer pool.Close()

	input, err := readStdin()
	if err != nil {
		fatal(fmt.Sprintf("read stdin: %v", err))
	}

	b := &Bridge{
		Pool:           pool,
		ConversationID: cfg.ConversationID,
		StateFile:      cfg.StateFile,
		StopTimeout:    cfg.StopTimeout,
	}

	switch cmd {
	case "session-start":
		err = b.HandleSessionStart(ctx, input)
	case "user-prompt-submit":
		err = b.HandleUserPromptSubmit(ctx, input)
	case "pre-tool-use":
		err = b.HandleToolUse(ctx, input, "pre_tool_use")
	case "post-tool-use":
		err = b.HandleToolUse(ctx, input, "post_tool_use")
	case "stop":
		err = b.HandleStop(ctx, input, os.Stdout)
	default:
		fatal(fmt.Sprintf("unknown subcommand: %q", cmd))
	}
	if err != nil {
		// Soft failure (exit 1): Claude Code will not abort the session,
		// but the failure is visible in container logs.
		slog.Error("hook failed", "cmd", cmd, "error", err)
		os.Exit(1)
	}
}

// Config bundles the env-driven configuration.
type Config struct {
	DatabaseURL    string
	ConversationID string
	StateFile      string
	StopTimeout    time.Duration
}

func loadConfig() (*Config, error) {
	dsn := os.Getenv("CC_HOOK_BRIDGE_DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("CC_HOOK_BRIDGE_DATABASE_URL is required")
	}
	convID := os.Getenv("CC_HOOK_BRIDGE_CONVERSATION_ID")
	if convID == "" {
		return nil, errors.New("CC_HOOK_BRIDGE_CONVERSATION_ID is required")
	}
	stateFile := os.Getenv("CC_HOOK_BRIDGE_STATE_FILE")
	if stateFile == "" {
		stateFile = "/tmp/cc-hook-bridge-state.json"
	}
	timeoutSec := 55
	if v := os.Getenv("CC_HOOK_BRIDGE_STOP_TIMEOUT_SEC"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid CC_HOOK_BRIDGE_STOP_TIMEOUT_SEC: %q", v)
		}
		timeoutSec = n
	}
	return &Config{
		DatabaseURL:    dsn,
		ConversationID: convID,
		StateFile:      stateFile,
		StopTimeout:    time.Duration(timeoutSec) * time.Second,
	}, nil
}

func readStdin() ([]byte, error) {
	// Hook JSON is small (a few KB at most), so reading it all is fine.
	return io.ReadAll(os.Stdin)
}

// State captures the {dispatch_id, assistant_message_id} pair for the
// turn currently being processed. The Stop hook writes it when it
// transitions a pending dispatch to delivered; pre-/post- hooks read it
// to know which dispatch to attach their event to.
type State struct {
	DispatchID         string `json:"dispatch_id"`
	AssistantMessageID string `json:"assistant_message_id"`
}

// Bridge groups dependencies that the hook handlers need.
type Bridge struct {
	Pool           *pgxpool.Pool
	ConversationID string
	StateFile      string
	StopTimeout    time.Duration
}

// HandleSessionStart records a session_start event tied to the active
// dispatch (if any) and writes any additionalContext to stdout. On first
// launch (no dispatch yet) it is a no-op write — Claude Code still gets
// an empty stdout (which means: no extra context).
func (b *Bridge) HandleSessionStart(ctx context.Context, raw []byte) error {
	st, _ := readState(b.StateFile) // missing state is OK on cold start
	if st == nil || st.DispatchID == "" {
		// Nothing to attach yet. Don't fail — this is normal on container start.
		return nil
	}
	payload := parsePayload(raw)
	return b.appendOutput(ctx, st, "session_start", payload, "partial")
}

// HandleUserPromptSubmit attaches a user_prompt_submit event to the
// current dispatch. Treated as informational — the prompt body itself is
// already recorded in agent_dispatches.prompt.
func (b *Bridge) HandleUserPromptSubmit(ctx context.Context, raw []byte) error {
	st, err := readState(b.StateFile)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	payload := parsePayload(raw)
	return b.appendOutput(ctx, st, "user_prompt_submit", payload, "partial")
}

// HandleToolUse records pre_tool_use / post_tool_use as event_type=kind.
func (b *Bridge) HandleToolUse(ctx context.Context, raw []byte, kind string) error {
	st, err := readState(b.StateFile)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	payload := parsePayload(raw)
	return b.appendOutput(ctx, st, kind, payload, "partial")
}

// HandleStop is the centrepiece. It:
//  1. marks the current dispatch as consumed and writes a final
//     event_type='stop' row to agent_outputs.
//  2. polls agent_dispatches for the next pending row on this
//     conversation. If one exists within StopTimeout, it transitions
//     to delivered, rewrites the state file, and writes a
//     {"decision":"block","reason":<prompt>} JSON to stdout — which
//     Claude Code interprets as "continue with this prompt in the same
//     session".
//  3. on timeout, exits 0 with no output (claude is allowed to idle).
func (b *Bridge) HandleStop(ctx context.Context, raw []byte, stdout io.Writer) error {
	st, err := readState(b.StateFile)
	if err == nil && st != nil && st.DispatchID != "" {
		payload := parsePayload(raw)
		if err := b.appendOutput(ctx, st, "stop", payload, "final"); err != nil {
			slog.Warn("append stop event failed", "error", err)
		}
		if err := b.markDispatchConsumed(ctx, st.DispatchID); err != nil {
			slog.Warn("mark dispatch consumed failed", "error", err)
		}
	}

	// Wait up to StopTimeout for a new pending dispatch.
	deadline := time.Now().Add(b.StopTimeout)
	for {
		next, err := b.claimNextPendingDispatch(ctx)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("claim next: %w", err)
		}
		if next != nil {
			// Persist new turn state for the upcoming hook chain.
			if err := writeState(b.StateFile, &State{
				DispatchID:         next.ID,
				AssistantMessageID: next.AssistantMessageID,
			}); err != nil {
				return fmt.Errorf("write state: %w", err)
			}
			out := map[string]string{
				"decision": "block",
				"reason":   next.Prompt,
			}
			return json.NewEncoder(stdout).Encode(out)
		}
		if time.Now().After(deadline) {
			return nil
		}
		// Poll interval is 1s; LISTEN/NOTIFY is a future improvement
		// (ADR §D).
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

// --- DB helpers ---

// claimedDispatch is the subset of agent_dispatches we need.
type claimedDispatch struct {
	ID                 string
	AssistantMessageID string
	Prompt             string
}

func (b *Bridge) claimNextPendingDispatch(ctx context.Context) (*claimedDispatch, error) {
	const q = `
		UPDATE agent_dispatches
		SET status = 'delivered', delivered_at = NOW(), updated_at = NOW()
		WHERE id = (
		    SELECT id FROM agent_dispatches
		    WHERE conversation_id = $1 AND status = 'pending'
		    ORDER BY created_at ASC
		    LIMIT 1
		    FOR UPDATE SKIP LOCKED
		)
		RETURNING id, assistant_message_id, prompt
	`
	row := b.Pool.QueryRow(ctx, q, b.ConversationID)
	d := &claimedDispatch{}
	if err := row.Scan(&d.ID, &d.AssistantMessageID, &d.Prompt); err != nil {
		return nil, err
	}
	return d, nil
}

func (b *Bridge) markDispatchConsumed(ctx context.Context, dispatchID string) error {
	const q = `
		UPDATE agent_dispatches
		SET status = 'consumed', consumed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	_, err := b.Pool.Exec(ctx, q, dispatchID)
	return err
}

func (b *Bridge) appendOutput(ctx context.Context, st *State, eventType string, payload map[string]any, status string) error {
	if st == nil || st.DispatchID == "" {
		return errors.New("state file missing dispatch_id")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO agent_outputs (dispatch_id, assistant_message_id, event_seq, event_type, payload, status)
		VALUES (
		    $1, $2,
		    (SELECT COALESCE(MAX(event_seq), 0) + 1 FROM agent_outputs WHERE dispatch_id = $1),
		    $3, $4, $5
		)
	`
	_, err = b.Pool.Exec(ctx, q, st.DispatchID, st.AssistantMessageID, eventType, body, status)
	return err
}

// --- state file helpers ---

func readState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st := &State{}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, err
	}
	return st, nil
}

func writeState(path string, st *State) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	// 0600: state file may transiently contain prompt context we don't
	// want other users on the host to read.
	return os.WriteFile(path, data, 0o600)
}

func parsePayload(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		// Claude Code might pass something we don't expect; keep the raw
		// text rather than failing the whole hook.
		return map[string]any{"raw": string(raw)}
	}
	return m
}

func fatal(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}
