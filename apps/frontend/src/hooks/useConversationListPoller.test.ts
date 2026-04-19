import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useConversationListPoller } from './useConversationListPoller';

describe('useConversationListPoller (TDD Cycle 2)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('hasRunning=true のとき 3 秒ごとに onPoll を呼ぶ', () => {
    const onPoll = vi.fn();
    renderHook(() => useConversationListPoller({ hasRunning: true, onPoll }));

    expect(onPoll).not.toHaveBeenCalled(); // 即座には呼ばれない

    act(() => { vi.advanceTimersByTime(3000); });
    expect(onPoll).toHaveBeenCalledTimes(1);

    act(() => { vi.advanceTimersByTime(3000); });
    expect(onPoll).toHaveBeenCalledTimes(2);
  });

  it('hasRunning=false のとき onPoll を呼ばない', () => {
    const onPoll = vi.fn();
    renderHook(() => useConversationListPoller({ hasRunning: false, onPoll }));

    act(() => { vi.advanceTimersByTime(9000); });
    expect(onPoll).not.toHaveBeenCalled();
  });

  it('hasRunning が true→false に変わるとポーリングが停止する', () => {
    const onPoll = vi.fn();
    let hasRunning = true;

    const { rerender } = renderHook(() =>
      useConversationListPoller({ hasRunning, onPoll }),
    );

    act(() => { vi.advanceTimersByTime(3000); });
    expect(onPoll).toHaveBeenCalledTimes(1);

    hasRunning = false;
    rerender();
    onPoll.mockClear();

    act(() => { vi.advanceTimersByTime(9000); });
    expect(onPoll).not.toHaveBeenCalled();
  });
});
