vi.mock('../hooks/useAppAuth', () => ({
  useAppAuth: vi.fn(),
}));

vi.mock('../api/credentials', () => ({
  startRelogin: vi.fn(),
  finalizeRelogin: vi.fn(),
}));

vi.mock('../api/client', () => ({
  initiateLogin: vi.fn(),
}));

let capturedOnTextOutput: ((text: string) => void) | undefined;
vi.mock('../components/AuthTerminal', () => ({
  AuthTerminal: ({ onTextOutput }: { onTextOutput?: (text: string) => void }) => {
    capturedOnTextOutput = onTextOutput;
    return <div aria-label="pty-output" data-testid="auth-terminal" />;
  },
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { useEffect } from 'react';
import { CredentialsLoginPage } from './CredentialsLoginPage';
import { useAppAuth } from '../hooks/useAppAuth';
import { startRelogin, finalizeRelogin } from '../api/credentials';
import { initiateLogin } from '../api/client';

let capturedPath = '/login/credentials';

function LocationCapture() {
  const location = useLocation();
  useEffect(() => {
    capturedPath = location.pathname + location.search;
  }, [location.pathname, location.search]);
  return null;
}

function renderPage(search = '?reason=missing&conversationId=conv-001') {
  capturedPath = `/login/credentials${search}`;
  return render(
    <MemoryRouter initialEntries={[`/login/credentials${search}`]}>
      <Routes>
        <Route path="/login/credentials" element={<CredentialsLoginPage />} />
        <Route path="/chat/:id" element={<LocationCapture />} />
        <Route path="/chat" element={<LocationCapture />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('CredentialsLoginPage', () => {
  beforeEach(() => {
    capturedOnTextOutput = undefined;
    vi.mocked(useAppAuth).mockReturnValue({
      user: { id: 'u1', name: 'alice' },
      token: 'test-token',
      isLoading: false,
      login: vi.fn(),
      logout: vi.fn(),
      updateNickname: vi.fn(),
    });
    vi.mocked(startRelogin).mockResolvedValue({ ready: true });
    vi.mocked(initiateLogin).mockResolvedValue({ message: 'ok' } as never);
    vi.mocked(finalizeRelogin).mockResolvedValue({ registered: true, isValid: true });
  });

  it('起動時に startRelogin と initiateLogin を呼ぶ', async () => {
    renderPage();

    await waitFor(() => {
      expect(startRelogin).toHaveBeenCalledWith('test-token', 'conv-001');
      expect(initiateLogin).toHaveBeenCalledWith('conv-001');
    });
  });

  it('PTY フェーズに入ると AuthTerminal が表示される', async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByTestId('auth-terminal')).toBeTruthy();
    });
  });

  it('onTextOutput で "Login successful" を渡すと finalizeRelogin が呼ばれチャット画面へ navigate', async () => {
    renderPage('?reason=missing&conversationId=conv-001');

    await waitFor(() => {
      expect(screen.getByTestId('auth-terminal')).toBeTruthy();
    });

    await act(async () => {
      capturedOnTextOutput?.('Login successful');
    });

    await waitFor(() => {
      expect(finalizeRelogin).toHaveBeenCalledWith('test-token', 'conv-001');
    });

    await waitFor(() => {
      expect(capturedPath).toBe('/chat/conv-001');
    });
  });

  it('startRelogin 失敗時にエラーメッセージを表示する', async () => {
    vi.mocked(startRelogin).mockRejectedValue(new Error('container start failed'));

    renderPage();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByRole('alert').textContent).toContain('container start failed');
    });
  });
});
