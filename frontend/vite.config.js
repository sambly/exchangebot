import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';

export default defineConfig(({ mode }) => ({
  root: path.resolve(__dirname),
  base: "/trade/",

  plugins: [
    vue(),
  ],

  build: {
    outDir: path.resolve(__dirname, 'dist'),
    rollupOptions: {
      input: path.resolve(__dirname, 'index.html'),
    },
    sourcemap: mode === 'development',
  },

  server: {
    port: 8080,
    hot: true,
    proxy: {
      // API-запросы проксируются на Go backend (порт 80)
      '/trade/api': {
        target: 'http://localhost:80',
        changeOrigin: true,
      },
      // WebSocket проксируется на Go backend
      '/trade/ws': {
        target: 'ws://localhost:80',
        ws: true,
      },
    },
  },

  resolve: {
    alias: {
      '~bootstrap': path.resolve(__dirname, 'node_modules/bootstrap'),
      '@': path.resolve(__dirname, 'src'),
    },
    extensions: ['.js', '.vue', '.json'],
  },

  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@import "~bootstrap/scss/bootstrap";`
      }
    }
  },

  optimizeDeps: {
    include: ['bootstrap'],
  },
}));