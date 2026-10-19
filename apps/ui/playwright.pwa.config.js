import { defineConfig } from "@playwright/test";
import base from "./playwright.config.js";
export default defineConfig({
  ...base,
  testMatch: "pwa.spec.js",
  timeout: 90000,
  use: {
    ...base.use,
    baseURL: "http://127.0.0.1:4176",
    serviceWorkers: "allow",
  },
  webServer: {
    command: "node browser/pwa-server.mjs",
    url: "http://127.0.0.1:4176",
    reuseExistingServer: false,
  },
});
