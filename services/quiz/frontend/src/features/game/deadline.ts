import { useEffect, useReducer } from 'react';

export interface DeadlineState {
  remainingMs: number;
  remainingSeconds: number;
  expired: boolean;
}

export function deadlineState(deadline: string | undefined, now = Date.now()): DeadlineState {
  const deadlineMs = deadline ? Date.parse(deadline) : Number.NaN;
  const remainingMs = Number.isFinite(deadlineMs) ? Math.max(0, deadlineMs - now) : 0;
  return {
    remainingMs,
    remainingSeconds: Math.ceil(remainingMs / 1000),
    expired: remainingMs === 0,
  };
}

export function deadlineProgress(remainingMs: number, totalSeconds: number): number {
  if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) return 0;
  return Math.min(100, Math.max(0, (remainingMs / (totalSeconds * 1000)) * 100));
}

export function useDeadline(deadline: string | undefined, refreshMs = 200): DeadlineState {
  const [, refresh] = useReducer((value: number) => value + 1, 0);
  const current = deadlineState(deadline);

  useEffect(() => {
    if (!deadline) return;
    let timer: number | undefined;
    const schedule = () => {
      const state = deadlineState(deadline);
      if (state.expired) return;
      timer = window.setTimeout(
        () => {
          refresh();
          schedule();
        },
        Math.min(refreshMs, state.remainingMs),
      );
    };
    schedule();
    return () => {
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [deadline, refreshMs]);

  return current;
}
