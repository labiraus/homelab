import { test, expect } from "@playwright/test";

// Isolated UI contract fixtures only. Real authorisation/publication is exercised
// separately by worldbuilder.spec.js against the owned PostgreSQL Go server.
async function fixture(page, options = {}) {
  const world = {
    id: "fixture-world",
    name: "The Shattered Empire",
    revision: 7,
    defaultTimeline: "main",
    role: "editor",
    ...options.world,
  };
  const worlds = options.worlds || [
    world,
    { ...world, id: "other-world", name: "Another world" },
  ];
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
  records.push(...(options.records || []));
  const calls = [],
    changes = options.changes || [],
    sessions = options.sessions || [],
    definitions = [],
    turns = options.turns || [];
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
    else if (url.pathname === "/api/worlds") {
      if (body) {
        data = { ...world, ...body, revision: 0, role: "owner" };
        worlds.push(data);
      } else data = worlds;
    } else if (url.pathname.endsWith("/generations"))
      data = { enabled: false, items: [] };
    else if (
      url.pathname.endsWith("/canon") ||
      url.pathname.endsWith("/search")
    )
      data = {
        world,
        revision: Number(url.searchParams.get("revision") || 7),
        timeline: url.searchParams.get("timeline") || "main",
        records: records.filter(
          (r) =>
            (!url.searchParams.get("kind") ||
              url.searchParams.get("kind").split(",").includes(r.kind)) &&
            (!url.searchParams.get("q") ||
              (r.label || "")
                .toLowerCase()
                .includes(url.searchParams.get("q").toLowerCase())),
        ),
        total: records.filter(
          (r) =>
            !url.searchParams.get("kind") ||
            url.searchParams.get("kind").split(",").includes(r.kind),
        ).length,
      };
    else if (url.pathname.endsWith("/reconciliation"))
      data = options.comparison || {
        baseRevision: 7,
        currentRevision: 7,
        records: [],
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
          options.answer ||
          (body.workflow === "story_turn"
            ? "The gates of Ashfall open before you."
            : body.workflow === "canon_proposal"
              ? "A new possibility for the northern houses."
              : "Aldric ruled the northern houses."),
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
        sessions.push({
          ...body,
          definition: definitions.find((d) => d.id === body.definitionId),
          version: 0,
          state: {},
          memory: {},
        });
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
  await page.goto(
    options.library ? "/#/worlds" : "/#/worlds/fixture-world/workspace",
  );
  await expect(
    page.getByRole("heading", {
      name: options.library ? "Your worlds" : "What happens next?",
      exact: true,
    }),
  ).toBeVisible();
  return { calls, changes, sessions, world };
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

test("world menu keeps its trigger in place and dismisses without losing the workspace", async ({
  page,
}) => {
  await fixture(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  const trigger = page.getByRole("button", { name: "World menu", exact: true });
  const before = await trigger.boundingBox();
  await trigger.click();
  await page.screenshot({
    path: test.info().outputPath("world-menu-desktop.png"),
  });
  const after = await trigger.boundingBox();
  expect(Math.abs(after.x - before.x)).toBeLessThan(1);
  expect(Math.abs(after.y - before.y)).toBeLessThan(1);
  await expect(
    page.getByRole("button", { name: "World menu", exact: true }),
  ).toHaveAttribute("aria-expanded", "true");
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  await trigger.press("Enter");
  await page.getByLabel("Question or creative direction").click();
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  await expect(page.getByLabel("Question or creative direction")).toBeFocused();
  await trigger.click();
  await trigger.press("Tab");
  await expect(
    page.getByRole("link", { name: "Browse all worlds" }),
  ).toBeFocused();
  await page.getByLabel("Open world").focus();
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
});

test("Worlds opens a searchable library and browser history restores the selected route", async ({
  page,
}) => {
  await fixture(page);
  await page.getByRole("button", { name: "Worlds", exact: true }).click();
  await expect(page).toHaveURL(/#\/worlds$/);
  await expect(
    page.getByRole("heading", { name: "Your worlds", exact: true }),
  ).toBeVisible();
  await expect(page.locator(".wb-world-card")).toHaveCount(2);
  await page.getByLabel("Find a world").fill("another");
  await expect(page.locator(".wb-world-card")).toHaveCount(1);
  await page.getByRole("link", { name: /Another world/ }).click();
  await expect(page).toHaveURL(/#\/worlds\/other-world\/workspace$/);
  await expect(page.getByLabel("Open world")).toHaveValue("other-world");
  await expect(page.getByLabel("Question or creative direction")).toBeVisible();
  await page.goBack();
  await expect(
    page.getByRole("heading", { name: "Your worlds", exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("Find a world")).toHaveValue("another");
  await page.getByLabel("Find a world").fill("no matching world");
  await expect(page.getByText(/No loaded worlds match/)).toBeVisible();
  await page.getByLabel("Find a world").fill("");
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.screenshot({ path: test.info().outputPath("worlds-desktop.png") });
});

test("world library and menu fit narrow viewports in both themes with long archived names", async ({
  page,
}) => {
  await fixture(page, {
    world: {
      name: "The long-forgotten kingdoms beyond the Shattered Empire and the northern sea "
        .repeat(2)
        .trim(),
      archived: true,
    },
  });
  for (const theme of ["light", "dark"]) {
    await page.evaluate((value) => {
      document.documentElement.dataset.theme = value;
    }, theme);
    for (const width of [320, 390, 768, 1024, 1440]) {
      await page.setViewportSize({ width, height: 844 });
      await page.getByRole("button", { name: "Worlds", exact: true }).click();
      await expect(
        page.getByRole("heading", { name: "Your worlds", exact: true }),
      ).toBeVisible();
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth),
      ).toBeLessThanOrEqual(width);
      const trigger = page.getByRole("button", {
        name: "World menu",
        exact: true,
      });
      const before = await trigger.boundingBox();
      await trigger.click();
      const panel = page.getByRole("region", { name: "World actions" });
      await expect(panel).toBeVisible();
      const after = await trigger.boundingBox();
      expect(Math.abs(after.x - before.x)).toBeLessThan(1);
      expect(Math.abs(after.y - before.y)).toBeLessThan(1);
      const box = await panel.boundingBox();
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(width);
      expect(box.y + box.height).toBeLessThanOrEqual(844);
      await panel
        .getByRole("button", { name: "Create a world", exact: true })
        .click();
      await expect(
        page.getByLabel("World name", { exact: true }),
      ).toBeFocused();
      if (width <= 390)
        await page.screenshot({
          path: test.info().outputPath(`world-menu-${width}-${theme}.png`),
        });
      await page.keyboard.press("Escape");
      await expect(trigger).toBeFocused();
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth),
      ).toBeLessThanOrEqual(width);
      if (width <= 390)
        await page.screenshot({
          path: test.info().outputPath(`worlds-${width}-${theme}.png`),
        });
    }
  }
});

test("empty world library supports creation and closes its menu after success", async ({
  page,
}) => {
  const state = await fixture(page, { worlds: [], library: true });
  await page.getByRole("button", { name: "Worlds", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Your next story starts here" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Create a world", exact: true })
    .click();
  await page.getByLabel("World name", { exact: true }).fill("A new beginning");
  await page.getByRole("button", { name: "Create world", exact: true }).click();
  await expect(page.getByLabel("Question or creative direction")).toBeVisible();
  await expect(page.getByRole("region", { name: "World actions" })).toHaveCount(
    0,
  );
  await expect(
    page.getByLabel("Open world").locator("option:checked"),
  ).toHaveText("A new beginning");
  expect(
    state.calls.filter((call) => call.path === "/api/worlds" && call.body),
  ).toHaveLength(1);
});

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
  await expect(proposal).toContainText("Draft");
  expect(state.changes[0].baseRevision).toBe(7);
  expect(state.changes[0].operations[0].record.label).toBe("The Rival");
  await proposal
    .getByRole("button", { name: "Submit for review", exact: true })
    .click();
  await expect(proposal).toContainText("Ready for review");
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
  await page.getByRole("button", { name: "Create story", exact: true }).click();
  await page
    .getByLabel("Story title", { exact: true })
    .fill("Ashfall adventure");
  await page
    .getByLabel("Premise", { exact: true })
    .fill("Arrive at the gates.");
  await page.getByRole("button", { name: "Start at revision 7" }).click();
  await page
    .getByText("Characters and session details", { exact: true })
    .click();
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

test("scrolled desktop navigation and inspector remain below the sticky header", async ({
  page,
}) => {
  await fixture(page);
  await send(page, "Who is Aldric?");
  await page
    .getByRole("button", { name: "Open source · aldric", exact: true })
    .click();
  for (const size of [
    { width: 1280, height: 720 },
    { width: 1120, height: 600 },
  ]) {
    await page.setViewportSize(size);
    await page.evaluate(() =>
      window.scrollTo(0, document.documentElement.scrollHeight),
    );
    for (const target of [
      page.getByRole("link", { name: "Workspace", exact: true }),
      page.getByRole("button", { name: "Close inspector", exact: true }),
    ]) {
      await expect
        .poll(() =>
          target.evaluate((element) => {
            const box = element.getBoundingClientRect();
            const header = document
              .querySelector(".topbar")
              .getBoundingClientRect();
            const hit = document.elementFromPoint(
              box.x + box.width / 2,
              box.y + box.height / 2,
            );
            return (
              box.top >= header.bottom &&
              (hit === element || element.contains(hit))
            );
          }),
        )
        .toBe(true);
    }
  }
  await page
    .getByRole("button", { name: "Close inspector", exact: true })
    .click();
  await page.getByRole("link", { name: "Explore", exact: true }).click();
  await expect(
    page.getByRole("link", { name: "Explore", exact: true }),
  ).toHaveAttribute("aria-current", "page");
});

test("required viewports contain routes and expose the complete empty composer", async ({
  page,
}) => {
  await fixture(page);
  for (const size of [
    { width: 320, height: 844 },
    { width: 390, height: 844 },
    { width: 768, height: 900 },
    { width: 1024, height: 900 },
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
  ]) {
    await page.setViewportSize(size);
    for (const section of [
      "Workspace",
      "Explore",
      "Drafts & Proposals",
      "Stories",
      "World Settings",
    ]) {
      await nav(page, section);
      await page.waitForTimeout(100);
      await page.screenshot({
        path: test.info().outputPath(`route-${size.width}-${section}.png`),
      });
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
        .toBeLessThanOrEqual(size.width);
      if (section === "Workspace") {
        await page.screenshot({
          path: test
            .info()
            .outputPath(`empty-${size.width}x${size.height}.png`),
        });
        await expect
          .poll(() =>
            page.locator(".wb-composer").evaluate((node) => {
              const box = node.getBoundingClientRect();
              return box.top >= 0 && box.bottom <= window.innerHeight;
            }),
          )
          .toBe(true);
        await page.screenshot({
          path: test
            .info()
            .outputPath(`empty-${size.width}x${size.height}.png`),
        });
      }
    }
  }
});

test("mobile drawer and source sheet trap focus and preserve references across sections", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await fixture(page);
  await send(page, "Who is Aldric?");
  const source = page.getByRole("button", { name: "Open source · aldric" });
  await source.click();
  const sheet = page.getByRole("dialog", { name: "Source inspector" });
  await expect(
    page.getByRole("button", { name: "Close inspector" }),
  ).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  expect(
    await sheet.evaluate((node) => node.contains(document.activeElement)),
  ).toBe(true);
  await page.keyboard.press("Escape");
  await expect(source).toBeFocused();
  await source.click();
  await sheet.getByRole("button", { name: "Ask AI about this" }).click();
  await expect(
    page.getByRole("button", { name: /Remove context · King Aldric/ }),
  ).toBeVisible();
  const toggle = page.getByRole("button", {
    name: "World navigation",
    exact: true,
  });
  await toggle.click();
  const drawer = page.getByRole("dialog", { name: "World", exact: true });
  await expect(
    drawer.getByRole("button", { name: "Close navigation" }),
  ).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  expect(
    await drawer.evaluate((node) => node.contains(document.activeElement)),
  ).toBe(true);
  await page.keyboard.press("Escape");
  await expect(toggle).toBeFocused();
  await nav(page, "Explore");
  await nav(page, "Workspace");
  await expect(
    page.getByText("Aldric ruled the northern houses.", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /Remove context · King Aldric/ }),
  ).toBeVisible();
});

test("archived proposal compares blue and green without JSON at every required width", async ({
  page,
}) => {
  const gate = {
    id: "gate",
    kind: "entity",
    label: "City gate",
    attributes: { description: "The city entrance", colour: "blue" },
  };
  const proposed = {
    ...gate,
    attributes: { ...gate.attributes, colour: "green" },
  };
  const state = await fixture(page, {
    world: {
      archived: true,
      name: "An extremely long archived world name that must remain accessible on the narrowest viewport",
    },
    records: [
      gate,
      {
        id: "settings",
        kind: "world_settings",
        label: "World settings",
        timeline: "main",
      },
    ],
    changes: [
      {
        id: "change",
        reason: "City gate: blue → green",
        status: "draft",
        author: "editor@example.test",
        baseRevision: 7,
        operations: [{ action: "put", id: "gate", record: proposed }],
        approvals: [],
        requiredApprovals: 1,
      },
    ],
    comparison: {
      baseRevision: 7,
      currentRevision: 7,
      records: [
        { id: "gate", base: gate, current: gate, proposed, conflict: false },
      ],
    },
  });
  for (const width of [320, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await nav(page, "Drafts & Proposals");
    await page
      .getByRole("button", { name: "Compare proposal with current canon" })
      .click();
    const comparison = page.getByRole("group", { name: "Reconcile changes" });
    await expect(comparison).toContainText("Original");
    await expect(comparison).toContainText("Current canon");
    await expect(comparison).toContainText("Proposed");
    await expect(comparison).toContainText("blue");
    await expect(comparison).toContainText("green");
    await expect(
      page.getByText("0 of 1 required approvals.", { exact: true }),
    ).toBeVisible();
    if (width === 390 || width === 1440)
      await page.screenshot({
        path: test.info().outputPath(`comparison-${width}.png`),
        fullPage: true,
      });
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(width);
    await page.getByRole("button", { name: "Close comparison" }).click();
    await nav(page, "Explore");
    await expect(
      page.getByRole("heading", {
        name: "World settings World settings",
        exact: true,
      }),
    ).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(width);
  }
  expect(state.calls.some((c) => c.body && !c.path.endsWith("/reason"))).toBe(
    false,
  );
});

test("saved story opens directly on its scene and character selection uses the supported pin", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const session = {
    id: "saved",
    title: "Saved journey",
    pinnedRevision: 3,
    timeline: "main",
    version: 4,
    state: {},
    memory: {},
    definition: { premise: "A private premise", participants: ["aldric"] },
  };
  const state = await fixture(page, {
    sessions: [session],
    turns: [{ version: 4, narration: "You reach the old city gate." }],
  });
  await nav(page, "Stories");
  await page.getByRole("button", { name: "Resume Saved journey" }).click();
  const scene = page.getByRole("article", { name: "Saved scene" });
  await expect(scene).toContainText("You reach the old city gate.");
  const box = await scene.boundingBox();
  expect(box.y + box.height).toBeLessThan(844);
  await expect(page.getByLabel("Story title", { exact: true })).toHaveCount(0);
  await page.screenshot({ path: test.info().outputPath("story-mobile.png") });
  await page.getByLabel("Your action", { exact: true }).fill("Open the gate");
  await page
    .getByRole("button", { name: "Continue scene", exact: true })
    .click();
  await expect(
    page
      .locator("p")
      .filter({ hasText: /^The gates of Ashfall open before you\.$/ }),
  ).toBeVisible();
  expect(
    state.calls.find((c) => c.path.endsWith("/reason")).body,
  ).toMatchObject({
    workflow: "story_turn",
    revision: 3,
    sessionId: "saved",
    expectedVersion: 4,
  });
  expect(state.calls.some((c) => c.path.endsWith("/turns") && c.body)).toBe(
    false,
  );
  await page.getByRole("button", { name: "Back to story library" }).click();
  await page.getByRole("button", { name: "Create story", exact: true }).click();
  await page.getByLabel("Find characters or places").fill("Aldric");
  await page.getByRole("checkbox", { name: /King Aldric/ }).check();
  await expect(
    page.getByRole("button", { name: "Remove King Aldric" }),
  ).toBeVisible();
  expect(
    state.calls.filter((c) => c.query.kind === "entity").at(-1).query,
  ).toMatchObject({ revision: "7", timeline: "main", q: "Aldric" });
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
});

test("reduced mobile viewport keeps the focused composer and send control usable", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await fixture(page);
  const input = page.getByLabel("Question or creative direction");
  await input.focus();
  await page.setViewportSize({ width: 390, height: 500 });
  await input.fill("Who is Aldric?");
  await page.screenshot({ path: test.info().outputPath("reduced-mobile.png") });
  await expect
    .poll(() =>
      page.locator(".wb-composer").evaluate((node) => {
        const rect = node.getBoundingClientRect();
        return rect.top >= 0 && rect.bottom <= window.innerHeight;
      }),
    )
    .toBe(true);
  await expect(input).toBeFocused();
  const send = page.getByRole("button", { name: "Send message", exact: true });
  const box = await send.boundingBox();
  expect(box.y + box.height).toBeLessThanOrEqual(500);
  await send.click();
  await expect(
    page.getByText("Aldric ruled the northern houses.", { exact: true }),
  ).toBeVisible();
});

test("long replies and bounded multiline input preserve the composer and independent scrolling", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await fixture(page, { answer: "A grounded fixture paragraph. ".repeat(180) });
  await send(page, "Who is Aldric?");
  await expect(
    page.getByRole("article", { name: "ask response" }),
  ).toBeVisible();
  const transcript = page.getByRole("log", { name: "Conversation" });
  expect(
    await transcript.evaluate((node) => node.scrollHeight > node.clientHeight),
  ).toBe(true);
  await transcript.evaluate((node) => {
    node.scrollTop = 0;
  });
  await page
    .getByLabel("Question or creative direction")
    .fill("A line of direction\n".repeat(20));
  await expect
    .poll(() =>
      page
        .locator(".wb-composer")
        .evaluate((node) => node.getBoundingClientRect().bottom <= innerHeight),
    )
    .toBe(true);
  expect(
    await page
      .getByLabel("Question or creative direction")
      .evaluate((node) => node.clientHeight < node.scrollHeight),
  ).toBe(true);
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
});
