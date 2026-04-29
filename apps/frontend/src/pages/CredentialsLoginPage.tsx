import { useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAppAuth } from '../hooks/useAppAuth';
import { startRelogin, finalizeRelogin } from '../api/credentials';
import { initiateLogin, getAuthOutput, submitAuthInput } from '../api/client';

type Phase = 'starting' | 'pty' | 'finalizing' | 'done' | 'error';

/**
 * CredentialsLoginPage handles the relogin flow:
 *   1. POST /api/credentials/relogin/start  (spin up session container)
 *   2. POST /auth/login                     (start PTY)
 *   3. Poll /auth/output, relay input via /auth/input
 *   4. Detect "Login successful" → POST /api/credentials/relogin/finalize
 *   5. Navigate back to the chat page
 */
export function CredentialsLoginPage() {
  const { token } = useAppAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const conversationId = searchParams.get('conversationId') ?? '';
  const reason = searchParams.get('reason') ?? 'missing';

  const [phase, setPhase] = useState<Phase>('starting');
  const [error, setError] = useState('');
  const [ptyLines, setPtyLines] = useState<string[]>([]);
  const [inputValue, setInputValue] = useState('');

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const cursorRef = useRef(0);
  const fullOutputRef = useRef('');
  const finalizedRef = useRef(false);

  // Stop polling
  const stopPolling = () => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  };

  // Finalize relogin after detecting success
  const doFinalize = async () => {
    if (finalizedRef.current || !token) return;
    finalizedRef.current = true;
    stopPolling();
    setPhase('finalizing');
    try {
      await finalizeRelogin(token, conversationId);
      setPhase('done');
      navigate(conversationId ? `/chat/${conversationId}` : '/chat', { replace: true });
    } catch (e) {
      setError(e instanceof Error ? e.message : 'finalize failed');
      setPhase('error');
    }
  };

  // Poll PTY output
  const pollOutput = async () => {
    try {
      const res = await getAuthOutput(conversationId, cursorRef.current);
      if (res.data && res.data.length > 0) {
        const binary = atob(res.data);
        fullOutputRef.current += binary;
        // Append new lines to display
        const newText = binary.replace(/\r/g, '');
        if (newText) {
          setPtyLines(prev => {
            const combined = (prev.join('\n') + newText).split('\n');
            // Keep last 200 lines
            return combined.slice(-200);
          });
        }
        // Detect login success
        if (/Login successful|Logged in|authentication successful/i.test(fullOutputRef.current)) {
          void doFinalize();
        }
      }
      cursorRef.current = res.cursor;
    } catch { /* ignore transient errors */ }
  };

  // Start the relogin flow
  useEffect(() => {
    if (!token || !conversationId) {
      setError('Missing token or conversationId');
      setPhase('error');
      return;
    }

    (async () => {
      try {
        await startRelogin(token, conversationId);
        await initiateLogin(conversationId);
        setPhase('pty');
        pollRef.current = setInterval(() => { void pollOutput(); }, 250);
      } catch (e) {
        setError(e instanceof Error ? e.message : 'startup failed');
        setPhase('error');
      }
    })();

    return stopPolling;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, conversationId]);

  const handleSendInput = async () => {
    if (!inputValue) return;
    try {
      await submitAuthInput(conversationId, inputValue);
      setInputValue('');
    } catch { /* ignore */ }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      void handleSendInput();
    }
  };

  const reasonLabel = reason === 'expired' ? 'credentials が失効しました' : 'credentials が未登録です';

  return (
    <div className="min-h-screen bg-[var(--color-bg)] flex flex-col items-center justify-center gap-6 p-4">
      <div className="w-full max-w-2xl bg-[var(--color-bg-secondary)] rounded-xl border border-[var(--color-border)] p-6 flex flex-col gap-4">
        <h2 className="text-xl font-bold text-[var(--color-text-bright)]">
          Claude 認証 — {reasonLabel}
        </h2>

        {phase === 'starting' && (
          <div className="flex items-center gap-2 text-[var(--color-text)]">
            <div className="animate-spin rounded-full h-5 w-5 border-2 border-[var(--color-accent)] border-t-transparent" />
            <span>セッションコンテナを起動中…</span>
          </div>
        )}

        {phase === 'pty' && (
          <>
            <p className="text-sm text-[var(--color-text-muted)]">
              認証フローが開始されました。画面の指示に従い認証を完了してください。
            </p>
            <div
              className="font-mono text-xs bg-[#1a1b26] text-[#a9b1d6] rounded-lg p-3 h-64 overflow-y-auto whitespace-pre-wrap"
              aria-label="pty-output"
            >
              {ptyLines.join('\n')}
            </div>
            <div className="flex gap-2">
              <input
                type="text"
                value={inputValue}
                onChange={e => setInputValue(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="入力…"
                className="flex-1 px-3 py-1.5 bg-[var(--color-bg)] border border-[var(--color-border)] rounded text-[var(--color-text)] text-sm focus:outline-none focus:border-[var(--color-accent)]"
              />
              <button
                onClick={() => { void handleSendInput(); }}
                className="px-3 py-1.5 bg-[var(--color-accent)] text-white text-sm rounded hover:bg-[var(--color-accent-hover)] transition-colors"
              >
                送信
              </button>
              <button
                onClick={() => submitAuthInput(conversationId, '\r').catch(() => {})}
                className="px-3 py-1.5 bg-[var(--color-bg-tertiary)] text-[var(--color-text-bright)] text-sm border border-[var(--color-border)] rounded hover:border-[var(--color-accent)] transition-colors"
              >
                ↵ Enter
              </button>
            </div>
            <button
              onClick={() => {
                navigator.clipboard.readText()
                  .then(text => { if (text) void submitAuthInput(conversationId, text); })
                  .catch(() => {});
              }}
              className="self-start px-3 py-1.5 bg-[var(--color-bg-tertiary)] text-[var(--color-text-bright)] text-xs border border-[var(--color-border)] rounded hover:border-[var(--color-accent)] transition-colors"
            >
              📋 クリップボードから貼り付け
            </button>
          </>
        )}

        {phase === 'finalizing' && (
          <div className="flex items-center gap-2 text-[var(--color-text)]">
            <div className="animate-spin rounded-full h-5 w-5 border-2 border-[var(--color-accent)] border-t-transparent" />
            <span>credentials を保存中…</span>
          </div>
        )}

        {phase === 'done' && (
          <p className="text-[var(--color-success)]">認証が完了しました。チャット画面に戻ります…</p>
        )}

        {phase === 'error' && (
          <p role="alert" className="text-[var(--color-danger)]">
            エラー: {error}
          </p>
        )}
      </div>
    </div>
  );
}
