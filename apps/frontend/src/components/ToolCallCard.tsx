import { useState } from 'react';
import type { ToolCall } from '../App';

const TOOL_ICONS: Record<string, string> = {
  Read: '📂', Bash: '💻', Edit: '✏️', Write: '📝',
  Glob: '🔍', Grep: '🔎', WebSearch: '🌐', WebFetch: '📡',
};

export function ToolCallCard({ toolCall }: { toolCall: ToolCall }) {
  const [open, setOpen] = useState(false);
  const icon = TOOL_ICONS[toolCall.toolName] ?? '🔧';

  return (
    <div className="mx-1 my-1 rounded-lg border border-[var(--color-border)] text-[12px] overflow-hidden">
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center gap-2 px-3 py-1.5 bg-[var(--color-bg-tertiary)] text-[var(--color-text)] hover:bg-[var(--color-border)] transition-colors text-left"
      >
        <span>{icon}</span>
        <span className="font-medium">{toolCall.toolName}</span>
        {toolCall.isRunning && (
          <span className="ml-1 inline-block w-2 h-2 rounded-full bg-yellow-400 animate-pulse" />
        )}
        {!toolCall.isRunning && (
          <span className="ml-1 text-green-400 text-[10px]">✓</span>
        )}
        <span className="ml-auto text-[10px] opacity-50">{open ? '▾' : '▸'}</span>
      </button>
      {open && (
        <div className="px-3 py-2 bg-[var(--color-bg)] space-y-2">
          {toolCall.inputJson && (
            <div>
              <div className="text-[10px] opacity-50 mb-1">引数</div>
              <pre className="text-[11px] font-mono text-[var(--color-text)] opacity-80 whitespace-pre-wrap break-all max-h-32 overflow-y-auto">
                {toolCall.inputJson}
              </pre>
            </div>
          )}
          {toolCall.result && (
            <div>
              <div className="text-[10px] opacity-50 mb-1">結果</div>
              <pre className="text-[11px] font-mono text-[var(--color-text)] opacity-70 whitespace-pre-wrap break-all max-h-48 overflow-y-auto">
                {toolCall.result}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
