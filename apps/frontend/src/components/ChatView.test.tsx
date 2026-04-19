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
    isStreaming: false,
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

  it('shows empty text block with streaming animation for status=streaming message with no content_blocks when isPolling is true', () => {
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

    const bubble = screen.getByTestId('message-bubble-msg-streaming-empty');
    expect(bubble).toBeTruthy();
    expect(bubble.getAttribute('data-streaming')).toBe('true');
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
});
