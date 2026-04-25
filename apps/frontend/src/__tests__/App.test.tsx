vi.mock('../components/ChatView', () => ({
  ChatView: ({
    conversationId,
    onSendStart,
  }: {
    conversationId: string | null;
    onConversationUpdate?: () => void;
    onSendStart?: () => void;
    onHamburger: () => void;
  }) => (
    <div
      data-testid="chat-view"
      data-conversation-id={conversationId ?? ''}
    >
      <button data-testid="send-start-btn" onClick={onSendStart}>
        SendStart
      </button>
    </div>
  ),
}));

vi.mock('../api/client', () => ({
  listConversations: vi.fn(),
  createConversation: vi.fn(),
  getConversation: vi.fn(),
  deleteConversation: vi.fn(),
  sendMessage: vi.fn(),
}));

vi.mock('../hooks/useAuth', () => ({
  useAuth: () => ({
    status: { loggedIn: true, authMethod: 'api_key' },
    isLoading: false,
    login: vi.fn(),
    logout: vi.fn(),
    cancelLogin: vi.fn(),
  }),
}));

vi.mock('../hooks/useConversationListPoller', () => ({
  useConversationListPoller: vi.fn(),
}));

vi.mock('../contexts/AppAuthProvider', () => ({
  AppAuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('../hooks/useAppAuth', () => ({
  useAppAuth: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, act, screen } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from '../App';
import * as clientModule from '../api/client';
import type { Conversation } from '../api/client';
import { useConversationListPoller } from '../hooks/useConversationListPoller';
import { useAppAuth } from '../hooks/useAppAuth';

function makeConv(overrides: Partial<Conversation> & { id: string }): Conversation {
  return {
    title: 'テスト会話',
    model: 'claude-sonnet-4-6',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    status: 'idle',
    ...overrides,
  };
}

const flush = async () => {
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
};

function mockAppAuth(overrides: { user?: { id: string; name: string } | null } = {}) {
  vi.mocked(useAppAuth).mockReturnValue({
    user: overrides.user !== undefined ? overrides.user : { id: 'test-user', name: 'Test User' },
    token: 'test-token',
    isLoading: false,
    login: vi.fn(),
    logout: vi.fn(),
    updateNickname: vi.fn(),
  });
}

describe('App (TDD Cycle 3 — ChatView receives conversationId)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAppAuth();
  });

  it('選択した会話のIDがChatViewのconversationIdとして渡されること', async () => {
    const conv = makeConv({ id: 'conv-1', title: '会話A' });

    vi.mocked(clientModule.listConversations).mockResolvedValue([conv]);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ ...conv, messages: [] } as any);

    render(<MemoryRouter initialEntries={['/chat']}><App /></MemoryRouter>);
    await flush();

    await act(async () => {
      fireEvent.click(screen.getByText('会話A'));
    });
    await flush();

    const chatView = screen.getByTestId('chat-view');
    expect(chatView.getAttribute('data-conversation-id')).toBe('conv-1');
  });

  it('AppはChatViewにisRunning/messages/isPollingを渡さないこと', async () => {
    const conv = makeConv({ id: 'conv-2', title: '会話B' });

    vi.mocked(clientModule.listConversations).mockResolvedValue([conv]);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ ...conv, messages: [] } as any);

    render(<MemoryRouter initialEntries={['/chat']}><App /></MemoryRouter>);
    await flush();

    await act(async () => {
      fireEvent.click(screen.getByText('会話B'));
    });
    await flush();

    const chatView = screen.getByTestId('chat-view');
    // これらの props は App が ChatView に渡さないこと
    expect(chatView.getAttribute('data-is-running')).toBeNull();
    expect(chatView.getAttribute('data-messages')).toBeNull();
    expect(chatView.getAttribute('data-is-polling')).toBeNull();
  });

  it('onSendStart が呼ばれると useConversationListPoller に hasRunning=true が渡されること', async () => {
    const conv = makeConv({ id: 'conv-spinning', status: 'idle', title: '会話C' });

    vi.mocked(clientModule.listConversations).mockResolvedValue([conv]);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ ...conv, messages: [] } as any);

    render(<MemoryRouter initialEntries={['/chat/conv-spinning']}><App /></MemoryRouter>);
    await flush();

    // 初期状態では hasRunning=false
    expect(vi.mocked(useConversationListPoller)).toHaveBeenLastCalledWith(
      expect.objectContaining({ hasRunning: false })
    );

    // onSendStart をシミュレート
    await act(async () => {
      fireEvent.click(screen.getByTestId('send-start-btn'));
    });
    await flush();

    // hasRunning=true になること
    expect(vi.mocked(useConversationListPoller)).toHaveBeenLastCalledWith(
      expect.objectContaining({ hasRunning: true })
    );
  });

  it('会話未選択時は ChatView が表示されないこと', async () => {
    vi.mocked(clientModule.listConversations).mockResolvedValue([]);

    render(<MemoryRouter initialEntries={['/chat']}><App /></MemoryRouter>);
    await flush();

    expect(screen.queryByTestId('chat-view')).toBeNull();
    expect(screen.getByTestId('agent-selector')).toBeTruthy();
  });
});

describe('App ルーティング (Phase 2)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(clientModule.listConversations).mockResolvedValue([]);
  });

  it('/ → HomePage が表示されること', async () => {
    mockAppAuth();

    render(<MemoryRouter initialEntries={['/']}><App /></MemoryRouter>);
    await flush();

    expect(screen.getByRole('heading', { name: 'cc-tunnel' })).toBeTruthy();
    expect(screen.getByText('チャット開始')).toBeTruthy();
  });

  it('/login → LoginPage が表示されること（未認証時）', async () => {
    mockAppAuth({ user: null });

    render(<MemoryRouter initialEntries={['/login']}><App /></MemoryRouter>);
    await flush();

    expect(screen.getByRole('heading', { name: 'ログイン' })).toBeTruthy();
    expect(screen.getByPlaceholderText('ユーザー名')).toBeTruthy();
  });

  it('/chat → 未認証時 /login にリダイレクトされること', async () => {
    mockAppAuth({ user: null });

    render(<MemoryRouter initialEntries={['/chat']}><App /></MemoryRouter>);
    await flush();

    expect(screen.getByRole('heading', { name: 'ログイン' })).toBeTruthy();
    expect(screen.queryByText(/左のサイドバーから/)).toBeNull();
  });

  it('/conversation/:id → /chat/:id にリダイレクトされること', async () => {
    mockAppAuth();
    vi.mocked(clientModule.listConversations).mockResolvedValue([]);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ id: 'c1', messages: [] } as any);

    render(<MemoryRouter initialEntries={['/conversation/c1']}><App /></MemoryRouter>);
    await flush();

    // /chat/c1 にリダイレクトされ ChatPage + ChatView が表示されること
    const chatView = screen.getByTestId('chat-view');
    expect(chatView).toBeTruthy();
    expect(chatView.getAttribute('data-conversation-id')).toBe('c1');
  });

  it('/settings/account → 未認証時 /login にリダイレクトされること', async () => {
    mockAppAuth({ user: null });

    render(<MemoryRouter initialEntries={['/settings/account']}><App /></MemoryRouter>);
    await flush();

    expect(screen.getByRole('heading', { name: 'ログイン' })).toBeTruthy();
  });
});
