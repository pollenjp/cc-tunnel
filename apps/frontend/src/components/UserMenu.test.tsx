vi.mock('../hooks/useAppAuth', () => ({
  useAppAuth: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useEffect } from 'react';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { UserMenu } from './UserMenu';
import { useAppAuth } from '../hooks/useAppAuth';
import type { AppUser } from '../api/app-auth';

type UseAppAuthReturn = {
  user: AppUser | null;
  token: string | null;
  isLoading: boolean;
  login: (username: string) => Promise<void>;
  logout: () => Promise<void>;
  updateNickname: (nickname: string) => Promise<void>;
};

function mockAuth(overrides: Partial<UseAppAuthReturn>) {
  vi.mocked(useAppAuth).mockReturnValue({
    user: null,
    token: null,
    isLoading: false,
    login: vi.fn(),
    logout: vi.fn(),
    updateNickname: vi.fn(),
    ...overrides,
  });
}

let capturedPath = '/';

function LocationCapture() {
  const location = useLocation();
  useEffect(() => {
    capturedPath = location.pathname + location.search;
  }, [location.pathname, location.search]);
  return null;
}

function renderMenu(path = '/') {
  capturedPath = path;
  return render(
    <MemoryRouter initialEntries={[path]}>
      <LocationCapture />
      <UserMenu />
    </MemoryRouter>,
  );
}

const flush = async () => {
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('UserMenu', () => {
  describe('未ログイン時', () => {
    it('ユーザーアイコン（👤）が表示される', () => {
      mockAuth({ user: null });
      renderMenu();
      const button = screen.getByTestId('user-menu-button');
      expect(button).toBeTruthy();
      expect(button.textContent).toContain('👤');
    });

    it('アイコンクリックで「ログイン」が表示される', () => {
      mockAuth({ user: null });
      renderMenu();
      fireEvent.click(screen.getByTestId('user-menu-button'));
      expect(screen.getByText('ログイン')).toBeTruthy();
    });

    it('右上ログインは現在のURLを redirect に含める', () => {
      mockAuth({ user: null });
      renderMenu('/settings/agents?tab=basic');

      fireEvent.click(screen.getByTestId('user-menu-button'));
      fireEvent.click(screen.getByText('ログイン'));

      expect(capturedPath).toBe('/login?redirect=%2Fsettings%2Fagents%3Ftab%3Dbasic');
    });
  });

  describe('ログイン済み時', () => {
    it('ユーザー名が表示される', () => {
      mockAuth({ user: { id: 'u1', name: 'Alice' } });
      renderMenu();
      expect(screen.getByText('Alice')).toBeTruthy();
    });

    it('クリックでドロップダウンが展開される（3項目）', () => {
      mockAuth({ user: { id: 'u1', name: 'Alice' } });
      renderMenu();
      fireEvent.click(screen.getByTestId('user-menu-button'));
      expect(screen.getByText('アカウント設定')).toBeTruthy();
      expect(screen.getByText('Agentログイン設定')).toBeTruthy();
      expect(screen.getByText('ログアウト')).toBeTruthy();
    });

    it('ログアウトクリック: logout() が呼び出される', async () => {
      const logout = vi.fn().mockResolvedValue(undefined);
      mockAuth({ user: { id: 'u1', name: 'Alice' }, logout });
      renderMenu();
      fireEvent.click(screen.getByTestId('user-menu-button'));
      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: 'ログアウト' }));
      });
      await flush();
      expect(logout).toHaveBeenCalledTimes(1);
    });

    it('ログアウトクリック: / へナビゲートする', async () => {
      const logout = vi.fn().mockResolvedValue(undefined);
      mockAuth({ user: { id: 'u1', name: 'Alice' }, logout });
      renderMenu('/chat');
      fireEvent.click(screen.getByTestId('user-menu-button'));
      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: 'ログアウト' }));
      });
      await flush();
      expect(capturedPath).toBe('/');
    });
  });
});
