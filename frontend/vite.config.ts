import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: '/trade/',
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('lightweight-charts')) return 'vendor-charts'
          if (id.includes('node_modules/primevue') || id.includes('node_modules/@primeuix') || id.includes('node_modules/primeicons')) return 'vendor-primevue'
          if (id.includes('node_modules/vue') || id.includes('node_modules/pinia') || id.includes('node_modules/@vue')) return 'vendor-vue'
        },
      },
    },
  },
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