import { createContext, useState, useEffect, useCallback } from 'react';
import * as appAuthApi from '../api/app-auth';
import type { AppUser } from '../api/app-auth';

const SESSION_STORAGE_KEY = 'app_auth_token';

interface AppAuthContextValue {
  user: AppUser | null;
  token: string | null;
  isLoading: boolean;
  login: (username: string) => Promise<void>;
  logout: () => Promise<void>;
  updateNickname: (nickname: string) => Promise<void>;
}

export const AppAuthContext = createContext<AppAuthContextValue>({
  user: null,
  token: null,
  isLoading: false,
  login: async () => {},
  logout: async () => {},
  updateNickname: async () => {},
});

export const AppAuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<AppUser | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const savedToken = sessionStorage.getItem(SESSION_STORAGE_KEY);
    if (!savedToken) {
      setIsLoading(false);
      return;
    }
    appAuthApi
      .getMe(savedToken)
      .then(resp => {
        setToken(savedToken);
        setUser(resp.user);
      })
      .catch(() => {
        sessionStorage.removeItem(SESSION_STORAGE_KEY);
      })
      .finally(() => {
        setIsLoading(false);
      });
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

export const useAppAuth = () => useContext(AppAuthContext);
