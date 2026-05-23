import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getConversation } from '../api/client';
import { Sidebar } from '../components/Sidebar';
import { ChatView } from '../components/ChatView';
import { AgentSelector } from '../components/AgentSelector';
import { useConversationListPoller } from '../hooks/useConversationListPoller';
import { useConversationsStore } from '../store/conversations';

export function ChatPage() {
  const { id: urlId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const conversations = useConversationsStore(s => s.conversations);
  const refreshConversations = useConversationsStore(s => s.refresh);
  const createConversationStore = useConversationsStore(s => s.create);
  const removeConversation = useConversationsStore(s => s.remove);
  const hasRunning = useConversationsStore(s => s.hasRunning());
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);

  const selectedId = urlId ?? null;

  useEffect(() => {
    void refreshConversations();
  }, [refreshConversations]);

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
    if (isCreating) return;
    setIsCreating(true);
    try {
      const conv = await createConversationStore();
      handleSelectConversation(conv.id);
    } catch (e) {
      console.error('Failed to create conversation:', e);
    } finally {
      setIsCreating(false);
    }
  };

  const handleDeleteConversation = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await removeConversation(id);
      if (selectedId === id) {
        navigate('/');
      }
    } catch (err) {
      console.error('Failed to delete conversation:', err);
    }
  };

  useConversationListPoller({
    hasRunning,
    onPoll: refreshConversations,
  });

  return (
    <div className="flex flex-row h-screen overflow-hidden bg-[var(--color-bg)]">
        <Sidebar
          conversations={conversations}
          selectedId={selectedId}
          onSelect={handleSelectConversation}
          onNew={handleNewConversation}
          onDelete={handleDeleteConversation}
          sidebarOpen={sidebarOpen}
          onClose={() => setSidebarOpen(false)}
        />
        <main className="flex-1 flex flex-col overflow-hidden min-w-0">
          {selectedId ? (
            <ChatView
              conversationId={selectedId}
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
              <AgentSelector onSelect={handleAgentSelect} isLoading={isCreating} />
            </div>
          )}
        </main>
      </div>
  );
}
