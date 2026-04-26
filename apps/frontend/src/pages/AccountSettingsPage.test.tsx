vi.mock('../hooks/useAppAuth', () => ({
  useAppAuth: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AccountSettingsPage } from './AccountSettingsPage';
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

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/settings/account']}>
      <AccountSettingsPage />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('AccountSettingsPage', () => {
  it('現在のユーザー名が表示される', () => {
    mockAuth({ user: { id: 'u1', name: 'Alice' } });
    renderPage();
    expect(screen.getByText('Alice')).toBeTruthy();
  });

  it('inputの初期値が現在のユーザー名になっている', () => {
    mockAuth({ user: { id: 'u1', name: 'Alice' } });
    renderPage();
    const input = screen.getByRole('textbox') as HTMLInputElement;
    expect(input.value).toBe('Alice');
  });

  it('入力変更 → 保存ボタン押下 → updateNickname が呼ばれる', async () => {
    const updateNickname = vi.fn().mockResolvedValue(undefined);
    mockAuth({ user: { id: 'u1', name: 'Alice' }, updateNickname });
    renderPage();

    const input = screen.getByRole('textbox');
    fireEvent.change(input, { target: { value: 'Bob' } });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /保存/ }));
    });
    await act(async () => { await Promise.resolve(); });

    expect(updateNickname).toHaveBeenCalledWith('Bob');
  });

  it('空文字のとき保存ボタンが disabled', () => {
    mockAuth({ user: { id: 'u1', name: 'Alice' } });
    renderPage();

    const input = screen.getByRole('textbox');
    fireEvent.change(input, { target: { value: '' } });

    const saveButton = screen.getByRole('button', { name: /保存/ });
    expect((saveButton as HTMLButtonElement).disabled).toBe(true);
  });

  it('updateNickname 成功後に成功メッセージが表示される', async () => {
    const updateNickname = vi.fn().mockResolvedValue(undefined);
    mockAuth({ user: { id: 'u1', name: 'Alice' }, updateNickname });
    renderPage();

    const input = screen.getByRole('textbox');
    fireEvent.change(input, { target: { value: 'Bob' } });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /保存/ }));
    });
    await act(async () => { await Promise.resolve(); });

    expect(screen.getByText(/保存しました/)).toBeTruthy();
  });

  it('戻るリンクが表示される', () => {
    mockAuth({ user: { id: 'u1', name: 'Alice' } });
    renderPage();
    expect(screen.getByRole('link')).toBeTruthy();
  });
});
