import { describe, it, expect, vi, afterEach } from 'vitest';
import { sendMessage } from '../api/client';

afterEach(() => {
  vi.restoreAllMocks();
});

function makeSSEStream(lines: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream({
    start(ctrl) {
      for (const line of lines) {
        ctrl.enqueue(encoder.encode(line));
      }
      ctrl.close();
    },
  });
}

describe('sendMessage abort', () => {
  it('passes signal to fetch', async () => {
    const controller = new AbortController();
    const body = makeSSEStream(['data: {"type":"cost","total_cost_usd":0,"duration_ms":0}\n\n']);
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, body });
    vi.stubGlobal('fetch', fetchMock);

    await sendMessage('conv-1', 'hello', vi.fn(), controller.signal);

    expect(fetchMock).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ signal: controller.signal }),
    );
  });

  it('returns without calling onEvent when signal is pre-aborted', async () => {
    const controller = new AbortController();
    controller.abort();

    const abortError = new DOMException('The operation was aborted', 'AbortError');
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(abortError));

    const onEvent = vi.fn();
    await expect(sendMessage('conv-1', 'hello', onEvent, controller.signal)).resolves.toBeUndefined();
    expect(onEvent).not.toHaveBeenCalled();
  });

  it('handles AbortError thrown by reader.read() gracefully', async () => {
    const controller = new AbortController();
    const abortError = new DOMException('The operation was aborted', 'AbortError');

    const encoder = new TextEncoder();
    let readCall = 0;
    const readerMock = {
      read: vi.fn().mockImplementation(() => {
        readCall++;
        if (readCall === 1) {
          return Promise.resolve({
            done: false,
            value: encoder.encode('data: {"type":"cost","total_cost_usd":0,"duration_ms":0}\n\n'),
          });
        }
        return Promise.reject(abortError);
      }),
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      body: { getReader: () => readerMock },
    }));

    const onEvent = vi.fn();
    await expect(sendMessage('conv-1', 'hello', onEvent, controller.signal)).resolves.toBeUndefined();
    expect(onEvent).toHaveBeenCalledTimes(1);
  });
});
