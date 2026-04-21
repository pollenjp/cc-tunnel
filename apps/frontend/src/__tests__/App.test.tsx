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
import { render, act, screen } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
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

describe('App (TDD Cycle 3 — ChatView receives conversationId)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('選択した会話のIDがChatViewのconversationIdとして渡されること', async () => {
    const conv = makeConv({ id: 'conv-1', title: '会話A' });

    vi.mocked(clientModule.listConversations).mockResolvedValue([conv]);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ ...conv, messages: [] } as any);

    render(<MemoryRouter initialEntries={['/']}><App /></MemoryRouter>);
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

    render(<MemoryRouter initialEntries={['/']}><App /></MemoryRouter>);
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

  it('会話未選択時は ChatView が表示されないこと', async () => {
    vi.mocked(clientModule.listConversations).mockResolvedValue([]);

    render(<MemoryRouter initialEntries={['/']}><App /></MemoryRouter>);
    await flush();

    expect(screen.queryByTestId('chat-view')).toBeNull();
    expect(screen.getByText(/左のサイドバーから/)).toBeTruthy();
  });
});
