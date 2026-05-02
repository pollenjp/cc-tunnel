package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// streamAggregator accumulates events from a cc-remote-agent stream into the
// shapes needed for persisting the assistant message. Safe for concurrent use:
// Handle (called from the executor) and Snapshot (called from the batch ticker)
// may run on different goroutines.
type streamAggregator struct {
	mu sync.Mutex

	contentBlocks []map[string]interface{}
	toolCalls     []ToolCallData

	assistantContent string
	thinkingContent  string
	thinkingBlocks   []string
	modelName        string
	costUSD          float64
	durationMs       int64
	hookEvents       []map[string]any
}

// Handle processes a single stream event. It must be called serially from the
// caller of executionProvider.Execute (the executor invokes onEvent in order).
func (a *streamAggregator) Handle(event remoteclient.StreamEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch event.Type {
	case "assistant":
		if event.Message == nil {
			return
		}
		for _, block := range event.Message.Content {
			switch {
			case block.Type == "thinking" && block.Thinking != "":
				a.contentBlocks = append(a.contentBlocks, map[string]interface{}{
					"type":    "thinking",
					"content": block.Thinking,
				})
				a.thinkingContent += block.Thinking
			case block.Type == "text" && block.Text != "":
				a.contentBlocks = append(a.contentBlocks, map[string]interface{}{
					"type":    "text",
					"content": block.Text,
				})
				a.assistantContent += block.Text
			case block.Type == "tool_use" && block.ID != "":
				a.contentBlocks = append(a.contentBlocks, map[string]interface{}{
					"type":        "tool_use",
					"tool_use_id": block.ID,
				})
			case block.Type == "tool_result":
				a.attachToolResult(block.ToolUseID, block.Content, 1000)
			}
		}
	case "user":
		if event.Message == nil {
			return
		}
		for _, block := range event.Message.Content {
			if block.Type == "tool_result" {
				a.attachToolResult(block.ToolUseID, block.Content, 2000)
			}
		}
	case "system":
		switch event.SubType {
		case "init":
			if event.Model != "" {
				a.modelName = event.Model
			}
		case "hook_started", "hook_response", "notification", "status":
			a.hookEvents = append(a.hookEvents, map[string]any{
				"subtype":    event.SubType,
				"hook_id":    event.HookID,
				"hook_name":  event.HookName,
				"hook_event": event.HookEvent,
			})
		}
	case "stream_event":
		a.handleStreamEvent(event)
	case "result":
		a.costUSD = event.CostUSD
		a.durationMs = event.DurationMs
	}
}

// attachToolResult finds the matching tool_use entry by id and sets its Result
// to a possibly-truncated string extracted from `content`. Caller must hold a.mu.
func (a *streamAggregator) attachToolResult(toolUseID string, content any, maxLen int) {
	text := ""
	switch v := content.(type) {
	case string:
		text = v
	case []any:
		if len(v) > 0 {
			if m, ok := v[0].(map[string]any); ok {
				text, _ = m["text"].(string)
			}
		}
	}
	if len(text) > maxLen {
		text = text[:maxLen] + "...[truncated]"
	}
	for i := range a.toolCalls {
		if a.toolCalls[i].ToolUseId == toolUseID {
			result := text
			a.toolCalls[i].Result = &result
			return
		}
	}
}

// handleStreamEvent processes the partial-message ("stream_event") variant.
// Caller must hold a.mu.
func (a *streamAggregator) handleStreamEvent(event remoteclient.StreamEvent) {
	var inner remoteclient.InnerEvent
	if err := json.Unmarshal(event.Event, &inner); err != nil {
		return
	}
	switch inner.Type {
	case "content_block_start":
		var cbInner struct {
			Type string `json:"type"`
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		}
		if err := json.Unmarshal(inner.ContentBlock, &cbInner); err != nil {
			return
		}
		if cbInner.Type == "tool_use" && cbInner.Name != "" {
			a.toolCalls = append(a.toolCalls, ToolCallData{
				ToolUseId: cbInner.ID,
				ToolName:  cbInner.Name,
				InputJson: "",
			})
		}
	case "content_block_delta":
		var delta map[string]string
		if err := json.Unmarshal(inner.Delta, &delta); err != nil {
			return
		}
		switch delta["type"] {
		case "thinking_delta":
			if delta["thinking"] != "" {
				if len(a.thinkingBlocks) == 0 {
					a.thinkingBlocks = append(a.thinkingBlocks, "")
				}
				a.thinkingBlocks[len(a.thinkingBlocks)-1] += delta["thinking"]
			}
		case "input_json_delta":
			if delta["partial_json"] != "" {
				if len(a.toolCalls) > 0 {
					a.toolCalls[len(a.toolCalls)-1].InputJson += delta["partial_json"]
				}
			}
		}
	}
}

// Snapshot returns deep copies of contentBlocks and toolCalls suitable for the
// batch persistence path. It is safe to call concurrently with Handle.
func (a *streamAggregator) Snapshot() ([]map[string]interface{}, []ToolCallData) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return cloneBlocks(a.contentBlocks), cloneToolCalls(a.toolCalls)
}

// Finalize returns the data needed for the final DB writes after Execute
// returns: the content blocks, the merge payload for message_data, and the
// concatenated assistant text (used for title generation). It must be called
// after Execute has returned (no further Handle calls).
func (a *streamAggregator) Finalize(newSessionID string) (finalBlocks []map[string]interface{}, extra map[string]interface{}, assistantText string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	finalBlocks = cloneBlocks(a.contentBlocks)
	extra = map[string]interface{}{"session_id": newSessionID}
	if a.assistantContent != "" {
		extra["content"] = a.assistantContent
	}
	switch {
	case len(a.thinkingBlocks) > 0:
		extra["thinking"] = a.thinkingBlocks
	case a.thinkingContent != "":
		extra["thinking"] = []string{a.thinkingContent}
	}
	if a.modelName != "" {
		extra["model"] = a.modelName
	}
	if a.costUSD > 0 {
		extra["cost_usd"] = a.costUSD
	}
	if a.durationMs > 0 {
		extra["duration_ms"] = a.durationMs
	}
	if len(a.hookEvents) > 0 {
		extra["hook_events"] = a.hookEvents
	}
	if len(a.toolCalls) > 0 {
		extra["tool_calls"] = a.toolCalls
	}
	return finalBlocks, extra, a.assistantContent
}

// findResumeSessionID walks history backwards and returns the most recent
// assistant message's claude session_id, if any. Empty when no prior assistant
// turn exists.
func findResumeSessionID(history []*db.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != "assistant" {
			continue
		}
		if sid, ok := history[i].MessageData["session_id"].(string); ok && sid != "" {
			return sid
		}
	}
	return ""
}

// buildConversationHistory converts the persisted messages into the shape the
// cc-remote-agent expects as a fallback when --resume cannot be used.
func buildConversationHistory(history []*db.Message) []remoteclient.Message {
	if len(history) == 0 {
		return nil
	}
	out := make([]remoteclient.Message, 0, len(history))
	for _, m := range history {
		content := ""
		switch m.Role {
		case "user":
			content, _ = m.MessageData["content"].(string)
		case "assistant":
			if cbs, ok := m.MessageData["content_blocks"].([]any); ok {
				for _, cb := range cbs {
					block, ok := cb.(map[string]any)
					if !ok || block["type"] != "text" {
						continue
					}
					if t, ok := block["content"].(string); ok {
						content += t
					}
				}
			}
		}
		out = append(out, remoteclient.Message{Role: m.Role, Content: content})
	}
	return out
}

// buildExecuteRequest assembles the cc-remote-agent execution request from a
// new prompt plus the persisted conversation state.
func buildExecuteRequest(prompt, convIDStr string, conv *db.Conversation, history []*db.Message, credJSON []byte) remoteclient.Request {
	req := remoteclient.Request{
		Prompt:                 prompt,
		SessionID:              findResumeSessionID(history),
		ConversationID:         convIDStr,
		Model:                  conv.Model,
		ConversationHistory:    buildConversationHistory(history),
		IncludePartialMessages: true,
		IncludeHookEvents:      true,
		Credentials:            credJSON,
	}
	if conv.SystemPrompt != nil {
		req.SystemPrompt = *conv.SystemPrompt
	}
	return req
}

// executeAndPersist runs cc-remote-agent.Execute, accumulates streaming output,
// batches DB writes on a ticker, and finalizes the assistant message + the
// conversation status when execution completes. Designed to run as a goroutine
// after SendMessage has already written its 202 response.
//
// execCtx must be derived with context.WithoutCancel so that a frontend
// disconnect does not abort the CLI execution or the post-execution DB writes.
func (h *Server) executeAndPersist(execCtx context.Context, executeReq remoteclient.Request, convIDStr, assistantMsgID string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in Execute goroutine", "err", r, "conversation_id", convIDStr)
		}
		if h.doneCh != nil {
			close(h.doneCh)
		}
	}()

	if err := h.repo.UpdateConversationStatus(execCtx, convIDStr, "running"); err != nil {
		slog.Warn("failed to update conversation status to running", "err", err, "conversation_id", convIDStr)
	}
	defer func() {
		if err := h.repo.UpdateConversationStatus(execCtx, convIDStr, "completed"); err != nil {
			slog.Warn("failed to update conversation status to completed", "err", err, "conversation_id", convIDStr)
		}
	}()

	agg := &streamAggregator{}

	batchInterval := h.batchInterval
	if batchInterval <= 0 {
		batchInterval = 2 * time.Second
	}
	ticker := time.NewTicker(batchInterval)
	tickerDone := make(chan struct{})
	var tickerWG sync.WaitGroup
	tickerWG.Add(1)
	go func() {
		defer tickerWG.Done()
		for {
			select {
			case <-tickerDone:
				return
			case <-ticker.C:
				blocks, calls := agg.Snapshot()
				if err := h.repo.UpdateMessageContentBlocks(execCtx, assistantMsgID, blocks); err != nil {
					slog.Warn("batch content_blocks update failed", "err", err, "message_id", assistantMsgID)
				}
				if err := h.repo.MergeMessageData(execCtx, assistantMsgID, map[string]interface{}{
					"tool_calls": calls,
				}); err != nil {
					slog.Warn("batch tool_calls update failed", "err", err, "message_id", assistantMsgID)
				}
			}
		}
	}()

	slog.Info("message processing started", "conversation_id", convIDStr, "has_resume_session", executeReq.SessionID != "")
	newSessionID, err := h.executionProvider.Execute(execCtx, executeReq, agg.Handle)

	// Stop the batcher and wait for it to exit before issuing any final
	// writes, so the final DB calls cannot race with an in-flight batch tick.
	// This also avoids leaking the batcher goroutine.
	ticker.Stop()
	close(tickerDone)
	tickerWG.Wait()

	if err != nil {
		slog.Error("message processing error", "conversation_id", convIDStr, "err", err)
		if serr := h.repo.UpdateMessageStatus(execCtx, assistantMsgID, "error"); serr != nil {
			slog.Error("failed to update message status to error", "err", serr, "message_id", assistantMsgID)
		}
		return
	}
	slog.Info("message processing completed", "conversation_id", convIDStr)

	finalBlocks, extra, assistantText := agg.Finalize(newSessionID)

	if err := h.repo.UpdateMessageContentBlocks(execCtx, assistantMsgID, finalBlocks); err != nil {
		slog.Error("failed to update final content_blocks", "err", err, "message_id", assistantMsgID)
	}
	if err := h.repo.MergeMessageData(execCtx, assistantMsgID, extra); err != nil {
		slog.Error("failed to merge assistant message data", "err", err, "message_id", assistantMsgID)
	}
	if err := h.repo.UpdateMessageStatus(execCtx, assistantMsgID, "completed"); err != nil {
		slog.Error("failed to update message status to completed", "err", err, "message_id", assistantMsgID)
	}

	if assistantText != "" {
		title := generateTitle(assistantText)
		if err := h.repo.UpdateConversationTitle(execCtx, convIDStr, title); err != nil {
			slog.Error("failed to update conversation title", "err", err, "conversation_id", convIDStr)
		}
	}
	if err := h.repo.UpdateConversationUpdatedAt(execCtx, convIDStr); err != nil {
		slog.Error("failed to update conversation updated_at", "err", err, "conversation_id", convIDStr)
	}
}
