import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      strategies: "injectManifest",
      srcDir: "src",
      filename: "sw.js",
      registerType: "prompt",
      injectRegister: false,
      includeAssets: ["worldbuilder-192.png", "worldbuilder-512.png"],
      manifest: {
        id: "/",
        name: "Worldbuilder",
        short_name: "Worldbuilder",
        description: "An editorial studio for shared worlds and stories.",
        start_url: "/#/worlds",
        scope: "/",
        display: "standalone",
        theme_color: "#f6f3ed",
        background_color: "#f6f3ed",
        icons: [
          {
            src: "/worldbuilder-192.png",
            sizes: "192x192",
            type: "image/png",
            purpose: "any",
          },
          {
            src: "/worldbuilder-512.png",
            sizes: "512x512",
            type: "image/png",
            purpose: "any maskable",
          },
        ],
      },
      injectManifest: {
        globPatterns: [
          "index.html",
          "assets/*.{js,css,woff2}",
          "worldbuilder-*.png",
        ],
        maximumFileSizeToCacheInBytes: 4000000,
      },
    }),
  ],
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
