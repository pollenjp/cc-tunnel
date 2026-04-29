vi.mock('../hooks/useAppAuth', () => ({
  useAppAuth: vi.fn(),
}));

vi.mock('../api/credentials', () => ({
  startRelogin: vi.fn(),
  finalizeRelogin: vi.fn(),
}));

vi.mock('../api/client', () => ({
  initiateLogin: vi.fn(),
  getAuthOutput: vi.fn(),
  submitAuthInput: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { useEffect } from 'react';
import { CredentialsLoginPage } from './CredentialsLoginPage';
import { useAppAuth } from '../hooks/useAppAuth';
import { startRelogin, finalizeRelogin } from '../api/credentials';
import { initiateLogin, getAuthOutput, submitAuthInput } from '../api/client';

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
    vi.mocked(getAuthOutput).mockResolvedValue({ data: '', cursor: 0 } as never);
    vi.mocked(finalizeRelogin).mockResolvedValue({ registered: true, isValid: true });
    vi.mocked(submitAuthInput).mockResolvedValue({ ok: true } as never);
  });

  it('起動時に startRelogin と initiateLogin を呼ぶ', async () => {
    renderPage();

    await waitFor(() => {
      expect(startRelogin).toHaveBeenCalledWith('test-token', 'conv-001');
      expect(initiateLogin).toHaveBeenCalledWith('conv-001');
    });
  });

  it('PTY フェーズに入ると pty-output エリアが表示される', async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByLabelText('pty-output')).toBeTruthy();
    });
  });

  it('Login successful 検知で finalizeRelogin が呼ばれチャット画面へ navigate', async () => {
    vi.mocked(getAuthOutput)
      .mockResolvedValueOnce({ data: '', cursor: 0 } as never)
      .mockResolvedValueOnce({ data: btoa('Login successful'), cursor: 20 } as never)
      .mockResolvedValue({ data: '', cursor: 20 } as never);

    renderPage('?reason=missing&conversationId=conv-001');

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

  it('入力を送信できる', async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByLabelText('pty-output')).toBeTruthy();
    });

    const input = screen.getByPlaceholderText('入力…');
    act(() => {
      fireEvent.change(input, { target: { value: 'my-code' } });
    });
    act(() => {
      fireEvent.click(screen.getByRole('button', { name: '送信' }));
    });

    await waitFor(() => {
      expect(submitAuthInput).toHaveBeenCalledWith('conv-001', 'my-code');
    });
  });
});
