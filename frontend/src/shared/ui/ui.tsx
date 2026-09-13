import {
  createContext,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type ReactNode,
} from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { AlertCircle, Check, LoaderCircle, X, Zap } from 'lucide-react';
import { Link } from 'react-router';

export function Button({
  variant = 'primary',
  busy,
  children,
  className = '',
  disabled,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  busy?: boolean;
}) {
  return (
    <button
      type="button"
      {...props}
      disabled={disabled || busy}
      className={`button button-${variant} ${className}`}
      aria-busy={busy || undefined}
    >
      {busy && <LoaderCircle size={17} className="spin" />}
      {children}
    </button>
  );
}
export function Brand({ light = false }: { light?: boolean }) {
  return (
    <Link
      to="/quizzes"
      className={`brand ${light ? 'brand-light' : ''}`}
      aria-label="QuizGo — мои квизы"
    >
      <Zap fill="currentColor" strokeWidth={1.6} />
      <span>
        QuizGo<span className="brand-dot">.</span>
      </span>
    </Link>
  );
}
export function Modal({
  open,
  onOpenChange,
  title,
  description,
  children,
  className = '',
  dismissible = true,
}: {
  open: boolean;
  onOpenChange: (value: boolean) => void;
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
  dismissible?: boolean;
}) {
  const descriptionId = useId();
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(v) => {
        if (dismissible || v) onOpenChange(v);
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="modal-overlay" />
        <Dialog.Content
          className={`modal ${className}`}
          aria-describedby={description ? descriptionId : undefined}
          onEscapeKeyDown={(e) => {
            if (!dismissible) e.preventDefault();
          }}
          onPointerDownOutside={(e) => {
            if (!dismissible) e.preventDefault();
          }}
        >
          <Dialog.Title className="modal-title">{title}</Dialog.Title>
          {description && (
            <Dialog.Description id={descriptionId} className="modal-description">
              {description}
            </Dialog.Description>
          )}
          {dismissible && (
            <Dialog.Close className="icon-button modal-close" aria-label="Закрыть">
              <X size={20} />
            </Dialog.Close>
          )}
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
export function ErrorBox({ message, children }: { message: string; children?: ReactNode }) {
  return (
    <div className="error-box" role="alert">
      <AlertCircle size={18} />
      <div>
        {message}
        {children}
      </div>
    </div>
  );
}
export function Loading({ label = 'Загружаем…' }: { label?: string }) {
  return (
    <div className="loading" role="status">
      <LoaderCircle className="spin" size={25} />
      <span>{label}</span>
    </div>
  );
}
const ToastContext = createContext<(message: string) => void>(() => {});
export function ToastProvider({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState('');
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );
  function toast(value: string) {
    setMessage(value);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setMessage(''), 4200);
  }
  return (
    <ToastContext.Provider value={toast}>
      {children}
      <div className={`toast ${message ? 'toast-visible' : ''}`} role="status">
        {message && (
          <>
            <Check size={18} />
            {message}
          </>
        )}
      </div>
    </ToastContext.Provider>
  );
}
export const useToast = () => useContext(ToastContext);
