import AxeBuilder from "@axe-core/playwright";
import { test, expect } from "@playwright/test";
async function device(page) {
  await page
    .getByRole("button", { name: "Authentication menu" })
    .first()
    .click();
  await page.getByText(/App and offline writing/, { exact: false }).click();
}
async function navigate(page, name) {
  const nav = page.getByRole("navigation", { name: "World" });
  if (!(await nav.getByRole("link", { name, exact: true }).isVisible())) {
    await page.getByRole("button", { name: "World navigation" }).click();
  }
  await nav.getByRole("link", { name, exact: true }).click();
}
test("production service worker, allowlist, offline restart, saved draft and user-controlled update", async ({
  page,
  context,
}) => {
  await page.goto("/#/worlds/pwa-world/workspace");
  await expect(page.getByLabel("Question or creative direction")).toBeVisible();
  await page.evaluate(() => navigator.serviceWorker.ready);
  await page.reload();
  await page.waitForFunction(() => !!navigator.serviceWorker.controller);
  const manifest = await (
    await page.request.get("/manifest.webmanifest")
  ).json();
  expect(manifest.display).toBe("standalone");
  expect(manifest.name).toBe("Worldbuilder");
  await device(page);
  await page
    .getByRole("button", { name: "Enable offline writing on this device" })
    .click();
  await page
    .getByRole("button", { name: "Download this timeline and private stories" })
    .click();
  await expect(page.getByText(/Downloaded 4 records/)).toBeVisible();
  await page.keyboard.press("Escape");
  const keys = await page.evaluate(async () => {
    const result = [];
    for (const name of await caches.keys())
      for (const r of await (await caches.open(name)).keys())
        result.push(new URL(r.url).pathname);
    return result;
  });
  expect(keys.some((k) => k.includes("/api/"))).toBe(false);
  expect(keys).toContain("/index.html");
  await context.setOffline(true);
  await page.reload();
  await expect(page.getByLabel("Question or creative direction")).toBeVisible();
  await page
    .getByLabel("Question or creative direction")
    .fill("A prompt saved without metered inference");
  await expect(
    page.getByText(/Saved on this device · AI prompts/),
  ).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("Question or creative direction")).toHaveValue(
    "A prompt saved without metered inference",
  );
  await navigate(page, "Drafts & Proposals");
  await page
    .getByRole("combobox", { name: "Record kind", exact: true })
    .selectOption("source");
  await page.getByLabel("Stable ID", { exact: true }).fill("offline-notes");
  await page.getByLabel("Label", { exact: true }).fill("Witness notes");
  await page.getByRole("button", { name: "Stage record", exact: true }).click();
  await page
    .getByLabel("Reason for change", { exact: true })
    .fill("Offline witnessed notes");
  await page
    .getByRole("button", { name: "Save draft proposal", exact: true })
    .click();
  await device(page);
  await expect(
    page.locator(".wb-device").getByText("Waiting to sync", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  const holdDelivery = (route) =>
    route.request().method() === "POST" ? route.abort() : route.continue();
  await context.route("**/api/worlds/pwa-world/changes", holdDelivery);
  await context.setOffline(false);
  await page.reload();
  await device(page);
  await page.request.post("/__fixture/update");
  await page.evaluate(async () => {
    await (await navigator.serviceWorker.getRegistration()).update();
  });
  await expect(
    page.getByRole("button", { name: "Update application", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Update application", exact: true })
    .click();
  await page.waitForLoadState("domcontentloaded");
  await expect(page.locator(".wb-layout")).toBeVisible();
  await device(page);
  await expect(
    page.locator(".wb-device").getByText("Waiting to sync", { exact: true }),
  ).toBeVisible();
  await context.unroute("**/api/worlds/pwa-world/changes", holdDelivery);
  await page
    .getByRole("button", { name: "Sync saved work", exact: true })
    .click();
  await expect(
    page.locator(".wb-device").getByText("Synced", { exact: true }),
  ).toBeVisible({ timeout: 45000 });
  expect(
    await page.evaluate(
      async () => (await fetch("/api/auth/status?refuse=1")).status,
    ),
  ).toBe(401);
  await context.setOffline(true);
  const response = await page.evaluate(async () => {
    try {
      return (await fetch("/api/auth/status")).status;
    } catch {
      return "network failure";
    }
  });
  expect(response).toBe("network failure");
});
for (const width of [320, 390, 768, 1024, 1440])
  for (const theme of ["light", "dark"])
    test(`editorial ${theme} ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
      await page.goto("/#/worlds/pwa-world/workspace");
      await expect(
        page.getByLabel("Question or creative direction"),
      ).toBeVisible();
      await page.evaluate((theme) => {
        document.documentElement.dataset.theme = theme;
      }, theme);
      const dimensions = await page.evaluate(() => {
        const composer = document
          .querySelector(".wb-composer")
          .getBoundingClientRect();
        return {
          width: document.documentElement.scrollWidth,
          viewport: innerWidth,
          top: composer.top,
          bottom: composer.bottom,
          height: innerHeight,
        };
      });
      expect(dimensions.width).toBeLessThanOrEqual(dimensions.viewport);
      expect(dimensions.top).toBeGreaterThanOrEqual(0);
      expect(dimensions.bottom).toBeLessThanOrEqual(dimensions.height);
      const accessibility = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
        .analyze();
      expect(accessibility.violations).toEqual([]);
      await page.screenshot({
        path: test.info().outputPath(`worldbuilder-${theme}-${width}.png`),
        fullPage: true,
      });
    });

test("sign-out locks downloaded writing across browser tabs and preserves it for the same account", async ({
  page,
  context,
}) => {
  await page.goto("/#/worlds/pwa-world/workspace");
  await expect(page.getByLabel("Question or creative direction")).toBeVisible();
  await device(page);
  await page
    .getByRole("button", { name: "Enable offline writing on this device" })
    .click();
  await page
    .getByRole("button", { name: "Download this timeline and private stories" })
    .click();
  await expect(page.getByText(/Downloaded 4 records/)).toBeVisible();
  await page.keyboard.press("Escape");
  const other = await context.newPage();
  await other.goto("/#/worlds/pwa-world/explore");
  await expect(other.locator(".wb-layout")).toBeVisible();
  await page
    .getByLabel("Question or creative direction")
    .fill("Retained only for the signed-in Author");
  await expect(
    page.getByText(/Saved on this device · AI prompts/),
  ).toBeVisible();
  await page.route("**/oauth2/sign_out", (route) =>
    route.fulfill({
      status: 200,
      contentType: "text/plain",
      body: "Signed out",
    }),
  );
  await page.getByRole("button", { name: "Authentication menu" }).click();
  await page.getByRole("button", { name: "Sign Out", exact: true }).click();
  await expect(other.locator(".wb-layout")).toHaveCount(0);
  await context.setOffline(true);
  await other.reload();
  await expect(other.locator(".wb-layout")).toHaveCount(0);
  await context.setOffline(false);
  await page.goto("/#/worlds/pwa-world/workspace");
  await expect(page.getByLabel("Question or creative direction")).toHaveValue(
    "Retained only for the signed-in Author",
  );
  await other.close();
});
