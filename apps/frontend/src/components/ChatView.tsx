import { useEffect, useRef } from 'react';
import type { Message, SSEHookEvent, ToolCallData } from '../api/client';
import type { StreamMeta, ToolCall, AssistantBlock } from '../App';
import { MessageBubble, ThinkingAccordion } from './MessageBubble';
import { MessageInput } from './MessageInput';
import { ToolCallCard } from './ToolCallCard';

interface ChatViewProps {
  messages: Message[];
  onSend: (content: string) => void;
  isStreaming: boolean;
  isPolling?: boolean;
  streamMeta?: StreamMeta | null;
  hookEvents?: SSEHookEvent[];
  streamBlocks?: AssistantBlock[];
  input: string;
  onInputChange: (value: string) => void;
  onHamburger: () => void;
}

type ContentBlockEntry =
  | { type: 'thinking'; content: string }
  | { type: 'text'; content: string }
  | { type: 'tool_use'; tool_use_id: string }

export function ChatView({ messages, onSend, isStreaming, isPolling, streamMeta, hookEvents, streamBlocks, input, onInputChange, onHamburger }: ChatViewProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamBlocks]);

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
            const isPollingStreamingMsg = isPolling === true && msg.status === 'streaming';

            if (msg.role !== 'assistant') {
              return (
                <div key={msg.id}>
                  <MessageBubble message={msg} />
                </div>
              );
            }

            // Error display
            if (msg.status === 'error') {
              return (
                <div key={msg.id} className="flex flex-col gap-1">
                  <div className="px-4 py-2 text-sm text-red-600 bg-red-50 rounded-lg border border-red-300">
                    エラーが発生しました
                  </div>
                </div>
              );
            }

            // Build ordered blocks for this assistant message
            let blocks: AssistantBlock[];

            if (isStreamingMsg) {
              // Live SSE streaming: use streamBlocks, fall back to empty text block
              blocks = (streamBlocks && streamBlocks.length > 0)
                ? streamBlocks
                : [{ type: 'text', content: '' }];
            } else if (isPollingStreamingMsg) {
              // DB polling streaming: use content_blocks from message_data
              const meta = msg.message_data as Record<string, unknown> | undefined;
              const contentBlocks = meta?.content_blocks as ContentBlockEntry[] | undefined;
              const toolCallsData = (meta?.tool_calls as ToolCallData[] | undefined) ?? [];
              const toolCallMap = new Map(toolCallsData.map(tc => [tc.tool_use_id, tc]));
              blocks = contentBlocks && contentBlocks.length > 0
                ? contentBlocks.flatMap((cb): AssistantBlock[] => {
                    if (cb.type === 'thinking') return [{ type: 'thinking', content: cb.content }];
                    if (cb.type === 'text') return [{ type: 'text', content: cb.content }];
                    const tc = toolCallMap.get(cb.tool_use_id);
                    if (!tc) return [];
                    const toolCall: ToolCall = {
                      index: 0,
                      toolUseId: tc.tool_use_id,
                      toolName: tc.tool_name,
                      inputJson: tc.input_json,
                      result: tc.result ?? undefined,
                      isRunning: true,
                    };
                    return [{ type: 'tool', toolCall }];
                  })
                : [{ type: 'text', content: '' }];
            } else {
              // Loaded from DB (completed message)
              const meta = msg.message_data as Record<string, unknown> | undefined;
              const contentBlocks = meta?.content_blocks as ContentBlockEntry[] | undefined;
              const toolCallsData = (meta?.tool_calls as ToolCallData[] | undefined) ?? [];
              const toolCallMap = new Map(toolCallsData.map(tc => [tc.tool_use_id, tc]));

              if (contentBlocks && contentBlocks.length > 0) {
                // New format: reconstruct interleaved blocks
                blocks = contentBlocks.flatMap((cb): AssistantBlock[] => {
                  if (cb.type === 'thinking') {
                    return [{ type: 'thinking', content: cb.content }];
                  }
                  if (cb.type === 'text') {
                    return [{ type: 'text', content: cb.content }];
                  }
                  const tc = toolCallMap.get(cb.tool_use_id);
                  if (!tc) return [];
                  const toolCall: ToolCall = {
                    index: 0,
                    toolUseId: tc.tool_use_id,
                    toolName: tc.tool_name,
                    inputJson: tc.input_json,
                    result: tc.result ?? undefined,
                    isRunning: false,
                  };
                  return [{ type: 'tool', toolCall }];
                });
              } else {
                // Old format: single text block + all tool calls below
                blocks = [
                  { type: 'text', content: (meta?.content as string | undefined) ?? '' },
                  ...toolCallsData.map((tc): AssistantBlock => ({
                    type: 'tool',
                    toolCall: {
                      index: 0,
                      toolUseId: tc.tool_use_id,
                      toolName: tc.tool_name,
                      inputJson: tc.input_json,
                      result: tc.result ?? undefined,
                      isRunning: false,
                    },
                  })),
                ];
              }
            }

            // Indices for metadata placement
            const lastTextIdx = blocks.map((b, i) => b.type === 'text' ? i : -1).filter(i => i >= 0).pop() ?? -1;
            const lastBlockIdx = blocks.length - 1;
            const isShowingStreaming = isStreamingMsg || isPollingStreamingMsg;

            return (
              <div key={msg.id} className="flex flex-col gap-1">
                {blocks.map((block, bi) => {
                  if (block.type === 'thinking') {
                    return <ThinkingAccordion key={bi} content={block.content} />;
                  } else if (block.type === 'text') {
                    return (
                      <MessageBubble
                        key={bi}
                        message={msg}
                        textContent={block.content}
                        isStreaming={isShowingStreaming && bi === lastBlockIdx}
                        streamMeta={isStreamingMsg && bi === lastTextIdx ? streamMeta : undefined}
                        hookEvents={isLast && bi === lastTextIdx ? hookEvents : undefined}
                      />
                    );
                  } else {
                    return <ToolCallCard key={bi} toolCall={block.toolCall} />;
                  }
                })}
                {isPollingStreamingMsg && (
                  <div className="flex items-center gap-1 px-4 py-1 text-xs text-[var(--color-text)]">
                    <span className="animate-pulse">●</span>
                    <span className="animate-pulse" style={{ animationDelay: '0.2s' }}>●</span>
                    <span className="animate-pulse" style={{ animationDelay: '0.4s' }}>●</span>
                    <span className="ml-1 text-[var(--color-text-muted)]">生成中...</span>
                  </div>
                )}
              </div>
            );
          })}
          <div ref={messagesEndRef} />
        </div>
      )}
      {isPolling && (
        <div className="px-4 py-2 text-sm text-[var(--color-text)] bg-[var(--color-bg-secondary)] border-t border-[var(--color-border)] flex items-center gap-2">
          <span className="animate-spin">⏳</span>
          <span>処理中...</span>
        </div>
      )}
      <MessageInput
        value={input}
        onChange={onInputChange}
        onSend={() => onSend(input)}
        disabled={isStreaming || isPolling}
        onHamburger={onHamburger}
      />
    </div>
  );
}
