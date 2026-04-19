import { useState, useEffect, useCallback, useRef } from 'react';
import type { Conversation, Message, SSEHookEvent } from './api/client';
import {
  listConversations,
  createConversation,
  getConversation,
  deleteConversation,
  sendMessage,
} from './api/client';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './components/ChatView';
import { useAuth } from './hooks/useAuth';
import { AuthGuard } from './components/AuthGuard';

import './App.css';

export interface ToolCall {
  index: number;
  toolUseId: string;
  toolName: string;
  inputJson: string;
  result?: string;
  isRunning: boolean;
}

export type AssistantBlock =
  | { type: 'thinking'; content: string }
  | { type: 'text'; content: string }
  | { type: 'tool'; toolCall: ToolCall }

export interface StreamMeta {
  model?: string;
  sessionId?: string;
  inputTokens?: number;
  outputTokens?: number;
  cacheCreationTokens?: number;
  cacheReadTokens?: number;
  totalCostUSD?: number;
  durationMs?: number;
  rateLimitStatus?: string;
  stopReason?: string;
}

function App() {
  const auth = useAuth();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [streamMeta, setStreamMeta] = useState<StreamMeta | null>(null);
  const [hookEvents, setHookEvents] = useState<SSEHookEvent[]>([]);
  const [streamBlocks, setStreamBlocks] = useState<AssistantBlock[]>([]);
  const rafIdRef = useRef<number>(0);
  const streamMetaRef = useRef<StreamMeta>({});
  const hookEventsRef = useRef<SSEHookEvent[]>([]);
  const streamBlocksRef = useRef<AssistantBlock[]>([]);

  const refreshConversations = useCallback(async () => {
    try {
      const list = await listConversations();
      setConversations(list ?? []);
    } catch (e) {
      console.error('Failed to list conversations:', e);
    }
  }, []);

  useEffect(() => {
    refreshConversations();
  }, [refreshConversations]);

  function scheduleRafUpdate() {
    if (rafIdRef.current) return;
    rafIdRef.current = requestAnimationFrame(() => {
      rafIdRef.current = 0;
      setStreamBlocks([...streamBlocksRef.current]);
    });
  }

  const handleSelectConversation = useCallback(async (id: string) => {
    setSelectedId(id);
    setMessages([]);
    streamBlocksRef.current = [];
    streamMetaRef.current = {};
    hookEventsRef.current = [];
    setStreamMeta(null);
    setHookEvents([]);
    setStreamBlocks([]);
    if (rafIdRef.current) {
      cancelAnimationFrame(rafIdRef.current);
      rafIdRef.current = 0;
    }
    setSidebarOpen(false);
    try {
      const detail = await getConversation(id);
      setMessages(detail.messages ?? []);
    } catch (e) {
      console.error('Failed to load conversation:', e);
    }
  }, []);

  const handleNewConversation = async () => {
    try {
      const conv = await createConversation();
      await refreshConversations();
      await handleSelectConversation(conv.id);
    } catch (e) {
      console.error('Failed to create conversation:', e);
    }
  };

  const handleDeleteConversation = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await deleteConversation(id);
      if (selectedId === id) {
        setSelectedId(null);
        setMessages([]);
      }
      await refreshConversations();
    } catch (err) {
      console.error('Failed to delete conversation:', err);
    }
  };

  const handleSend = async (content: string) => {
    if (!content.trim() || !selectedId || sending) return;
    setInput('');
    setSending(true);
    streamBlocksRef.current = [];
    streamMetaRef.current = {};
    hookEventsRef.current = [];
    setStreamMeta(null);
    setHookEvents([]);
    setStreamBlocks([]);
    if (rafIdRef.current) {
      cancelAnimationFrame(rafIdRef.current);
      rafIdRef.current = 0;
    }

    const userMsg: Message = {
      id: crypto.randomUUID(),
      conversation_id: selectedId,
      role: 'user' as Message['role'],
      message_data: { content: content.trim() },
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, userMsg]);

    const assistantMsg: Message = {
      id: crypto.randomUUID(),
      conversation_id: selectedId,
      role: 'assistant' as Message['role'],
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, assistantMsg]);

    try {
      await sendMessage(selectedId, content.trim(), (event) => {
        if (event.type === 'text') {
          const blocks = streamBlocksRef.current;
          const last = blocks[blocks.length - 1];
          if (last?.type === 'text') {
            streamBlocksRef.current = [...blocks.slice(0, -1), { type: 'text', content: last.content + event.content }];
          } else {
            streamBlocksRef.current = [...blocks, { type: 'text', content: event.content }];
          }
          scheduleRafUpdate();
        } else if (event.type === 'thinking') {
          const blocks = streamBlocksRef.current;
          const last = blocks[blocks.length - 1];
          if (last?.type === 'thinking') {
            streamBlocksRef.current = [...blocks.slice(0, -1), { type: 'thinking', content: last.content + event.content }];
          } else {
            streamBlocksRef.current = [...blocks, { type: 'thinking', content: event.content }];
          }
          scheduleRafUpdate();
        } else if (event.type === 'text_delta') {
          const blocks = streamBlocksRef.current;
          const last = blocks[blocks.length - 1];
          if (last?.type === 'text') {
            streamBlocksRef.current = [...blocks.slice(0, -1), { type: 'text', content: last.content + event.content }];
          } else {
            streamBlocksRef.current = [...blocks, { type: 'text', content: event.content }];
          }
          scheduleRafUpdate();
        } else if (event.type === 'thinking_delta') {
          const blocks = streamBlocksRef.current;
          const last = blocks[blocks.length - 1];
          if (last?.type === 'thinking') {
            streamBlocksRef.current = [...blocks.slice(0, -1), { type: 'thinking', content: last.content + event.content }];
          } else {
            streamBlocksRef.current = [...blocks, { type: 'thinking', content: event.content }];
          }
          scheduleRafUpdate();
        } else if (event.type === 'init') {
          streamMetaRef.current = {
            ...streamMetaRef.current,
            model: event.model,
            sessionId: event.session_id,
          };
          setStreamMeta({ ...streamMetaRef.current });
        } else if (event.type === 'rate_limit') {
          streamMetaRef.current = {
            ...streamMetaRef.current,
            rateLimitStatus: event.status,
          };
          setStreamMeta({ ...streamMetaRef.current });
        } else if (event.type === 'cost') {
          streamMetaRef.current = {
            ...streamMetaRef.current,
            totalCostUSD: event.total_cost_usd,
            durationMs: event.duration_ms,
          };
          setStreamMeta({ ...streamMetaRef.current });
        } else if (event.type === 'hook_event') {
          hookEventsRef.current = [...hookEventsRef.current, event];
          setHookEvents(prev => [...prev, event]);
        } else if (event.type === 'tool_use_start') {
          const newTc: ToolCall = {
            index: event.index,
            toolUseId: event.tool_use_id,
            toolName: event.tool_name,
            inputJson: '',
            isRunning: true,
          };
          streamBlocksRef.current = [...streamBlocksRef.current, { type: 'tool', toolCall: newTc }];
          scheduleRafUpdate();
        } else if (event.type === 'tool_input_delta') {
          streamBlocksRef.current = streamBlocksRef.current.map(block =>
            block.type === 'tool' && block.toolCall.index === event.index
              ? { ...block, toolCall: { ...block.toolCall, inputJson: block.toolCall.inputJson + event.partial_json } }
              : block
          );
          scheduleRafUpdate();
        } else if (event.type === 'tool_result') {
          streamBlocksRef.current = streamBlocksRef.current.map(block =>
            block.type === 'tool' && block.toolCall.toolUseId === event.tool_use_id
              ? { ...block, toolCall: { ...block.toolCall, result: event.content, isRunning: false } }
              : block
          );
          scheduleRafUpdate();
        }
      });
    } catch (err) {
      console.error('Failed to send message:', err);
    } finally {
      const finalBlocks = streamBlocksRef.current;
      const finalText = finalBlocks
        .filter((b): b is { type: 'text'; content: string } => b.type === 'text')
        .map(b => b.content)
        .join('');
      const toolCallsList = finalBlocks
        .filter((b): b is { type: 'tool'; toolCall: ToolCall } => b.type === 'tool')
        .map(b => b.toolCall);
      const hasThinkingOrTool = finalBlocks.some(b => b.type === 'thinking' || b.type === 'tool');
      const contentBlocks = finalBlocks.map(b => {
        if (b.type === 'text') return { type: 'text' as const, content: b.content };
        if (b.type === 'thinking') return { type: 'thinking' as const, content: b.content };
        return { type: 'tool_use' as const, tool_use_id: b.toolCall.toolUseId };
      });

      setMessages(prev => {
        const copy = [...prev];
        const last = copy[copy.length - 1];
        if (last?.role === 'assistant') {
          copy[copy.length - 1] = {
            ...last,
            message_data: {
              ...((last.message_data as Record<string, unknown>) ?? {}),
              content: finalText,
              ...(streamMetaRef.current.model ? { model: streamMetaRef.current.model } : {}),
              ...(streamMetaRef.current.totalCostUSD ? { cost_usd: streamMetaRef.current.totalCostUSD } : {}),
              ...(streamMetaRef.current.durationMs ? { duration_ms: streamMetaRef.current.durationMs } : {}),
              ...(hookEventsRef.current?.length > 0 ? { hook_events: hookEventsRef.current } : {}),
              ...(toolCallsList.length > 0 ? {
                tool_calls: toolCallsList.map(tc => ({
                  tool_use_id: tc.toolUseId,
                  tool_name: tc.toolName,
                  input_json: tc.inputJson,
                  result: tc.result ?? null,
                })),
              } : {}),
              ...(hasThinkingOrTool ? { content_blocks: contentBlocks } : {}),
            },
          };
        }
        return copy;
      });
      setSending(false);
      await refreshConversations();
    }
  };

  return (
    <AuthGuard auth={auth}>
      <div className="flex flex-row h-screen overflow-hidden bg-[var(--color-bg)]">
        <Sidebar
          conversations={conversations}
          selectedId={selectedId}
          onSelect={handleSelectConversation}
          onNew={handleNewConversation}
          onDelete={handleDeleteConversation}
          sidebarOpen={sidebarOpen}
          onClose={() => setSidebarOpen(false)}
          authMethod={auth.status?.authMethod}
          authEmail={auth.status?.email ?? undefined}
          onLogout={auth.logout}
        />
        <main className="flex-1 flex flex-col overflow-hidden min-w-0">
          {selectedId ? (
            <ChatView
              messages={messages}
              onSend={handleSend}
              isStreaming={sending}
              streamMeta={streamMeta}
              hookEvents={hookEvents}
              streamBlocks={streamBlocks}
              input={input}
              onInputChange={setInput}
              onHamburger={() => setSidebarOpen(true)}
            />
          ) : (
            <div className="flex-1 flex items-center justify-center text-[var(--color-text)]">
              <div className="flex flex-col items-center gap-4 text-center">
                <button
                  className="md:hidden px-3.5 py-2.5 text-xl rounded-md bg-[var(--color-bg-tertiary)] text-[var(--color-text-bright)] cursor-pointer hover:bg-[var(--color-border)] transition-colors"
                  onClick={() => setSidebarOpen(true)}
                >
                  ☰
                </button>
                <p>左のサイドバーから会話を選択するか、「+ 新しい会話」を押して開始してください。</p>
              </div>
            </div>
          )}
        </main>
      </div>
    </AuthGuard>
  );
}

export default App;
