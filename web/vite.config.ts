import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev server proxies API calls to a locally running arraydeck backend.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8417",
        changeOrigin: false,
      },
      "/healthz": "http://localhost:8417",
    },
  },
  build: {
    outDir: "dist",
    sourcemap: false,
  },
});
