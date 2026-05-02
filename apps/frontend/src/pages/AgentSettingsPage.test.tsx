vi.mock('../hooks/useAppAuth', () => ({
  useAppAuth: vi.fn(),
}));

vi.mock('../api/credentials', () => ({
  getCredentialsStatus: vi.fn(),
}));

vi.mock('../api/client', () => ({
  createConversation: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { useEffect } from 'react';
import { AgentSettingsPage } from './AgentSettingsPage';
import { useAppAuth } from '../hooks/useAppAuth';
import { getCredentialsStatus } from '../api/credentials';
import { createConversation } from '../api/client';

let capturedPath = '/settings/agents';

function LocationCapture() {
  const location = useLocation();
  useEffect(() => {
    capturedPath = location.pathname + location.search;
  }, [location.pathname, location.search]);
  return null;
}

function renderPage() {
  capturedPath = '/settings/agents';
  return render(
    <MemoryRouter initialEntries={['/settings/agents']}>
      <Routes>
        <Route path="/settings/agents" element={<AgentSettingsPage />} />
        <Route path="/login/credentials" element={<LocationCapture />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useAppAuth).mockReturnValue({
    user: { id: 'u1', name: 'alice' },
    token: 'test-token',
    isLoading: false,
    login: vi.fn(),
    logout: vi.fn(),
    updateNickname: vi.fn(),
  });
});

describe('AgentSettingsPage', () => {
  describe('Agent カード表示', () => {
    it('3つの Agent カードが表示される', async () => {
      vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: true, isValid: true });
      renderPage();
      await waitFor(() => expect(screen.getByText(/認証済み/)).toBeTruthy());
      expect(screen.getAllByText(/Claude Code/).length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText('GitHub Copilot')).toBeTruthy();
      expect(screen.getByText('Cursor CLI')).toBeTruthy();
    });

    it('Claude Code カードに「対応済み」バッジが表示される', async () => {
      vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: true, isValid: true });
      renderPage();
      await waitFor(() => expect(screen.getByText(/対応済み/)).toBeTruthy());
    });

    it('GitHub Copilot と Cursor CLI のボタンは disabled', async () => {
      vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: false, isValid: false });
      renderPage();
      await waitFor(() =>
        expect(screen.getByRole('button', { name: /Claude Code でログイン/ })).toBeTruthy(),
      );
      const disabledButtons = screen.getAllByRole('button').filter(
        btn => (btn as HTMLButtonElement).disabled,
      );
      expect(disabledButtons.length).toBeGreaterThanOrEqual(2);
    });
  });

  describe('credential 未登録状態', () => {
    beforeEach(() => {
      vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: false, isValid: false });
    });

    it('「Claude Code でログイン」ボタンが表示される', async () => {
      renderPage();
      await waitFor(() =>
        expect(screen.getByRole('button', { name: /Claude Code でログイン/ })).toBeTruthy(),
      );
    });

    it('「Claude Code でログイン」ボタン押下で /login/credentials に遷移する', async () => {
      vi.mocked(createConversation).mockResolvedValue({ id: 'conv-abc', status: 'idle', createdAt: '' } as never);
      renderPage();

      await waitFor(() =>
        expect(screen.getByRole('button', { name: /Claude Code でログイン/ })).toBeTruthy(),
      );

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: /Claude Code でログイン/ }));
      });

      await waitFor(() => {
        expect(capturedPath).toContain('/login/credentials');
        expect(capturedPath).toContain('conversationId=conv-abc');
      });
    });
  });

  describe('credential 有効状態', () => {
    it('「認証済み ✓」表示が出る', async () => {
      vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: true, isValid: true });
      renderPage();
      await waitFor(() => expect(screen.getByText(/認証済み/)).toBeTruthy());
    });

    it('ログアウトボタンは表示されない', async () => {
      vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: true, isValid: true });
      renderPage();
      await waitFor(() => expect(screen.getByText(/認証済み/)).toBeTruthy());
      expect(screen.queryByRole('button', { name: /ログアウト/ })).toBeNull();
    });
  });

  describe('ローディング状態', () => {
    it('取得中はスピナーが表示される', () => {
      vi.mocked(getCredentialsStatus).mockReturnValue(new Promise(() => {}));
      renderPage();
      expect(screen.getByRole('status')).toBeTruthy();
    });
  });

  describe('戻るリンク', () => {
    it('ホームへの戻るリンクが表示される', async () => {
      vi.mocked(getCredentialsStatus).mockResolvedValue({ registered: true, isValid: true });
      renderPage();
      await waitFor(() => expect(screen.getByText(/認証済み/)).toBeTruthy());
      expect(screen.getByRole('link')).toBeTruthy();
    });
  });
});
