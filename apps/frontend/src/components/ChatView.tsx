import { useEffect, useRef } from 'react';
import type { Message, SSEHookEvent } from '../api/client';
import type { StreamMeta, ToolCall } from '../App';
import { MessageBubble } from './MessageBubble';
import { MessageInput } from './MessageInput';
import { ToolCallCard } from './ToolCallCard';

interface ChatViewProps {
  messages: Message[];
  onSend: (content: string) => void;
  isStreaming: boolean;
  streamMeta?: StreamMeta | null;
  hookEvents?: SSEHookEvent[];
  toolCalls?: ToolCall[];
  streamingThinkings?: string[];
  input: string;
  onInputChange: (value: string) => void;
  onHamburger: () => void;
}

export function ChatView({ messages, onSend, isStreaming, streamMeta, hookEvents, toolCalls, streamingThinkings, input, onInputChange, onHamburger }: ChatViewProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const isEmpty = messages.length === 0;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {isEmpty ? (
        <div className="flex-1 flex items-center justify-center text-[var(--color-text)]">
          <p>メッセージを入力して会話を始めましょう。</p>
        </div>
      ) : (
        <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
          {messages.map((msg, idx) => {
            const isLast = idx === messages.length - 1;
            const isStreamingMsg = isStreaming && isLast && msg.role === 'assistant';
            return (
              <div key={msg.id}>
                <MessageBubble
                  message={msg}
                  isStreaming={isStreamingMsg}
                  streamingThinkings={isLast && msg.role === 'assistant' ? streamingThinkings : undefined}
                  streamMeta={isStreamingMsg ? streamMeta : undefined}
                  hookEvents={isLast && msg.role === 'assistant' ? hookEvents : undefined}
                />
                {isLast && msg.role === 'assistant' && toolCalls && toolCalls.length > 0 && (
                  <div className="mt-1 space-y-1">
                    {toolCalls.map((tc, i) => (
                      <ToolCallCard key={i} toolCall={tc} />
                    ))}
                  </div>
                )}
              </div>
            );
          })}
          <div ref={messagesEndRef} />
        </div>
      )}
      <MessageInput
        value={input}
        onChange={onInputChange}
        onSend={() => onSend(input)}
        disabled={isStreaming}
        onHamburger={onHamburger}
      />
    </div>
  );
}
