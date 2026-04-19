import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSSEAbort } from './useSSEAbort';

describe('useSSEAbort', () => {
  it('startStream returns a non-aborted signal initially', () => {
    const { result } = renderHook(() => useSSEAbort());

    let signal!: AbortSignal;
    act(() => {
      const out = result.current.startStream('session-1');
      signal = out.signal;
    });

    expect(signal.aborted).toBe(false);
  });

  it('startStream aborts the previous controller when called again', () => {
    const { result } = renderHook(() => useSSEAbort());

    let signal1!: AbortSignal;
    act(() => {
      signal1 = result.current.startStream('session-1').signal;
    });

    act(() => {
      result.current.startStream('session-2');
    });

    expect(signal1.aborted).toBe(true);
  });

  it('switchSession aborts the current controller', () => {
    const { result } = renderHook(() => useSSEAbort());

    let signal!: AbortSignal;
    act(() => {
      signal = result.current.startStream('session-1').signal;
    });

    act(() => {
      result.current.switchSession('session-2');
    });

    expect(signal.aborted).toBe(true);
  });

  it('isActiveSession returns true for the current session', () => {
    const { result } = renderHook(() => useSSEAbort());

    act(() => {
      result.current.startStream('session-1');
    });

    expect(result.current.isActiveSession('session-1')).toBe(true);
    expect(result.current.isActiveSession('session-2')).toBe(false);
  });

  it('isActiveSession returns false for old session after switchSession', () => {
    const { result } = renderHook(() => useSSEAbort());

    act(() => {
      result.current.startStream('session-1');
    });

    act(() => {
      result.current.switchSession('session-2');
    });

    expect(result.current.isActiveSession('session-1')).toBe(false);
    expect(result.current.isActiveSession('session-2')).toBe(true);
  });

  it('aborts the current controller on unmount', () => {
    const { result, unmount } = renderHook(() => useSSEAbort());

    let signal!: AbortSignal;
    act(() => {
      signal = result.current.startStream('session-1').signal;
    });

    unmount();

    expect(signal.aborted).toBe(true);
  });
});
