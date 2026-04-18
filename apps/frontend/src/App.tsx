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
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([]);
  const [streamingThinkings, setStreamingThinkings] = useState<string[]>([]);
  const streamContentRef = useRef('');
  const streamThinkingRef = useRef<string[]>([]);
  const rafIdRef = useRef<number>(0);
  const streamMetaRef = useRef<StreamMeta>({});
  const hookEventsRef = useRef<SSEHookEvent[]>([]);
  const toolCallsRef = useRef<ToolCall[]>([]);

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
      const content = streamContentRef.current;
      setStreamingThinkings([...streamThinkingRef.current.filter(s => s !== '')]);
      setMessages(prev => {
        const copy = [...prev];
        const last = copy[copy.length - 1];
        if (last?.role === 'assistant') {
          copy[copy.length - 1] = {
            ...last,
            content: content || last.content,
          };
        }
        return copy;
      });
    });
  }

  const handleSelectConversation = useCallback(async (id: string) => {
    setSelectedId(id);
    setMessages([]);
    streamContentRef.current = '';
    streamThinkingRef.current = [];
    streamMetaRef.current = {};
    hookEventsRef.current = [];
    setStreamMeta(null);
    setHookEvents([]);
    setToolCalls([]);
    toolCallsRef.current = [];
    setStreamingThinkings([]);
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
    streamContentRef.current = '';
    streamThinkingRef.current = [];
    streamMetaRef.current = {};
    hookEventsRef.current = [];
    setStreamMeta(null);
    setHookEvents([]);
    setToolCalls([]);
    toolCallsRef.current = [];
    setStreamingThinkings([]);
    if (rafIdRef.current) {
      cancelAnimationFrame(rafIdRef.current);
      rafIdRef.current = 0;
    }

    const userMsg: Message = {
      id: crypto.randomUUID(),
      conversation_id: selectedId,
      role: 'user' as Message['role'],
      content: content.trim(),
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, userMsg]);

    const assistantMsg: Message = {
      id: crypto.randomUUID(),
      conversation_id: selectedId,
      role: 'assistant' as Message['role'],
      content: '',
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, assistantMsg]);

    try {
      await sendMessage(selectedId, content.trim(), (event) => {
        if (event.type === 'text') {
          setMessages(prev => {
            const copy = [...prev];
            const last = copy[copy.length - 1];
            if (last?.role === 'assistant') {
              copy[copy.length - 1] = { ...last, content: last.content + event.content };
            }
            return copy;
          });
        } else if (event.type === 'thinking') {
          const arr = streamThinkingRef.current;
          if (arr.length === 0) arr.push('');
          arr[arr.length - 1] += event.content;
          setStreamingThinkings([...arr.filter(s => s !== '')]);
        } else if (event.type === 'text_delta') {
          const arr = streamThinkingRef.current;
          if (arr.length > 0 && arr[arr.length - 1] !== '') {
            arr.push('');
          }
          streamContentRef.current += event.content;
          scheduleRafUpdate();
        } else if (event.type === 'thinking_delta') {
          const arr = streamThinkingRef.current;
          if (arr.length === 0) arr.push('');
          arr[arr.length - 1] += event.content;
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
          toolCallsRef.current = [...toolCallsRef.current, newTc];
          setToolCalls(prev => [...prev, newTc]);
        } else if (event.type === 'tool_input_delta') {
          toolCallsRef.current = toolCallsRef.current.map(tc =>
            tc.index === event.index
              ? { ...tc, inputJson: tc.inputJson + event.partial_json }
              : tc
          );
          setToolCalls(prev => prev.map(tc =>
            tc.index === event.index
              ? { ...tc, inputJson: tc.inputJson + event.partial_json }
              : tc
          ));
        } else if (event.type === 'tool_result') {
          toolCallsRef.current = toolCallsRef.current.map(tc =>
            tc.toolUseId === event.tool_use_id
              ? { ...tc, result: event.content, isRunning: false }
              : tc
          );
          setToolCalls(prev => prev.map(tc =>
            tc.toolUseId === event.tool_use_id
              ? { ...tc, result: event.content, isRunning: false }
              : tc
          ));
        }
      });
    } catch (err) {
      console.error('Failed to send message:', err);
    } finally {
      const completedThinkings = streamThinkingRef.current.filter(s => s !== '');
      setMessages(prev => {
        const copy = [...prev];
        const last = copy[copy.length - 1];
        if (last?.role === 'assistant') {
          copy[copy.length - 1] = {
            ...last,
            metadata: {
              ...(last.metadata as Record<string, unknown> ?? {}),
              ...(completedThinkings.length > 0 ? { thinking: completedThinkings.join('\n') } : {}),
              ...(streamMetaRef.current.model ? { model: streamMetaRef.current.model } : {}),
              ...(streamMetaRef.current.totalCostUSD ? { cost_usd: streamMetaRef.current.totalCostUSD } : {}),
              ...(streamMetaRef.current.durationMs ? { duration_ms: streamMetaRef.current.durationMs } : {}),
              ...(hookEventsRef.current?.length > 0 ? { hook_events: hookEventsRef.current } : {}),
              ...(toolCallsRef.current.length > 0 ? {
                tool_calls: toolCallsRef.current.map(tc => ({
                  tool_use_id: tc.toolUseId,
                  tool_name: tc.toolName,
                  input_json: tc.inputJson,
                  result: tc.result ?? null,
                })),
              } : {}),
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
              toolCalls={toolCalls}
              streamingThinkings={streamingThinkings}
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
