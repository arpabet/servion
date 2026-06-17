import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  base: './', // Ensures correct asset paths for static deployment
  build: {
    outDir: '../assets', // Output directory
    emptyOutDir: true, // Clean stale hashed assets before each build (outDir is outside project root)
  },
})