import { describe, it, expect, vi, afterEach } from 'vitest';
import { sendMessage } from '../api/client';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('sendMessage', () => {
  it('makes a POST request with content and returns message_id', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ message_id: 'test-uuid' }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await sendMessage('conv-1', 'hello');

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/conversations/conv-1/messages',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: 'hello' }),
      }),
    );
    expect(result).toEqual({ message_id: 'test-uuid' });
  });

  it('throws when response is not ok', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500 }));

    await expect(sendMessage('conv-1', 'hello')).rejects.toThrow('sendMessage failed: 500');
  });

  it('returns parsed JSON body on 202 response', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ message_id: 'abc-def' }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await sendMessage('conv-2', 'test message');

    expect(result.message_id).toBe('abc-def');
  });

  it('401 + redirect body → window.location.assign を呼ぶ（throw しない）', async () => {
    const assignMock = vi.fn();
    vi.stubGlobal('location', { assign: assignMock });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: vi.fn().mockResolvedValue({ redirect: '/login/credentials?reason=missing&conversationId=conv-1' }),
    }));

    await sendMessage('conv-1', 'hello');

    expect(assignMock).toHaveBeenCalledWith('/login/credentials?reason=missing&conversationId=conv-1');
  });

  it('401 + redirect なし → /login へリダイレクトしてトークンをクリア', async () => {
    sessionStorage.setItem('app_auth_token', 'stale-token');
    const assignMock = vi.fn();
    vi.stubGlobal('location', {
      assign: assignMock,
      pathname: '/chat',
      search: '',
      hash: '',
    });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: vi.fn().mockResolvedValue({ error: 'unauthorized' }),
    }));

    const result = await sendMessage('conv-1', 'hello');

    expect(result).toEqual({ message_id: '' });
    expect(sessionStorage.getItem('app_auth_token')).toBeNull();
    expect(assignMock).toHaveBeenCalledWith(expect.stringContaining('/login?'));
    expect(assignMock.mock.calls[0][0]).toContain('redirect=%2Fchat');
  });
});
