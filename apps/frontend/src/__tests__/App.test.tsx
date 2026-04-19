vi.mock('../components/ChatView', () => ({
  ChatView: ({ onSend }: { onSend: (content: string) => void }) => (
    <div data-testid="chat-view">
      <button data-testid="send-btn" onClick={() => onSend('hello')}>送信</button>
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

vi.mock('../hooks/useConversationPoller', () => ({
  useConversationPoller: vi.fn(),
}));

vi.mock('../hooks/useConversationListPoller', () => ({
  useConversationListPoller: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, act, screen } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
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

/** React 19 + @testing-library/react v16 での非同期 state flush パターン */
const flush = async () => {
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
};

describe('App 楽観的 status 更新 (TDD Cycle 1)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('handleSend 呼び出し後すぐ、サイドバーの当該会話が status=running になる', async () => {
    const conv = makeConv({ id: 'conv-1', status: 'idle', title: '会話A' });

    vi.mocked(clientModule.listConversations).mockResolvedValue([conv]);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(clientModule.getConversation).mockResolvedValue({ ...conv, messages: [] } as any);
    // sendMessage は永遠に解決しない（await 中の中間状態をテストするため）
    vi.mocked(clientModule.sendMessage).mockReturnValue(new Promise<void>(() => {}));

    render(<App />);
    await flush(); // listConversations が解決し conversations が更新される

    // Sidebar に会話が表示されていること
    expect(screen.queryByText('会話A')).not.toBeNull();

    // 会話をクリックして選択 → getConversation が呼ばれ ChatView が表示される
    await act(async () => {
      fireEvent.click(screen.getByText('会話A'));
    });
    await flush();

    expect(screen.queryByTestId('send-btn')).not.toBeNull();

    // 送信前: サイドバーの会話一覧にスピナーなし
    const sidebarList = document.querySelector('aside ul');
    expect(sidebarList?.querySelectorAll('.animate-spin').length).toBe(0);

    // 送信ボタンをクリック → handleSend が呼ばれ楽観的更新が走る
    act(() => {
      fireEvent.click(screen.getByTestId('send-btn'));
    });

    // 送信後（sendMessage 待機中）: サイドバーの当該会話にスピナーが表示される
    const spinnersAfter = sidebarList?.querySelectorAll('.animate-spin');
    expect(spinnersAfter?.length).toBeGreaterThanOrEqual(1);
  });
});
