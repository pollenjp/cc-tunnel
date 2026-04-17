import { useEffect, useRef, useCallback } from 'react';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { getAuthOutput, submitAuthInput } from '../api/client';

interface Props {
  onLoginComplete?: () => void;
}

export function AuthTerminal({ onLoginComplete: _onLoginComplete }: Props) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<Terminal | null>(null);
  const cursorRef = useRef(0);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const pollOutput = useCallback(async () => {
    try {
      const res = await getAuthOutput(cursorRef.current);
      if (res.data && res.data.length > 0) {
        const binary = atob(res.data);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        xtermRef.current?.write(bytes);
      }
      cursorRef.current = res.cursor;
    } catch { /* ignore */ }
  }, []);

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
      submitAuthInput(data).catch(() => {});
    });

    pollRef.current = setInterval(pollOutput, 250);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
      term.dispose();
    };
  }, [pollOutput]);

  return (
    <div
      ref={terminalRef}
      className="rounded-lg overflow-hidden border border-[var(--color-border)]"
      style={{ width: '640px', height: '384px' }}
    />
  );
}
