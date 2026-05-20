import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 8080,
    hmr: true,
    proxy: {
      '/trade/api': {
        target: 'http://localhost:80',
        changeOrigin: true,
      },
      '/trade/ws': {
        target: 'ws://localhost:80',
        ws: true,
      },
    },
  },
})