import { Link, Navigate, useNavigate, useSearchParams } from 'react-router';
import { Sparkles, Star, Zap } from 'lucide-react';
import { Brand } from '../shared/ui/ui';
import { AuthForm } from '../features/auth/AuthForm';
import { useAuth } from '../features/auth/AuthProvider';
export function AuthPage({ register = false }: { register?: boolean }) {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const { user } = useAuth();
  const target = params.get('next');
  const next =
    target?.startsWith('/') &&
    !target.startsWith('//') &&
    !target.startsWith('/login') &&
    !target.startsWith('/register')
      ? target
      : '/quizzes';
  if (user) return <Navigate to={next} replace />;
  return (
    <main className="auth-page">
      <div className="auth-card">
        <section className="auth-story">
          <Brand light />
          <div className="story-copy">
            <span className="eyebrow light">
              <span />
              МАЛЕНЬКИЙ КВИЗ. БОЛЬШОЕ ОТКРЫТИЕ.
            </span>
            <h1>
              Учиться.
              <br />
              Играть.
              <br />
              <span>Удивляться.</span>
            </h1>
            <p>
              Превращайте любопытство в знания.
              <br />
              Один вопрос за раз.
            </p>
          </div>
          <div className="mascot-scene" aria-hidden="true">
            <span className="orbit orbit-one" />
            <span className="orbit orbit-two" />
            <Star className="scene-star star-one" fill="currentColor" />
            <Sparkles className="scene-spark" />
            <span className="scene-dot dot-one" />
            <span className="scene-dot dot-two" />
            <div className="question-tile">?</div>
            <div className="mascot">
              <Zap className="mascot-body" fill="currentColor" strokeWidth={1.2} />
              <div className="mascot-face">
                <i />
                <i />
                <b />
              </div>
            </div>
            <div className="pencil" />
            <Star className="scene-star star-two" fill="currentColor" />
            <span className="scene-scribble">✦</span>
          </div>
          <div className="story-footer">
            <span>
              Хорошие вопросы.
              <br />
              Новые открытия.
            </span>
            <span className="tiny-stars">✦ ✦ ✦</span>
          </div>
        </section>
        <section className="auth-content">
          <div className="auth-mobile-brand">
            <Brand />
          </div>
          <span className="eyebrow">ВАШЕ СЛЕДУЮЩЕЕ ОТКРЫТИЕ — ЗДЕСЬ</span>
          <h2>{register ? 'Начнём знакомство!' : 'С возвращением!'}</h2>
          <p className="auth-subtitle">
            {register
              ? 'Создайте аккаунт и соберите свой первый квиз.'
              : 'Войдите, чтобы воплотить ваши идеи в квизах.'}
          </p>
          <AuthForm
            key={String(register)}
            register={register}
            onSuccess={() => navigate(next, { replace: true })}
          />
          <p className="auth-switch">
            {register ? 'Уже есть аккаунт?' : 'Пока нет аккаунта?'}{' '}
            <Link
              to={`${register ? '/login' : '/register'}${target ? `?next=${encodeURIComponent(next)}` : ''}`}
            >
              {register ? 'Войти' : 'Зарегистрироваться'}
            </Link>
          </p>
          <div className="auth-note">
            <span className="mini-bolt">
              <Zap size={14} />
            </span>
            Создавайте. Делитесь. Вдохновляйте.
          </div>
        </section>
      </div>
      <p className="auth-bottom">Сделано для любопытных умов.</p>
    </main>
  );
}
