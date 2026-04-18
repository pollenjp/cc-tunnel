import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import type { Message } from '../api/client';

interface MessageBubbleProps {
  message: Message;
  isStreaming?: boolean;
}

export function MessageBubble({ message, isStreaming }: MessageBubbleProps) {
  const isUser = message.role === 'user';
  const thinking = message.metadata?.thinking as string | undefined;
  const [thinkingOpen, setThinkingOpen] = useState(false);

  return (
    <div className={`flex flex-col gap-1 max-w-[75%] ${isUser ? 'self-end' : 'self-start'}`}>
      <div className={`text-[11px] font-semibold uppercase tracking-wider text-[var(--color-text)] px-1 ${isUser ? 'text-right' : ''}`}>
        {isUser ? 'You' : 'Assistant'}
      </div>
      {!isUser && thinking && (
        <div className="text-xs rounded-lg overflow-hidden border border-[var(--color-border)]">
          <button
            onClick={() => setThinkingOpen(o => !o)}
            className="w-full flex items-center gap-1.5 px-3 py-1.5 bg-[var(--color-bg-tertiary)] text-[var(--color-text)] hover:bg-[var(--color-border)] transition-colors text-left"
          >
            <span className="text-[10px]">{thinkingOpen ? '▾' : '▸'}</span>
            思考過程
          </button>
          {thinkingOpen && (
            <div className="px-3 py-2 bg-[var(--color-bg)] text-[var(--color-text)] italic whitespace-pre-wrap leading-relaxed max-h-64 overflow-y-auto">
              {thinking}
            </div>
          )}
        </div>
      )}
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
