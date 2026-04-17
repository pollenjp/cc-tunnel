import { useState, useEffect, useRef, useCallback } from 'react';
import type { Conversation, Message } from './api/client';
import {
  listConversations,
  createConversation,
  getConversation,
  deleteConversation,
  sendMessage,
} from './api/client';

import './App.css';

function App() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

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

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSelectConversation = useCallback(async (id: string) => {
    setSelectedId(id);
    setMessages([]);
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

  const handleSend = async () => {
    if (!input.trim() || !selectedId || sending) return;
    const content = input.trim();
    setInput('');
    setSending(true);

    // ユーザーメッセージを即座に表示
    const userMsg: Message = {
      id: crypto.randomUUID(),
      conversation_id: selectedId,
      role: 'user' as Message['role'],
      content,
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, userMsg]);

    // アシスタントの空メッセージを追加（ストリーミング先）
    const assistantMsg: Message = {
      id: crypto.randomUUID(),
      conversation_id: selectedId,
      role: 'assistant' as Message['role'],
      content: '',
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, assistantMsg]);

    try {
      await sendMessage(selectedId, content, (event) => {
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

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      handleSend();
    }
  };

  const getConversationTitle = (conv: Conversation) =>
    conv.title && conv.title.trim() ? conv.title : '新しい会話';

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="sidebar-header">
          <h1 className="app-title">cc-tunnel</h1>
          <button className="btn btn-primary new-conv-btn" onClick={handleNewConversation}>
            + 新しい会話
          </button>
        </div>
        <ul className="conversation-list">
          {conversations.map(conv => (
            <li
              key={conv.id}
              className={`conversation-item ${conv.id === selectedId ? 'active' : ''}`}
              onClick={() => handleSelectConversation(conv.id)}
            >
              <span className="conversation-title">{getConversationTitle(conv)}</span>
              <button
                className="btn btn-danger btn-sm delete-btn"
                onClick={(e) => handleDeleteConversation(conv.id, e)}
              >
                x
              </button>
            </li>
          ))}
        </ul>
      </aside>

      <main className="main">
        {selectedId ? (
          <div className="chat-container">
            <div className="messages">
              {messages.map(msg => (
                <div
                  key={msg.id}
                  className={`message message-${msg.role}`}
                >
                  <div className="message-role">{msg.role === 'user' ? 'You' : 'Assistant'}</div>
                  <div className="message-content">{msg.content}</div>
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>

            <div className="input-bar">
              <textarea
                className="input-field"
                value={input}
                onChange={e => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                rows={3}
                placeholder="メッセージを入力... (Ctrl+Enter で送信)"
                disabled={sending}
              />
              <button
                className="btn btn-primary send-btn"
                onClick={handleSend}
                disabled={sending || !input.trim()}
              >
                {sending ? '送信中...' : '送信'}
              </button>
            </div>
          </div>
        ) : (
          <div className="empty-state">
            <p>左のサイドバーから会話を選択するか、「+ 新しい会話」を押して開始してください。</p>
          </div>
        )}
      </main>
    </div>
  );
}

export default App;
