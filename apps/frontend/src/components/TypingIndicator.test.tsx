import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { TypingIndicator } from './TypingIndicator';

describe('TypingIndicator', () => {
  it('renders 3 dots with animate-pulse class', () => {
    const { container } = render(<TypingIndicator />);
    const dots = container.querySelectorAll('.animate-pulse');
    expect(dots.length).toBe(3);
  });

  it('sets staggered animation delays on each dot (0s, 0.2s, 0.4s)', () => {
    const { container } = render(<TypingIndicator />);
    const dots = container.querySelectorAll('.animate-pulse');
    expect((dots[0] as HTMLElement).style.animationDelay).toBe('0s');
    expect((dots[1] as HTMLElement).style.animationDelay).toBe('0.2s');
    expect((dots[2] as HTMLElement).style.animationDelay).toBe('0.4s');
  });
});
