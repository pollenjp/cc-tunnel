import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import type { Message, SSEHookEvent } from '../api/client';
import type { StreamMeta } from '../App';

interface MessageBubbleProps {
  message: Message;
  textContent?: string;
  isStreaming?: boolean;
  streamMeta?: StreamMeta | null;
  hookEvents?: SSEHookEvent[];
}

export function ThinkingAccordion({ content }: { content: string }) {
  const [open, setOpen] = useState(false);
  const preview = content.slice(0, 40).replace(/\n/g, ' ');
  return (
    <div className="my-1 rounded border border-[var(--color-border)] text-[12px]">
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center gap-2 px-3 py-1.5 bg-[var(--color-bg-tertiary)] text-[var(--color-text)] hover:bg-[var(--color-border)] transition-colors text-left"
      >
        <span>🤔</span>
        {!open && (
          <span className="opacity-60 truncate">{preview}{content.length > 40 ? '...' : ''}</span>
        )}
        {open && <span className="opacity-60">思考過程</span>}
        <span className="ml-auto opacity-50">{open ? '▾' : '▸'}</span>
      </button>
      {open && (
        <div className="px-3 py-2 bg-[var(--color-bg)]">
          <pre className="text-[11px] font-mono text-[var(--color-text)] opacity-70 whitespace-pre-wrap break-words max-h-64 overflow-y-auto">
            {content}
          </pre>
        </div>
      )}
    </div>
  );
}

export function MessageBubble({ message, textContent, isStreaming, streamMeta, hookEvents }: MessageBubbleProps) {
  const isUser = message.role === 'user';

  const thinkings: string[] = (() => {
    const arr = message.metadata?.thinkings;
    if (Array.isArray(arr)) return arr as string[];
    const t = message.metadata?.thinking;
    if (Array.isArray(t)) return t as string[];
    if (typeof t === 'string') return [t];
    return [];
  })();

  const model = streamMeta?.model ?? (message.metadata as Record<string, unknown> | undefined)?.model as string | undefined;
  const costUSD = streamMeta?.totalCostUSD ?? (message.metadata as Record<string, unknown> | undefined)?.cost_usd as number | undefined;
  const durationMs = streamMeta?.durationMs ?? (message.metadata as Record<string, unknown> | undefined)?.duration_ms as number | undefined;
  const msgHookEvents = hookEvents ?? (message.metadata as Record<string, unknown> | undefined)?.hook_events as SSEHookEvent[] | undefined;

  return (
    <div className={`flex flex-col gap-1 max-w-[75%] ${isUser ? 'self-end' : 'self-start'}`}>
      <div className={`text-[11px] font-semibold uppercase tracking-wider text-[var(--color-text)] px-1 ${isUser ? 'text-right' : ''}`}>
        {isUser ? 'You' : 'Assistant'}
      </div>
      {!isUser && thinkings.map((t, i) => (
        <ThinkingAccordion key={i} content={t} />
      ))}
      <div
        className={[
          'px-[14px] py-[10px] text-[14px] leading-relaxed prose-chat',
          isUser
            ? 'bg-[var(--color-accent)] text-[#1a1b26] rounded-[18px_18px_4px_18px]'
            : 'bg-[var(--color-bg-secondary)] text-[var(--color-text-bright)] rounded-[18px_18px_18px_4px]',
          isStreaming ? 'streaming-cursor' : '',
        ].join(' ')}
      >
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            code({ className, children, ...props }) {
              const match = /language-(\w+)/.exec(className ?? '');
              const isBlock = Boolean(match);
              const codeString = String(children).replace(/\n$/, '');
              if (isBlock) {
                return (
                  <SyntaxHighlighter
                    style={oneDark}
                    language={match![1]}
                    PreTag="div"
                  >
                    {codeString}
                  </SyntaxHighlighter>
                );
              }
              return (
                <code className={className} {...props}>
                  {children}
                </code>
              );
            },
          }}
        >
          {textContent ?? message.content}
        </ReactMarkdown>
      </div>
      {!isUser && (model || costUSD != null || msgHookEvents && msgHookEvents.length > 0) && (
        <div className="mt-2 text-[11px] opacity-60 space-y-1">
          <div className="flex flex-wrap gap-x-3">
            {model && <span>🤖 {model}</span>}
            {costUSD != null && <span>${costUSD.toFixed(4)}</span>}
            {durationMs != null && <span>{durationMs}ms</span>}
          </div>
          {msgHookEvents && msgHookEvents.length > 0 && (
            <details className="mt-1">
              <summary className="cursor-pointer">▸ Hook Events ({msgHookEvents.length})</summary>
              <div className="mt-1 space-y-1">
                {msgHookEvents.map((ev, i) => (
                  <div key={i} className="px-2 py-0.5 rounded bg-[var(--color-bg-tertiary)]">
                    {ev.subtype} {ev.hook_name && `— ${ev.hook_name}`}
                  </div>
                ))}
              </div>
            </details>
          )}
        </div>
      )}
    </div>
  );
}
