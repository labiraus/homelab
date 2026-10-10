import { test, expect } from "@playwright/test";

test("independent browser users propose, review, publish and keep private stories isolated", async ({
  browser,
}) => {
  const world = process.env.WORLDBUILDER_BROWSER_WORLD;
  if (!world || !process.env.WORLDBUILDER_BROWSER_URL)
    throw new Error(
      "Run only through the disposable PostgreSQL browser harness",
    );
  const editorContext = await browser.newContext({
    baseURL: process.env.WORLDBUILDER_BROWSER_URL,
    extraHTTPHeaders: { "Test-Actor": "editor@example.test" },
  });
  const reviewerContext = await browser.newContext({
    baseURL: process.env.WORLDBUILDER_BROWSER_URL,
    extraHTTPHeaders: { "Test-Actor": "reviewer@example.test" },
  });
  for (const context of [editorContext, reviewerContext]) {
    await context.route("**/*", (route) =>
      new URL(route.request().url()).origin ===
      new URL(process.env.WORLDBUILDER_BROWSER_URL).origin
        ? route.continue()
        : route.abort(),
    );
  }
  const editor = await editorContext.newPage(),
    reviewer = await reviewerContext.newPage();
  async function open(page) {
    await page.goto("/");
    await page.getByRole("button", { name: "Worlds", exact: true }).click();
    await page.getByLabel("Open world").selectOption(world);
    await expect(page.getByText(/canon revision 1/)).toBeVisible();
  }
  await open(editor);
  await editor
    .getByRole("combobox", { name: "Record kind", exact: true })
    .selectOption("entity");
  await editor.getByLabel("Stable ID", { exact: true }).fill("pilgrim");
  await editor.getByLabel("Label", { exact: true }).fill("Pilgrim");
  await editor
    .getByRole("combobox", { name: "Entity type", exact: true })
    .selectOption("person");
  await editor
    .getByRole("button", { name: "Stage record", exact: true })
    .click();
  await editor
    .getByLabel("Reason for change", { exact: true })
    .fill("Browser acceptance pilgrim");
  await editor
    .getByRole("button", { name: "Save draft proposal", exact: true })
    .click();
  const proposal = editor.locator("article").filter({
    has: editor.getByRole("heading", {
      name: "Browser acceptance pilgrim",
      exact: true,
    }),
  });
  await proposal
    .getByRole("button", { name: "Submit for review", exact: true })
    .click();
  await expect(proposal.getByText(/ready_for_review/)).toBeVisible();
  await expect(
    proposal.getByRole("button", { name: "Approve", exact: true }),
  ).toHaveCount(0);
  await open(reviewer);
  const review = reviewer.locator("article").filter({
    has: reviewer.getByRole("heading", {
      name: "Browser acceptance pilgrim",
      exact: true,
    }),
  });
  await reviewer
    .getByLabel("Review reason", { exact: true })
    .fill("Independently checked typed entity");
  await review.getByRole("button", { name: "Approve", exact: true }).click();
  await review
    .getByRole("button", { name: "Publish approved change", exact: true })
    .click();
  await expect(reviewer.getByText(/canon revision 2/)).toBeVisible();
  await reviewer.getByLabel("Search canon", { exact: true }).fill("Pilgrim");
  await reviewer
    .getByRole("button", { name: "Load canon", exact: true })
    .click();
  await expect(
    reviewer.getByRole("heading", { name: "Pilgrim entity", exact: true }),
  ).toBeVisible();
  await reviewer
    .getByRole("button", {
      name: "Explore connections of Pilgrim",
      exact: true,
    })
    .click();
  await expect(
    reviewer.getByRole("complementary", { name: "Canon connections" }),
  ).toContainText("Revision 2, timeline main");
  await editor.getByRole("button", { name: "Load canon", exact: true }).click();
  await editor
    .getByLabel("Story title", { exact: true })
    .fill("Private bridge journey");
  await editor
    .getByLabel("Premise", { exact: true })
    .fill("The pilgrim reaches the bridge.");
  await editor
    .getByRole("button", { name: "Start at revision 2", exact: true })
    .click();
  await expect(
    editor.getByText(/Pinned revision 2, timeline main, turn 0/),
  ).toBeVisible();
  const sessions = await editorContext.request.get(
    `/api/worlds/${world}/sessions`,
  );
  const rows = await sessions.json();
  const privateStory = rows.find((s) => s.title === "Private bridge journey");
  expect(privateStory).toBeTruthy();
  const denied = await reviewerContext.request.get(
    `/api/worlds/${world}/sessions/${privateStory.id}`,
  );
  expect(denied.status()).toBe(404);
  await editor.reload();
  await editor.getByRole("button", { name: "Worlds", exact: true }).click();
  await editor.getByLabel("Open world").selectOption(world);
  await editor
    .getByRole("button", { name: "Resume Private bridge journey", exact: true })
    .click();
  await expect(
    editor.getByText(/Pinned revision 2, timeline main, turn 0/),
  ).toBeVisible();
  await editorContext.close();
  await reviewerContext.close();
});
