import { useState, useEffect, useRef, useCallback } from 'react';
import type { Session } from './api';
import { createSession, listSessions, sendInput, getOutput, deleteSession } from './api';
import './App.css';

function App() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [output, setOutput] = useState('');
  const [input, setInput] = useState('');
  const [polling, setPolling] = useState(false);
  const outputRef = useRef<HTMLPreElement>(null);
  const intervalRef = useRef<number | null>(null);

  const refreshSessions = useCallback(async () => {
    try {
      const list = await listSessions();
      setSessions(list ?? []);
    } catch (e) {
      console.error('Failed to list sessions:', e);
    }
  }, []);

  useEffect(() => {
    refreshSessions();
  }, [refreshSessions]);

  const pollOutput = useCallback(async (id: string) => {
    try {
      const text = await getOutput(id);
      setOutput(text);
      if (outputRef.current) {
        outputRef.current.scrollTop = outputRef.current.scrollHeight;
      }
    } catch (e) {
      console.error('Failed to get output:', e);
    }
  }, []);

  useEffect(() => {
    if (activeId && polling) {
      pollOutput(activeId);
      intervalRef.current = window.setInterval(() => pollOutput(activeId), 2000);
    }
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [activeId, polling, pollOutput]);

  const handleCreate = async () => {
    try {
      const session = await createSession();
      await refreshSessions();
      setActiveId(session.id);
      setOutput('');
      setPolling(true);
    } catch (e) {
      alert(`Failed to create session: ${e}`);
    }
  };

  const handleSelect = (id: string) => {
    setActiveId(id);
    setOutput('');
    setPolling(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteSession(id);
      if (activeId === id) {
        setActiveId(null);
        setOutput('');
        setPolling(false);
      }
      await refreshSessions();
    } catch (e) {
      alert(`Failed to delete session: ${e}`);
    }
  };

  const handleSend = async () => {
    if (!activeId || !input.trim()) return;
    try {
      await sendInput(activeId, input);
      setInput('');
    } catch (e) {
      alert(`Failed to send input: ${e}`);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="app">
      <header className="header">
        <h1>cc-tunnel</h1>
      </header>

      <div className="layout">
        <aside className="sidebar">
          <button className="btn btn-primary" onClick={handleCreate}>
            + New Session
          </button>
          <ul className="session-list">
            {sessions.map((s) => (
              <li
                key={s.id}
                className={`session-item ${s.id === activeId ? 'active' : ''}`}
              >
                <span className="session-name" onClick={() => handleSelect(s.id)}>
                  {s.tmux_name}
                </span>
                <button
                  className="btn btn-danger btn-sm"
                  onClick={() => handleDelete(s.id)}
                >
                  x
                </button>
              </li>
            ))}
          </ul>
        </aside>

        <main className="main">
          {activeId ? (
            <>
              <div className="toolbar">
                <span className="session-label">Session: {activeId}</span>
                <label className="polling-toggle">
                  <input
                    type="checkbox"
                    checked={polling}
                    onChange={(e) => setPolling(e.target.checked)}
                  />
                  Auto-refresh
                </label>
                <button className="btn btn-sm" onClick={() => pollOutput(activeId)}>
                  Refresh
                </button>
              </div>
              <pre className="output" ref={outputRef}>
                {output || 'Waiting for output...'}
              </pre>
              <div className="input-bar">
                <input
                  type="text"
                  className="input-field"
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Type a message and press Enter..."
                />
                <button className="btn btn-primary" onClick={handleSend}>
                  Send
                </button>
              </div>
            </>
          ) : (
            <div className="empty-state">
              Select a session or create a new one.
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

export default App;
