import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 前端构建产物输出到 web/dist，由 Go 通过 go:embed 内嵌进面板二进制。
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 800,
    rollupOptions: {
      output: {
        // 把体积较大的第三方库单独拆分，改善缓存与首屏加载。
        manualChunks: {
          echarts: ['echarts/core', 'echarts/charts', 'echarts/components', 'echarts/renderers'],
          vue: ['vue', 'vue-router'],
        },
      },
    },
  },
  server: {
    port: 5173,
    // 开发期把 API 与 WebSocket 代理到本地面板（默认 :8008）。
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8008',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
