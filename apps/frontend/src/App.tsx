import { useState, useEffect, useRef, useCallback } from 'react';
import type { Session } from './api';
import { createSession, listSessions, sendKeys, getOutput, deleteSession, resizeSession } from './api';

import './App.css';

const MODIFIER_KEYS = ['Ctrl', 'Shift', 'Alt'] as const;
type Modifier = typeof MODIFIER_KEYS[number];

const SPECIAL_KEYS = [
  { label: 'Enter', tmux: 'Enter' },
  { label: 'Esc', tmux: 'Escape' },
  { label: 'Tab', tmux: 'Tab' },
  { label: 'Space', tmux: 'Space' },
  { label: 'BS', tmux: 'BSpace' },
  { label: 'Del', tmux: 'DC' },
  { label: 'Up', tmux: 'Up' },
  { label: 'Down', tmux: 'Down' },
  { label: 'Left', tmux: 'Left' },
  { label: 'Right', tmux: 'Right' },
  { label: 'Home', tmux: 'Home' },
  { label: 'End', tmux: 'End' },
  { label: 'PgUp', tmux: 'PageUp' },
  { label: 'PgDn', tmux: 'PageDown' },
] as const;

const QUICK_COMBOS = [
  { label: 'Ctrl+C', keys: ['C-c'] },
  { label: 'Ctrl+D', keys: ['C-d'] },
  { label: 'Ctrl+Z', keys: ['C-z'] },
  { label: 'Ctrl+L', keys: ['C-l'] },
  { label: 'Ctrl+A', keys: ['C-a'] },
  { label: 'Ctrl+E', keys: ['C-e'] },
] as const;

function buildTmuxKey(tmuxKeyName: string, modifiers: Set<Modifier>): string {
  let key = tmuxKeyName;
  // tmux modifier prefix order: C- M- S-
  if (modifiers.has('Shift')) key = `S-${key}`;
  if (modifiers.has('Alt')) key = `M-${key}`;
  if (modifiers.has('Ctrl')) key = `C-${key}`;
  return key;
}

function App() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [output, setOutput] = useState('');
  const [input, setInput] = useState('');
  const [polling, setPolling] = useState(false);
  const [activeModifiers, setActiveModifiers] = useState<Set<Modifier>>(new Set());
  const [tmuxWidth, setTmuxWidth] = useState(200);
  const [tmuxHeight, setTmuxHeight] = useState(50);
  const [commandHistory, setCommandHistory] = useState<string[]>([]);
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

  const doSendKeys = async (keys: string[]) => {
    if (!activeId) return;
    try {
      await sendKeys(activeId, keys);
      setCommandHistory((prev) => [...prev, keys.join(' ')]);
    } catch (e) {
      alert(`Failed to send keys: ${e}`);
    }
  };

  const handleCreate = async () => {
    try {
      const session = await createSession({ width: tmuxWidth, height: tmuxHeight });
      await refreshSessions();
      setActiveId(session.id);
      setOutput('');
      setPolling(true);
      setCommandHistory([]);
    } catch (e) {
      alert(`Failed to create session: ${e}`);
    }
  };

  const handleResize = async () => {
    if (!activeId) return;
    try {
      await resizeSession(activeId, tmuxWidth, tmuxHeight);
    } catch (e) {
      alert(`Failed to resize session: ${e}`);
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

  const handleSendText = async () => {
    if (!activeId || !input.trim()) return;
    await doSendKeys([input, 'Enter']);
    setInput('');
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendText();
    }
  };

  const toggleModifier = (mod: Modifier) => {
    setActiveModifiers((prev) => {
      const next = new Set(prev);
      if (next.has(mod)) {
        next.delete(mod);
      } else {
        next.add(mod);
      }
      return next;
    });
  };

  const handleSpecialKey = async (tmuxKeyName: string) => {
    const key = buildTmuxKey(tmuxKeyName, activeModifiers);
    await doSendKeys([key]);
    setActiveModifiers(new Set());
  };

  const handleQuickCombo = async (keys: readonly string[]) => {
    await doSendKeys([...keys]);
    setActiveModifiers(new Set());
  };

  return (
    <div className="app">
      <header className="header">
        <h1>cc-tunnel</h1>
      </header>

      <div className="layout">
        <aside className="sidebar">
          <div className="size-controls">
            <label className="size-label">
              Width
              <input
                type="number"
                className="size-input"
                value={tmuxWidth}
                onChange={(e) => setTmuxWidth(Number(e.target.value))}
                min={40}
                max={500}
              />
            </label>
            <label className="size-label">
              Height
              <input
                type="number"
                className="size-input"
                value={tmuxHeight}
                onChange={(e) => setTmuxHeight(Number(e.target.value))}
                min={10}
                max={200}
              />
            </label>
          </div>
          <div className="sidebar-buttons">
            <button className="btn btn-primary" onClick={handleCreate}>
              + New Session
            </button>
            {activeId && (
              <button className="btn" onClick={handleResize}>
                Resize
              </button>
            )}
          </div>
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

              <div className="keys-panel">
                <div className="keys-row">
                  <span className="keys-label">Modifiers</span>
                  {MODIFIER_KEYS.map((mod) => (
                    <button
                      key={mod}
                      className={`key-btn modifier ${activeModifiers.has(mod) ? 'active' : ''}`}
                      onClick={() => toggleModifier(mod)}
                    >
                      {mod}
                    </button>
                  ))}
                  {activeModifiers.size > 0 && (
                    <span className="modifier-indicator">
                      {[...activeModifiers].join('+')}+
                    </span>
                  )}
                </div>

                <div className="keys-row">
                  <span className="keys-label">Keys</span>
                  {SPECIAL_KEYS.map((k) => (
                    <button
                      key={k.tmux}
                      className="key-btn"
                      onClick={() => handleSpecialKey(k.tmux)}
                    >
                      {k.label}
                    </button>
                  ))}
                </div>

                <div className="keys-row">
                  <span className="keys-label">Quick</span>
                  {QUICK_COMBOS.map((c) => (
                    <button
                      key={c.label}
                      className="key-btn combo"
                      onClick={() => handleQuickCombo(c.keys)}
                    >
                      {c.label}
                    </button>
                  ))}
                </div>
              </div>

              <div className="input-bar">
                <input
                  type="text"
                  className="input-field"
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Type text and press Enter to send with Enter key..."
                />
                <button className="btn btn-primary" onClick={handleSendText}>
                  Send
                </button>
              </div>

              {commandHistory.length > 0 && (
                <div className="history-panel">
                  <div className="history-header">
                    <span className="history-title">Command History</span>
                    <button
                      className="btn btn-sm"
                      onClick={() => setCommandHistory([])}
                    >
                      Clear
                    </button>
                  </div>
                  <ul className="history-list">
                    {[...commandHistory].reverse().map((cmd, i) => (
                      <li key={commandHistory.length - 1 - i} className="history-item">
                        <span className="history-index">{commandHistory.length - i}</span>
                        <code className="history-cmd">{cmd}</code>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
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
