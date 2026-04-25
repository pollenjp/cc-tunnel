vi.mock('../contexts/AppAuthContext', () => ({
  useAppAuth: vi.fn(),
}));

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import AppAuthGuard from './AppAuthGuard';
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

function renderGuard(path = '/') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppAuthGuard>
        <div data-testid="children">protected content</div>
      </AppAuthGuard>
    </MemoryRouter>,
  );
}

describe('AppAuthGuard', () => {
  it('isLoading=true のとき LoadingSpinner を表示し children は非表示', () => {
    mockAuth({ isLoading: true, user: null });

    renderGuard();

    const spinner = document.querySelector('.animate-spin');
    expect(spinner).not.toBeNull();
    expect(screen.queryByTestId('children')).toBeNull();
  });

  it('未認証 (user=null, isLoading=false) のとき children は非表示（/login にリダイレクト）', () => {
    mockAuth({ isLoading: false, user: null });

    renderGuard('/dashboard');

    expect(screen.queryByTestId('children')).toBeNull();
  });

  it('認証済み (user存在) のとき children を表示する', () => {
    mockAuth({ isLoading: false, user: { id: 'u1', name: 'Alice' }, token: 'tok' });

    renderGuard('/dashboard');

    expect(screen.getByTestId('children')).toBeTruthy();
  });
});
