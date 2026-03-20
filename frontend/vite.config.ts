import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'

const defaultDevPort = 5218

function resolveDevPort() {
  const raw = Number.parseInt(process.env.FRONTEND_PORT || '', 10)
  if (Number.isInteger(raw) && raw > 0 && raw <= 65535) {
    return raw
  }
  return defaultDevPort
}

const devPort = resolveDevPort()

export default defineConfig({
  plugins: [react()],
  server: {
    port: devPort,
    strictPort: true,
    host: '127.0.0.1',
    cors: true,
    hmr: {
      host: '127.0.0.1',
      protocol: 'ws',
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
        },
      },
    },
  },
})

