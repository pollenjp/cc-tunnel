// openapi-fetch の use() で登録された middleware を捕捉し、直接テストする
const { mockGet, registeredMiddlewares } = vi.hoisted(() => {
  type MiddlewareFn = { onRequest: (ctx: { request: Request }) => Request | Promise<Request> };
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
