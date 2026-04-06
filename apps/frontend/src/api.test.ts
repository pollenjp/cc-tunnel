import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  createSession,
  listSessions,
  discoverSessions,
  sendKeys,
  getOutput,
  getAllOutputs,
  deleteSession,
  resizeSession,
} from './api';
import type { Session, DiscoveredSession } from './api';

const mockFetch = vi.fn();
globalThis.fetch = mockFetch;

beforeEach(() => {
  mockFetch.mockReset();
});

function jsonResponse(data: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  } as Response;
}

describe('createSession', () => {
  it('sends POST and returns session', async () => {
    const session: Session = {
      id: 'abc123',
      type: 'claude_code',
      tmux_name: 'claude-abc123',
      pane_count: 1,
      created_at: '2024-01-01T00:00:00Z',
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(session, 201));

    const result = await createSession({ type: 'claude_code' });
    expect(result).toEqual(session);
    expect(mockFetch).toHaveBeenCalledWith('/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: 'claude_code' }),
    });
  });

  it('throws on error response', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ error: 'fail' }, 500));
    await expect(createSession()).rejects.toThrow();
  });
});

describe('listSessions', () => {
  it('returns session list', async () => {
    const sessions: Session[] = [];
    mockFetch.mockResolvedValueOnce(jsonResponse(sessions));

    const result = await listSessions();
    expect(result).toEqual([]);
    expect(mockFetch).toHaveBeenCalledWith('/sessions');
  });
});

describe('discoverSessions', () => {
  it('returns discovered sessions', async () => {
    const discovered: DiscoveredSession[] = [
      { type: 'claude_code', tmux_names: ['claude-abc'] },
    ];
    mockFetch.mockResolvedValueOnce(jsonResponse(discovered));

    const result = await discoverSessions();
    expect(result).toEqual(discovered);
  });
});

describe('sendKeys', () => {
  it('sends keys without paneIndex', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ status: 'ok' }));

    await sendKeys('id1', ['h', 'i']);
    expect(mockFetch).toHaveBeenCalledWith('/sessions/id1/input', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ keys: ['h', 'i'] }),
    });
  });

  it('sends keys with paneIndex', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ status: 'ok' }));

    await sendKeys('id1', ['Enter'], 2);
    expect(mockFetch).toHaveBeenCalledWith(
      '/sessions/id1/input?paneIndex=2',
      expect.objectContaining({ method: 'POST' }),
    );
  });
});

describe('resizeSession', () => {
  it('sends resize request', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ status: 'ok' }));

    await resizeSession('id1', 120, 40);
    expect(mockFetch).toHaveBeenCalledWith('/sessions/id1/resize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ width: 120, height: 40 }),
    });
  });
});

describe('getOutput', () => {
  it('returns output string', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ output: 'hello world' }));

    const result = await getOutput('id1');
    expect(result).toBe('hello world');
  });

  it('passes paneIndex query param', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ output: 'pane1' }));

    await getOutput('id1', 1);
    expect(mockFetch).toHaveBeenCalledWith('/sessions/id1/output?paneIndex=1');
  });
});

describe('getAllOutputs', () => {
  it('returns panes map', async () => {
    const panes = { '0': 'output0', '1': 'output1' };
    mockFetch.mockResolvedValueOnce(jsonResponse({ panes }));

    const result = await getAllOutputs('id1');
    expect(result).toEqual(panes);
  });
});

describe('deleteSession', () => {
  it('sends DELETE request', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ status: 'deleted' }));

    await deleteSession('id1');
    expect(mockFetch).toHaveBeenCalledWith('/sessions/id1', { method: 'DELETE' });
  });
});
