import { useParams } from 'react-router-dom'
import CredentialGuard from './CredentialGuard'
import { ChatPage } from '../pages/ChatPage'

export function ChatRouteWithCredGuard() {
  const { id } = useParams<{ id: string }>()
  return (
    <CredentialGuard conversationId={id ?? ''}>
      <ChatPage />
    </CredentialGuard>
  )
}
