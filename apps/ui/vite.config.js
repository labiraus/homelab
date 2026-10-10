import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 3000,
  },
  test: {
    include: ["src/**/*.test.{js,jsx}"],
    environment: "jsdom",
    globals: true,
    setupFiles: "./src/test/setup.js",
  },
});
