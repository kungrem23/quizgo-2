import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const target = env.QUIZ_API_PROXY || 'http://127.0.0.1:8080';
  return {
    plugins: [react(), tailwindcss()],
    server: { proxy: { '/api': { target }, '/auth': { target } } },
  };
});
