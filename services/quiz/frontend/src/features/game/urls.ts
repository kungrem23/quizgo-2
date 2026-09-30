function configuredOrigin(value: string | undefined): string {
  const configured = value?.trim();
  if (!configured) return window.location.origin;

  try {
    return new URL(configured).origin;
  } catch {
    return window.location.origin;
  }
}

export function gameJoinURL(code: string): string {
  return new URL(
    `/join/${encodeURIComponent(code)}`,
    configuredOrigin(import.meta.env.VITE_GAME_ORIGIN),
  ).toString();
}

export function quizURL(path: string): string {
  return new URL(path, configuredOrigin(import.meta.env.VITE_QUIZ_ORIGIN)).toString();
}
