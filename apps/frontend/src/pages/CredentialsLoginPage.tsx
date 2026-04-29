import { useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAppAuth } from '../hooks/useAppAuth';
import { startRelogin, finalizeRelogin } from '../api/credentials';
import { initiateLogin } from '../api/client';
import { AuthTerminal } from '../components/AuthTerminal';

type Phase = 'starting' | 'pty' | 'finalizing' | 'done' | 'error';

/**
 * CredentialsLoginPage handles the relogin flow:
 *   1. POST /api/credentials/relogin/start  (spin up session container)
 *   2. POST /auth/login                     (start PTY)
 *   3. AuthTerminal renders PTY via xterm.js, notifies text via onTextOutput
 *   4. Detect "Login successful" → POST /api/credentials/relogin/finalize
 *   5. Navigate back to the chat page
 */
export function CredentialsLoginPage() {
  const { token } = useAppAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const conversationId = searchParams.get('conversationId') ?? '';
  const reason = searchParams.get('reason') ?? 'missing';

  const isInvalid = !token || !conversationId;
  const [phase, setPhase] = useState<Phase>(isInvalid ? 'error' : 'starting');
  const [error, setError] = useState(isInvalid ? 'Missing token or conversationId' : '');

  const fullOutputRef = useRef('');
  const finalizedRef = useRef(false);

  // Finalize relogin after detecting success
  const doFinalize = async () => {
    if (finalizedRef.current || !token) return;
    finalizedRef.current = true;
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

  // Start the relogin flow
  useEffect(() => {
    if (!token || !conversationId) return;

    (async () => {
      try {
        await startRelogin(token, conversationId);
        await initiateLogin(conversationId);
        setPhase('pty');
      } catch (e) {
        setError(e instanceof Error ? e.message : 'startup failed');
        setPhase('error');
      }
    })();
  }, [token, conversationId]);

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
            <AuthTerminal
              conversationId={conversationId}
              onTextOutput={(text) => {
                fullOutputRef.current += text;
                if (/Login successful|Logged in|authentication successful/i.test(fullOutputRef.current)) {
                  void doFinalize();
                }
              }}
            />
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
