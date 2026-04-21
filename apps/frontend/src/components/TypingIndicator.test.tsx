import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TypingIndicator } from './TypingIndicator';

describe('TypingIndicator', () => {
  it("renders shimmer text '進行中...'", () => {
    render(<TypingIndicator />);
    expect(screen.getByText('進行中...')).toBeTruthy();
  });

  it('has typing-shimmer class on the text element', () => {
    const { container } = render(<TypingIndicator />);
    const shimmer = container.querySelector('.typing-shimmer');
    expect(shimmer).toBeTruthy();
  });

  it('is wrapped in a div with data-testid="typing-indicator"', () => {
    render(<TypingIndicator />);
    expect(screen.getByTestId('typing-indicator')).toBeTruthy();
  });
});
