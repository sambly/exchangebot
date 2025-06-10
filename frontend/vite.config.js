import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';

export default defineConfig(({ mode }) => ({
  root: path.resolve(__dirname),
  base: "/trade/",

  plugins: [
    vue(), // Подключаем плагин Vue
  ],

  build: {
    outDir: path.resolve(__dirname, 'dist'),
    rollupOptions: {
      input: path.resolve(__dirname, 'index.html'),
    },
    sourcemap: mode === 'development', // Используем mode для включения sourcemap
  },
  resolve: {
    alias: {
      '~bootstrap': path.resolve(__dirname, 'node_modules/bootstrap'),
      '@': path.resolve(__dirname, 'src'), // Алиас для каталога src
    },
    extensions: ['.js', '.vue', '.json'], // Поддержка расширений файлов
  },
  server: {
    port: 8080,
    hot: true,
  },
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@import "~bootstrap/scss/bootstrap";` // Поддержка SCSS
      }
    }
  },
  optimizeDeps: {
    include: ['bootstrap'], // Оптимизация зависимостей
  }  
}));