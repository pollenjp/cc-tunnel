import { useRef, useEffect, useCallback } from 'react';

/**
 * SSE ストリームの AbortController を管理するフック。
 *
 * - startStream(sessionId): 新しい AbortController を作成して返す。既存のものは abort。
 * - switchSession(sessionId): 現在の SSE を abort し、アクティブセッションを更新する。
 * - isActiveSession(sessionId): 現在アクティブなセッションかどうかを返す。
 */
export function useSSEAbort() {
  const abortControllerRef = useRef<AbortController | null>(null);
  const activeSessionIdRef = useRef<string | null>(null);

  useEffect(() => {
    return () => {
      abortControllerRef.current?.abort();
    };
  }, []);

  const startStream = useCallback((sessionId: string) => {
    abortControllerRef.current?.abort();
    const controller = new AbortController();
    abortControllerRef.current = controller;
    activeSessionIdRef.current = sessionId;
    return { signal: controller.signal, sessionId };
  }, []);

  const switchSession = useCallback((sessionId: string) => {
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;
    activeSessionIdRef.current = sessionId;
  }, []);

  const isActiveSession = useCallback((sessionId: string) => {
    return activeSessionIdRef.current === sessionId;
  }, []);

  return { startStream, switchSession, isActiveSession };
}
