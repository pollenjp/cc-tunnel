import { useEffect } from 'react';

/**
 * conversations に status='running' の会話がある間、
 * intervalMs ごとに onPoll を呼んで会話一覧を更新する。
 */
export function useConversationListPoller({
  hasRunning,
  onPoll,
  intervalMs = 3000,
}: {
  hasRunning: boolean;
  onPoll: () => void;
  intervalMs?: number;
}): void {
  useEffect(() => {
    if (!hasRunning) return;
    const id = setInterval(onPoll, intervalMs);
    return () => clearInterval(id);
  }, [hasRunning, onPoll, intervalMs]);
}
