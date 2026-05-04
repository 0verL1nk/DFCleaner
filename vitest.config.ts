/// <reference types="vitest" />
import { defineConfig } from 'vitest/config'
import path from 'path'

const frontendDir = path.resolve(__dirname, 'frontend')

export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(frontendDir, 'src'),
      'react': path.resolve(frontendDir, 'node_modules/react'),
      'react-dom': path.resolve(frontendDir, 'node_modules/react-dom'),
      'react/jsx-runtime': path.resolve(frontendDir, 'node_modules/react/jsx-runtime'),
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./frontend/src/test/setup.ts'],
    include: ['frontend/src/**/*.test.{ts,tsx}'],
  },
})
