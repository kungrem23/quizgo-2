import { useState, type FormEvent } from 'react';
import { Eye, EyeOff, LockKeyhole, UserRound, ArrowRight } from 'lucide-react';
import { api, ApiError, errorMessage } from '../../shared/api/client';
import { Button, ErrorBox } from '../../shared/ui/ui';
import { useAuth } from './AuthProvider';
export function AuthForm({
  register = false,
  onSuccess,
  lockedUsername,
}: {
  register?: boolean;
  onSuccess: () => void;
  lockedUsername?: string;
}) {
  const { signIn } = useAuth();
  const [username, setUsername] = useState(lockedUsername || '');
  const [password, setPassword] = useState('');
  const [show, setShow] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  async function submit(e: FormEvent) {
    e.preventDefault();
    setError('');
    if (!/^[a-zA-Z0-9._\-#$!]{3,40}$/.test(username.trim())) {
      setError('Имя: 3–40 символов. Используйте латинские буквы, цифры или . _ - # $ !');
      return;
    }
    if (
      password.trim().length < 3 ||
      password.trim().length > 40 ||
      /[^\x21-\x7E]/.test(password.trim()) ||
      password.includes('`')
    ) {
      setError('Пароль: 3–40 символов. Используйте латинские буквы, цифры и знаки без пробелов.');
      return;
    }
    setPending(true);
    try {
      if (register) await api.register(username.trim(), password);
      await signIn(username.trim(), password);
      onSuccess();
    } catch (e) {
      setError(
        e instanceof ApiError && e.fields.username === 'already exists'
          ? 'Это имя уже занято. Попробуйте другое.'
          : e instanceof ApiError && e.status === 401
            ? 'Неверное имя пользователя или пароль.'
            : errorMessage(e),
      );
    } finally {
      setPending(false);
    }
  }
  return (
    <form onSubmit={submit} className="auth-form">
      <label className="field-label" htmlFor={lockedUsername ? 'renew-username' : 'username'}>
        Имя пользователя
      </label>
      <div className="input-icon">
        <UserRound size={18} />
        <input
          id={lockedUsername ? 'renew-username' : 'username'}
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="Например, alex"
          autoComplete="username"
          required
          minLength={3}
          maxLength={40}
          readOnly={!!lockedUsername}
          disabled={pending}
        />
      </div>
      <label className="field-label" htmlFor={lockedUsername ? 'renew-password' : 'password'}>
        Пароль
      </label>
      <div className="input-icon">
        <LockKeyhole size={18} />
        <input
          id={lockedUsername ? 'renew-password' : 'password'}
          type={show ? 'text' : 'password'}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={register ? 'Придумайте пароль' : 'Введите пароль'}
          autoComplete={register ? 'new-password' : 'current-password'}
          minLength={3}
          maxLength={40}
          required
          disabled={pending}
        />
        <button
          type="button"
          className="icon-button"
          onClick={() => setShow(!show)}
          aria-label={show ? 'Скрыть пароль' : 'Показать пароль'}
        >
          {show ? <EyeOff size={18} /> : <Eye size={18} />}
        </button>
      </div>
      {error && <ErrorBox message={error} />}
      <Button type="submit" busy={pending} className="auth-submit">
        {register ? 'Создать аккаунт' : 'Войти'}
        <ArrowRight size={18} />
      </Button>
    </form>
  );
}
