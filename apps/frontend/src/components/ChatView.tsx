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

export function ChatView({ messages, onSend, isStreaming, streamMeta, hookEvents, streamBlocks, input, onInputChange, onHamburger }: ChatViewProps) {
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

            if (msg.role !== 'assistant') {
              return (
                <div key={msg.id}>
                  <MessageBubble message={msg} />
                </div>
              );
            }

            // Build ordered blocks for this assistant message
            let blocks: AssistantBlock[];

            if (isStreamingMsg) {
              // Live streaming: use streamBlocks, fall back to empty text block
              blocks = (streamBlocks && streamBlocks.length > 0)
                ? streamBlocks
                : [{ type: 'text', content: '' }];
            } else {
              // Loaded from DB
              const meta = msg.metadata as Record<string, unknown> | undefined;
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
                  { type: 'text', content: msg.content },
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
                        isStreaming={isStreamingMsg && bi === lastBlockIdx}
                        streamMeta={isStreamingMsg && bi === lastTextIdx ? streamMeta : undefined}
                        hookEvents={isLast && bi === lastTextIdx ? hookEvents : undefined}
                      />
                    );
                  } else {
                    return <ToolCallCard key={bi} toolCall={block.toolCall} />;
                  }
                })}
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
