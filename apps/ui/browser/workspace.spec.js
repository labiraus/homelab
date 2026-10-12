import { test, expect } from "@playwright/test";

// Isolated UI contract fixtures only. Real authorisation/publication is exercised
// separately by worldbuilder.spec.js against the owned PostgreSQL Go server.
async function fixture(page) {
  const world = {
    id: "fixture-world",
    name: "The Shattered Empire",
    revision: 7,
    defaultTimeline: "main",
    role: "editor",
  };
  const records = [
    {
      id: "person",
      kind: "type",
      label: "Person",
      fields: { alive: "boolean" },
    },
    { id: "main", kind: "timeline", label: "Primary timeline" },
    { id: "source", kind: "source", label: "The chronicle" },
    {
      id: "aldric",
      kind: "entity",
      type: "person",
      label: "King Aldric",
      attributes: { description: "The last ruler of the northern houses." },
    },
    {
      id: "battle",
      kind: "event",
      label: "Battle of Ashfall",
      timeline: "main",
      source: "source",
      from: 842,
    },
  ];
  const calls = [],
    changes = [],
    sessions = [],
    definitions = [],
    turns = [];
  await page.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (url.origin !== new URL(test.info().project.use.baseURL).origin)
      return route.abort();
    if (!url.pathname.startsWith("/api/")) return route.continue();
    const body = route.request().postDataJSON();
    calls.push({
      path: url.pathname,
      body,
      query: Object.fromEntries(url.searchParams),
    });
    let data = [];
    if (url.pathname === "/api/auth/status")
      data = { valid: true, email: "editor@example.test", mode: "fixture" };
    else if (url.pathname === "/api/auth/providers") data = { providers: [] };
    else if (url.pathname === "/api/worlds")
      data = [world, { ...world, id: "other-world", name: "Another world" }];
    else if (url.pathname.endsWith("/generations"))
      data = { enabled: false, items: [] };
    else if (
      url.pathname.endsWith("/canon") ||
      url.pathname.endsWith("/search")
    )
      data = {
        world,
        revision: Number(url.searchParams.get("revision") || 7),
        timeline: url.searchParams.get("timeline") || "main",
        records,
        total: records.length,
      };
    else if (url.pathname.includes("/records/"))
      data = {
        record: records.find((r) => r.id === url.pathname.split("/").at(-1)),
        revision: Number(url.searchParams.get("revision")),
        timeline: url.searchParams.get("timeline"),
      };
    else if (url.pathname.endsWith("/reason")) {
      if (body.question.endsWith("fail please"))
        return route.fulfill({
          status: 503,
          json: { error: "AI is temporarily unavailable" },
        });
      data = {
        baseRevision: body.revision,
        answer:
          body.workflow === "story_turn"
            ? "The gates of Ashfall open before you."
            : body.workflow === "canon_proposal"
              ? "A new possibility for the northern houses."
              : "Aldric ruled the northern houses.",
        unknown: false,
        citations: [`fixture-world:${body.revision}:main:aldric`],
        operations:
          body.workflow === "canon_proposal"
            ? [
                {
                  action: "put",
                  id: "rival",
                  record: {
                    id: "rival",
                    kind: "entity",
                    type: "person",
                    label: "Rival",
                  },
                },
              ]
            : [],
        sessionVersion: body.expectedVersion,
        choices:
          body.workflow === "story_turn"
            ? [
                {
                  id: "enter",
                  label: "Enter the city",
                  direction: "Enter the city",
                },
              ]
            : [],
      };
    } else if (url.pathname.endsWith("/changes")) {
      if (body) {
        changes.push({
          ...body,
          author: "editor@example.test",
          status: "draft",
        });
        data = changes.at(-1);
      } else data = changes;
    } else if (url.pathname.endsWith("/submit")) {
      const c = changes.find((c) => c.id === url.pathname.split("/").at(-2));
      c.status = "ready_for_review";
      data = c;
    } else if (url.pathname.endsWith("/story-definitions")) {
      if (body) {
        definitions.push(body);
        data = body;
      } else data = definitions;
    } else if (url.pathname.endsWith("/sessions")) {
      if (body) {
        sessions.push({ ...body, version: 0, state: {}, memory: {} });
        data = sessions.at(-1);
      } else data = sessions;
    } else if (url.pathname.endsWith("/turns")) {
      if (body) {
        turns.push({
          version: ++sessions[0].version,
          narration: body.narration,
        });
        data = turns.at(-1);
        sessions[0].memory.choices = body.choices;
      } else data = turns;
    } else if (url.pathname.includes("/sessions/"))
      data = sessions.find((s) => s.id === url.pathname.split("/").at(-1));
    await route.fulfill({ json: data });
  });
  await page.goto("/#/worlds/fixture-world/workspace");
  await expect(
    page.getByRole("heading", { name: "What happens next?" }),
  ).toBeVisible();
  return { calls, changes, sessions };
}
async function nav(page, name) {
  const link = page.getByRole("link", { name, exact: true });
  await page.locator("#world-navigation").waitFor({ state: "attached" });
  if (!(await link.isVisible()))
    await page
      .getByRole("button", { name: "World navigation", exact: true })
      .click();
  await link.click();
}
async function send(page, text) {
  await page.getByLabel("Question or creative direction").fill(text);
  await page.getByRole("button", { name: "Send message", exact: true }).click();
}

test("desktop conversation, pinned sources, edited generated draft and human review handoff", async ({
  page,
}) => {
  const state = await fixture(page);
  await send(page, "Who is Aldric?");
  await expect(
    page.getByText("Aldric ruled the northern houses.", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Open source · aldric", exact: true })
    .click();
  const inspector = page.getByRole("complementary", {
    name: "Source inspector",
  });
  await expect(inspector).toContainText("The last ruler");
  await expect(inspector).toContainText("Revision 7 · main");
  await inspector.getByRole("button", { name: "Ask AI about this" }).click();
  await page
    .getByRole("button", { name: /Remove context · King Aldric/ })
    .click();
  await send(page, "Who ruled before him?");
  await expect(
    page.getByText("Aldric ruled the northern houses.", { exact: true }),
  ).toHaveCount(2);
  expect(
    state.calls.filter((c) => c.path.endsWith("/reason"))[1].body.question,
  ).toContain("Who is Aldric?");
  const transcript = await page
    .getByRole("log", { name: "Conversation" })
    .boundingBox();
  const composer = await page.locator(".wb-composer").boundingBox();
  expect(transcript.y + transcript.height).toBeLessThanOrEqual(composer.y);
  await page.screenshot({
    path: test.info().outputPath("workspace-desktop.png"),
    fullPage: true,
  });
  await page
    .getByRole("combobox", { name: "Intent", exact: true })
    .selectOption("create");
  await send(page, "Create a rival character");
  await page
    .getByRole("button", { name: "Edit draft in structured editor" })
    .click();
  await page
    .getByRole("button", { name: "Edit draft Rival", exact: true })
    .click();
  await page.getByLabel("Label", { exact: true }).fill("The Rival");
  await page.getByRole("button", { name: "Stage record", exact: true }).click();
  await page
    .getByLabel("Reason for change", { exact: true })
    .fill("Introduce a rival");
  await page
    .getByRole("button", { name: "Save draft proposal", exact: true })
    .click();
  const proposal = page.locator("article").filter({
    has: page.getByRole("heading", {
      name: "Introduce a rival",
      exact: true,
    }),
  });
  await expect(proposal).toContainText("draft");
  expect(state.changes[0].baseRevision).toBe(7);
  expect(state.changes[0].operations[0].record.label).toBe("The Rival");
  await proposal
    .getByRole("button", { name: "Submit for review", exact: true })
    .click();
  await expect(proposal).toContainText("ready_for_review");
  expect(state.calls.some((c) => c.path.endsWith("/publish"))).toBe(false);
  await nav(page, "Workspace");
  await expect(
    page.getByText("Aldric ruled the northern houses.", { exact: true }),
  ).toHaveCount(2);
  await page.getByLabel("Open world").selectOption("other-world");
  await expect(
    page.getByRole("heading", { name: "What happens next?" }),
  ).toBeVisible();
  await expect(
    page.getByText("Aldric ruled the northern houses.", { exact: true }),
  ).toHaveCount(0);
});

test("mobile keyboard navigation, ambiguity, error recovery and persistent story turns", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const state = await fixture(page);
  await send(page, "Aldric survived");
  await expect(
    page.getByText(
      "Do you want to ask about published canon or prepare a change?",
    ),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Ask about this", exact: true })
    .click();
  await page
    .getByLabel("Question or creative direction")
    .press("Control+Enter");
  await expect(
    page.getByText("Aldric ruled the northern houses.", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Open source · aldric" }).click();
  await expect(
    page.getByRole("button", { name: "Close inspector" }),
  ).toBeFocused();
  await page.getByRole("button", { name: "Close inspector" }).press("Escape");
  await expect(
    page.getByRole("button", { name: "Close inspector" }),
  ).toHaveCount(0);
  await send(page, "fail please");
  await expect(page.getByRole("alert")).toContainText(
    "AI is temporarily unavailable",
  );
  await page.getByRole("button", { name: "Edit and retry message" }).click();
  await expect(page.getByLabel("Question or creative direction")).toHaveValue(
    "fail please",
  );
  await nav(page, "Stories");
  await page
    .getByLabel("Story title", { exact: true })
    .fill("Ashfall adventure");
  await page
    .getByLabel("Premise", { exact: true })
    .fill("Arrive at the gates.");
  await page.getByRole("button", { name: "Start at revision 7" }).click();
  await page
    .getByRole("button", { name: "Continue story in workspace" })
    .click();
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(
    page
      .locator("p")
      .filter({ hasText: /^The gates of Ashfall open before you\.$/ }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Save candidate as provisional turn" })
    .click();
  await expect(
    page.getByText(/Turn saved as private session state/),
  ).toBeVisible();
  expect(state.sessions[0].version).toBe(1);
  await page
    .getByRole("button", { name: "Enter the city", exact: true })
    .click();
  await expect(page.getByLabel("Question or creative direction")).toHaveValue(
    "Enter the city",
  );
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.getByRole("button", { name: "Authentication menu" }).click();
  await page.getByRole("button", { name: "Use Dark Mode" }).click();
  await page.getByRole("button", { name: "Authentication menu" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.screenshot({
    path: test.info().outputPath("workspace-mobile.png"),
    fullPage: true,
  });
  await nav(page, "Stories");
  await page.reload();
  await page.getByRole("button", { name: "Resume Ashfall adventure" }).click();
  await expect(
    page.getByText(/Pinned revision 7, timeline main, turn 1/),
  ).toBeVisible();
  await expect(
    page
      .locator("p")
      .filter({ hasText: /^The gates of Ashfall open before you\.$/ }),
  ).toBeVisible();
  expect(state.calls.some((c) => c.path.endsWith("/changes") && c.body)).toBe(
    false,
  );
});

test("scrolled desktop navigation and inspector remain below the sticky header", async ({ page }) => {
  await fixture(page);
  await send(page, "Who is Aldric?");
  await page.getByRole("button", { name: "Open source · aldric", exact: true }).click();
  for (const size of [{ width: 1280, height: 720 }, { width: 1120, height: 600 }]) {
    await page.setViewportSize(size);
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    for (const target of [
      page.getByRole("link", { name: "Workspace", exact: true }),
      page.getByRole("button", { name: "Close inspector", exact: true }),
    ]) {
      await expect.poll(() => target.evaluate((element) => {
        const box = element.getBoundingClientRect();
        const header = document.querySelector(".topbar").getBoundingClientRect();
        const hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
        return box.top >= header.bottom && (hit === element || element.contains(hit));
      })).toBe(true);
    }
  }
  await page.getByRole("button", { name: "Close inspector", exact: true }).click();
  await page.getByRole("link", { name: "Explore", exact: true }).click();
  await expect(page.getByRole("link", { name: "Explore", exact: true })).toHaveAttribute("aria-current", "page");
});
