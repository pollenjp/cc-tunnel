export function TypingIndicator() {
  return (
    <div className="flex items-center px-1 py-1" data-testid="typing-indicator">
      <span className="typing-shimmer text-sm font-medium">進行中...</span>
    </div>
  );
}
