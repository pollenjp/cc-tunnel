import { useState, useEffect, useCallback } from 'react';
import * as appAuthApi from '../api/app-auth';
import type { AppUser } from '../api/app-auth';
import { AppAuthContext } from './AppAuthContext';

const SESSION_STORAGE_KEY = 'app_auth_token';

export const AppAuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<AppUser | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const savedToken = sessionStorage.getItem(SESSION_STORAGE_KEY);
        if (!savedToken) {
          return;
        }
        const resp = await appAuthApi.getMe(savedToken);
        setToken(savedToken);
        setUser(resp.user);
      } catch {
        sessionStorage.removeItem(SESSION_STORAGE_KEY);
      } finally {
        setIsLoading(false);
      }
    };
    void checkAuth();
  }, []);

  const login = useCallback(async (username: string) => {
    const resp = await appAuthApi.login(username);
    sessionStorage.setItem(SESSION_STORAGE_KEY, resp.token);
    setToken(resp.token);
    setUser(resp.user);
  }, []);

  const logout = useCallback(async () => {
    if (token) {
      await appAuthApi.logout(token);
    }
    sessionStorage.removeItem(SESSION_STORAGE_KEY);
    setToken(null);
    setUser(null);
  }, [token]);

  const updateNickname = useCallback(
    async (nickname: string) => {
      if (!token) return;
      const resp = await appAuthApi.updateMe(token, nickname);
      setUser(resp.user);
    },
    [token],
  );

  return (
    <AppAuthContext.Provider value={{ user, token, isLoading, login, logout, updateNickname }}>
      {children}
    </AppAuthContext.Provider>
  );
};
