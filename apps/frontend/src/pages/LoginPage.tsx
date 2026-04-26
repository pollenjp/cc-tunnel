import { useState } from 'react'
import { useNavigate, useSearchParams, Navigate } from 'react-router-dom'
import { useAppAuth } from '../hooks/useAppAuth'

export const LoginPage = () => {
  const [username, setUsername] = useState('')
  const [error, setError] = useState('')
  const { login, user, isLoading } = useAppAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const redirect = searchParams.get('redirect') || '/chat'

  if (!isLoading && user) {
    return <Navigate to={redirect} replace />
  }

  const handleLogin = async () => {
    if (!username.trim()) {
      setError('ユーザー名を入力してください')
      return
    }
    setError('')
    await login(username)
    navigate(redirect)
  }

  return (
    <div className="min-h-screen bg-[var(--color-bg)] flex items-center justify-center">
      <div className="w-full max-w-sm p-8 bg-[var(--color-bg-secondary)] rounded-xl border border-[var(--color-border)]">
        <h2 className="text-2xl font-bold text-[var(--color-text-bright)] mb-6 text-center">ログイン</h2>
        {error && (
          <p role="alert" className="mb-4 text-[var(--color-danger)] text-sm">
            {error}
          </p>
        )}
        <input
          value={username}
          onChange={e => setUsername(e.target.value)}
          placeholder="ユーザー名"
          className="w-full px-4 py-2 mb-4 bg-[var(--color-bg)] border border-[var(--color-border)] rounded-lg text-[var(--color-text)] focus:outline-none focus:border-[var(--color-accent)]"
        />
        <button
          onClick={() => { void handleLogin() }}
          className="w-full px-4 py-2 bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-white font-semibold rounded-lg transition-colors"
        >
          ログイン
        </button>
      </div>
    </div>
  )
}
