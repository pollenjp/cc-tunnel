// ---------- mocks (hoisted) ----------
vi.mock('../api/client', () => ({
  getConversation: vi.fn(),
  sendMessage: vi.fn(),
}));

vi.mock('../hooks/useConversationPoller', () => ({
  useConversationPoller: vi.fn(),
}));

vi.mock('./MessageBubble', () => ({
  MessageBubble: ({
    message,
    textContent,
    isStreaming,
  }: {
    message: { id: string };
    textContent?: string;
    isStreaming?: boolean;
  }) => (
    <div
      data-testid={`message-bubble-${message.id}`}
      data-streaming={isStreaming ? 'true' : 'false'}
    >
      {textContent ?? ''}
    </div>
  ),
  ThinkingAccordion: ({ content }: { content: string }) => (
    <div data-testid="thinking">{content}</div>
  ),
}));

vi.mock('./MessageInput', () => ({
  MessageInput: ({
    onSend,
    onChange,
    value,
    disabled,
  }: {
    onSend: () => void;
    onChange: (v: string) => void;
    value: string;
    disabled: boolean;
    onHamburger: () => void;
  }) => (
    <div>
      <input
        data-testid="message-input-text"
        value={value}
        onChange={e => onChange(e.target.value)}
      />
      <button
        data-testid="message-send-btn"
        onClick={onSend}
        disabled={disabled}
      >
        Send
      </button>
    </div>
  ),
}));

vi.mock('./ToolCallCard', () => ({
  ToolCallCard: () => <div data-testid="tool-call-card" />,
}));

vi.mock('./TypingIndicator', () => ({
  TypingIndicator: () => <div data-testid="typing-indicator" />,
}));

// ---------- imports ----------
import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import { ChatView } from './ChatView';
import * as clientModule from '../api/client';
import type { Message } from '../api/client';

beforeAll(() => {
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
});

const flush = async () => {
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
};

function makeMsg(overrides: Partial<Message> & { id: string }): Message {
  return {
    conversation_id: 'conv-1',
    role: 'assistant',
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function makeConvDetail(status: string, messages: Message[]): any {
  return {
    id: 'conv-1',
    title: 'テスト会話',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    status,
    messages,
  };
}

describe('ChatView', () => {
  const defaultProps = {
    conversationId: 'conv-1' as string | null,
    onConversationUpdate: vi.fn(),
    onHamburger: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [])
    );
    vi.mocked(clientModule.sendMessage).mockResolvedValue({ message_id: 'new-msg' });
  });

  // ===== TDD Cycle 1: ChatView が conversationId から内部で messages を管理 =====

  it('[Cycle1] ChatView が conversationId を受け取り内部で messages を取得すること', async () => {
    const msg = makeMsg({
      id: 'msg-internal',
      role: 'user',
      message_data: { content: 'こんにちは' },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(clientModule.getConversation).toHaveBeenCalledWith('conv-1');
    expect(screen.getByTestId('message-bubble-msg-internal')).toBeTruthy();
  });

  it('[Cycle1] conversationId=null のとき空状態（会話選択プロンプト）が表示されること', () => {
    render(<ChatView {...defaultProps} conversationId={null} />);

    expect(screen.getByText('メッセージを入力して会話を始めましょう。')).toBeTruthy();
  });

  it('[Cycle1] conversationId が変わったとき messages がクリアされ新しい会話が取得されること', async () => {
    const msg1 = makeMsg({ id: 'msg-conv1', role: 'user', message_data: { content: '会話1' } });
    const msg2 = makeMsg({ id: 'msg-conv2', conversation_id: 'conv-2', role: 'user', message_data: { content: '会話2' } });

    vi.mocked(clientModule.getConversation)
      .mockResolvedValueOnce(makeConvDetail('completed', [msg1]))
      .mockResolvedValueOnce(makeConvDetail('completed', [msg2]));

    const { rerender } = render(<ChatView {...defaultProps} conversationId="conv-1" />);
    await flush();
    expect(screen.getByTestId('message-bubble-msg-conv1')).toBeTruthy();

    rerender(<ChatView {...defaultProps} conversationId="conv-2" />);
    await flush();

    expect(screen.queryByTestId('message-bubble-msg-conv1')).toBeNull();
    expect(screen.getByTestId('message-bubble-msg-conv2')).toBeTruthy();
    expect(clientModule.getConversation).toHaveBeenCalledWith('conv-2');
  });

  it('[Cycle1] isRunning が ChatView 内部で streaming messages から計算されること', async () => {
    const msg = makeMsg({
      id: 'msg-streaming-internal',
      role: 'assistant',
      status: 'streaming',
      message_data: {},
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    // streaming message → isRunning=true → TypingIndicator shown
    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  // ===== TDD Cycle 2: メッセージ送信が ChatView 内部で完結 =====

  it('[Cycle2] ChatView 内でメッセージを送信し、送信後にポーリングが開始されること', async () => {
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [])
    );
    vi.mocked(clientModule.sendMessage).mockResolvedValue({ message_id: 'new-msg' });

    render(<ChatView {...defaultProps} />);
    await flush();

    await act(async () => {
      fireEvent.change(screen.getByTestId('message-input-text'), {
        target: { value: 'テストメッセージ' },
      });
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId('message-send-btn'));
    });
    await flush();

    expect(clientModule.sendMessage).toHaveBeenCalledWith('conv-1', 'テストメッセージ');
    // ポーリング開始 → 処理中バナー表示
    expect(screen.getByText('処理中...')).toBeTruthy();
  });

  it('[Cycle3] メッセージ送信開始時に onSendStart が呼ばれること', async () => {
    const onSendStart = vi.fn();
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [])
    );
    vi.mocked(clientModule.sendMessage).mockResolvedValue({ message_id: 'new-msg' });

    render(
      <ChatView
        conversationId="conv-1"
        onConversationUpdate={vi.fn()}
        onSendStart={onSendStart}
        onHamburger={vi.fn()}
      />
    );
    await flush();

    await act(async () => {
      fireEvent.change(screen.getByTestId('message-input-text'), {
        target: { value: 'テスト' },
      });
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId('message-send-btn'));
    });

    expect(onSendStart).toHaveBeenCalledTimes(1);
  });

  it('[Cycle2] 送信中（sending=true）のとき TypingIndicator が表示されること', async () => {
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [])
    );
    // sendMessage が永遠に解決しない → sending=true が継続
    vi.mocked(clientModule.sendMessage).mockReturnValue(new Promise<{ message_id: string }>(() => {}));

    render(<ChatView {...defaultProps} />);
    await flush();

    await act(async () => {
      fireEvent.change(screen.getByTestId('message-input-text'), {
        target: { value: 'テスト' },
      });
    });
    act(() => {
      fireEvent.click(screen.getByTestId('message-send-btn'));
    });

    // sending=true → standalone TypingIndicator 表示
    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  // ===== 既存テスト（新インタフェースに更新） =====

  it('shows content_blocks text with streaming animation for status=streaming message when isPolling is true', async () => {
    const msg = makeMsg({
      id: 'msg-streaming',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '部分的な応答テキスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('running', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByText('部分的な応答テキスト')).toBeTruthy();
    const bubble = screen.getByTestId('message-bubble-msg-streaming');
    expect(bubble.getAttribute('data-streaming')).toBe('true');
  });

  it('shows TypingIndicator (not empty bubble) for status=streaming message with no content_blocks when isPolling is true', async () => {
    const msg = makeMsg({
      id: 'msg-streaming-empty',
      role: 'assistant',
      status: 'streaming',
      message_data: {},
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('running', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
    expect(screen.queryByTestId('message-bubble-msg-streaming-empty')).toBeNull();
  });

  it('shows error display for status=error message', async () => {
    const msg = makeMsg({
      id: 'msg-error',
      role: 'assistant',
      status: 'error',
      message_data: {},
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByText('エラーが発生しました')).toBeTruthy();
  });

  it('does not apply streaming animation for status=streaming message when isPolling is false', async () => {
    const msg = makeMsg({
      id: 'msg-streaming-no-poll',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: 'ポーリングなし' }],
      },
    });
    // completed → isPolling=false
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByText('ポーリングなし')).toBeTruthy();
    const bubble = screen.getByTestId('message-bubble-msg-streaming-no-poll');
    expect(bubble.getAttribute('data-streaming')).toBe('false');
  });

  it('shows TypingIndicator for status=streaming when isPolling is true', async () => {
    const msg = makeMsg({
      id: 'msg-pulse',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '部分テキスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('running', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  it('does not show pulse indicator for status=streaming when isPolling is false', async () => {
    const msg = makeMsg({
      id: 'msg-no-pulse',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '部分テキスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.queryByText('生成中...')).toBeNull();
  });

  it('renders ToolCallCard for tool_use block when status=streaming and isPolling is true', async () => {
    const msg = makeMsg({
      id: 'msg-tool-polling',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'tool_use', tool_use_id: 'tu-1' }],
        tool_calls: [
          {
            tool_use_id: 'tu-1',
            tool_name: 'bash',
            input_json: '{"command":"ls"}',
          },
        ],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('running', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByTestId('tool-call-card')).toBeTruthy();
  });

  it('renders placeholder ToolCallCard for tool_use block when tool_calls data is missing during polling', async () => {
    const msg = makeMsg({
      id: 'msg-tool-no-data',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'tool_use', tool_use_id: 'tu-missing' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('running', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByTestId('tool-call-card')).toBeTruthy();
  });

  it('shows TypingIndicator instead of empty bubble when isPolling=true and content_blocks is empty', async () => {
    const msg = makeMsg({
      id: 'msg-typing-empty',
      role: 'assistant',
      status: 'streaming',
      message_data: {},
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('running', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
    expect(screen.queryByTestId('message-bubble-msg-typing-empty')).toBeNull();
  });

  it('shows text content and TypingIndicator when isPolling=true and content_blocks has text', async () => {
    const msg = makeMsg({
      id: 'msg-typing-text',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '途中のテキスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('running', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByText('途中のテキスト')).toBeTruthy();
    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  it('does not show TypingIndicator when no streaming messages and isPolling=false', async () => {
    // 完了済みメッセージ（streaming ではない）
    const msg = makeMsg({
      id: 'msg-no-typing',
      role: 'assistant',
      status: 'completed',
      message_data: {
        content_blocks: [{ type: 'text', content: '完了テキスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.queryByTestId('typing-indicator')).toBeNull();
  });

  it('shows TypingIndicator when isRunning=true even if isPolling=false (reconnect race condition)', async () => {
    // 会話は completed だが message が streaming のまま（再接続レース）
    const msg = makeMsg({
      id: 'msg-reconnect-race',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '途中テキスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  it('shows TypingIndicator when streaming message has empty message_data', async () => {
    const msg = makeMsg({
      id: 'msg-isrunning-empty',
      role: 'assistant',
      status: 'streaming',
      message_data: {},
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  it('does not show TypingIndicator when no streaming messages', async () => {
    // 完了済みメッセージ（non-streaming）
    const msg = makeMsg({
      id: 'msg-no-running',
      role: 'assistant',
      status: 'completed',
      message_data: {
        content_blocks: [{ type: 'text', content: '完了テキスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.queryByTestId('typing-indicator')).toBeNull();
  });

  it('shows content_blocks text when isPolling=false with streaming message', async () => {
    const msg = makeMsg({
      id: 'msg-running-content',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: 'isRunning途中テキスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByText('isRunning途中テキスト')).toBeTruthy();
  });

  it('shows TypingIndicator only (no bubble) when streaming message with empty content_blocks', async () => {
    const msg = makeMsg({
      id: 'msg-running-empty',
      role: 'assistant',
      status: 'streaming',
      message_data: {},
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
    expect(screen.queryByTestId('message-bubble-msg-running-empty')).toBeNull();
  });

  it('shows TypingIndicator after content when streaming message has content_blocks', async () => {
    const msg = makeMsg({
      id: 'msg-running-indicator',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: 'インジケータテスト' }],
      },
    });
    vi.mocked(clientModule.getConversation).mockResolvedValue(
      makeConvDetail('completed', [msg])
    );

    render(<ChatView {...defaultProps} />);
    await flush();

    expect(screen.getByText('インジケータテスト')).toBeTruthy();
    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });
});
