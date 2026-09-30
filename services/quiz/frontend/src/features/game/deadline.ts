import { useEffect, useState } from 'react';

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
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    setNow(Date.now());
    if (!deadline || deadlineState(deadline).expired) return;
    const timer = window.setInterval(() => {
      const next = Date.now();
      setNow(next);
      if (deadlineState(deadline, next).expired) window.clearInterval(timer);
    }, refreshMs);
    return () => window.clearInterval(timer);
  }, [deadline, refreshMs]);

  return deadlineState(deadline, now);
}
