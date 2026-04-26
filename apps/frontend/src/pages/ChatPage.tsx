import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import type { Conversation } from '../api/client';
import {
  listConversations,
  createConversation,
  getConversation,
  deleteConversation,
} from '../api/client';
import { Sidebar } from '../components/Sidebar';
import { ChatView } from '../components/ChatView';
import { AgentSelector } from '../components/AgentSelector';
import { useAuth } from '../hooks/useAuth';
import { AuthGuard } from '../components/AuthGuard';
import { useConversationListPoller } from '../hooks/useConversationListPoller';

export function ChatPage() {
  const { id: urlId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const auth = useAuth();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [sidebarOpen, setSidebarOpen] = useState(false);

  const selectedId = urlId ?? null;

  const refreshConversations = useCallback(async () => {
    try {
      const list = await listConversations();
      setConversations(list ?? []);
    } catch (e) {
      console.error('Failed to list conversations:', e);
    }
  }, []);

  useEffect(() => {
    listConversations()
      .then(list => setConversations(list ?? []))
      .catch(e => console.error('Failed to list conversations:', e));
  }, []);

  useEffect(() => {
    if (!urlId) return;
    getConversation(urlId).catch(() => {
      navigate('/');
    });
  }, [urlId, navigate]);

  const handleSelectConversation = useCallback((id: string) => {
    navigate(`/chat/${id}`);
    setSidebarOpen(false);
  }, [navigate]);

  const handleNewConversation = () => {
    navigate('/chat');
  };

  const handleAgentSelect = async () => {
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
            <div className="flex-1 flex flex-col overflow-hidden">
              <button
                className="md:hidden self-start m-2 px-3.5 py-2.5 text-xl rounded-md bg-[var(--color-bg-tertiary)] text-[var(--color-text-bright)] cursor-pointer hover:bg-[var(--color-border)] transition-colors"
                onClick={() => setSidebarOpen(true)}
              >
                ☰
              </button>
              <AgentSelector onSelect={handleAgentSelect} />
            </div>
          )}
        </main>
      </div>
    </AuthGuard>
  );
}
