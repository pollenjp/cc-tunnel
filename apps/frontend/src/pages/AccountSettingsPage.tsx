import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useAppAuth } from '../hooks/useAppAuth'

export const AccountSettingsPage = () => {
  const { user, updateNickname } = useAppAuth()
  const [nickname, setNickname] = useState(user?.name ?? '')
  const [saved, setSaved] = useState(false)

  const handleSave = async () => {
    await updateNickname(nickname)
    setSaved(true)
  }

  return (
    <div className="min-h-screen bg-[var(--color-bg)] text-[var(--color-text)] p-8">
      <div className="max-w-md mx-auto">
        <Link
          to="/"
          className="inline-flex items-center gap-1 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text)] mb-6"
        >
          &larr; ホーム
        </Link>
        <h1 className="text-2xl font-bold text-[var(--color-text-bright)] mb-6">アカウント設定</h1>
        <div className="mb-4">
          <p className="text-sm text-[var(--color-text-muted)] mb-1">現在のユーザー名</p>
          <p className="font-medium">{user?.name}</p>
        </div>
        <div className="mb-4">
          <label className="block text-sm text-[var(--color-text-muted)] mb-1" htmlFor="nickname">
            ニックネーム
          </label>
          <input
            id="nickname"
            type="text"
            value={nickname}
            onChange={e => {
              setNickname(e.target.value)
              setSaved(false)
            }}
            className="w-full px-3 py-2 rounded border border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text)] focus:outline-none focus:border-[var(--color-accent)]"
          />
        </div>
        <button
          onClick={handleSave}
          disabled={nickname === ''}
          className="px-5 py-2 rounded bg-[var(--color-accent)] text-white font-medium hover:bg-[var(--color-accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed"
        >
          保存
        </button>
        {saved && (
          <p className="mt-3 text-sm text-green-500">保存しました</p>
        )}
      </div>
    </div>
  )
}
