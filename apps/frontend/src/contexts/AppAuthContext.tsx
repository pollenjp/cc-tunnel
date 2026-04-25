import { createContext } from 'react';
import type { AppUser } from '../api/app-auth';

export interface AppAuthContextValue {
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
