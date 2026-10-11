const assert = require("node:assert/strict");
const { chromium } = require("../../ui/node_modules/playwright");

(async () => {
  const address = new URL(process.env.MCP_OAUTH_BROWSER_URL);
  const callback = new URL(process.env.MCP_OAUTH_BROWSER_CALLBACK);
  assert.equal(address.hostname, "127.0.0.1");
  assert.equal(callback.hostname, "127.0.0.1");
  const browser = await chromium.launch();
  try {
    // TLS exemption applies only to the owned httptest loopback certificate.
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    context.setDefaultTimeout(10000);
    context.setDefaultNavigationTimeout(10000);
    await context.route("**/*", (route) => {
      const origin = new URL(route.request().url()).origin;
      return [address.origin, callback.origin].includes(origin)
        ? route.continue() : route.abort();
    });
    const page = await context.newPage();
    page.on("console", message => { if (message.type() === "error") console.error(message.text().split("?")[0]); });
    for (const decision of ["Allow connection", "Cancel"]) {
      assert.equal((await page.goto(address.href)).status(), 200);
      const form = { request: await page.locator('input[name="request"]').inputValue(), decision: "allow" };
      for (const origin of ["null", "https://foreign.example"]) {
        const refused = await context.request.post(address.origin + address.pathname, { form, headers: { Origin: origin } });
        assert.equal(refused.status(), 403, "untrusted Origin must remain refused");
      }
      const sent = page.waitForRequest(r => r.method() === "POST");
      const response = page.waitForResponse(r => r.request().method() === "POST");
      await page.getByRole("button", { name: decision, exact: true }).click();
      assert.equal((await sent).headers().origin, address.origin, "browser must preserve the consent Origin");
      assert.equal((await response).status(), 303, "real form/CSRF submission must succeed");
      await page.waitForURL(u => u.origin === callback.origin && u.pathname === callback.pathname);
      await page.getByText("Returned to fixture client", { exact: true }).waitFor({ state: "attached" });
      assert.equal((await page.locator("body").textContent()).trim(), "Returned to fixture client");
      const result = new URL(page.url()).searchParams;
      assert.equal(result.get("state"), "browser-state");
      assert.equal(result.get("iss"), address.origin);
      assert.equal(result.has("code"), decision === "Allow connection");
      assert.equal(result.get("error"), decision === "Cancel" ? "access_denied" : null);
    }
    console.log("PASS: Chromium consent, secure cookie, allow/cancel and loopback return");
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });
