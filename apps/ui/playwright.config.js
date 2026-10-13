import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./browser",
  testMatch: "worldbuilder.spec.js",
  workers: 1,
  timeout: 45000,
  outputDir:
    process.env.WORLDBUILDER_BROWSER_ARTIFACTS ||
    "/tmp/worldbuilder-browser-artifacts",
  reporter: "list",
  use: {
    baseURL: process.env.WORLDBUILDER_BROWSER_URL,
    headless: true,
    trace: "retain-on-failure",
  },
});
