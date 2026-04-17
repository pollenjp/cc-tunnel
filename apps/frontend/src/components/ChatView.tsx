import { useEffect, useRef } from 'react';
import type { Message } from '../api/client';
import { MessageBubble } from './MessageBubble';
import { MessageInput } from './MessageInput';

interface ChatViewProps {
  messages: Message[];
  onSend: (content: string) => void;
  sending: boolean;
  input: string;
  onInputChange: (value: string) => void;
  onHamburger: () => void;
}

export function ChatView({ messages, onSend, sending, input, onInputChange, onHamburger }: ChatViewProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const isEmpty = messages.length === 0;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {isEmpty ? (
        <div className="flex-1 flex items-center justify-center text-[var(--color-text)]">
          <p>メッセージを入力して会話を始めましょう。</p>
        </div>
      ) : (
        <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
          {messages.map((msg, idx) => {
            const isLast = idx === messages.length - 1;
            const isStreamingMsg = sending && isLast && msg.role === 'assistant';
            return (
              <MessageBubble
                key={msg.id}
                message={msg}
                isStreaming={isStreamingMsg}
              />
            );
          })}
          <div ref={messagesEndRef} />
        </div>
      )}
      <MessageInput
        value={input}
        onChange={onInputChange}
        onSend={() => onSend(input)}
        disabled={sending}
        onHamburger={onHamburger}
      />
    </div>
  );
}
