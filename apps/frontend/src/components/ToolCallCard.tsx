import { useState } from 'react';
import type { ToolCall } from '../App';

const TOOL_ICONS: Record<string, string> = {
  Read: '📂', Bash: '💻', Edit: '✏️', Write: '📝',
  Glob: '🔍', Grep: '🔎', WebSearch: '🌐', WebFetch: '📡',
};

function getInputPreview(toolName: string, inputJson: string): string {
  try {
    const input = JSON.parse(inputJson) as Record<string, unknown>;
    if (toolName === 'Bash' && typeof input.command === 'string') return input.command;
    if ((toolName === 'Read' || toolName === 'Edit' || toolName === 'Write') && typeof input.file_path === 'string') return input.file_path;
    if (toolName === 'Glob' && typeof input.pattern === 'string') return input.pattern;
    if (toolName === 'Grep' && typeof input.pattern === 'string') return input.pattern;
    if (toolName === 'WebSearch' && typeof input.query === 'string') return input.query;
    if (toolName === 'WebFetch' && typeof input.url === 'string') return input.url;
    const firstKey = Object.keys(input)[0];
    if (firstKey !== undefined) return String(input[firstKey]);
  } catch {
    // ignore parse errors
  }
  return '';
}

function trimPreview(text: string, maxLen = 70): string {
  const firstLine = text.split('\n')[0] ?? text;
  if (firstLine.length <= maxLen) return firstLine;
  return firstLine.slice(0, maxLen) + '…';
}

function getResultPreview(result: string, maxLines = 4): string {
  return result.split('\n').slice(0, maxLines).join('\n');
}

export function ToolCallCard({ toolCall }: { toolCall: ToolCall }) {
  const [open, setOpen] = useState(false);
  const icon = TOOL_ICONS[toolCall.toolName] ?? '🔧';
  const inputPreview = trimPreview(getInputPreview(toolCall.toolName, toolCall.inputJson));

  return (
    <div className="mx-1 my-1 rounded-lg border border-[var(--color-border)] text-[12px] overflow-hidden">
      {/* Header row */}
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center gap-2 px-3 py-1.5 bg-[var(--color-bg-tertiary)] text-[var(--color-text)] hover:bg-[var(--color-border)] transition-colors text-left"
      >
        <span className="flex-shrink-0">{icon}</span>
        <span className="font-bold flex-shrink-0">{toolCall.toolName}</span>
        {inputPreview && (
          <span className="opacity-60 truncate text-[11px] font-mono flex-1 min-w-0">{inputPreview}</span>
        )}
        {!inputPreview && <span className="flex-1" />}
        {toolCall.isRunning && (
          <span className="flex-shrink-0 inline-block w-2 h-2 rounded-full bg-yellow-400 animate-pulse" />
        )}
        {!toolCall.isRunning && (
          <span className="flex-shrink-0 text-green-400 text-[10px]">✓</span>
        )}
        <span className="flex-shrink-0 text-[10px] opacity-50">{open ? '▾' : '▸'}</span>
      </button>

      {/* Result preview (closed state only) */}
      {!open && toolCall.isRunning && !toolCall.result && (
        <div className="px-3 py-1 bg-[var(--color-bg-tertiary)] border-t border-[var(--color-border)]">
          <span className="text-[10px] opacity-40">実行中...</span>
        </div>
      )}
      {!open && !toolCall.isRunning && toolCall.result && (
        <div
          className="px-3 py-1 bg-[var(--color-bg-tertiary)] border-t border-[var(--color-border)] cursor-pointer"
          onClick={() => setOpen(o => !o)}
        >
          <pre className="text-[10px] font-mono opacity-40 whitespace-pre-wrap break-all line-clamp-4">
            {getResultPreview(toolCall.result)}
          </pre>
        </div>
      )}

      {/* Expanded state: full input + result */}
      {open && (
        <div className="px-3 py-2 bg-[var(--color-bg)] space-y-2">
          {toolCall.isRunning && !toolCall.result && (
            <div className="text-[11px] opacity-60">🔄 実行中...</div>
          )}
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
