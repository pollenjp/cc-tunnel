import { useState, useEffect, useRef, useCallback } from 'react';
import type { Session, SessionType, DiscoveredSession } from './api';
import {
  createSession,
  listSessions,
  discoverSessions,
  sendKeys,
  getOutput,
  getAllOutputs,
  deleteSession,
  resizeSession,
} from './api';

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

type ViewMode = 'single' | 'grid';

function buildTmuxKey(tmuxKeyName: string, modifiers: Set<Modifier>): string {
  let key = tmuxKeyName;
  if (modifiers.has('Shift')) key = `S-${key}`;
  if (modifiers.has('Alt')) key = `M-${key}`;
  if (modifiers.has('Ctrl')) key = `C-${key}`;
  return key;
}

function getPaneName(paneIndex: number, sessionType: SessionType): string {
  if (sessionType === 'multi_agent_shogun') {
    if (paneIndex === 0) return 'Shogun';
    return `Agent ${paneIndex}`;
  }
  return 'Main';
}

function App() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [output, setOutput] = useState('');
  const [allPaneOutputs, setAllPaneOutputs] = useState<Record<string, string>>({});
  const [input, setInput] = useState('');
  const [polling, setPolling] = useState(false);
  const [activeModifiers, setActiveModifiers] = useState<Set<Modifier>>(new Set());
  const [tmuxWidth, setTmuxWidth] = useState(200);
  const [tmuxHeight, setTmuxHeight] = useState(50);
  const [commandHistory, setCommandHistory] = useState<string[]>([]);
  const [newSessionType, setNewSessionType] = useState<SessionType>('claude_code');
  const [viewMode, setViewMode] = useState<ViewMode>('single');
  const [activePaneIndex, setActivePaneIndex] = useState(0);
  const [discovered, setDiscovered] = useState<DiscoveredSession[]>([]);
  const [showDiscover, setShowDiscover] = useState(false);
  const [creating, setCreating] = useState(false);
  const outputRef = useRef<HTMLPreElement>(null);
  const intervalRef = useRef<number | null>(null);

  const activeSession = sessions.find((s) => s.id === activeId);
  const isMultiAgent = activeSession?.type === 'multi_agent_shogun';

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

  const pollOutput = useCallback(
    async (id: string) => {
      try {
        const session = sessions.find((s) => s.id === id);
        if (session && session.type === 'multi_agent_shogun' && viewMode === 'grid') {
          const outputs = await getAllOutputs(id);
          setAllPaneOutputs(outputs);
        } else {
          const text = await getOutput(id, activePaneIndex > 0 ? activePaneIndex : undefined);
          setOutput(text);
          if (outputRef.current) {
            outputRef.current.scrollTop = outputRef.current.scrollHeight;
          }
        }
      } catch (e) {
        console.error('Failed to get output:', e);
      }
    },
    [sessions, viewMode, activePaneIndex],
  );

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
      await sendKeys(activeId, keys, activePaneIndex > 0 ? activePaneIndex : undefined);
      setCommandHistory((prev) => [...prev, keys.join(' ')]);
    } catch (e) {
      alert(`Failed to send keys: ${e}`);
    }
  };

  const handleCreate = async () => {
    try {
      setCreating(true);
      const opts: Parameters<typeof createSession>[0] = { type: newSessionType };
      if (newSessionType === 'claude_code') {
        opts.width = tmuxWidth;
        opts.height = tmuxHeight;
      }
      const session = await createSession(opts);
      await refreshSessions();
      setActiveId(session.id);
      setOutput('');
      setAllPaneOutputs({});
      setPolling(true);
      setCommandHistory([]);
      setActivePaneIndex(0);
      if (session.type === 'multi_agent_shogun') {
        setViewMode('grid');
      } else {
        setViewMode('single');
      }
    } catch (e) {
      alert(`Failed to create session: ${e}`);
    } finally {
      setCreating(false);
    }
  };

  const handleAdopt = async (d: DiscoveredSession) => {
    try {
      setCreating(true);
      const opts: Parameters<typeof createSession>[0] = { type: d.type };
      if (d.type === 'claude_code') {
        opts.tmux_name = d.tmux_names[0];
        opts.width = tmuxWidth;
        opts.height = tmuxHeight;
      }
      const session = await createSession(opts);
      await refreshSessions();
      setActiveId(session.id);
      setOutput('');
      setAllPaneOutputs({});
      setPolling(true);
      setCommandHistory([]);
      setActivePaneIndex(0);
      setShowDiscover(false);
      if (session.type === 'multi_agent_shogun') {
        setViewMode('grid');
      } else {
        setViewMode('single');
      }
    } catch (e) {
      alert(`Failed to adopt session: ${e}`);
    } finally {
      setCreating(false);
    }
  };

  const handleDiscover = async () => {
    try {
      const list = await discoverSessions();
      setDiscovered(list);
      setShowDiscover(true);
    } catch (e) {
      alert(`Failed to discover sessions: ${e}`);
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
    const session = sessions.find((s) => s.id === id);
    setActiveId(id);
    setOutput('');
    setAllPaneOutputs({});
    setPolling(true);
    setActivePaneIndex(0);
    if (session?.type === 'multi_agent_shogun') {
      setViewMode('grid');
    } else {
      setViewMode('single');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteSession(id);
      if (activeId === id) {
        setActiveId(null);
        setOutput('');
        setAllPaneOutputs({});
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

  const handleGridPaneClick = (paneIndex: number) => {
    setActivePaneIndex(paneIndex);
    setViewMode('single');
    setOutput('');
  };

  const sessionTypeLabel = (type: SessionType) =>
    type === 'claude_code' ? 'CC' : 'Shogun';

  return (
    <div className="app">
      <header className="header">
        <h1>cc-tunnel</h1>
      </header>

      <div className="layout">
        <aside className="sidebar">
          {/* Session type selector */}
          <div className="type-selector">
            <label className="type-option">
              <input
                type="radio"
                name="sessionType"
                value="claude_code"
                checked={newSessionType === 'claude_code'}
                onChange={() => setNewSessionType('claude_code')}
              />
              Claude Code
            </label>
            <label className="type-option">
              <input
                type="radio"
                name="sessionType"
                value="multi_agent_shogun"
                checked={newSessionType === 'multi_agent_shogun'}
                onChange={() => setNewSessionType('multi_agent_shogun')}
              />
              Multi-Agent Shogun
            </label>
          </div>

          {newSessionType === 'claude_code' && (
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
          )}

          <div className="sidebar-buttons">
            <button className="btn btn-primary" onClick={handleCreate} disabled={creating}>
              {creating ? 'Starting...' : '+ New'}
            </button>
            <button className="btn" onClick={handleDiscover} disabled={creating}>
              Discover
            </button>
            {activeId && activeSession?.type === 'claude_code' && (
              <button className="btn" onClick={handleResize}>
                Resize
              </button>
            )}
          </div>

          {/* Discover panel */}
          {showDiscover && (
            <div className="discover-panel">
              <div className="discover-header">
                <span className="discover-title">Discovered Sessions</span>
                <button className="btn btn-sm" onClick={() => setShowDiscover(false)}>
                  Close
                </button>
              </div>
              {discovered.length === 0 ? (
                <div className="discover-empty">No unmanaged sessions found</div>
              ) : (
                <ul className="discover-list">
                  {discovered.map((d, i) => (
                    <li key={i} className="discover-item">
                      <span className="discover-info">
                        <span className={`type-badge ${d.type}`}>{sessionTypeLabel(d.type)}</span>
                        {d.tmux_names.join(', ')}
                      </span>
                      <button className="btn btn-sm btn-primary" onClick={() => handleAdopt(d)}>
                        Attach
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}

          <ul className="session-list">
            {sessions.map((s) => (
              <li
                key={s.id}
                className={`session-item ${s.id === activeId ? 'active' : ''}`}
              >
                <span className="session-name" onClick={() => handleSelect(s.id)}>
                  <span className={`type-badge ${s.type}`}>{sessionTypeLabel(s.type)}</span>
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
          {creating && (
            <div className="loading-overlay">
              <div className="loading-content">
                <div className="spinner" />
                <div className="loading-text">
                  {newSessionType === 'multi_agent_shogun'
                    ? 'Starting Multi-Agent Shogun...'
                    : 'Creating session...'}
                </div>
                <div className="loading-sub">
                  {newSessionType === 'multi_agent_shogun'
                    ? 'Running startup script and waiting for tmux sessions'
                    : 'Setting up tmux session'}
                </div>
              </div>
            </div>
          )}
          {!creating && activeId && activeSession ? (
            <>
              <div className="toolbar">
                <span className="session-label">
                  {activeSession.tmux_name}
                  {isMultiAgent && (
                    <span className="pane-label">
                      {' '}/ {getPaneName(activePaneIndex, activeSession.type)}
                    </span>
                  )}
                </span>

                {isMultiAgent && (
                  <div className="view-toggle">
                    <button
                      className={`btn btn-sm ${viewMode === 'single' ? 'btn-active' : ''}`}
                      onClick={() => setViewMode('single')}
                    >
                      Single
                    </button>
                    <button
                      className={`btn btn-sm ${viewMode === 'grid' ? 'btn-active' : ''}`}
                      onClick={() => setViewMode('grid')}
                    >
                      Grid
                    </button>
                  </div>
                )}

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

              {/* Pane selector tabs for multi-agent in single view */}
              {isMultiAgent && viewMode === 'single' && (
                <div className="pane-tabs">
                  {Array.from({ length: activeSession.pane_count }, (_, i) => (
                    <button
                      key={i}
                      className={`pane-tab ${activePaneIndex === i ? 'active' : ''}`}
                      onClick={() => {
                        setActivePaneIndex(i);
                        setOutput('');
                      }}
                    >
                      {getPaneName(i, activeSession.type)}
                    </button>
                  ))}
                </div>
              )}

              {/* Single pane view */}
              {viewMode === 'single' && (
                <pre className="output" ref={outputRef}>
                  {output || 'Waiting for output...'}
                </pre>
              )}

              {/* Grid view for multi-agent */}
              {viewMode === 'grid' && isMultiAgent && (
                <div className="grid-container">
                  {/* Shogun pane banner */}
                  <div
                    className="grid-shogun"
                    onClick={() => handleGridPaneClick(0)}
                  >
                    <div className="grid-pane-header">Shogun</div>
                    <pre className="grid-pane-output">
                      {allPaneOutputs['0'] || 'Waiting...'}
                    </pre>
                  </div>
                  {/* 3x3 grid of agent panes */}
                  <div className="grid-agents">
                    {Array.from({ length: 9 }, (_, i) => (
                      <div
                        key={i + 1}
                        className="grid-pane"
                        onClick={() => handleGridPaneClick(i + 1)}
                      >
                        <div className="grid-pane-header">Agent {i + 1}</div>
                        <pre className="grid-pane-output">
                          {allPaneOutputs[String(i + 1)] || 'Waiting...'}
                        </pre>
                      </div>
                    ))}
                  </div>
                </div>
              )}

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
                  placeholder={
                    isMultiAgent
                      ? `Send to ${getPaneName(activePaneIndex, activeSession.type)}...`
                      : 'Type text and press Enter to send with Enter key...'
                  }
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
          ) : !creating ? (
            <div className="empty-state">
              Select a session or create a new one.
            </div>
          ) : null}
        </main>
      </div>
    </div>
  );
}

export default App;
