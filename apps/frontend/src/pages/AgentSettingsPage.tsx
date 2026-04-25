import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { AuthTerminal } from '../components/AuthTerminal'

export const AgentSettingsPage = () => {
  const auth = useAuth()
  const { status, isLoading, login, logout } = auth

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

            {status?.loginPending ? (
              <div className="flex flex-col gap-4">
                <p className="text-sm text-[var(--color-text-muted)]">Claude 認証中...</p>
                <AuthTerminal />
                <button
                  onClick={() => auth.cancelLogin()}
                  className="self-start text-sm text-[var(--color-text-muted)] hover:text-[var(--color-danger)]"
                >
                  キャンセル
                </button>
              </div>
            ) : status?.loggedIn ? (
              <div className="flex items-center gap-4">
                <span className="text-sm font-medium text-green-500">認証済み ✓</span>
                <button
                  onClick={() => logout()}
                  disabled={isLoading}
                  className="text-sm px-3 py-1.5 rounded border border-[var(--color-border)] hover:border-[var(--color-danger)] hover:text-[var(--color-danger)] disabled:opacity-50"
                >
                  ログアウト
                </button>
              </div>
            ) : (
              <button
                onClick={() => login()}
                disabled={isLoading}
                className="px-5 py-2 rounded bg-[var(--color-accent)] text-white font-medium hover:bg-[var(--color-accent-hover)] disabled:opacity-50 flex items-center gap-2"
              >
                {isLoading && (
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
  )
}
