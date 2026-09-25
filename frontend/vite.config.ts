import { readFileSync } from "node:fs";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const appVersion = readFileSync(new URL("../version.txt", import.meta.url), "utf8").trim();

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  server: {
    proxy: {
      "/api": localAPIProxy(),
      "/health": localAPIProxy(),
    },
  },
});

function localAPIProxy() {
  return {
    target: "http://localhost:8080",
    changeOrigin: true,
    configure(proxy: {
      on: (
        event: string,
        listener: (request: { setHeader: (name: string, value: string) => void }) => void,
      ) => void;
    }) {
      proxy.on("proxyReq", (request) => request.setHeader("Origin", "http://localhost:8080"));
    },
  };
}
