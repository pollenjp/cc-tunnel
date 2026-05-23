// openapi-fetch の use() で登録された middleware を捕捉し、直接テストする
const { mockGet, registeredMiddlewares } = vi.hoisted(() => {
  type MiddlewareFn = {
    onRequest: (ctx: { request: Request }) => Request | Promise<Request>;
    onResponse?: (ctx: { response: Response }) => Response | Promise<Response>;
  };
  const middlewares: MiddlewareFn[] = [];
  return {
    registeredMiddlewares: middlewares,
    mockGet: vi.fn(),
  };
});

vi.mock('openapi-fetch', () => ({
  default: () => ({
    GET: mockGet,
    POST: vi.fn(),
    DELETE: vi.fn(),
    use: vi.fn((mw) => registeredMiddlewares.push(mw)),
  }),
}));

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { sendMessage, listConversations } from './client';

describe('client module', () => {
  it('exports conversations API functions', () => {
    expect(listConversations).toBeDefined();
    expect(sendMessage).toBeDefined();
  });
});

describe('openapi-fetch middleware - Authorization injection', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('middleware adds Authorization: Bearer header when token exists in sessionStorage', async () => {
    sessionStorage.setItem('app_auth_token', 'test-token-middleware');
    expect(registeredMiddlewares.length).toBeGreaterThan(0);

    const request = new Request('http://localhost/api/conversations');
    const result = await registeredMiddlewares[0].onRequest({ request });

    expect(result.headers.get('Authorization')).toBe('Bearer test-token-middleware');
  });

  it('middleware does not add Authorization header when sessionStorage has no token', async () => {
    expect(registeredMiddlewares.length).toBeGreaterThan(0);

    const request = new Request('http://localhost/api/conversations');
    const result = await registeredMiddlewares[0].onRequest({ request });

    expect(result.headers.get('Authorization')).toBeNull();
  });
});

describe('openapi-fetch middleware - 401 handling', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('401 レスポンス時に sessionStorage をクリアして /login へリダイレクト', async () => {
    sessionStorage.setItem('app_auth_token', 'stale-token');
    const assignMock = vi.fn();
    vi.stubGlobal('location', {
      assign: assignMock,
      pathname: '/chat',
      search: '?x=1',
      hash: '',
    });
    const mw = registeredMiddlewares.find(m => m.onResponse);
    expect(mw?.onResponse).toBeDefined();

    const response = new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 });
    mw!.onResponse!({ response });

    expect(sessionStorage.getItem('app_auth_token')).toBeNull();
    expect(assignMock).toHaveBeenCalledWith(expect.stringContaining('/login?'));
    expect(assignMock.mock.calls[0][0]).toContain('redirect=%2Fchat%3Fx%3D1');
  });

  it('/login にいる時は再度 /login へリダイレクトしない', async () => {
    const assignMock = vi.fn();
    vi.stubGlobal('location', {
      assign: assignMock,
      pathname: '/login',
      search: '',
      hash: '',
    });
    const mw = registeredMiddlewares.find(m => m.onResponse);

    const response = new Response(null, { status: 401 });
    mw!.onResponse!({ response });

    expect(assignMock).not.toHaveBeenCalled();
  });

  it('200 レスポンスではリダイレクトしない', async () => {
    sessionStorage.setItem('app_auth_token', 'valid-token');
    const assignMock = vi.fn();
    vi.stubGlobal('location', { assign: assignMock, pathname: '/chat', search: '', hash: '' });
    const mw = registeredMiddlewares.find(m => m.onResponse);

    const response = new Response('{}', { status: 200 });
    mw!.onResponse!({ response });

    expect(sessionStorage.getItem('app_auth_token')).toBe('valid-token');
    expect(assignMock).not.toHaveBeenCalled();
  });
});

describe('sendMessage (raw fetch) - Authorization injection', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    sessionStorage.clear();
    fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('sends Authorization: Bearer header when token is in sessionStorage', async () => {
    const token = 'test-token-sendmsg';
    sessionStorage.setItem('app_auth_token', token);
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ message_id: 'msg-1' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await sendMessage('conv-1', 'hello');

    expect(fetchMock).toHaveBeenCalledOnce();
    const [, options] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = options.headers as Record<string, string>;
    expect(headers['Authorization']).toBe(`Bearer ${token}`);
  });

  it('does not send Authorization header when sessionStorage has no token', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ message_id: 'msg-2' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await sendMessage('conv-2', 'world');

    expect(fetchMock).toHaveBeenCalledOnce();
    const [, options] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = options.headers as Record<string, string>;
    expect(headers['Authorization']).toBeUndefined();
  });
});
