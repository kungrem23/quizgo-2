import type { ReactNode } from 'react';
import * as Dropdown from '@radix-ui/react-dropdown-menu';
import { ChevronDown, LogOut, ArrowLeft, Sparkles } from 'lucide-react';
import { Link, NavLink } from 'react-router';
import { Brand } from './ui';
import { useAuth } from '../../features/auth/AuthProvider';
export function SiteHeader({
  editor = false,
  children,
}: {
  editor?: boolean;
  children?: ReactNode;
}) {
  const { user, signOut } = useAuth();
  return (
    <header className={`site-header ${editor ? 'editor-header' : ''}`}>
      <div className="header-inner">
        <Brand />
        <span className="header-divider" />
        {editor ? (
          <Link to="/quizzes" className="back-link">
            <ArrowLeft size={15} />
            <span>Мои квизы</span>
          </Link>
        ) : (
          <nav className="main-nav" aria-label="Основная навигация">
            <NavLink to="/quizzes">Мои квизы</NavLink>
            <span className="nav-soon">
              Открытия <span>Скоро</span>
            </span>
          </nav>
        )}
        {children}
        <div className="header-account">
          {user ? (
            <Dropdown.Root>
              <Dropdown.Trigger className="account-button">
                <span className="avatar">{user.username[0].toUpperCase()}</span>
                <span className="account-name">{user.username}</span>
                <ChevronDown size={14} />
              </Dropdown.Trigger>
              <Dropdown.Portal>
                <Dropdown.Content className="dropdown" align="end" sideOffset={12}>
                  <Dropdown.Label className="dropdown-label">Ваш аккаунт</Dropdown.Label>
                  <Dropdown.Item className="dropdown-item" onSelect={signOut}>
                    <LogOut size={16} />
                    Выйти
                  </Dropdown.Item>
                </Dropdown.Content>
              </Dropdown.Portal>
            </Dropdown.Root>
          ) : (
            <Link className="button button-primary" to="/login">
              Войти
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
export function Footer() {
  return (
    <footer className="site-footer">
      <span>© {new Date().getFullYear()} QuizGo</span>
      <span>
        <Sparkles size={13} />
        Для любопытных умов
      </span>
    </footer>
  );
}
