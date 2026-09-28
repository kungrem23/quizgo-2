import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const quizTarget = env.QUIZ_API_PROXY || 'http://127.0.0.1:8080';
  const gameTarget = env.QUIZ_GAME_PROXY || 'http://127.0.0.1:8081';
  return {
    plugins: [react(), tailwindcss()],
    server: {
      proxy: {
        '/api/games': { target: gameTarget },
        '/ws': { target: gameTarget, ws: true },
        '/api': { target: quizTarget },
        '/auth': { target: quizTarget },
      },
    },
  };
});
