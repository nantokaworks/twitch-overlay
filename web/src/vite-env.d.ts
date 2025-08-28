/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_HOST: string
  readonly VITE_API_PORT: string
  readonly VITE_SSE_HOST: string
  readonly VITE_SSE_PORT: string
  readonly DEV: string
  readonly MODE: string
  readonly BASE_URL: string
  readonly PROD: string
  readonly SSR: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}