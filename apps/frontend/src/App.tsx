import { Routes, Route, useParams, Navigate } from 'react-router-dom';
import { AppAuthProvider } from './contexts/AppAuthProvider';
import AppAuthGuard from './components/AppAuthGuard';
import { HomePage } from './pages/HomePage';
import { LoginPage } from './pages/LoginPage';
import { ChatPage } from './pages/ChatPage';
import { AccountSettingsPage } from './pages/AccountSettingsPage';
import { AgentSettingsPage } from './pages/AgentSettingsPage';

export interface ToolCall {
  index: number;
  toolUseId: string;
  toolName: string;
  inputJson: string;
  result?: string;
  isRunning: boolean;
}

export type AssistantBlock =
  | { type: 'thinking'; content: string }
  | { type: 'text'; content: string }
  | { type: 'tool'; toolCall: ToolCall }

const ConversationRedirect = () => {
  const { id } = useParams()
  return <Navigate to={`/chat/${id}`} replace />
}

function App() {
  return (
    <AppAuthProvider>
      <Routes>
        {/* 公開ルート */}
        <Route path="/" element={<HomePage />} />
        <Route path="/login" element={<LoginPage />} />

        {/* 保護ルート: AppAuthGuard */}
        <Route path="/chat" element={<AppAuthGuard><ChatPage /></AppAuthGuard>} />
        <Route path="/chat/:id" element={<AppAuthGuard><ChatPage /></AppAuthGuard>} />
        <Route path="/settings/account" element={<AppAuthGuard><AccountSettingsPage /></AppAuthGuard>} />
        <Route path="/settings/agents" element={<AppAuthGuard><AgentSettingsPage /></AppAuthGuard>} />

        {/* 後方互換: /conversation/:id → /chat/:id (AF002) */}
        <Route path="/conversation/:id" element={<ConversationRedirect />} />
      </Routes>
    </AppAuthProvider>
  );
}

export default App;
