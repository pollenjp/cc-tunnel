vi.mock('@xterm/xterm', () => {
  class Terminal {
    open = vi.fn();
    write = vi.fn();
    onData = vi.fn();
    dispose = vi.fn();
    constructor() {}
  }
  return { Terminal };
});

vi.mock('../api/client', () => ({
  getAuthOutput: vi.fn(),
  submitAuthInput: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';
import { AuthTerminal } from './AuthTerminal';
import { getAuthOutput } from '../api/client';

describe('AuthTerminal', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.mocked(getAuthOutput).mockResolvedValue({ data: '', cursor: 0 } as never);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('onTextOutput callback が getAuthOutput のレスポンス内容で呼ばれる', async () => {
    const onTextOutput = vi.fn();
    const testText = 'Hello, PTY!';
    vi.mocked(getAuthOutput).mockResolvedValue({ data: btoa(testText), cursor: 10 } as never);

    render(<AuthTerminal conversationId="test-conv" onTextOutput={onTextOutput} />);

    await vi.advanceTimersByTimeAsync(250);

    expect(onTextOutput).toHaveBeenCalledWith(testText);
  });

  it('callback 未指定でも例外なく動作する（後方互換）', async () => {
    vi.mocked(getAuthOutput).mockResolvedValue({ data: btoa('some text'), cursor: 5 } as never);

    expect(() => {
      render(<AuthTerminal conversationId="test-conv" />);
    }).not.toThrow();

    await expect(vi.advanceTimersByTimeAsync(250)).resolves.not.toThrow();
  });
});
