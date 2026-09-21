import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  envDir: '..',
  envPrefix: ['VITE_', 'SECTILE_PRESET_'],
  plugins: [react(), tailwindcss()],
  build: {
    // La compilation sort dans le paquet Go qui l'embarque : go:embed ne sait
    // pas remonter au dessus de son propre paquet, et un binaire qui se suffit
    // à lui même est ce qui sépare un programme qu'on distribue d'un programme
    // qu'on installe.
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8090',
        changeOrigin: true,
      },
      '/auth': {
        target: 'http://localhost:8090',
        changeOrigin: true,
      },
    },
  },
})
