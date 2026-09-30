// @vitest-environment jsdom
import { act, cleanup, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  deadlineProgress,
  deadlineState,
  parseServerTimestamp,
  useDeadline,
} from '../../../src/features/game/deadline';

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
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

  it('normalizes Go RFC3339Nano timestamps before browser parsing', () => {
    const nativeParse = Date.parse;
    vi.spyOn(Date, 'parse').mockImplementation((value) => {
      if (/\.\d{4,}(?:Z|[+-]\d{2}:\d{2})$/.test(value)) return Number.NaN;
      return nativeParse(value);
    });

    expect(parseServerTimestamp('2026-09-30T12:00:03.123456789Z')).toBe(
      Date.UTC(2026, 8, 30, 12, 0, 3, 123),
    );
    expect(
      deadlineState('2026-09-30T12:00:03.123456789Z', Date.UTC(2026, 8, 30, 12, 0, 0, 223))
        .remainingSeconds,
    ).toBe(3);
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

  it('switches to a new server deadline immediately and cleans up the previous timer', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-30T12:00:00.000Z'));
    const timer = renderHook(({ deadline }) => useDeadline(deadline, 100), {
      initialProps: { deadline: '2026-09-30T12:00:01.000Z' },
    });

    expect(timer.result.current.remainingSeconds).toBe(1);
    expect(vi.getTimerCount()).toBe(1);

    timer.rerender({ deadline: '2026-09-30T12:00:10.000Z' });

    expect(timer.result.current.remainingSeconds).toBe(10);
    expect(vi.getTimerCount()).toBe(1);

    timer.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
