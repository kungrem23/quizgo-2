import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, refreshAuthorization, setAuthorization } from '../../shared/api/client';
import type { User } from '../../shared/api/types';
import { Modal } from '../../shared/ui/ui';
import { AuthForm } from './AuthForm';
const SESSION = 'quizgo:session';
interface Session {
  user: User;
}
function readSession(): Session | null {
  try {
    const s = JSON.parse(sessionStorage.getItem(SESSION) || 'null');
    if (!s || !Number.isInteger(s.user?.id) || typeof s.user.username !== 'string') return null;
    const session = { user: s.user as User };
    sessionStorage.setItem(SESSION, JSON.stringify(session));
    return session;
  } catch {
    return null;
  }
}
function storeSession(session: Session | null) {
  try {
    if (session) sessionStorage.setItem(SESSION, JSON.stringify(session));
    else sessionStorage.removeItem(SESSION);
  } catch {
    /* In-memory sessions still work when browser storage is unavailable. */
  }
}
interface Auth {
  user: User | null;
  expired: boolean;
  signIn: (username: string, password: string) => Promise<void>;
  signOut: () => void;
}
const AuthContext = createContext<Auth | null>(null);
export function AuthProvider({ children }: { children: ReactNode }) {
  const query = useQueryClient();
  const [session, setSession] = useState<Session | null>(() => readSession());
  const [expired, setExpired] = useState(false);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    const handler = () => setExpired(true);
    window.addEventListener('quizgo:session-expired', handler);
    return () => window.removeEventListener('quizgo:session-expired', handler);
  }, []);
  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        await refreshAuthorization();
        const user = await api.me();
        if (!active) return;
        const restored = { user };
        storeSession(restored);
        setSession(restored);
        setExpired(false);
      } catch {
        setAuthorization('');
        if (active && readSession()) setExpired(true);
      } finally {
        if (active) setReady(true);
      }
    })();
    return () => {
      active = false;
    };
  }, []);
  async function signIn(username: string, password: string) {
    const { token } = await api.login(username, password);
    setAuthorization(token);
    let user: User;
    try {
      user = await api.me();
    } catch (e) {
      setAuthorization('');
      throw e;
    }
    const next = { user };
    storeSession(next);
    setSession(next);
    setExpired(false);
    if (session && session.user.id !== user.id) {
      await query.cancelQueries();
      query.clear();
    } else await query.invalidateQueries({ queryKey: ['private'] });
  }
  function signOut() {
    void api.logout().catch(() => {});
    void query.cancelQueries();
    query.clear();
    storeSession(null);
    setAuthorization('');
    setSession(null);
    setExpired(false);
    try {
      Object.keys(sessionStorage)
        .filter((k) => k.startsWith(`quizgo:draft:${session?.user.id}:`))
        .forEach((k) => sessionStorage.removeItem(k));
    } catch {
      /* optional local drafts */
    }
  }
  return (
    <AuthContext.Provider value={{ user: session?.user || null, expired, signIn, signOut }}>
      {ready ? children : null}
      <Modal
        open={expired && !!session}
        onOpenChange={() => {}}
        dismissible={false}
        title="Продолжим с того же места"
        description="Время сессии истекло. Войдите снова — ваши правки останутся в редакторе."
      >
        <AuthForm lockedUsername={session?.user.username} onSuccess={() => setExpired(false)} />
      </Modal>
    </AuthContext.Provider>
  );
}
export function useAuth() {
  const v = useContext(AuthContext);
  if (!v) throw new Error('AuthProvider missing');
  return v;
}
