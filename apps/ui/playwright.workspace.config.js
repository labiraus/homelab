import { defineConfig } from "@playwright/test";
import base from "./playwright.config.js";
const servedURL = process.env.WORLDBUILDER_BROWSER_URL;
if (servedURL) {
  const url = new URL(servedURL);
  if (
    url.protocol !== "http:" ||
    !["127.0.0.1", "localhost", "[::1]"].includes(url.hostname)
  )
    throw new Error("Workspace fixtures require an HTTP loopback URL");
}
export default defineConfig({
  ...base,
  testMatch: "workspace.spec.js",
  use: { ...base.use, baseURL: servedURL || "http://127.0.0.1:4175" },
  webServer: servedURL
    ? undefined
    : {
        command: "npm run dev -- --host 127.0.0.1 --port 4175 --strictPort",
        url: "http://127.0.0.1:4175",
        reuseExistingServer: false,
      },
});
