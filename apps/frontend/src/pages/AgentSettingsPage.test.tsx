vi.mock('../hooks/useAuth', () => ({
  useAuth: vi.fn(),
}));

vi.mock('../components/AuthTerminal', () => ({
  AuthTerminal: () => <div data-testid="auth-terminal" />,
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AgentSettingsPage } from './AgentSettingsPage';
import { useAuth } from '../hooks/useAuth';
import type { UseAuthReturn } from '../hooks/useAuth';
import type { AuthStatus } from '../api/client';

function mockUseAuth(overrides: Partial<UseAuthReturn>) {
  vi.mocked(useAuth).mockReturnValue({
    status: null,
    isLoading: false,
    login: vi.fn(),
    logout: vi.fn(),
    cancelLogin: vi.fn(),
    ...overrides,
  });
}

function makeStatus(overrides: Partial<AuthStatus>): AuthStatus {
  return {
    loggedIn: false,
    loginPending: false,
    authMethod: null,
    ...overrides,
  } as AuthStatus;
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/settings/agents']}>
      <AgentSettingsPage />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('AgentSettingsPage', () => {
  describe('Agent カード表示', () => {
    it('3つの Agent カードが表示される', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: false }) });
      renderPage();
      expect(screen.getAllByText(/Claude Code/).length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText('GitHub Copilot')).toBeTruthy();
      expect(screen.getByText('Cursor CLI')).toBeTruthy();
    });

    it('Claude Code カードに「対応済み」バッジが表示される', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: false }) });
      renderPage();
      expect(screen.getByText(/対応済み/)).toBeTruthy();
    });

    it('GitHub Copilot と Cursor CLI のボタンは disabled', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: false }) });
      renderPage();
      const disabledButtons = screen.getAllByRole('button').filter(
        btn => (btn as HTMLButtonElement).disabled,
      );
      expect(disabledButtons.length).toBeGreaterThanOrEqual(2);
    });
  });

  describe('未認証状態', () => {
    it('「Claude Code でログイン」ボタンが表示される', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: false, loginPending: false }) });
      renderPage();
      expect(screen.getByRole('button', { name: /Claude Code でログイン/ })).toBeTruthy();
    });

    it('「Claude Code でログイン」ボタンは enabled', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: false, loginPending: false }) });
      renderPage();
      const btn = screen.getByRole('button', { name: /Claude Code でログイン/ });
      expect((btn as HTMLButtonElement).disabled).toBe(false);
    });

    it('「Claude Code でログイン」ボタン押下で login() が呼ばれる', async () => {
      const login = vi.fn().mockResolvedValue(undefined);
      mockUseAuth({ status: makeStatus({ loggedIn: false, loginPending: false }), login });
      renderPage();

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: /Claude Code でログイン/ }));
      });
      expect(login).toHaveBeenCalled();
    });
  });

  describe('認証中（loginPending）状態', () => {
    it('AuthTerminal が表示される', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: false, loginPending: true }) });
      renderPage();
      expect(screen.getByTestId('auth-terminal')).toBeTruthy();
    });
  });

  describe('認証済み状態', () => {
    it('「認証済み」表示が出る', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: true, loginPending: false }) });
      renderPage();
      expect(screen.getByText(/認証済み/)).toBeTruthy();
    });

    it('「ログアウト」ボタンが表示される', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: true, loginPending: false }) });
      renderPage();
      expect(screen.getByRole('button', { name: /ログアウト/ })).toBeTruthy();
    });

    it('「ログアウト」ボタン押下で logout() が呼ばれる', async () => {
      const logout = vi.fn().mockResolvedValue(undefined);
      mockUseAuth({ status: makeStatus({ loggedIn: true, loginPending: false }), logout });
      renderPage();

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: /ログアウト/ }));
      });
      expect(logout).toHaveBeenCalled();
    });
  });

  describe('戻るリンク', () => {
    it('戻るリンクが表示される', () => {
      mockUseAuth({ status: makeStatus({ loggedIn: false }) });
      renderPage();
      expect(screen.getByRole('link')).toBeTruthy();
    });
  });
});
