import { useState, useEffect, useCallback } from 'react';
import { Routes, Route, useNavigate, useParams } from 'react-router-dom';
import type { Conversation } from './api/client';
import {
  listConversations,
  createConversation,
  getConversation,
  deleteConversation,
} from './api/client';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './components/ChatView';
import { useAuth } from './hooks/useAuth';
import { AuthGuard } from './components/AuthGuard';
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
  const [sidebarOpen, setSidebarOpen] = useState(false);

  // selectedId を URL から直接導出（setState-in-effect を避けるため）
  const selectedId = urlId ?? null;

  const refreshConversations = useCallback(async () => {
    try {
      const list = await listConversations();
      setConversations(list ?? []);
    } catch (e) {
      console.error('Failed to list conversations:', e);
    }
  }, []);

  // 初回マウント時に会話一覧を取得（インライン async で setState-in-effect を避ける）
  useEffect(() => {
    listConversations()
      .then(list => setConversations(list ?? []))
      .catch(e => console.error('Failed to list conversations:', e));
  }, []);

  // URL直接アクセス時: 会話の存在確認（存在しなければ / へリダイレクト）
  useEffect(() => {
    if (!urlId) return;
    getConversation(urlId).catch(() => {
      navigate('/');
    });
  }, [urlId, navigate]);

  const handleSelectConversation = useCallback((id: string) => {
    navigate(`/conversation/${id}`);
    setSidebarOpen(false);
  }, [navigate]);

  const handleNewConversation = async () => {
    try {
      const conv = await createConversation();
      await refreshConversations();
      handleSelectConversation(conv.id);
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

  // running な会話がある間、3秒ごとに conversations を更新してサイドバーのスピナーを維持する
  const hasRunning = conversations.some(c => c.status === 'running');
  useConversationListPoller({
    hasRunning,
    onPoll: refreshConversations,
  });

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
              conversationId={selectedId}
              onConversationUpdate={() => refreshConversations()}
              onSendStart={() => {
                setConversations(prev =>
                  prev.map(c => c.id === selectedId ? { ...c, status: 'running' } : c)
                );
              }}
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
