// mocks (hoisted)
vi.mock('../api/client', () => ({
  listConversations: vi.fn(),
  createConversation: vi.fn(),
  getConversation: vi.fn(),
  deleteConversation: vi.fn(),
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

vi.mock('../components/Sidebar', () => ({
  Sidebar: () => <div data-testid="sidebar" />,
}));

vi.mock('../components/ChatView', () => ({
  ChatView: ({ conversationId }: { conversationId: string }) => (
    <div data-testid="chat-view" data-conversation-id={conversationId} />
  ),
}));

vi.mock('../components/AuthGuard', () => ({
  AuthGuard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { ChatPage } from './ChatPage';
import * as clientModule from '../api/client';
import type { Conversation } from '../api/client';

const flush = async () => {
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
};

function makeConv(id: string): Conversation {
  return {
    id,
    title: 'テスト会話',
    model: 'claude-sonnet-4-6',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    status: 'idle',
  };
}

function renderChatPage(initialPath = '/chat') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/chat" element={<ChatPage />} />
        <Route path="/chat/:id" element={<ChatPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ChatPage', () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    vi.mocked(clientModule.listConversations).mockResolvedValue([]);
    vi.mocked(clientModule.createConversation).mockResolvedValue(makeConv('new-conv-id'));
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ ...makeConv('new-conv-id'), messages: [] } as any);
    const { useConversationsStore } = await import('../store/conversations');
    useConversationsStore.setState({ conversations: [] });
  });

  it('新規会話時（会話未選択）に AgentSelector が表示されること', async () => {
    renderChatPage('/chat');
    await flush();

    expect(screen.getByTestId('agent-selector')).toBeTruthy();
    expect(screen.queryByTestId('chat-view')).toBeNull();
  });

  it('会話ID付き URL では ChatView が表示され AgentSelector は表示されないこと', async () => {
    renderChatPage('/chat/existing-conv');
    await flush();

    expect(screen.queryByTestId('agent-selector')).toBeNull();
    expect(screen.getByTestId('chat-view')).toBeTruthy();
  });

  it('Claude Code 選択後に createConversation が呼ばれ ChatView が表示されること', async () => {
    renderChatPage('/chat');
    await flush();

    expect(screen.getByTestId('agent-selector')).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByTestId('agent-btn-claude-code'));
    });
    await flush();

    expect(clientModule.createConversation).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId('chat-view')).toBeTruthy();
    expect(screen.queryByTestId('agent-selector')).toBeNull();
  });
});
