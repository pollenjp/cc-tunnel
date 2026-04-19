export function TypingIndicator() {
  return (
    <div className="flex items-center gap-1 px-1 py-1" data-testid="typing-indicator">
      {[0, 0.2, 0.4].map((delay, i) => (
        <span
          key={i}
          className="inline-block h-2 w-2 rounded-full bg-[var(--color-text-muted)] animate-pulse"
          style={{ animationDelay: `${delay}s` }}
        />
      ))}
    </div>
  );
}
