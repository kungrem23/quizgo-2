// @vitest-environment jsdom
import { act, cleanup, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { deadlineProgress, deadlineState, useDeadline } from '../../../src/features/game/deadline';

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('server deadline display', () => {
  it('derives remaining time from the absolute deadline and rounds seconds up', () => {
    const now = Date.parse('2026-09-30T12:00:00.000Z');
    expect(deadlineState('2026-09-30T12:00:01.001Z', now)).toEqual({
      remainingMs: 1001,
      remainingSeconds: 2,
      expired: false,
    });
  });

  it('clamps at zero without creating a phase transition', () => {
    const now = Date.parse('2026-09-30T12:00:02.000Z');
    expect(deadlineState('2026-09-30T12:00:01.000Z', now)).toEqual({
      remainingMs: 0,
      remainingSeconds: 0,
      expired: true,
    });
    expect(deadlineState(undefined, now).expired).toBe(true);
  });

  it('calculates a bounded progress percentage from the question duration', () => {
    expect(deadlineProgress(15_000, 30)).toBe(50);
    expect(deadlineProgress(45_000, 30)).toBe(100);
    expect(deadlineProgress(-1, 30)).toBe(0);
    expect(deadlineProgress(1_000, 0)).toBe(0);
  });

  it('updates the display to zero but does not drive any game phase', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-30T12:00:00.000Z'));
    const deadline = '2026-09-30T12:00:01.000Z';
    const timer = renderHook(() => useDeadline(deadline, 100));
    expect(timer.result.current.remainingSeconds).toBe(1);

    act(() => vi.advanceTimersByTime(1_100));

    expect(timer.result.current).toEqual({
      remainingMs: 0,
      remainingSeconds: 0,
      expired: true,
    });
  });
});
