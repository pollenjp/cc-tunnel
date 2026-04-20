import { useState, useEffect, useCallback, useRef } from 'react';
import { Routes, Route, useNavigate, useParams } from 'react-router-dom';
import type { Conversation, Message } from './api/client';
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
import { useConversationPoller } from './hooks/useConversationPoller';
import { useConversationListPoller } from './hooks/useConversationListPoller';

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

function AppContent() {
  const { id: urlId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const auth = useAuth();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [isPolling, setIsPolling] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);

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

  const handleSelectConversation = useCallback(async (id: string) => {
    setSelectedId(id);
    navigate(`/conversation/${id}`);
    setMessages([]);
    setIsPolling(false);
    setSending(false);
    setSidebarOpen(false);
    try {
      const detail = await getConversation(id);
      const msgs = detail.messages ?? [];
      setMessages(msgs);
      // status === 'running' の場合はポーリング開始（CLI実行継続中）
      if (detail.status === 'running') {
        setIsPolling(true);
      }
    } catch (e) {
      console.error('Failed to load conversation:', e);
    }
  }, [navigate]);

  // URL直接アクセス時: URLパラメータから会話を初期化
  const urlInitHandled = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!urlId) {
      // ルート (/) へのナビゲーション時: 選択状態をクリア
      if (selectedId !== null) {
        setSelectedId(null);
        setMessages([]);
        setIsPolling(false);
      }
      return;
    }
    // handleSelectConversation 経由で既に処理済みの場合はスキップ
    if (selectedId === urlId) {
      urlInitHandled.current = urlId;
      return;
    }
    if (urlInitHandled.current === urlId) return;
    urlInitHandled.current = urlId;

    // 直接URLアクセス: 会話をロード
    setSelectedId(urlId);
    setMessages([]);
    setIsPolling(false);
    setSending(false);

    getConversation(urlId).then(detail => {
      const msgs = detail.messages ?? [];
      setMessages(msgs);
      if (detail.status === 'running') setIsPolling(true);
    }).catch(() => {
      // 存在しない会話ID → / へリダイレクト
      setSelectedId(null);
      navigate('/');
    });
  }, [urlId, selectedId, navigate]);

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
        navigate('/');
      }
      await refreshConversations();
    } catch (err) {
      console.error('Failed to delete conversation:', err);
    }
  };

  // ポーリングフック: 実行継続中なら全メッセージ上書き取得
  useConversationPoller({
    conversationId: isPolling ? selectedId : null,
    isRunning: isPolling,
    onMessages: (msgs) => {
      setMessages(msgs);  // 全置換（差分でなく全上書き）
    },
    onCompleted: () => {
      setIsPolling(false);
      refreshConversations();
    },
    intervalMs: 1000,
  });

  // running な会話がある間、3秒ごとに conversations を更新してサイドバーのスピナーを維持する
  const hasRunning = conversations.some(c => c.status === 'running');
  useConversationListPoller({
    hasRunning,
    onPoll: refreshConversations,
  });

  const handleSend = async (content: string) => {
    if (!content.trim() || !selectedId || sending) return;
    setInput('');
    setSending(true);

    // 楽観的更新: 会話をrunning状態に
    setConversations(prev =>
      prev.map(c => c.id === selectedId ? { ...c, status: 'running' as const } : c)
    );

    // 楽観的ユーザーメッセージ追加
    const userMsg: Message = {
      id: crypto.randomUUID(),
      conversation_id: selectedId,
      role: 'user' as Message['role'],
      message_data: { content: content.trim() },
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, userMsg]);

    try {
      await sendMessage(selectedId, content.trim());
      setIsPolling(true); // 202返却後、pollingでassistant応答を追跡
    } catch (err) {
      console.error('Failed to send message:', err);
    } finally {
      setSending(false);
      await refreshConversations();
    }
  };

  const hasStreamingMessage = messages.some(m => m.status === 'streaming');
  const isRunning = sending || isPolling || hasStreamingMessage;

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
              isPolling={isPolling}
              isRunning={isRunning}
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

function App() {
  return (
    <Routes>
      <Route path="/" element={<AppContent />} />
      <Route path="/conversation/:id" element={<AppContent />} />
    </Routes>
  );
}

export default App;
