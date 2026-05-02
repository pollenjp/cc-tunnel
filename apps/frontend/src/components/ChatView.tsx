import { useState, useEffect, useRef } from 'react';
import type { Message, ToolCallData } from '../api/client';
import type { ToolCall, AssistantBlock } from '../App';
import { getConversation, sendMessage } from '../api/client';
import { MessageBubble, ThinkingAccordion } from './MessageBubble';
import { MessageInput } from './MessageInput';
import { ToolCallCard } from './ToolCallCard';
import { TypingIndicator } from './TypingIndicator';
import { useConversationPoller } from '../hooks/useConversationPoller';
import { useConversationsStore } from '../store/conversations';

interface ChatViewProps {
  conversationId: string | null;
  onHamburger: () => void;
}

type ContentBlockEntry =
  | { type: 'thinking'; content: string }
  | { type: 'text'; content: string }
  | { type: 'tool_use'; tool_use_id: string }

export function ChatView({ conversationId, onHamburger }: ChatViewProps) {
  const markRunning = useConversationsStore(s => s.markRunning);
  const refreshConversations = useConversationsStore(s => s.refresh);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [sending, setSending] = useState(false);
  const [isPolling, setIsPolling] = useState(false);
  const [input, setInput] = useState('');

  const isRunning = sending || isPolling || messages.some(m => m.status === 'streaming');

  // conversationId 変更時: messages クリア → 初期ロード
  useEffect(() => {
    setMessages([]);
    setIsPolling(false);
    setSending(false);
    setInput('');

    if (!conversationId) return;

    getConversation(conversationId).then(detail => {
      const msgs = detail.messages ?? [];
      setMessages(msgs);
      if (detail.status === 'running') setIsPolling(true);
    }).catch(console.error);
  }, [conversationId]);

  // messages 更新時にスクロール
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // ポーリングフック
  useConversationPoller({
    conversationId: isPolling ? conversationId : null,
    isRunning: isPolling,
    onMessages: (msgs) => setMessages(msgs),
    onCompleted: () => {
      setIsPolling(false);
      void refreshConversations();
    },
    intervalMs: 1000,
  });

  const handleSend = async (content: string) => {
    if (!content.trim() || !conversationId || sending) return;
    setSending(true);
    setInput('');
    markRunning(conversationId);

    const userMsg: Message = {
      id: crypto.randomUUID(),
      conversation_id: conversationId,
      role: 'user' as Message['role'],
      message_data: { content: content.trim() },
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, userMsg]);

    try {
      await sendMessage(conversationId, content.trim());
      setIsPolling(true);
    } catch (err) {
      console.error('Failed to send message:', err);
    } finally {
      setSending(false);
    }
  };

  const isEmpty = messages.length === 0;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {isEmpty ? (
        <div className="flex-1 flex items-center justify-center text-[var(--color-text)]">
          <p>メッセージを入力して会話を始めましょう。</p>
        </div>
      ) : (
        <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
          {messages.map((msg) => {
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

            if (isPollingStreamingMsg) {
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
                    const toolCall: ToolCall = tc
                      ? {
                          index: 0,
                          toolUseId: tc.tool_use_id,
                          toolName: tc.tool_name,
                          inputJson: tc.input_json,
                          result: tc.result ?? undefined,
                          isRunning: true,
                        }
                      : {
                          // tool_calls 未保存時のフォールバック
                          index: 0,
                          toolUseId: cb.tool_use_id,
                          toolName: '',
                          inputJson: '',
                          isRunning: true,
                        };
                    return [{ type: 'tool', toolCall }];
                  })
                : [{ type: 'text', content: '' }];
            } else {
              // Loaded from DB (completed or non-polling streaming message)
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

            const lastBlockIdx = blocks.length - 1;
            const isInProgress = isRunning;
            const isEmptyBlocks =
              blocks.length === 1 && blocks[0].type === 'text' && blocks[0].content === '';

            return (
              <div key={msg.id} className="flex flex-col gap-1">
                {!isEmptyBlocks && blocks.map((block, bi) => {
                  if (block.type === 'thinking') {
                    return <ThinkingAccordion key={bi} content={block.content} />;
                  } else if (block.type === 'text') {
                    return (
                      <MessageBubble
                        key={bi}
                        message={msg}
                        textContent={block.content}
                        isStreaming={isPollingStreamingMsg && bi === lastBlockIdx}
                      />
                    );
                  } else {
                    return <ToolCallCard key={bi} toolCall={block.toolCall} />;
                  }
                })}
                {isInProgress && <TypingIndicator />}
              </div>
            );
          })}
          {/* 送信中（アシスタントメッセージがまだない）ときの TypingIndicator */}
          {sending && (
            <div className="flex flex-col gap-1">
              <TypingIndicator />
            </div>
          )}
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
        onChange={setInput}
        onSend={() => handleSend(input)}
        disabled={isRunning}
        onHamburger={onHamburger}
      />
    </div>
  );
}
