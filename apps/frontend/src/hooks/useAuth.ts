import { useState, useEffect, useRef, useCallback } from 'react';
import { getAuthStatus, initiateLogin, logout as apiLogout, cancelLogin } from '../api/client';
import type { AuthStatus } from '../api/client';

export interface UseAuthReturn {
  status: AuthStatus | null;
  isLoading: boolean;
  login: () => Promise<void>;
  logout: () => Promise<void>;
  cancelLogin: () => Promise<void>;
}

export function useAuth(conversationId = ''): UseAuthReturn {
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const fetchStatus = useCallback(async () => {
    try {
      const s = await getAuthStatus(conversationId);
      setStatus(s);
      if (!s.loginPending) {
        stopPolling();
      }
      return s;
    } catch {
      return null;
    }
  }, [conversationId, stopPolling]);

  useEffect(() => {
    fetchStatus().finally(() => setIsLoading(false));
    return () => {
      stopPolling();
    };
  }, [fetchStatus, stopPolling]);

  const login = async () => {
    setIsLoading(true);
    try {
      await initiateLogin(conversationId);
      const s = await fetchStatus();
      if (s?.loginPending) {
        pollRef.current = setInterval(fetchStatus, 3000);
      }
    } finally {
      setIsLoading(false);
    }
  };

  const logoutFn = async () => {
    setIsLoading(true);
    try {
      const s = await apiLogout(conversationId);
      setStatus(s);
      stopPolling();
    } finally {
      setIsLoading(false);
    }
  };

  const cancelLoginFn = async () => {
    setIsLoading(true);
    try {
      await cancelLogin(conversationId);
      const s = await fetchStatus();
      if (!s?.loginPending) {
        stopPolling();
      }
    } finally {
      setIsLoading(false);
    }
  };

  return { status, isLoading, login, logout: logoutFn, cancelLogin: cancelLoginFn };
}
