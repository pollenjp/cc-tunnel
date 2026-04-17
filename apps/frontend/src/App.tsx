import { useState, useEffect, useCallback } from 'react';
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

import './App.css';

function App() {
  const auth = useAuth();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
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
    setMessages([]);
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
        }
      });
    } catch (err) {
      console.error('Failed to send message:', err);
    } finally {
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
        />
        <main className="flex-1 flex flex-col overflow-hidden min-w-0">
          {selectedId ? (
            <ChatView
              messages={messages}
              onSend={handleSend}
              sending={sending}
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
