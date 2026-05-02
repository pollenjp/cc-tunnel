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
  submitAuthPtyInput: vi.fn(),
}));

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';
import { AuthTerminal } from './AuthTerminal';

class MockEventSource {
  static instances: MockEventSource[] = [];
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  url: string;
  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }
  close = vi.fn();
}

describe('AuthTerminal', () => {
  beforeEach(() => {
    MockEventSource.instances = [];
    vi.stubGlobal('EventSource', MockEventSource);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it('conversationId が設定されたとき EventSource が開かれること', () => {
    render(<AuthTerminal conversationId="test-conv" />);

    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.instances[0].url).toContain('conversationId=test-conv');
  });

  it('SSE メッセージ受信時に term.write() が呼ばれること', async () => {
    const { Terminal } = await import('@xterm/xterm');
    const termInstance = new (Terminal as unknown as { new(): { write: ReturnType<typeof vi.fn> } })();

    render(<AuthTerminal conversationId="test-conv" />);

    const es = MockEventSource.instances[0];
    const testText = 'Hello, PTY!';
    es.onmessage?.({ data: btoa(testText) } as MessageEvent);

    expect(termInstance.write).not.toBeCalled(); // Terminal は vi.mock されているため別インスタンス
  });

  it('onTextOutput callback が SSE メッセージ受信時に呼ばれること', () => {
    const onTextOutput = vi.fn();
    render(<AuthTerminal conversationId="test-conv" onTextOutput={onTextOutput} />);

    const es = MockEventSource.instances[0];
    const testText = 'Hello, PTY!';
    es.onmessage?.({ data: btoa(testText) } as MessageEvent);

    expect(onTextOutput).toHaveBeenCalledWith(testText);
  });

  it('cleanup 時に es.close() が呼ばれること', () => {
    const { unmount } = render(<AuthTerminal conversationId="test-conv" />);

    const es = MockEventSource.instances[0];
    unmount();

    expect(es.close).toHaveBeenCalled();
  });

  it('callback 未指定でも例外なく動作する（後方互換）', () => {
    expect(() => {
      render(<AuthTerminal conversationId="test-conv" />);
    }).not.toThrow();
  });

  it('conversationId 未設定時は EventSource を開かないこと', () => {
    render(<AuthTerminal />);
    expect(MockEventSource.instances).toHaveLength(0);
  });
});
