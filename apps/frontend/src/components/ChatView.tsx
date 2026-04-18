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
  input: string;
  onInputChange: (value: string) => void;
  onHamburger: () => void;
}

export function ChatView({ messages, onSend, isStreaming, streamMeta, hookEvents, toolCalls, input, onInputChange, onHamburger }: ChatViewProps) {
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
                />
                {isStreamingMsg && toolCalls && toolCalls.length > 0 && (
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
      {isStreaming && streamMeta && (
        <div className="px-4 py-1 text-[11px] text-[var(--color-text)] opacity-60 flex gap-4 border-t border-[var(--color-border)]">
          {streamMeta.model && <span>🤖 {streamMeta.model}</span>}
          {streamMeta.totalCostUSD !== undefined && (
            <span>💰 ${streamMeta.totalCostUSD.toFixed(4)}</span>
          )}
          {streamMeta.durationMs !== undefined && (
            <span>⏱ {(streamMeta.durationMs / 1000).toFixed(1)}s</span>
          )}
          {streamMeta.rateLimitStatus && streamMeta.rateLimitStatus !== 'allowed' && (
            <span className="text-yellow-400">⚠ rate limit</span>
          )}
        </div>
      )}
      {hookEvents && hookEvents.length > 0 && (
        <details className="mx-4 mb-2 text-[11px] border border-[var(--color-border)] rounded">
          <summary className="px-3 py-1.5 cursor-pointer text-[var(--color-text)] opacity-60 select-none">
            Hook Events ({hookEvents.length})
          </summary>
          <div className="px-3 py-2 max-h-40 overflow-y-auto space-y-1">
            {hookEvents.map((ev, i) => (
              <div key={i} className="font-mono text-[10px] text-[var(--color-text)] opacity-50">
                [{ev.subtype}] {ev.hook_name ?? ev.hook_event ?? ev.session_id ?? ''}
              </div>
            ))}
          </div>
        </details>
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
