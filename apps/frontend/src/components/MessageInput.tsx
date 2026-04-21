interface MessageInputProps {
  value: string;
  onChange: (value: string) => void;
  onSend: () => void;
  disabled: boolean;
  onHamburger: () => void;
}

export function MessageInput({ value, onChange, onSend, disabled, onHamburger }: MessageInputProps) {
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      onSend();
    }
  };

  const handleInput = (e: React.FormEvent<HTMLTextAreaElement>) => {
    const el = e.currentTarget;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  };

  return (
    <div className="flex gap-2 px-4 py-3 border-t border-[var(--color-border)] bg-[var(--color-bg-secondary)] items-end">
      <button
        className="md:hidden shrink-0 px-3 py-2 text-base rounded-md bg-[var(--color-bg-tertiary)] text-[var(--color-text-bright)] cursor-pointer hover:bg-[var(--color-border)] transition-colors"
        onClick={onHamburger}
        aria-label="メニュー"
      >
        ☰
      </button>
      <textarea
        className="flex-1 px-3.5 py-2.5 border border-[var(--color-border)] rounded-xl bg-[var(--color-bg)] text-[var(--color-text-bright)] text-[14px] outline-none transition-colors resize-none min-h-[44px] max-h-[200px] overflow-y-auto leading-relaxed placeholder:text-[var(--color-text)] placeholder:opacity-60 focus:border-[var(--color-accent)] disabled:opacity-50 disabled:cursor-not-allowed"
        value={value}
        onChange={e => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        onInput={handleInput}
        placeholder="メッセージを入力... (Enter で送信、Shift+Enter で改行)"
        disabled={disabled}
        rows={1}
      />
      <button
        className="shrink-0 px-4 py-2 rounded-md bg-[var(--color-accent)] text-[#1a1b26] text-sm font-medium cursor-pointer hover:bg-[var(--color-accent-hover)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        onClick={onSend}
        disabled={disabled || !value.trim()}
      >
        {disabled ? '送信中...' : '送信'}
      </button>
    </div>
  );
}
