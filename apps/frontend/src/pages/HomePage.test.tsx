vi.mock('../contexts/AppAuthContext', () => ({
  useAppAuth: vi.fn(),
}));

vi.mock('../components/UserMenu', () => ({
  UserMenu: () => <div data-testid="user-menu" />,
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useEffect } from 'react';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { HomePage } from './HomePage';
import { useAppAuth } from '../contexts/AppAuthContext';
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

function renderHome() {
  capturedPath = '/';
  return render(
    <MemoryRouter initialEntries={['/']}>
      <LocationCapture />
      <HomePage />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('HomePage', () => {
  it('チャット開始ボタンが表示される', () => {
    mockAuth({ user: null });
    renderHome();
    expect(screen.getByRole('button', { name: /チャット開始/ })).toBeTruthy();
  });

  it('未ログイン時: チャット開始ボタンクリックで /login?redirect=/chat に遷移', async () => {
    mockAuth({ user: null });
    renderHome();
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /チャット開始/ }));
    });
    expect(capturedPath).toBe('/login?redirect=/chat');
  });

  it('ログイン済み時: チャット開始ボタンクリックで /chat に遷移', async () => {
    mockAuth({ user: { id: 'u1', name: 'Alice' } });
    renderHome();
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /チャット開始/ }));
    });
    expect(capturedPath).toBe('/chat');
  });
});
