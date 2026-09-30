import { useEffect, useReducer } from 'react';

export interface DeadlineState {
  remainingMs: number;
  remainingSeconds: number;
  expired: boolean;
}

const goRFC3339Timestamp = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:\d{2})$/;

/**
 * Go serializes time.Time as RFC3339Nano, while ECMAScript's standard date
 * interchange format only guarantees three fractional-second digits. Normalize
 * the server value to milliseconds before handing it to Date.parse so the
 * result does not depend on browser-specific parsing extensions.
 */
export function parseServerTimestamp(timestamp: string | undefined): number {
  if (!timestamp) return Number.NaN;
  const parts = goRFC3339Timestamp.exec(timestamp);
  if (!parts) return Number.NaN;
  const milliseconds = (parts[2] ?? '').padEnd(3, '0').slice(0, 3);
  return Date.parse(`${parts[1]}.${milliseconds}${parts[3]}`);
}

export function serverClockOffset(
  serverTime: string | undefined,
  clientReceivedAtMs: number,
): number | undefined {
  const serverTimeMs = parseServerTimestamp(serverTime);
  return Number.isFinite(serverTimeMs) && Number.isFinite(clientReceivedAtMs)
    ? serverTimeMs - clientReceivedAtMs
    : undefined;
}

export function deadlineState(
  deadline: string | undefined,
  now = Date.now(),
  serverTimeOffsetMs = 0,
): DeadlineState {
  const deadlineMs = parseServerTimestamp(deadline);
  const serverNow = now + serverTimeOffsetMs;
  const remainingMs = Number.isFinite(deadlineMs) ? Math.max(0, deadlineMs - serverNow) : 0;
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

export function useDeadline(
  deadline: string | undefined,
  serverTimeOffsetMs = 0,
  refreshMs = 200,
): DeadlineState {
  const [, refresh] = useReducer((value: number) => value + 1, 0);
  const current = deadlineState(deadline, Date.now(), serverTimeOffsetMs);

  useEffect(() => {
    if (!deadline) return;
    let timer: number | undefined;
    const schedule = () => {
      const state = deadlineState(deadline, Date.now(), serverTimeOffsetMs);
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
  }, [deadline, refreshMs, serverTimeOffsetMs]);

  return current;
}
