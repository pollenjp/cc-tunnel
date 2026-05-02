import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { getCredentialsStatus } from '../api/credentials';
import { createConversation } from '../api/client';
import { useAppAuth } from '../hooks/useAppAuth';
import type { CredentialsStatus } from '../api/credentials';

export const AgentSettingsPage = () => {
  const { token } = useAppAuth();
  const navigate = useNavigate();
  const [credentials, setCredentials] = useState<CredentialsStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoggingIn, setIsLoggingIn] = useState(false);

  useEffect(() => {
    if (!token) return;
    getCredentialsStatus(token)
      .then(setCredentials)
      .catch(() => setCredentials({ registered: false, isValid: false }))
      .finally(() => setIsLoading(false));
  }, [token]);

  const handleLogin = async () => {
    setIsLoggingIn(true);
    try {
      const conv = await createConversation();
      navigate(`/login/credentials?reason=missing&conversationId=${encodeURIComponent(conv.id)}`);
    } catch (e) {
      console.error('Failed to create conversation:', e);
      setIsLoggingIn(false);
    }
  };

  return (
    <div className="min-h-screen bg-[var(--color-bg)] text-[var(--color-text)] p-8">
      <div className="max-w-2xl mx-auto">
        <Link
          to="/"
          className="inline-flex items-center gap-1 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text)] mb-6"
        >
          &larr; ホーム
        </Link>
        <h1 className="text-2xl font-bold text-[var(--color-text-bright)] mb-6">Agentログイン設定</h1>

        <div className="flex flex-col gap-4">
          {/* Claude Code カード */}
          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-5">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-lg font-semibold text-[var(--color-text-bright)]">Claude Code</h2>
              <span className="text-xs px-2 py-0.5 rounded-full bg-[var(--color-accent)] text-white">
                対応済み
              </span>
            </div>

            {isLoading ? (
              <div
                role="status"
                className="inline-block h-5 w-5 rounded-full border-2 border-[var(--color-accent)] border-t-transparent animate-spin"
              />
            ) : credentials?.registered && credentials.isValid ? (
              <span className="text-sm font-medium text-green-500">認証済み ✓</span>
            ) : (
              <button
                onClick={() => void handleLogin()}
                disabled={isLoggingIn}
                className="px-5 py-2 rounded bg-[var(--color-accent)] text-white font-medium hover:bg-[var(--color-accent-hover)] disabled:opacity-50 flex items-center gap-2"
              >
                {isLoggingIn && (
                  <span className="inline-block h-4 w-4 rounded-full border-2 border-white border-t-transparent animate-spin" />
                )}
                Claude Code でログイン
              </button>
            )}
          </div>

          {/* GitHub Copilot カード */}
          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-5 opacity-60">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-lg font-semibold text-[var(--color-text-bright)]">GitHub Copilot</h2>
              <span className="text-xs px-2 py-0.5 rounded-full bg-gray-500 text-white">
                将来対応
              </span>
            </div>
            <button
              disabled
              className="px-5 py-2 rounded border border-[var(--color-border)] text-[var(--color-text-muted)] cursor-not-allowed opacity-50"
            >
              未対応（準備中）
            </button>
          </div>

          {/* Cursor CLI カード */}
          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-5 opacity-60">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-lg font-semibold text-[var(--color-text-bright)]">Cursor CLI</h2>
              <span className="text-xs px-2 py-0.5 rounded-full bg-gray-500 text-white">
                将来対応
              </span>
            </div>
            <button
              disabled
              className="px-5 py-2 rounded border border-[var(--color-border)] text-[var(--color-text-muted)] cursor-not-allowed opacity-50"
            >
              未対応（準備中）
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
