import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import sveltePreprocess from 'svelte-preprocess'

export default defineConfig({
  plugins: [svelte({
    preprocess: sveltePreprocess({
      scss: {
        api: 'modern-compiler',
        silenceDeprecations: ['legacy-js-api']
      }
    })
  })],
  esbuild: {
    legalComments: 'none'
  },
  css: {
    preprocessorOptions: {
      scss: {
        api: 'modern-compiler', // Fix Sass deprecation warning
        silenceDeprecations: ['legacy-js-api'] // Temporary fallback
      }
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    // Ensure all assets are bundled (no external CDN)
    rollupOptions: {
      external: []
    }
  },
  server: {
    // Development: proxy API calls to Go server
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})