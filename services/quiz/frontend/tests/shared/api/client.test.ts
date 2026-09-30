// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, request, setAuthorization } from '../../../src/shared/api/client';

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

  it('creates a game through the authenticated game-service route', async () => {
    setAuthorization('test-token');
    const created = {
      id: 'game-1',
      code: 'ABC123',
      phase: 'lobby' as const,
      quiz_revision: 3,
      host_ticket: 'host-secret',
    };
    const fetchMock = vi.fn(async () => new Response(JSON.stringify(created), { status: 201 }));
    vi.stubGlobal('fetch', fetchMock);
    await expect(api.createGame(42)).resolves.toEqual(created);
    const [path, options] = (fetchMock.mock.calls as unknown as [string, RequestInit][])[0];
    expect(path).toBe('/api/games');
    expect(options.method).toBe('POST');
    expect(options.body).toBe('{"quiz_id":42}');
    expect(new Headers(options.headers).get('Authorization')).toBe('Bearer test-token');
  });
});
