import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

async function navigate(page, name) {
  const link = page.getByRole("link", { name, exact: true });
  await page.locator("#world-navigation").waitFor({ state: "attached" });
  if (!(await link.isVisible()))
    await page
      .getByRole("button", { name: "World navigation", exact: true })
      .click();
  await link.click();
  if (
    name === "Explore" &&
    !(await page.locator(".wb-advanced-filters").evaluate((node) => node.open))
  )
    await page
      .getByText("Timeline and history filters", { exact: true })
      .click();
  if (
    name === "World Settings" &&
    !(await page
      .getByText("Advanced timelines and branches", { exact: true })
      .evaluate((node) => node.parentElement.open))
  )
    await page
      .getByText("Advanced timelines and branches", { exact: true })
      .click();
}

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
  const secondContext = await browser.newContext({
    baseURL: process.env.WORLDBUILDER_BROWSER_URL,
    extraHTTPHeaders: { "Test-Actor": "second@example.test" },
  });
  for (const context of [editorContext, reviewerContext, secondContext]) {
    await context.route("**/*", (route) =>
      new URL(route.request().url()).origin ===
      new URL(process.env.WORLDBUILDER_BROWSER_URL).origin
        ? route.continue()
        : route.abort(),
    );
  }
  const editor = await editorContext.newPage(),
    reviewer = await reviewerContext.newPage(),
    second = await secondContext.newPage();
  async function open(page) {
    await page.goto("/");
    await page.getByRole("button", { name: "Worlds", exact: true }).click();
    await page.getByLabel("Open world").selectOption(world);
    await expect(page.locator(".wb-scope-bar")).toContainText("revision 1");
  }
  const administratorHeaders = {
    "Test-Actor": "browser-superuser@example.test",
  };
  const initial = await (
    await editorContext.request.get(`/api/worlds/${world}`, {
      headers: administratorHeaders,
    })
  ).json();
  const policyChange = await editorContext.request.post(
    `/api/worlds/${world}/administration`,
    {
      headers: administratorHeaders,
      data: {
        requestId: randomUUID(),
        expectedVersion: initial.adminVersion,
        action: "set_policy",
        policy: { mode: "independent", requiredApprovals: 2 },
        reason: "Two independent Authors for this acceptance",
      },
    },
  );
  expect(policyChange.status(), await policyChange.text()).toBe(200);
  await open(editor);
  await navigate(editor, "Explore");
  await editor.getByText("Relationship graph", { exact: true }).click();
  const graph = editor.getByRole("region", {
    name: "Relationship graph",
    exact: true,
  });
  await expect(
    graph.getByRole("link", {
      name: "Inspect friend: Hero to Friend",
      exact: true,
    }),
  ).toHaveAttribute("href", "#canon-edge");
  await expect(
    graph.getByRole("link", {
      name: "Inspect friend: Friend to Hero",
      exact: true,
    }),
  ).toHaveCount(0);
  await graph
    .getByRole("link", { name: "Inspect friend: Hero to Friend", exact: true })
    .click();
  await expect(editor.locator("#canon-edge")).toBeInViewport();
  await graph.getByRole("link", { name: "Open Hero", exact: true }).focus();
  await graph
    .getByRole("link", { name: "Open Hero", exact: true })
    .press("Enter");
  await expect(editor.locator("#canon-hero")).toBeInViewport();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("combobox", { name: "Record kind", exact: true })
    .selectOption("entity");
  const stableID = editor.getByLabel("Stable ID", { exact: true });
  await stableID.fill("invalid identity");
  expect(
    await stableID.evaluate((input) => input.validity.patternMismatch),
  ).toBe(true);
  await stableID.fill("pilgrim");
  await navigate(editor, "Drafts & Proposals");
  await editor.getByLabel("Label", { exact: true }).fill("Pilgrim");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("combobox", { name: "Entity type", exact: true })
    .selectOption("person");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Stage record", exact: true })
    .click();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Start a new record", exact: true })
    .click();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("combobox", { name: "Record kind", exact: true })
    .selectOption("event");
  await navigate(editor, "Drafts & Proposals");
  await editor.getByLabel("Stable ID", { exact: true }).fill("epoch");
  await navigate(editor, "Drafts & Proposals");
  await editor.getByLabel("Label", { exact: true }).fill("Epoch");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("combobox", { name: "Source", exact: true })
    .selectOption("source");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByLabel("Valid from", { exact: true })
    .fill("-9223372036854775808");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByLabel("Valid until", { exact: true })
    .fill("9223372036854775807");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Stage record", exact: true })
    .click();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByLabel("Reason for change", { exact: true })
    .fill("Browser acceptance pilgrim");
  await navigate(editor, "Explore");
  await editor
    .getByRole("button", { name: "Edit Test world", exact: true })
    .click();
  await navigate(editor, "Drafts & Proposals");
  await editor.getByLabel("Use a fictional calendar", { exact: true }).check();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByLabel("Calendar name", { exact: true })
    .fill("Harbour calendar");
  await navigate(editor, "Drafts & Proposals");
  await expect(
    editor.getByLabel("Required independent approvals", { exact: true }),
  ).toHaveCount(0);
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByLabel("Epoch tick", { exact: true })
    .fill("-9223372036854775808");
  await navigate(editor, "Drafts & Proposals");
  await editor.getByLabel("Name 1", { exact: true }).fill("Tide");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Stage record", exact: true })
    .click();
  await navigate(editor, "Explore");
  await expect(
    editor.getByRole("region", { name: "Canon visualisations", exact: true }),
  ).not.toContainText("Dates use Harbour calendar");
  await navigate(editor, "Drafts & Proposals");
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
  await expect(proposal.getByText(/Ready for review/)).toBeVisible();
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
  await navigate(reviewer, "Drafts & Proposals");
  await reviewer
    .getByLabel("Review reason", { exact: true })
    .fill("Independently checked typed entity");
  await review.getByRole("button", { name: "Approve", exact: true }).click();
  await expect(review).toContainText("1 of 2 required approvals.");
  await expect(
    review.getByRole("button", {
      name: "Publish approved change",
      exact: true,
    }),
  ).toHaveCount(0);
  await expect(
    review.getByRole("button", { name: "Approve", exact: true }),
  ).toHaveCount(0);
  await open(second);
  const secondReview = second.locator("article").filter({
    has: second.getByRole("heading", {
      name: "Browser acceptance pilgrim",
      exact: true,
    }),
  });
  await navigate(second, "Drafts & Proposals");
  await second
    .getByLabel("Review reason", { exact: true })
    .fill("Second independent review of settings and canon");
  await secondReview
    .getByRole("button", { name: "Approve", exact: true })
    .click();
  await expect(secondReview).toContainText("2 of 2 required approvals.");
  await secondReview
    .getByRole("button", { name: "Publish approved change", exact: true })
    .click();
  await expect(secondReview).toContainText(
    "Approvals at publication: 2 of 2 required approvals.",
  );
  await navigate(reviewer, "Explore");
  await reviewer
    .getByRole("button", { name: "Load canon", exact: true })
    .click();
  await expect(reviewer.getByText(/canon revision 2/)).toBeVisible();
  await navigate(reviewer, "Explore");
  await reviewer.getByLabel("Search canon", { exact: true }).fill("Pilgrim");
  await navigate(reviewer, "Explore");
  await reviewer
    .getByRole("button", { name: "Load canon", exact: true })
    .click();
  await expect(
    reviewer.getByRole("heading", {
      name: "Pilgrim Character or place",
      exact: true,
    }),
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
  // Match the event's end tick, not the calendar's epoch field names.
  await navigate(reviewer, "Explore");
  await reviewer
    .getByLabel("Search canon", { exact: true })
    .fill("9223372036854775807");
  await navigate(reviewer, "Explore");
  await reviewer
    .getByRole("button", { name: "Load canon", exact: true })
    .click();
  await navigate(reviewer, "Explore");
  await reviewer.getByText("Chronology", { exact: true }).click();
  await expect(reviewer.locator("#canon-world_settings")).toHaveCount(0);
  await expect(
    reviewer.getByRole("region", { name: "Chronology", exact: true }),
  ).toContainText(
    "1 Tide, year 1 (Harbour calendar; tick -9223372036854775808)",
  );
  await navigate(editor, "Explore");
  await editor.getByRole("button", { name: "Load canon", exact: true }).click();
  await navigate(editor, "Explore");
  await editor.getByRole("button", { name: "Edit Epoch", exact: true }).click();
  await navigate(editor, "Explore");
  await editor.getByText("Chronology", { exact: true }).click();
  const chronology = editor.getByRole("region", {
    name: "Chronology",
    exact: true,
  });
  await expect(
    chronology.getByRole("link", { name: "Epoch", exact: true }),
  ).toHaveAttribute("href", "#canon-epoch");
  await expect(chronology).toContainText("-9223372036854775808");
  await expect(chronology).not.toContainText("Alternate era");
  await navigate(editor, "Drafts & Proposals");
  await expect(editor.getByLabel("Valid from", { exact: true })).toHaveValue(
    "-9223372036854775808",
  );
  await expect(editor.getByLabel("Valid until", { exact: true })).toHaveValue(
    "9223372036854775807",
  );
  // The real Go response emits numeric tokens. UI must retain them exactly on
  // read and during complete-record edits, including a one-tick boundary.
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByLabel("Valid from", { exact: true })
    .fill("9223372036854775806");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Stage record", exact: true })
    .click();
  const canonResponse = await editorContext.request.get(
    `/api/worlds/${world}/records/epoch?revision=2&timeline=main&at=9223372036854775806`,
  );
  expect(canonResponse.status()).toBe(200);
  expect(await canonResponse.text()).toContain('"until":9223372036854775807');
  const endResponse = await editorContext.request.get(
    `/api/worlds/${world}/records/epoch?revision=2&timeline=main&at=9223372036854775807`,
  );
  expect(endResponse.status()).toBe(404);
  // Staging a candidate cannot move the published chronology.
  await navigate(editor, "Explore");
  await expect(chronology).toContainText("-9223372036854775808");
  await expect(chronology).not.toContainText("9223372036854775806");
  await navigate(editor, "Explore");
  await editor
    .getByRole("button", { name: "Edit Test world", exact: true })
    .click();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByLabel("Calendar name", { exact: true })
    .fill("Unpublished calendar");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Stage record", exact: true })
    .click();
  await navigate(editor, "Explore");
  await expect(chronology).toContainText("Harbour calendar");
  await expect(chronology).not.toContainText("Unpublished calendar");
  await navigate(editor, "Explore");
  await editor
    .getByRole("region", { name: "Canon visualisations", exact: true })
    .screenshot({ path: test.info().outputPath("canon-views.png") });
  await navigate(editor, "Stories");
  await editor
    .getByRole("button", { name: "Create story", exact: true })
    .click();
  await editor
    .getByLabel("Story title", { exact: true })
    .fill("Private bridge journey");
  await navigate(editor, "Stories");
  await editor
    .getByLabel("Premise", { exact: true })
    .fill("The pilgrim reaches the bridge.");
  await navigate(editor, "Stories");
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
  await navigate(editor, "Stories");
  await editor
    .getByRole("button", { name: "Resume Private bridge journey", exact: true })
    .click();
  await expect(
    editor.getByText(/Pinned revision 2, timeline main, turn 0/),
  ).toBeVisible();
  await navigate(editor, "Explore");
  await editor.getByText("Chronology", { exact: true }).click();
  await navigate(editor, "Explore");
  await editor
    .getByLabel("Fictional time", { exact: true })
    .fill("09223372036854775807");
  await navigate(editor, "Explore");
  await editor.getByRole("button", { name: "Load canon", exact: true }).click();
  await expect(chronology).toContainText("No events in the loaded scope.");
  await navigate(editor, "Explore");
  await editor.getByLabel("Fictional time", { exact: true }).fill("");
  await navigate(editor, "Explore");
  await editor.getByLabel("Historical revision", { exact: true }).fill("1");
  await navigate(editor, "Explore");
  await editor.getByRole("button", { name: "Load canon", exact: true }).click();
  await expect(
    editor.getByRole("region", { name: "Canon visualisations", exact: true }),
  ).toContainText("Revision 1, timeline main");
  await navigate(editor, "Explore");
  await expect(
    editor.getByRole("region", { name: "Canon visualisations", exact: true }),
  ).not.toContainText("Dates use Harbour calendar");
  await expect(chronology).toContainText("No events in the loaded scope.");
  await navigate(editor, "Explore");
  await editor.getByLabel("Historical revision", { exact: true }).fill("2");
  await navigate(editor, "Explore");
  await editor.getByLabel("Timeline", { exact: true }).fill("alternate");
  await navigate(editor, "Explore");
  await editor.getByRole("button", { name: "Load canon", exact: true }).click();
  await expect(
    chronology.getByRole("link", { name: "Alternate era", exact: true }),
  ).toBeVisible();
  await expect(
    chronology.getByRole("link", { name: "Epoch", exact: true }),
  ).toHaveCount(0);
  await navigate(editor, "Explore");
  await expect(chronology).toContainText("Harbour calendar");

  // A resumed private story becomes canon only through a fresh reviewed draft.
  await navigate(editor, "Explore");
  await editor.getByLabel("Historical revision", { exact: true }).fill("");
  await navigate(editor, "Explore");
  await editor.getByLabel("Timeline", { exact: true }).fill("main");
  await navigate(editor, "Explore");
  await editor.getByRole("button", { name: "Load canon", exact: true }).click();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Start a new record", exact: true })
    .click();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("combobox", { name: "Record kind", exact: true })
    .selectOption("event");
  await navigate(editor, "Drafts & Proposals");
  await editor.getByLabel("Stable ID", { exact: true }).fill("homecoming");
  await navigate(editor, "Drafts & Proposals");
  await editor.getByLabel("Label", { exact: true }).fill("Homecoming");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("combobox", { name: "Source", exact: true })
    .selectOption("source");
  await navigate(editor, "Drafts & Proposals");
  await editor.getByLabel("Valid from", { exact: true }).fill("42");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Stage record", exact: true })
    .click();
  await navigate(editor, "Stories");
  await editor.getByText("Manual narration authoring", { exact: true }).click();
  await editor
    .getByLabel("Next narration", { exact: true })
    .fill("The pilgrim returns home after crossing the bridge.");
  await navigate(editor, "Stories");
  await editor
    .getByRole("button", { name: "Save turn with staged changes", exact: true })
    .click();
  await expect(
    editor.getByText(/Pinned revision 2, timeline main, turn 1/),
  ).toBeVisible();
  const searchPath = `/api/worlds/${world}/search?timeline=main&q=Homecoming&at=42`;
  async function search(context, revision) {
    const response = await context.request.get(
      `${searchPath}&revision=${revision}`,
    );
    expect(response.status()).toBe(200);
    return response.json();
  }
  expect((await search(reviewerContext, 2)).records).toEqual([]);
  expect(
    (
      await reviewerContext.request.get(
        `/api/worlds/${world}/sessions/${privateStory.id}`,
      )
    ).status(),
  ).toBe(404);

  await editor.reload();
  await editor.getByRole("button", { name: "Worlds", exact: true }).click();
  await editor.getByLabel("Open world").selectOption(world);
  await navigate(editor, "Stories");
  await editor
    .getByRole("button", { name: "Resume Private bridge journey", exact: true })
    .click();
  await expect(
    editor.getByText(/Pinned revision 2, timeline main, turn 1/),
  ).toBeVisible();
  await expect(
    editor.getByText("The pilgrim returns home after crossing the bridge.", {
      exact: true,
    }),
  ).toBeVisible();
  await navigate(editor, "Stories");
  await editor
    .getByRole("button", {
      name: "Review story outcomes",
      exact: true,
    })
    .click();
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByLabel("Reconciliation reason", { exact: true })
    .fill("Adopt the private homecoming outcome");
  await navigate(editor, "Drafts & Proposals");
  await editor
    .getByRole("button", { name: "Save reconciled draft", exact: true })
    .click();
  await expect(
    editor.getByRole("status").filter({ hasText: "Reconciled draft saved" }),
  ).toContainText("Reconciled draft saved");
  const outcomeFor = (page) =>
    page.locator("article").filter({
      has: page.getByRole("heading", {
        name: "Adopt the private homecoming outcome",
        exact: true,
      }),
    });
  const outcome = outcomeFor(editor);
  await outcome
    .getByRole("button", { name: "Submit for review", exact: true })
    .click();
  await expect(outcome).toContainText("Ready for review");
  await expect(
    outcome.getByRole("button", { name: "Approve", exact: true }),
  ).toHaveCount(0);
  expect((await search(reviewerContext, 2)).records).toEqual([]);

  // The established two-human policy also applies to story outcomes.
  await navigate(reviewer, "Explore");
  await reviewer
    .getByRole("button", { name: "Load canon", exact: true })
    .click();
  await navigate(reviewer, "Drafts & Proposals");
  await reviewer
    .getByLabel("Review reason", { exact: true })
    .fill("Checked the homecoming against current canon");
  await outcomeFor(reviewer)
    .getByRole("button", { name: "Approve", exact: true })
    .click();
  await expect(outcomeFor(reviewer)).toContainText(
    "1 of 2 required approvals.",
  );
  await expect(
    outcomeFor(reviewer).getByRole("button", {
      name: "Publish approved change",
      exact: true,
    }),
  ).toHaveCount(0);
  expect((await search(reviewerContext, 2)).records).toEqual([]);
  await navigate(second, "Explore");
  await second.getByRole("button", { name: "Load canon", exact: true }).click();
  await navigate(second, "Drafts & Proposals");
  await second
    .getByLabel("Review reason", { exact: true })
    .fill("Second independent outcome review");
  await outcomeFor(second)
    .getByRole("button", { name: "Approve", exact: true })
    .click();
  await outcomeFor(second)
    .getByRole("button", { name: "Publish approved change", exact: true })
    .click();
  await expect(outcomeFor(second)).toContainText(
    "Approvals at publication: 2 of 2 required approvals.",
  );
  await expect(second.getByText(/canon revision 3/)).toBeVisible();

  await navigate(reviewer, "Explore");
  await reviewer.getByLabel("Search canon", { exact: true }).fill("Homecoming");
  await navigate(reviewer, "Explore");
  await reviewer.getByLabel("Fictional time", { exact: true }).fill("42");
  await navigate(reviewer, "Explore");
  await reviewer
    .getByRole("button", { name: "Load canon", exact: true })
    .click();
  await expect(
    reviewer.getByRole("heading", { name: "Homecoming Event", exact: true }),
  ).toBeVisible();
  const published = await search(reviewerContext, 3);
  expect(published.records.map((r) => r.id)).toEqual(["homecoming"]);
  expect(published.citations.length).toBeGreaterThan(0);
  expect(
    published.citations.every((c) => c.revision === 3 && c.timeline === "main"),
  ).toBe(true);
  expect((await search(reviewerContext, 2)).records).toEqual([]);
  expect(
    (
      await reviewerContext.request.get(
        `/api/worlds/${world}/records/homecoming?revision=3&timeline=main&at=41`,
      )
    ).status(),
  ).toBe(404);
  const retained = await editorContext.request.get(
    `/api/worlds/${world}/sessions/${privateStory.id}`,
  );
  expect(retained.status()).toBe(200);
  expect(await retained.json()).toMatchObject({
    pinnedRevision: 2,
    version: 1,
    state: { homecoming: { id: "homecoming" } },
  });
  expect(
    (
      await reviewerContext.request.get(
        `/api/worlds/${world}/sessions/${privateStory.id}`,
      )
    ).status(),
  ).toBe(404);
  await navigate(editor, "Explore");
  await editor.getByRole("button", { name: "Load canon", exact: true }).click();
  await expect(editor.getByText(/canon revision 3/)).toBeVisible();
  await navigate(editor, "Stories");
  await expect(
    editor.getByText(/Pinned revision 2, timeline main, turn 1/),
  ).toBeVisible();
  await editorContext.close();
  await reviewerContext.close();
  await secondContext.close();
});

test("Administrator registers a user and explicitly overrides approval counts", async ({
  browser,
}) => {
  const baseURL = process.env.WORLDBUILDER_BROWSER_URL;
  if (!baseURL) throw new Error("Disposable browser harness required");
  const context = await browser.newContext({
    baseURL,
    extraHTTPHeaders: { "Test-Actor": "browser-superuser@example.test" },
  });
  await context.route("**/*", (route) =>
    new URL(route.request().url()).origin === new URL(baseURL).origin
      ? route.continue()
      : route.abort(),
  );
  try {
    const page = await context.newPage();
    await page.goto("/");
    await page.getByRole("button", { name: "Worlds", exact: true }).click();
    await page.getByRole("button", { name: "World menu", exact: true }).click();
    await page.getByText("Account administration", { exact: true }).click();
    await page
      .getByText("Administrator · Manage users", { exact: true })
      .click();
    await page
      .getByLabel("Google account email", { exact: true })
      .fill("browser-new@example.test");
    await page
      .getByLabel("Display name", { exact: true })
      .fill("New browser user");
    await page.getByRole("button", { name: "Add user", exact: true }).click();
    await expect(
      page.getByRole("list", { name: "Registered users" }),
    ).toContainText("New browser user · browser-new@example.test");
    await page
      .getByRole("region", { name: "World actions" })
      .getByRole("button", { name: "Create a world", exact: true })
      .click();
    await page
      .getByLabel("World name", { exact: true })
      .fill("Superuser world");
    await page
      .getByRole("button", { name: "Create world", exact: true })
      .click();
    await navigate(page, "Drafts & Proposals");
    await page
      .getByRole("combobox", { name: "Record kind", exact: true })
      .selectOption("source");
    await navigate(page, "Drafts & Proposals");
    await page
      .getByLabel("Stable ID", { exact: true })
      .fill("self-reviewed-source");
    await navigate(page, "Drafts & Proposals");
    await page
      .getByLabel("Label", { exact: true })
      .fill("Self reviewed source");
    await navigate(page, "Drafts & Proposals");
    await page
      .getByRole("button", { name: "Stage record", exact: true })
      .click();
    await navigate(page, "Drafts & Proposals");
    await page
      .getByLabel("Reason for change", { exact: true })
      .fill("Superuser authored proposal");
    await navigate(page, "Drafts & Proposals");
    await page
      .getByRole("button", { name: "Save draft proposal", exact: true })
      .click();
    const proposal = page.locator("article").filter({
      has: page.getByRole("heading", {
        name: "Superuser authored proposal",
        exact: true,
      }),
    });
    await navigate(page, "Drafts & Proposals");
    await proposal
      .getByRole("button", { name: "Submit for review", exact: true })
      .click();
    await navigate(page, "Drafts & Proposals");
    await page
      .getByLabel("Review reason", { exact: true })
      .fill("I explicitly approve my own proposal");
    await expect(
      proposal.getByRole("button", { name: "Approve", exact: true }),
    ).toHaveCount(0);
    await proposal
      .getByRole("button", {
        name: "Publish with recorded Administrator override",
        exact: true,
      })
      .click();
    await expect(page.locator(".wb-scope-bar")).toContainText("revision 1");
    await expect(proposal).toContainText("Published");
  } finally {
    await context.close();
  }
});

test("a timeline branch is reviewed before publication and retains its parent snapshot", async ({
  browser,
}) => {
  const baseURL = process.env.WORLDBUILDER_BROWSER_URL;
  if (!baseURL) throw new Error("Disposable browser harness required");
  const context = await browser.newContext({
    baseURL,
    extraHTTPHeaders: { "Test-Actor": "browser-superuser@example.test" },
  });
  await context.route("**/*", (route) =>
    new URL(route.request().url()).origin === new URL(baseURL).origin
      ? route.continue()
      : route.abort(),
  );
  const world = randomUUID();
  const path = `/api/worlds/${world}`;
  async function post(url, data) {
    const response = await context.request.post(url, { data });
    expect(response.status(), await response.text()).toBe(200);
    return response.json();
  }
  async function publish(revision, records) {
    const id = randomUUID();
    await post(`${path}/changes`, {
      id,
      baseRevision: revision,
      reason: "Browser branch fixture",
      operations: records.map((record) => ({
        action: "put",
        id: record.id,
        record,
      })),
    });
    await post(`${path}/changes/${id}/submit`, {});
    await post(`${path}/changes/${id}/publish`, {
      overrideReason: "Explicit fixture Administrator override",
    });
  }
  try {
    await post("/api/worlds", { id: world, name: "Branch browser world" });
    const fact = {
      id: "age",
      kind: "fact",
      label: "Opening age",
      subject: "hero",
      predicate: "age",
      valueType: "number",
      value: 20,
      timeline: "main",
      source: "source",
      from: "-9223372036854775808",
      until: "9223372036854775807",
    };
    await publish(0, [
      {
        id: "person",
        kind: "type",
        label: "Person",
        fields: { age: "number" },
      },
      { id: "hero", kind: "entity", label: "Hero", type: "person" },
      { id: "source", kind: "source", label: "Notes" },
      fact,
    ]);
    const page = await context.newPage();
    await page.goto("/");
    await page.getByRole("button", { name: "Worlds", exact: true }).click();
    await page.getByLabel("Open world").selectOption(world);
    await expect(page.locator(".wb-scope-bar")).toContainText("revision 1");
    await navigate(page, "Explore");
    await page
      .getByLabel("Search canon", { exact: true })
      .fill("unmatchedfilter");
    await navigate(page, "Explore");
    await page.getByRole("button", { name: "Load canon", exact: true }).click();
    await navigate(page, "World Settings");
    await page
      .getByRole("button", { name: "Prepare timeline branch", exact: true })
      .click();
    const timelineInput = page.getByLabel("New timeline ID", { exact: true });
    await timelineInput.fill("invalid timeline");
    expect(
      await timelineInput.evaluate((input) => input.validity.patternMismatch),
    ).toBe(true);
    await timelineInput.fill("fork");
    expect(await timelineInput.evaluate((input) => input.validity.valid)).toBe(
      true,
    );
    await navigate(page, "World Settings");
    await page
      .getByLabel("New timeline name", { exact: true })
      .fill("Fork history");
    await navigate(page, "World Settings");
    await page
      .getByLabel("Branch reason", { exact: true })
      .fill("Explore the fork");
    await navigate(page, "World Settings");
    await page
      .getByRole("button", { name: "Create branch draft", exact: true })
      .click();
    await expect(
      page.getByRole("status").filter({ hasText: "Review and publish" }),
    ).toContainText("Review and publish");
    expect((await (await context.request.get(path)).json()).revision).toBe(1);
    const proposal = page.locator("article").filter({
      has: page.getByRole("heading", {
        name: "Explore the fork",
        exact: true,
      }),
    });
    await navigate(page, "Drafts & Proposals");
    await proposal
      .getByRole("button", { name: "Submit for review", exact: true })
      .click();
    await navigate(page, "Drafts & Proposals");
    await page
      .getByLabel("Review reason", { exact: true })
      .fill("Reviewed the complete fixed parent snapshot");
    await proposal
      .getByRole("button", {
        name: "Publish with recorded Administrator override",
        exact: true,
      })
      .click();
    await expect(page.getByText(/canon revision 2/)).toBeVisible();
    await publish(2, [{ ...fact, value: 30 }]);
    await navigate(page, "Explore");
    await page.getByLabel("Timeline", { exact: true }).fill("fork");
    await navigate(page, "Explore");
    await page.getByLabel("Search canon", { exact: true }).fill("Opening age");
    await navigate(page, "Explore");
    await page.getByLabel("Fictional time", { exact: true }).fill("10");
    await navigate(page, "Explore");
    await page.getByRole("button", { name: "Load canon", exact: true }).click();
    await expect(page.getByText(/canon revision 3/)).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Opening age Fact", exact: true }),
    ).toBeVisible();
    await navigate(page, "World Settings");
    await page
      .getByRole("button", { name: "View timeline origin", exact: true })
      .click();
    await expect(
      page.getByText(/created from main at revision 1/),
    ).toBeVisible();
    const result = await (
      await context.request.get(
        `${path}/search?timeline=fork&revision=3&q=age&at=10`,
      )
    ).json();
    expect(
      result.records.filter((r) => r.kind === "fact").map((r) => r.value),
    ).toEqual([20]);
    expect(
      result.citations.every((c) => c.timeline === "fork" && c.revision === 3),
    ).toBe(true);
    expect(
      (
        await context.request.get(`${path}/timelines/fork/origin?revision=1`)
      ).status(),
    ).toBe(404);
  } finally {
    await context.close();
  }
});
