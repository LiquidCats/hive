import { fileURLToPath, URL } from 'node:url'
import path from 'node:path'

import { defineConfig, type ProxyOptions } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'
import tailwindcss from '@tailwindcss/vite'

// /auth/sign-in and /users are also SPA routes: page loads stay with vite, fetches go to the BFF.
const bff: ProxyOptions = {
  target: process.env.BFF_URL ?? 'http://localhost:8080',
  changeOrigin: true,
  bypass: (req) => (req.headers.accept?.includes('text/html') ? req.url : undefined),
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss(), vueDevTools()],
  // Same-origin BFF in dev: the session cookie is SameSite=Strict and no CORS is needed.
  server: {
    allowedHosts: ['web.hive.orb.local'],
    proxy: Object.fromEntries(['/webauthn', '/auth', '/users'].map((prefix) => [prefix, bff])),
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
