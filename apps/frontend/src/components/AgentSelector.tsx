interface Agent {
  id: string
  name: string
  description: string
  available: boolean
}

const AGENTS: Agent[] = [
  { id: 'claude-code', name: 'Claude Code', description: 'Anthropic Claude Code', available: true },
  { id: 'github-copilot', name: 'GitHub Copilot', description: '将来対応予定', available: false },
  { id: 'cursor-cli', name: 'Cursor CLI', description: '将来対応予定', available: false },
]

interface AgentSelectorProps {
  onSelect: (agentId: string) => void
  onCancel?: () => void
  isLoading?: boolean
}

export const AgentSelector: React.FC<AgentSelectorProps> = ({ onSelect, onCancel, isLoading = false }) => {
  return (
    <div
      className="flex flex-col items-center justify-center h-full gap-6 text-[var(--color-text)]"
      data-testid="agent-selector"
    >
      <h2 className="text-xl font-semibold text-[var(--color-text-bright)]">AIエージェントを選択</h2>
      <div className="flex flex-col gap-3 w-full max-w-sm">
        {AGENTS.map(agent => {
          const disabled = !agent.available || isLoading;
          return (
            <button
              key={agent.id}
              data-testid={`agent-btn-${agent.id}`}
              disabled={disabled}
              onClick={() => onSelect(agent.id)}
              className={[
                'flex flex-col items-start px-4 py-3 rounded-lg border transition-colors text-left',
                agent.available && !isLoading
                  ? 'border-[var(--color-accent)] bg-[var(--color-bg-secondary)] hover:bg-[var(--color-bg-tertiary)] cursor-pointer text-[var(--color-text-bright)]'
                  : 'border-[var(--color-border)] bg-[var(--color-bg-secondary)] opacity-50 cursor-not-allowed text-[var(--color-text)]',
              ].join(' ')}
            >
              <div className="flex items-center gap-2">
                <span className="font-medium">{agent.name}</span>
                {!agent.available && (
                  <span className="text-xs px-1.5 py-0.5 rounded bg-[var(--color-border)] text-[var(--color-text)]">
                    未対応
                  </span>
                )}
              </div>
              <span className="text-sm mt-0.5">{agent.description}</span>
            </button>
          );
        })}
      </div>
      {isLoading && (
        <div
          data-testid="agent-selector-loading"
          className="flex items-center gap-2 text-sm text-[var(--color-text-bright)]"
        >
          <span className="inline-block h-4 w-4 rounded-full border-2 border-[var(--color-accent)] border-t-transparent animate-spin" />
          <span>セッションを準備中... (VM/コンテナを起動しています)</span>
        </div>
      )}
      {onCancel && !isLoading && (
        <button
          onClick={onCancel}
          className="text-sm text-[var(--color-text)] hover:text-[var(--color-text-bright)] transition-colors"
        >
          キャンセル
        </button>
      )}
    </div>
  );
};
