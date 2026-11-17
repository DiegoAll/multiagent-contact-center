import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// En desarrollo, Vite hace de proxy hacia la API e inyecta la X-API-Key
// (en compose lo hace nginx, así la clave nunca llega al navegador).
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  return {
    plugins: [react(), tailwindcss()],
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: env.API_URL || 'http://localhost:8080',
          changeOrigin: true,
          headers: env.API_KEY ? { 'X-API-Key': env.API_KEY } : {},
        },
      },
    },
  }
})
