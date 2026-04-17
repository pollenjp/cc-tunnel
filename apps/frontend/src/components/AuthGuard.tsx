import type { UseAuthReturn } from '../hooks/useAuth';
import { AuthTerminal } from './AuthTerminal';

interface Props {
  auth: UseAuthReturn;
  children: React.ReactNode;
}

export function AuthGuard({ auth, children }: Props) {
  const { status, isLoading, login } = auth;

  if (isLoading && !status) {
    return (
      <div className="flex items-center justify-center h-screen bg-[var(--color-bg)]">
        <div className="animate-spin rounded-full h-10 w-10 border-4 border-[var(--color-accent)] border-t-transparent" />
      </div>
    );
  }

  if (status?.loggedIn) {
    return <>{children}</>;
  }

  if (status?.loginPending) {
    return (
      <div className="flex flex-col items-center justify-center h-screen gap-6 bg-[var(--color-bg)] text-[var(--color-text)]">
        <h2 className="text-xl font-semibold text-[var(--color-text-bright)]">Claude 認証中...</h2>
        <AuthTerminal />
        <button
          onClick={auth.cancelLogin}
          disabled={isLoading}
          className="text-sm text-[var(--color-text)] hover:text-[var(--color-danger)] disabled:opacity-50 flex items-center gap-2"
        >
          {isLoading ? (
            <span className="inline-block h-3.5 w-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
          ) : null}
          キャンセル
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center h-screen gap-6 bg-[var(--color-bg)] text-[var(--color-text)]">
      <h1 className="text-3xl font-bold text-[var(--color-text-bright)]">cc-tunnel</h1>
      <p className="text-sm">Claude CLI に認証してください</p>
      <button
        onClick={() => login()}
        disabled={isLoading}
        className="px-5 py-2.5 rounded-lg bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] font-medium disabled:opacity-60 flex items-center gap-2"
      >
        {isLoading ? (
          <span className="inline-block h-4 w-4 rounded-full border-2 border-white border-t-transparent animate-spin" />
        ) : null}
        Claude でログイン
      </button>
    </div>
  );
}
