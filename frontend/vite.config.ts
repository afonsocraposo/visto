import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const versionFile = [
  resolve(process.cwd(), "../version.txt"),
  resolve(process.cwd(), "version.txt"),
].find(existsSync);
if (!versionFile) throw new Error("version.txt is missing from the build context");
const appVersion = readFileSync(versionFile, "utf8").trim();

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
