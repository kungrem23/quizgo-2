function configuredGameOrigin(): string {
  const configured = import.meta.env.VITE_GAME_ORIGIN?.trim();
  if (!configured) return window.location.origin;

  try {
    return new URL(configured).origin;
  } catch {
    return window.location.origin;
  }
}

export function gameJoinURL(code: string): string {
  return new URL(`/join/${encodeURIComponent(code)}`, configuredGameOrigin()).toString();
}
