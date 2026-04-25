import { useNavigate } from 'react-router-dom'
import { useAppAuth } from '../contexts/AppAuthContext'
import { UserMenu } from '../components/UserMenu'

export const HomePage = () => {
  const { user } = useAppAuth()
  const navigate = useNavigate()

  return (
    <div className="min-h-screen bg-[var(--color-bg)] flex flex-col">
      <header className="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
        <h1 className="text-xl font-bold text-[var(--color-text-bright)]">cc-tunnel</h1>
        <UserMenu />
      </header>
      <main className="flex flex-col items-center justify-center flex-1 gap-6 p-8">
        <p className="text-[var(--color-text-muted)] text-center">
          AI エージェントとのチャットトンネル
        </p>
        <button
          onClick={() => navigate(user ? '/chat' : '/login?redirect=/chat')}
          className="px-6 py-3 bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-white font-semibold rounded-lg transition-colors"
        >
          チャット開始
        </button>
      </main>
    </div>
  )
}
