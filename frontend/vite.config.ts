import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
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
    configure(proxy: { on: (event: string, listener: (request: { setHeader: (name: string, value: string) => void }) => void) => void }) {
      proxy.on("proxyReq", request => request.setHeader("Origin", "http://localhost:8080"));
    },
  };
}
