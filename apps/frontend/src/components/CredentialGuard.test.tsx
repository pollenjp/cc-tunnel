vi.mock('../hooks/useAppAuth', () => ({
  useAppAuth: vi.fn(),
}));

vi.mock('../api/credentials', () => ({
  getCredentialsStatus: vi.fn(),
}));

import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { useEffect } from 'react';
import CredentialGuard from './CredentialGuard';
import { useAppAuth } from '../hooks/useAppAuth';
import { getCredentialsStatus } from '../api/credentials';

type UseAppAuthReturn = {
  user: { id: string; name: string } | null;
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

function renderGuard(conversationId = 'conv-123', initialPath = '/chat/conv-123') {
  capturedPath = initialPath;
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/login/credentials" element={<LocationCapture />} />
        <Route
          path="*"
          element={
            <CredentialGuard conversationId={conversationId}>
              <div data-testid="children">protected content</div>
            </CredentialGuard>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('CredentialGuard', () => {
  it('credentials 有効のとき children を表示する', async () => {
    mockAuth({ token: 'tok', user: { id: 'u1', name: 'alice' } });
    vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: true, isValid: true });

    renderGuard();

    await waitFor(() => {
      expect(screen.getByTestId('children')).toBeTruthy();
    });
  });

  it('registered=false のとき /login/credentials?reason=missing へ navigate', async () => {
    mockAuth({ token: 'tok', user: { id: 'u1', name: 'alice' } });
    vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: false, isValid: false });

    renderGuard('conv-abc');

    await waitFor(() => {
      expect(capturedPath).toContain('/login/credentials');
      expect(capturedPath).toContain('reason=missing');
      expect(capturedPath).toContain('conversationId=conv-abc');
    });
    expect(screen.queryByTestId('children')).toBeNull();
  });

  it('isValid=false のとき /login/credentials?reason=expired へ navigate', async () => {
    mockAuth({ token: 'tok', user: { id: 'u1', name: 'alice' } });
    vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: true, isValid: false });

    renderGuard('conv-xyz');

    await waitFor(() => {
      expect(capturedPath).toContain('reason=expired');
    });
  });

  it('status API がエラーのとき children を表示する（fail-open）', async () => {
    mockAuth({ token: 'tok', user: { id: 'u1', name: 'alice' } });
    vi.mocked(getCredentialsStatus).mockRejectedValue(new Error('network error'));

    renderGuard();

    await waitFor(() => {
      expect(screen.getByTestId('children')).toBeTruthy();
    });
  });
});
