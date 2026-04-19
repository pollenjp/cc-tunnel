import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useConversationPoller } from './useConversationPoller';

/** Flush pending microtasks (resolved Promises) without advancing fake timers. */
const flushMicrotasks = () => act(async () => { await Promise.resolve(); });

// Mock getConversation API
vi.mock('../api/client', () => ({
  getConversation: vi.fn(),
}));

import { getConversation } from '../api/client';
const mockGetConversation = vi.mocked(getConversation);

function makeDetail(status: 'idle' | 'running' | 'completed', messages: { id: string; updated_at?: string }[]) {
  return {
    id: 'conv-1',
    title: 'Test',
    model: 'claude-sonnet-4-6',
    status,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    messages: messages.map(m => ({
      id: m.id,
      conversation_id: 'conv-1',
      role: 'assistant' as const,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: m.updated_at ?? '2026-01-01T00:00:00Z',
    })),
  };
}

describe('useConversationPoller', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockGetConversation.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not poll when isRunning is false', async () => {
    const onMessages = vi.fn();
    const onCompleted = vi.fn();

    renderHook(() =>
      useConversationPoller({
        conversationId: 'conv-1',
        isRunning: false,
        onMessages,
        onCompleted,
        intervalMs: 2000,
      }),
    );

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockGetConversation).not.toHaveBeenCalled();
  });

  it('polls on interval when isRunning is true', async () => {
    mockGetConversation.mockResolvedValue(makeDetail('running', []));

    const onMessages = vi.fn();
    const onCompleted = vi.fn();

    renderHook(() =>
      useConversationPoller({
        conversationId: 'conv-1',
        isRunning: true,
        onMessages,
        onCompleted,
        intervalMs: 2000,
      }),
    );

    await act(async () => {
      vi.advanceTimersByTime(6000);
    });

    // Should have been called at least 2-3 times in 6 seconds with 2s interval
    expect(mockGetConversation).toHaveBeenCalledWith('conv-1');
    expect(mockGetConversation.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it('calls onMessages with full message list on every poll when running', async () => {
    // First poll: 1 message
    mockGetConversation
      .mockResolvedValueOnce(makeDetail('running', [{ id: 'msg-1' }]))
      // Second poll: 2 messages (both should be passed to onMessages)
      .mockResolvedValueOnce(makeDetail('running', [{ id: 'msg-1' }, { id: 'msg-2' }]));

    const onMessages = vi.fn();
    const onCompleted = vi.fn();

    renderHook(() =>
      useConversationPoller({
        conversationId: 'conv-1',
        isRunning: true,
        onMessages,
        onCompleted,
        intervalMs: 2000,
      }),
    );

    // First poll
    await act(async () => { vi.advanceTimersByTime(2000); });
    await flushMicrotasks();
    expect(mockGetConversation).toHaveBeenCalledTimes(1);
    expect(onMessages).toHaveBeenCalledTimes(1);
    // First poll: onMessages called with [msg-1]
    expect(onMessages.mock.calls[0][0]).toHaveLength(1);
    expect(onMessages.mock.calls[0][0][0]).toMatchObject({ id: 'msg-1' });

    // Second poll
    await act(async () => { vi.advanceTimersByTime(2000); });
    await flushMicrotasks();
    expect(mockGetConversation).toHaveBeenCalledTimes(2);
    expect(onMessages).toHaveBeenCalledTimes(2);
    // Second poll: onMessages called with ALL messages [msg-1, msg-2]
    expect(onMessages.mock.calls[1][0]).toHaveLength(2);
    expect(onMessages.mock.calls[1][0][0]).toMatchObject({ id: 'msg-1' });
    expect(onMessages.mock.calls[1][0][1]).toMatchObject({ id: 'msg-2' });
  });

  it('calls onMessages on every poll even when message ids are unchanged (streaming update)', async () => {
    // Same message ID, but updated_at differs → simulates streaming content update
    mockGetConversation
      .mockResolvedValueOnce(makeDetail('running', [{ id: 'msg-streaming', updated_at: '2026-01-01T00:00:01Z' }]))
      .mockResolvedValueOnce(makeDetail('running', [{ id: 'msg-streaming', updated_at: '2026-01-01T00:00:02Z' }]));

    const onMessages = vi.fn();
    const onCompleted = vi.fn();

    renderHook(() =>
      useConversationPoller({
        conversationId: 'conv-1',
        isRunning: true,
        onMessages,
        onCompleted,
        intervalMs: 2000,
      }),
    );

    // First poll
    await act(async () => { vi.advanceTimersByTime(2000); });
    await flushMicrotasks();
    expect(onMessages).toHaveBeenCalledTimes(1);

    // Second poll: same id, should still call onMessages (full overwrite needed)
    await act(async () => { vi.advanceTimersByTime(2000); });
    await flushMicrotasks();
    expect(onMessages).toHaveBeenCalledTimes(2);
  });

  it('calls onMessages with all messages then onCompleted when status becomes completed', async () => {
    mockGetConversation
      .mockResolvedValueOnce(makeDetail('running', []))
      .mockResolvedValueOnce(makeDetail('completed', [{ id: 'msg-final' }]));

    const callOrder: string[] = [];
    const onMessages = vi.fn(() => { callOrder.push('onMessages'); });
    const onCompleted = vi.fn(() => { callOrder.push('onCompleted'); });

    renderHook(() =>
      useConversationPoller({
        conversationId: 'conv-1',
        isRunning: true,
        onMessages,
        onCompleted,
        intervalMs: 2000,
      }),
    );

    // First poll (running)
    await act(async () => { vi.advanceTimersByTime(2000); });
    await flushMicrotasks();
    expect(mockGetConversation).toHaveBeenCalledTimes(1);

    // Second poll (completed)
    await act(async () => { vi.advanceTimersByTime(2000); });
    await flushMicrotasks();
    expect(onCompleted).toHaveBeenCalled();
    // onMessages should be called before onCompleted with msg-final
    expect(callOrder).toEqual(['onMessages', 'onMessages', 'onCompleted']);
    const lastOnMessageCall = onMessages.mock.calls[onMessages.mock.calls.length - 1][0];
    expect(lastOnMessageCall).toHaveLength(1);
    expect(lastOnMessageCall[0]).toMatchObject({ id: 'msg-final' });

    const callCountAfterComplete = mockGetConversation.mock.calls.length;

    // After completed, no more polling
    await act(async () => {
      vi.advanceTimersByTime(6000);
    });
    expect(mockGetConversation.mock.calls.length).toBe(callCountAfterComplete);
  });

  it('stops polling when conversationId becomes null', async () => {
    mockGetConversation.mockResolvedValue(makeDetail('running', []));

    const onMessages = vi.fn();
    const onCompleted = vi.fn();

    const { rerender } = renderHook(
      ({ id, running }: { id: string | null; running: boolean }) =>
        useConversationPoller({
          conversationId: id,
          isRunning: running,
          onMessages,
          onCompleted,
          intervalMs: 2000,
        }),
      { initialProps: { id: 'conv-1', running: true } },
    );

    await act(async () => {
      vi.advanceTimersByTime(2000);
    });
    const callsBefore = mockGetConversation.mock.calls.length;
    expect(callsBefore).toBeGreaterThanOrEqual(1);

    // Set conversationId to null
    rerender({ id: null, running: false });

    await act(async () => {
      vi.advanceTimersByTime(6000);
    });
    expect(mockGetConversation.mock.calls.length).toBe(callsBefore);
  });
});
