import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ChatView } from './ChatView';

// jsdom doesn't implement scrollIntoView
beforeAll(() => {
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
});

// Mock subcomponents to isolate ChatView logic
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
  MessageInput: () => <div data-testid="message-input" />,
}));

vi.mock('./ToolCallCard', () => ({
  ToolCallCard: () => <div data-testid="tool-call-card" />,
}));

vi.mock('./TypingIndicator', () => ({
  TypingIndicator: () => <div data-testid="typing-indicator" />,
}));

import type { Message } from '../api/client';

function makeMsg(overrides: Partial<Message> & { id: string }): Message {
  return {
    conversation_id: 'conv-1',
    role: 'assistant',
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('ChatView', () => {
  const defaultProps = {
    onSend: vi.fn(),
    input: '',
    onInputChange: vi.fn(),
    onHamburger: vi.fn(),
  };

  it('shows content_blocks text with streaming animation for status=streaming message when isPolling is true', () => {
    const msg = makeMsg({
      id: 'msg-streaming',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '部分的な応答テキスト' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={true}
      />,
    );

    expect(screen.getByText('部分的な応答テキスト')).toBeTruthy();
    // Should render with streaming animation (isStreaming=true)
    const bubble = screen.getByTestId('message-bubble-msg-streaming');
    expect(bubble.getAttribute('data-streaming')).toBe('true');
  });

  it('shows TypingIndicator (not empty bubble) for status=streaming message with no content_blocks when isPolling is true', () => {
    const msg = makeMsg({
      id: 'msg-streaming-empty',
      role: 'assistant',
      status: 'streaming',
      message_data: {},
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={true}
      />,
    );

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
    expect(screen.queryByTestId('message-bubble-msg-streaming-empty')).toBeNull();
  });

  it('shows error display for status=error message', () => {
    const msg = makeMsg({
      id: 'msg-error',
      role: 'assistant',
      status: 'error',
      message_data: {},
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={false}
      />,
    );

    expect(screen.getByText('エラーが発生しました')).toBeTruthy();
  });

  it('does not apply streaming animation for status=streaming message when isPolling is false', () => {
    const msg = makeMsg({
      id: 'msg-streaming-no-poll',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: 'ポーリングなし' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={false}
      />,
    );

    // content renders normally but without streaming animation
    expect(screen.getByText('ポーリングなし')).toBeTruthy();
    const bubble = screen.getByTestId('message-bubble-msg-streaming-no-poll');
    expect(bubble.getAttribute('data-streaming')).toBe('false');
  });

  it('shows TypingIndicator for status=streaming when isPolling is true', () => {
    const msg = makeMsg({
      id: 'msg-pulse',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '部分テキスト' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={true}
      />,
    );

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  it('does not show pulse indicator for status=streaming when isPolling is false', () => {
    const msg = makeMsg({
      id: 'msg-no-pulse',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '部分テキスト' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={false}
      />,
    );

    expect(screen.queryByText('生成中...')).toBeNull();
  });

  it('renders ToolCallCard for tool_use block when status=streaming and isPolling is true', () => {
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

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={true}
      />,
    );

    expect(screen.getByTestId('tool-call-card')).toBeTruthy();
  });

  it('renders placeholder ToolCallCard for tool_use block when tool_calls data is missing during polling', () => {
    const msg = makeMsg({
      id: 'msg-tool-no-data',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'tool_use', tool_use_id: 'tu-missing' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={true}
      />,
    );

    // Even without tool_calls data, a placeholder ToolCallCard should be rendered.
    expect(screen.getByTestId('tool-call-card')).toBeTruthy();
  });

  it('shows TypingIndicator instead of empty bubble when isPolling=true and content_blocks is empty', () => {
    const msg = makeMsg({
      id: 'msg-typing-empty',
      role: 'assistant',
      status: 'streaming',
      message_data: {},
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={true}
      />,
    );

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
    expect(screen.queryByTestId('message-bubble-msg-typing-empty')).toBeNull();
  });

  it('shows text content and TypingIndicator when isPolling=true and content_blocks has text', () => {
    const msg = makeMsg({
      id: 'msg-typing-text',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '途中のテキスト' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={true}
      />,
    );

    expect(screen.getByText('途中のテキスト')).toBeTruthy();
    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  it('does not show TypingIndicator when isPolling=false and isRunning=false', () => {
    const msg = makeMsg({
      id: 'msg-no-typing',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '完了テキスト' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={false}
      />,
    );

    expect(screen.queryByTestId('typing-indicator')).toBeNull();
  });

  it('shows TypingIndicator when isRunning=true even if isPolling=false (reconnect race condition)', () => {
    const msg = makeMsg({
      id: 'msg-reconnect-race',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '途中テキスト' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={false}
        isStreaming={false}
        isRunning={true}
      />,
    );

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  it('shows TypingIndicator when isRunning=true with empty message_data', () => {
    const msg = makeMsg({
      id: 'msg-isrunning-empty',
      role: 'assistant',
      status: 'streaming',
      message_data: {},
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={false}
        isRunning={true}
      />,
    );

    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });

  it('does not show TypingIndicator when isRunning=false and isPolling=false', () => {
    const msg = makeMsg({
      id: 'msg-no-running',
      role: 'assistant',
      status: 'streaming',
      message_data: {
        content_blocks: [{ type: 'text', content: '完了テキスト' }],
      },
    });

    render(
      <ChatView
        {...defaultProps}
        messages={[msg]}
        isPolling={false}
        isRunning={false}
      />,
    );

    expect(screen.queryByTestId('typing-indicator')).toBeNull();
  });

});
