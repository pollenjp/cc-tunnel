import { useEffect, useRef, useCallback } from 'react';
import { getConversation } from '../api/client';
import type { Message } from '../api/client';

export interface UseConversationPollerOptions {
  conversationId: string | null;
  isRunning: boolean;
  onMessages: (messages: Message[]) => void;
  onCompleted: () => void;
  intervalMs?: number;
}

/**
 * Polls GET /conversations/:id every `intervalMs` ms while `isRunning` is true.
 * Calls `onMessages` with all messages on every poll when status is 'running'.
 * Calls `onMessages` with all messages then `onCompleted` when status becomes 'completed'.
 */
export function useConversationPoller({
  conversationId,
  isRunning,
  onMessages,
  onCompleted,
  intervalMs = 1000,
}: UseConversationPollerOptions): void {
  const stoppedRef = useRef(false);

  const poll = useCallback(async () => {
    if (!conversationId) return;
    try {
      const detail = await getConversation(conversationId);
      const msgs = detail.messages ?? [];

      // Always deliver full message list for full overwrite
      onMessages(msgs);

      if (detail.status === 'completed') {
        stoppedRef.current = true;
        onCompleted();
      }
    } catch {
      // Ignore transient errors; keep polling.
    }
  }, [conversationId, onMessages, onCompleted]);

  useEffect(() => {
    if (!isRunning || !conversationId) return;

    stoppedRef.current = false;

    const id = setInterval(() => {
      if (stoppedRef.current) {
        clearInterval(id);
        return;
      }
      poll();
    }, intervalMs);

    return () => {
      clearInterval(id);
    };
  }, [isRunning, conversationId, intervalMs, poll]);
}
