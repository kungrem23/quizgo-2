import { lazy, Suspense } from 'react';
import {
  createBrowserRouter,
  Navigate,
  Outlet,
  RouterProvider,
  useLocation,
  useRouteError,
  Link,
} from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider, useAuth } from '../features/auth/AuthProvider';
import { Loading, ToastProvider } from '../shared/ui/ui';
import { AuthPage } from '../pages/AuthPage';
import { QuizzesPage } from '../pages/QuizzesPage';
const EditorPage = lazy(() =>
  import('../pages/EditorPage').then((m) => ({ default: m.EditorPage })),
);
const PreviewPage = lazy(() =>
  import('../pages/PreviewPage').then((m) => ({ default: m.PreviewPage })),
);
import { ApiError } from '../shared/api/client';
const client = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (count, error) =>
        !(error instanceof ApiError && [400, 401, 403, 404, 409].includes(error.status)) &&
        count < 1,
    },
    mutations: { retry: false },
  },
});
function Protected() {
  const { user } = useAuth();
  const location = useLocation();
  return user ? (
    <Outlet />
  ) : (
    <Navigate
      to={`/login?next=${encodeURIComponent(location.pathname + location.search)}`}
      replace
    />
  );
}
function Root() {
  return (
    <AuthProvider>
      <ToastProvider>
        <Suspense fallback={<Loading />}>
          <Outlet />
        </Suspense>
      </ToastProvider>
    </AuthProvider>
  );
}
function NotFound() {
  return (
    <main className="not-found">
      <span>404</span>
      <h1>Кажется, этот вопрос без ответа.</h1>
      <p>Такой страницы нет. Вернёмся к вашим идеям?</p>
      <Link className="button button-primary" to="/quizzes">
        К моим квизам
      </Link>
    </main>
  );
}
function AppError() {
  const error = useRouteError();
  console.error(error);
  return (
    <main className="not-found">
      <h1>Не удалось открыть страницу</h1>
      <p>Попробуйте обновить страницу. Черновик останется в этой вкладке.</p>
      <a className="button button-primary" href={window.location.pathname}>
        Обновить
      </a>
    </main>
  );
}
const router = createBrowserRouter([
  {
    element: <Root />,
    errorElement: <AppError />,
    children: [
      { path: '/', element: <Navigate to="/quizzes" replace /> },
      { path: '/login', element: <AuthPage /> },
      { path: '/register', element: <AuthPage register /> },
      { path: '/quizzes/:quizId', element: <PreviewPage /> },
      {
        element: <Protected />,
        children: [
          { path: '/quizzes', element: <QuizzesPage /> },
          { path: '/quizzes/:quizId/edit', element: <EditorPage /> },
        ],
      },
      { path: '*', element: <NotFound /> },
    ],
  },
]);
export function App() {
  return (
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
