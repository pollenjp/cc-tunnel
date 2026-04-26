// vi.hoisted で変数を vi.mock ファクトリより前に初期化する
const { mockPost, mockGet, mockPatch } = vi.hoisted(() => ({
  mockPost: vi.fn(),
  mockGet: vi.fn(),
  mockPatch: vi.fn(),
}));

vi.mock('openapi-fetch', () => ({
  default: () => ({
    POST: mockPost,
    GET: mockGet,
    PATCH: mockPatch,
  }),
}));

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { login, getMe, logout, updateMe } from './app-auth';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('app-auth API', () => {
  describe('login', () => {
    it('POST /app-auth/login を username で呼び出し AppAuthLoginResponse を返す', async () => {
      const mockResp = { token: 'tok123', user: { id: 'u1', name: 'Alice' } };
      mockPost.mockResolvedValue({ data: mockResp, error: undefined });

      const result = await login('Alice');

      expect(mockPost).toHaveBeenCalledWith('/app-auth/login', { body: { username: 'Alice' } });
      expect(result).toEqual(mockResp);
    });

    it('エラーレスポンス時に throw する', async () => {
      mockPost.mockResolvedValue({ data: undefined, error: { message: 'login failed' } });

      await expect(login('fail')).rejects.toEqual({ message: 'login failed' });
    });
  });

  describe('getMe', () => {
    it('GET /app-auth/me を Authorization ヘッダー付きで呼び出し AppAuthMeResponse を返す', async () => {
      const mockResp = { user: { id: 'u1', name: 'Alice' } };
      mockGet.mockResolvedValue({ data: mockResp, error: undefined });

      const result = await getMe('tok123');

      expect(mockGet).toHaveBeenCalledWith('/app-auth/me', {
        headers: { Authorization: 'Bearer tok123' },
      });
      expect(result).toEqual(mockResp);
    });

    it('エラーレスポンス時に throw する', async () => {
      mockGet.mockResolvedValue({ data: undefined, error: { message: 'unauthorized' } });

      await expect(getMe('bad-token')).rejects.toEqual({ message: 'unauthorized' });
    });
  });

  describe('logout', () => {
    it('POST /app-auth/logout を Authorization ヘッダー付きで呼び出す', async () => {
      mockPost.mockResolvedValue({ data: undefined, error: undefined });

      await logout('tok123');

      expect(mockPost).toHaveBeenCalledWith('/app-auth/logout', {
        headers: { Authorization: 'Bearer tok123' },
      });
    });
  });

  describe('updateMe', () => {
    it('PATCH /app-auth/me を Authorization ヘッダー + nickname で呼び出し AppAuthMeResponse を返す', async () => {
      const mockResp = { user: { id: 'u1', name: 'NewName' } };
      mockPatch.mockResolvedValue({ data: mockResp, error: undefined });

      const result = await updateMe('tok123', 'NewName');

      expect(mockPatch).toHaveBeenCalledWith('/app-auth/me', {
        headers: { Authorization: 'Bearer tok123' },
        body: { nickname: 'NewName' },
      });
      expect(result).toEqual(mockResp);
    });

    it('エラーレスポンス時に throw する', async () => {
      mockPatch.mockResolvedValue({ data: undefined, error: { message: 'update failed' } });

      await expect(updateMe('tok123', 'Bad')).rejects.toEqual({ message: 'update failed' });
    });
  });
});
