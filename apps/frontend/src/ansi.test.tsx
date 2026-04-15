import { describe, it, expect } from 'vitest';
import { isValidElement, type ReactNode } from 'react';
import { parseAnsi } from './ansi';

type SpanProps = { style?: Record<string, unknown>; children?: ReactNode };

function asSpan(node: ReactNode): { style: Record<string, unknown>; text: string } {
  if (!isValidElement(node)) {
    throw new Error(`expected span element, got ${typeof node}`);
  }
  const props = node.props as SpanProps;
  return {
    style: props.style ?? {},
    text: String(props.children ?? ''),
  };
}

describe('parseAnsi', () => {
  it('returns an empty array for empty input', () => {
    expect(parseAnsi('')).toEqual([]);
  });

  it('passes through plain text unchanged', () => {
    const nodes = parseAnsi('hello world');
    expect(nodes).toEqual(['hello world']);
  });

  it('applies foreground color from an SGR code', () => {
    const nodes = parseAnsi('\x1b[31mred\x1b[0m');
    expect(nodes).toHaveLength(1);
    const span = asSpan(nodes[0]);
    expect(span.text).toBe('red');
    expect(span.style.color).toBeDefined();
  });

  it('applies bold and underline attributes', () => {
    const nodes = parseAnsi('\x1b[1;4mbold-underline\x1b[0m');
    expect(nodes).toHaveLength(1);
    const span = asSpan(nodes[0]);
    expect(span.text).toBe('bold-underline');
    expect(span.style.fontWeight).toBe('bold');
    expect(span.style.textDecoration).toBe('underline');
  });

  it('resets to default style after ESC[0m', () => {
    const nodes = parseAnsi('\x1b[31mred\x1b[0m plain');
    expect(nodes).toHaveLength(2);
    const redSpan = asSpan(nodes[0]);
    expect(redSpan.text).toBe('red');
    expect(nodes[1]).toBe(' plain');
  });

  it('handles 24-bit truecolor foreground', () => {
    const nodes = parseAnsi('\x1b[38;2;10;20;30mtc\x1b[0m');
    const span = asSpan(nodes[0]);
    expect(span.style.color).toBe('rgb(10,20,30)');
  });

  it('handles 256-color foreground', () => {
    const nodes = parseAnsi('\x1b[38;5;196mx\x1b[0m');
    const span = asSpan(nodes[0]);
    expect(typeof span.style.color).toBe('string');
  });

  it('strips non-SGR CSI sequences like cursor positioning', () => {
    const nodes = parseAnsi('\x1b[2J\x1b[Hhello');
    expect(nodes).toEqual(['hello']);
  });

  it('strips OSC sequences', () => {
    const nodes = parseAnsi('before\x1b]0;title\x07after');
    expect(nodes).toEqual(['before', 'after']);
  });

  it('continues a color span across the text until reset', () => {
    const nodes = parseAnsi('\x1b[32mgreen line 1\ngreen line 2\x1b[0m');
    expect(nodes).toHaveLength(1);
    const span = asSpan(nodes[0]);
    expect(span.text).toBe('green line 1\ngreen line 2');
  });
});
