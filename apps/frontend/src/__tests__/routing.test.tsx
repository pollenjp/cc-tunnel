// TDD Cycle: URLルーティング (React Router)

vi.mock('../components/ChatView', () => ({
  ChatView: ({
    conversationId,
  }: {
    conversationId: string | null;
    onConversationUpdate?: () => void;
    onHamburger: () => void;
  }) => (
    <div
      data-testid="chat-view"
      data-conversation-id={conversationId ?? ''}
    />
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

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useEffect } from 'react';
import { render, act, screen } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import App from '../App';
import * as clientModule from '../api/client';
import type { Conversation } from '../api/client';

function makeConv(overrides: Partial<Conversation> & { id: string }): Conversation {
  return {
    title: 'テスト会話',
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

// URL変化を追跡するコンポーネント
let capturedPath = '/';
function LocationCapture() {
  const location = useLocation();
  useEffect(() => {
    capturedPath = location.pathname;
  }, [location.pathname]);
  return null;
}

describe('URLルーティング (TDD Cycle 1)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedPath = '/';
  });

  it('/ にアクセスすると会話が選択されず ChatView が非表示', async () => {
    vi.mocked(clientModule.listConversations).mockResolvedValue([]);

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>
    );
    await flush();

    expect(screen.queryByTestId('chat-view')).toBeNull();
  });

  it('/conversation/:id アクセス時 getConversation(id) が呼ばれ ChatView が表示される', async () => {
    const conv = makeConv({ id: 'conv-xxx', title: '会話XXX' });
    vi.mocked(clientModule.listConversations).mockResolvedValue([conv]);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ ...conv, messages: [] } as any);

    render(
      <MemoryRouter initialEntries={['/conversation/conv-xxx']}>
        <App />
      </MemoryRouter>
    );
    await flush();

    expect(clientModule.getConversation).toHaveBeenCalledWith('conv-xxx');
    expect(screen.queryByTestId('chat-view')).not.toBeNull();
  });

  it('サイドバーで会話を選択すると URL が /conversation/{id} に変わる', async () => {
    const conv = makeConv({ id: 'conv-abc', title: '会話ABC' });
    vi.mocked(clientModule.listConversations).mockResolvedValue([conv]);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ ...conv, messages: [] } as any);

    render(
      <MemoryRouter initialEntries={['/']}>
        <LocationCapture />
        <App />
      </MemoryRouter>
    );
    await flush();

    await act(async () => {
      fireEvent.click(screen.getByText('会話ABC'));
    });
    await flush();

    expect(capturedPath).toBe('/conversation/conv-abc');
  });

  it('存在しない会話IDにアクセスした場合 / へリダイレクトされ ChatView が非表示', async () => {
    const conv = makeConv({ id: 'conv-existing', title: '既存会話' });
    vi.mocked(clientModule.listConversations).mockResolvedValue([conv]);
    vi.mocked(clientModule.getConversation).mockRejectedValue(new Error('Not found'));

    render(
      <MemoryRouter initialEntries={['/conversation/non-existent']}>
        <LocationCapture />
        <App />
      </MemoryRouter>
    );
    await flush();

    expect(screen.queryByTestId('chat-view')).toBeNull();
    expect(capturedPath).toBe('/');
  });
});
