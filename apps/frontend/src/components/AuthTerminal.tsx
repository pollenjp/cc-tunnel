import { useEffect, useRef, useCallback, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { getAuthOutput, submitAuthInput } from '../api/client';

interface Props {
  conversationId?: string;
}

export function AuthTerminal({ conversationId = '' }: Props) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<Terminal | null>(null);
  const cursorRef = useRef(0);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [authUrl, setAuthUrl] = useState<string | null>(null);
  const fullOutputRef = useRef<string>('');

  const pollOutput = useCallback(async () => {
    try {
      const res = await getAuthOutput(conversationId, cursorRef.current);
      if (res.data && res.data.length > 0) {
        const binary = atob(res.data);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        xtermRef.current?.write(bytes);

        fullOutputRef.current += binary;
        const flat = fullOutputRef.current.replace(/[\r\n]/g, '');
        const urlMatch = flat.match(/https?:\/\/[^\s'"<>]+/);
        if (urlMatch) {
          setAuthUrl((prev) => prev ?? urlMatch[0]);
        }
      }
      cursorRef.current = res.cursor;
    } catch { /* ignore */ }
  }, [conversationId]);

  useEffect(() => {
    if (!terminalRef.current) return;

    const term = new Terminal({
      rows: 24,
      cols: 80,
      theme: {
        background: '#1a1b26',
        foreground: '#a9b1d6',
        cursor: '#7aa2f7',
      },
      fontFamily: "'SF Mono', 'Fira Code', monospace",
      fontSize: 13,
      cursorBlink: true,
    });

    term.open(terminalRef.current);
    xtermRef.current = term;

    term.onData((data) => {
      submitAuthInput(conversationId, data).catch(() => {});
    });

    pollRef.current = setInterval(pollOutput, 250);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
      term.dispose();
    };
  }, [pollOutput, conversationId]);

  return (
    <div className="flex flex-col items-center gap-2">
      <div
        ref={terminalRef}
        className="rounded-lg overflow-hidden border border-[var(--color-border)]"
        style={{ width: '640px', height: '384px' }}
      />
      {authUrl && (
        <a
          href={authUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="px-4 py-2 rounded-md bg-[var(--color-accent)] text-[#1a1b26] text-sm font-medium hover:bg-[var(--color-accent-hover)] transition-colors"
        >
          認証URLを開く →
        </a>
      )}
      <div className="flex gap-2">
        <button
          onClick={async () => {
            try {
              const text = await navigator.clipboard.readText();
              if (text) await submitAuthInput(conversationId, text);
            } catch { /* ignore */ }
          }}
          className="px-3 py-1.5 rounded text-xs bg-[var(--color-bg-tertiary)] text-[var(--color-text-bright)] border border-[var(--color-border)] hover:border-[var(--color-accent)] transition-colors flex items-center gap-1.5"
        >
          📋 貼り付け
        </button>
        <button
          onClick={() => submitAuthInput(conversationId, '\r').catch(() => {})}
          className="px-3 py-1.5 rounded text-xs bg-[var(--color-bg-tertiary)] text-[var(--color-text-bright)] border border-[var(--color-border)] hover:border-[var(--color-accent)] transition-colors"
        >
          ↵ Enter
        </button>
      </div>
    </div>
  );
}
