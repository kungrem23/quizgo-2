import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, setAuthorization } from '../../shared/api/client';
import type { User } from '../../shared/api/types';
import { Modal } from '../../shared/ui/ui';
import { AuthForm } from './AuthForm';
const SESSION = 'quizgo:session';
interface Session {
  token: string;
  user: User;
}
function readSession(): Session | null {
  try {
    const s = JSON.parse(sessionStorage.getItem(SESSION) || 'null');
    return s &&
      typeof s.token === 'string' &&
      Number.isInteger(s.user?.id) &&
      typeof s.user.username === 'string'
      ? s
      : null;
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
  const [session, setSession] = useState<Session | null>(() => {
    const s = readSession();
    setAuthorization(s?.token || '');
    return s;
  });
  const [expired, setExpired] = useState(false);
  useEffect(() => {
    const handler = () => setExpired(true);
    window.addEventListener('quizgo:session-expired', handler);
    return () => window.removeEventListener('quizgo:session-expired', handler);
  }, []);
  async function signIn(username: string, password: string) {
    const { token } = await api.login(username, password);
    setAuthorization(token);
    let user: User;
    try {
      user = await api.me();
    } catch (e) {
      setAuthorization(session?.token || '');
      throw e;
    }
    const next = { token, user };
    storeSession(next);
    setSession(next);
    setExpired(false);
    if (session && session.user.id !== user.id) {
      await query.cancelQueries();
      query.clear();
    } else await query.invalidateQueries({ queryKey: ['private'] });
  }
  function signOut() {
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
      {children}
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
