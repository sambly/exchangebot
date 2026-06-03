import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: '/trade/v2/',
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 8080,
    hmr: {
      port: 8081 
    },
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