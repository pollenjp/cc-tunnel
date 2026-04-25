import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAppAuth } from '../hooks/useAppAuth'

export const UserMenu: React.FC = () => {
  const { user, logout } = useAppAuth()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)

  const handleLogout = async () => {
    await logout()
    navigate('/')
  }

  return (
    <div className="relative">
      <button
        data-testid="user-menu-button"
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 px-3 py-1.5 rounded-lg hover:bg-[var(--color-bg-secondary)] text-[var(--color-text)] transition-colors"
      >
        {user ? (
          <>
            <span className="text-[var(--color-text-bright)]">{user.name}</span>
            <span>👤</span>
          </>
        ) : (
          <span>👤</span>
        )}
      </button>
      {open && (
        <div className="absolute right-0 mt-1 w-48 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg shadow-lg z-10">
          {user ? (
            <>
              <Link
                to="/settings/account"
                onClick={() => setOpen(false)}
                className="block px-4 py-2 text-[var(--color-text)] hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text-bright)] rounded-t-lg"
              >
                アカウント設定
              </Link>
              <Link
                to="/settings/agents"
                onClick={() => setOpen(false)}
                className="block px-4 py-2 text-[var(--color-text)] hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text-bright)]"
              >
                Agentログイン設定
              </Link>
              <button
                onClick={() => { void handleLogout() }}
                className="block w-full text-left px-4 py-2 text-[var(--color-danger)] hover:bg-[var(--color-bg-tertiary)] rounded-b-lg"
              >
                ログアウト
              </button>
            </>
          ) : (
            <Link
              to="/login"
              onClick={() => setOpen(false)}
              className="block px-4 py-2 text-[var(--color-accent)] hover:bg-[var(--color-bg-tertiary)] rounded-lg"
            >
              ログイン
            </Link>
          )}
        </div>
      )}
    </div>
  )
}
