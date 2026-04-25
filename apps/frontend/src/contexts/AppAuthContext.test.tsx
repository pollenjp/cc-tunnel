vi.mock('../api/app-auth', () => ({
  login: vi.fn(),
  getMe: vi.fn(),
  logout: vi.fn(),
  updateMe: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import { AppAuthProvider, useAppAuth } from './AppAuthContext';
import * as appAuthApi from '../api/app-auth';

function TestConsumer() {
  const { user, token, isLoading, login, logout, updateNickname } = useAppAuth();
  return (
    <div>
      <div data-testid="loading">{isLoading ? 'loading' : 'ready'}</div>
      <div data-testid="user">{user ? user.name : 'null'}</div>
      <div data-testid="token">{token ?? 'null'}</div>
      <button onClick={() => { void login('Alice'); }}>login</button>
      <button onClick={() => { void logout(); }}>logout</button>
      <button onClick={() => { void updateNickname('NewName'); }}>update</button>
    </div>
  );
}

function renderWithProvider() {
  return render(
    <AppAuthProvider>
      <TestConsumer />
    </AppAuthProvider>,
  );
}

const flush = async () => {
  await act(async () => {
    await Promise.resolve();
  });
  await act(async () => {
    await Promise.resolve();
  });
};

beforeEach(() => {
  vi.clearAllMocks();
  sessionStorage.clear();
});

describe('AppAuthContext', () => {
  it('sessionStorage にトークンなし → isLoading=false, user=null', async () => {
    renderWithProvider();
    await flush();

    expect(screen.getByTestId('loading').textContent).toBe('ready');
    expect(screen.getByTestId('user').textContent).toBe('null');
    expect(screen.getByTestId('token').textContent).toBe('null');
  });

  it('sessionStorage にトークンあり → getMe が呼ばれ user が復元される', async () => {
    sessionStorage.setItem('app_auth_token', 'saved-token');
    vi.mocked(appAuthApi.getMe).mockResolvedValue({ user: { id: 'u1', name: 'Alice' } });

    renderWithProvider();
    await flush();

    expect(appAuthApi.getMe).toHaveBeenCalledWith('saved-token');
    expect(screen.getByTestId('user').textContent).toBe('Alice');
    expect(screen.getByTestId('token').textContent).toBe('saved-token');
  });

  it('sessionStorage のトークンが無効 → sessionStorage がクリアされ user=null', async () => {
    sessionStorage.setItem('app_auth_token', 'bad-token');
    vi.mocked(appAuthApi.getMe).mockRejectedValue(new Error('Unauthorized'));

    renderWithProvider();
    await flush();

    expect(sessionStorage.getItem('app_auth_token')).toBeNull();
    expect(screen.getByTestId('user').textContent).toBe('null');
  });

  it('login() → sessionStorage にトークン保存, user が設定される', async () => {
    vi.mocked(appAuthApi.login).mockResolvedValue({
      token: 'new-token',
      user: { id: 'u1', name: 'Alice' },
    });

    renderWithProvider();
    await flush();

    await act(async () => {
      fireEvent.click(screen.getByText('login'));
    });

    expect(sessionStorage.getItem('app_auth_token')).toBe('new-token');
    expect(screen.getByTestId('user').textContent).toBe('Alice');
    expect(screen.getByTestId('token').textContent).toBe('new-token');
  });

  it('logout() → sessionStorage がクリアされ user=null', async () => {
    sessionStorage.setItem('app_auth_token', 'tok');
    vi.mocked(appAuthApi.getMe).mockResolvedValue({ user: { id: 'u1', name: 'Alice' } });
    vi.mocked(appAuthApi.logout).mockResolvedValue(undefined);

    renderWithProvider();
    await flush();

    await act(async () => {
      fireEvent.click(screen.getByText('logout'));
    });

    expect(sessionStorage.getItem('app_auth_token')).toBeNull();
    expect(screen.getByTestId('user').textContent).toBe('null');
  });

  it('updateNickname() → user.name が更新される', async () => {
    sessionStorage.setItem('app_auth_token', 'tok');
    vi.mocked(appAuthApi.getMe).mockResolvedValue({ user: { id: 'u1', name: 'Alice' } });
    vi.mocked(appAuthApi.updateMe).mockResolvedValue({ user: { id: 'u1', name: 'NewName' } });

    renderWithProvider();
    await flush();

    await act(async () => {
      fireEvent.click(screen.getByText('update'));
    });

    expect(appAuthApi.updateMe).toHaveBeenCalledWith('tok', 'NewName');
    expect(screen.getByTestId('user').textContent).toBe('NewName');
  });
});
