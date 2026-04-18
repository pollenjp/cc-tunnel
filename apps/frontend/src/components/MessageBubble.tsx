import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import type { Message } from '../api/client';

interface MessageBubbleProps {
  message: Message;
  isStreaming?: boolean;
  streamingThinkings?: string[];
}

function ThinkingAccordion({ content }: { content: string }) {
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

export function MessageBubble({ message, isStreaming, streamingThinkings }: MessageBubbleProps) {
  const isUser = message.role === 'user';

  const thinkings: string[] = streamingThinkings && streamingThinkings.length > 0
    ? streamingThinkings
    : message.metadata?.thinkings as string[] | undefined
      ?? (message.metadata?.thinking
          ? [message.metadata.thinking as string]
          : []);

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
          {message.content}
        </ReactMarkdown>
      </div>
    </div>
  );
}
