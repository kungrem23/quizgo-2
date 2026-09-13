// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { request, setAuthorization } from './client';

afterEach(() => {
  setAuthorization('');
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('session-aware API client', () => {
  it('uses the login token without duplicating its Bearer prefix', async () => {
    setAuthorization('Bearer test-token');
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ id: 42 }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await request('/api/me');
    const options = (fetchMock.mock.calls as unknown as [string, RequestInit][])[0][1];
    expect(new Headers(options.headers).get('Authorization')).toBe('Bearer test-token');
  });

  it('requests reauthentication on an expired session without touching the draft', async () => {
    sessionStorage.setItem('quizgo:draft:42:1', 'unsaved content');
    const dispatch = vi.spyOn(window, 'dispatchEvent');
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify({ error: 'invalid token' }), { status: 401 })),
    );
    await expect(request('/api/quizzes/1/author')).rejects.toMatchObject({ status: 401 });
    expect(dispatch).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'quizgo:session-expired' }),
    );
    expect(sessionStorage.getItem('quizgo:draft:42:1')).toBe('unsaved content');
    sessionStorage.clear();
  });

  it.each([403, 401])('does not end the session on an ownership error (%i)', async (status) => {
    const dispatch = vi.spyOn(window, 'dispatchEvent');
    const error = status === 401 ? 'you cant edit this question' : 'forbidden';
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify({ error }), { status })),
    );
    await expect(request('/api/questions/1')).rejects.toMatchObject({ status });
    expect(dispatch).not.toHaveBeenCalled();
  });
});
