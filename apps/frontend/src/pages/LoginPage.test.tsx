vi.mock('../hooks/useAppAuth', () => ({
  useAppAuth: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useEffect } from 'react';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { LoginPage } from './LoginPage';
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

const flush = async () => {
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
};

function renderLogin(path = '/login') {
  capturedPath = path;
  return render(
    <MemoryRouter initialEntries={[path]}>
      <LocationCapture />
      <LoginPage />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('LoginPage', () => {
  it('フォームが表示される', () => {
    mockAuth({ user: null });
    renderLogin();
    expect(screen.getByPlaceholderText('ユーザー名')).toBeTruthy();
    expect(screen.getByRole('button', { name: /ログイン/ })).toBeTruthy();
  });

  it('username 入力 → ログインボタン押下 → login() が呼び出される', async () => {
    const login = vi.fn().mockResolvedValue(undefined);
    mockAuth({ user: null, login });
    renderLogin();

    fireEvent.change(screen.getByPlaceholderText('ユーザー名'), {
      target: { value: 'Alice' },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /ログイン/ }));
    });
    await flush();
    expect(login).toHaveBeenCalledWith('Alice');
  });

  it('ログイン成功後 → redirect パラメーターへ遷移', async () => {
    const login = vi.fn().mockResolvedValue(undefined);
    mockAuth({ user: null, login });
    renderLogin('/login?redirect=/chat');

    fireEvent.change(screen.getByPlaceholderText('ユーザー名'), {
      target: { value: 'Alice' },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /ログイン/ }));
    });
    await flush();
    expect(capturedPath).toBe('/chat');
  });

  it('ログイン済みなら redirect へリダイレクト（二重ログイン防止）', () => {
    mockAuth({ user: { id: 'u1', name: 'Alice' } });
    renderLogin('/login?redirect=/chat');
    expect(screen.queryByPlaceholderText('ユーザー名')).toBeNull();
  });
});
