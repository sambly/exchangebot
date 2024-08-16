// const path = require('path')
import path from 'path';

export default {
  root: path.resolve(__dirname, 'src'),
  base:"/trade/",
  build: {
    outDir: '../dist',
    rollupOptions: {
      input: {
        // Указываем входной файл для Rollup
        index: path.resolve(__dirname, 'src/index.html')
      }
    }
  },
  resolve: {
    alias: {
      '~bootstrap': path.resolve(__dirname, 'node_modules/bootstrap'),
    }
  },
  server: {
    port: 8080,
    hot: true
  }
}