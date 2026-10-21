import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { defineConfig } from "vitest/config";

const proxyTarget = process.env.VITE_API_PROXY_TARGET || "http://localhost:8080";

// emptyOutDir wipes dist/, but the Go binary embeds web/dist and needs the
// tracked placeholder to exist in fresh checkouts (see embed.go).
const keepDist = {
  name: "keep-dist-placeholder",
  closeBundle() {
    writeFileSync(resolve(__dirname, "dist/.gitkeep"), "");
  },
};

export default defineConfig({
  plugins: [react(), tailwindcss(), keepDist],
  server: {
    host: true,
    port: 5173,
    proxy: {
      "/api": {
        target: proxyTarget,
        changeOrigin: true,
      },
      "/healthz": {
        target: proxyTarget,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: false,
    restoreMocks: true,
  },
});
