// Loopback-only production-asset fixture. This is not an authentication bypass in the app.
import http from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
const root = resolve("dist");
let release = 1;
const drafts = new Map();
const world = {
  id: "pwa-world",
  name: "The Shattered Empire",
  revision: 7,
  role: "author",
  defaultTimeline: "main",
  publicationPolicy: { mode: "independent", requiredApprovals: 1 },
  policyVersion: 1,
  capabilities: {
    author: true,
    offlineDrafts: true,
    establishedAccounts: true,
    conversations: true,
    sandbox: true,
    guides: true,
  },
};
const records = [
  { id: "main", kind: "timeline", label: "Primary timeline" },
  {
    id: "settings",
    kind: "world_settings",
    label: world.name,
    timeline: "main",
    worldGuide: "An empire divided by conflicting chronicles.",
    styleGuide: "Concrete observations, uncertain interpretations.",
  },
  {
    id: "source",
    kind: "source",
    label: "The northern chronicle",
    evidenceStatus: "reported",
    attributes: {
      description: "The chronicler claims the western bridge survived.",
    },
  },
  {
    id: "witness",
    kind: "established_account",
    label: "The western bridge",
    synopsis:
      "The party witnessed the western bridge collapse during the siege.",
    timeline: "main",
    references: ["source"],
  },
];
http
  .createServer(async (req, res) => {
    const url = new URL(req.url, "http://127.0.0.1");
    let raw = "";
    for await (const chunk of req) raw += chunk;
    const body = raw ? JSON.parse(raw) : undefined;
    const json = (data, status = 200) => {
      res.writeHead(status, {
        "Content-Type": "application/json",
        "Cache-Control": "no-store",
      });
      res.end(JSON.stringify(data));
    };
    if (url.pathname === "/__fixture/update") {
      release++;
      return json({ release });
    }
    if (url.pathname === "/api/auth/status" && url.searchParams.has("refuse"))
      return json({ error: "Sign in" }, 401);
    if (url.pathname === "/api/auth/status")
      return json({ valid: true, email: "author@example.test" });
    if (url.pathname === "/api/auth/providers") return json({ providers: [] });
    if (url.pathname === "/api/worlds") return json([world]);
    if (url.pathname === "/api/worlds/account")
      return json({ email: "author@example.test", superuser: false });
    if (url.pathname === "/api/worlds/pwa-world") return json(world);
    if (url.pathname === "/api/worlds/pwa-world/changes") {
      if (body) {
        const old = drafts.get(body.id);
        if (old && JSON.stringify(old.body) !== JSON.stringify(body))
          return json({ error: "Request changed" }, 409);
        if (body.baseRevision !== 7)
          return json({ error: "Canon changed" }, 409);
        drafts.set(body.id, {
          body,
          result: {
            ...body,
            status: "draft",
            author: "author@example.test",
            reviews: [],
            approvals: [],
            requiredApprovals: 1,
          },
        });
        return json(drafts.get(body.id).result);
      }
      return json([...drafts.values()].map((d) => d.result));
    }
    if (url.pathname.endsWith("/established-accounts"))
      return json({
        records: records.filter((r) => r.kind === "established_account"),
        successors: {},
        origins: { witness: { actor: "Mira", revision: 6 } },
        revision: 7,
      });
    if (url.pathname.endsWith("/canon")) {
      const filtered = records.filter(
        (r) =>
          !url.searchParams.get("kind") ||
          url.searchParams.get("kind").split(",").includes(r.kind),
      );
      return json({
        world,
        records: filtered,
        total: filtered.length,
        revision: 7,
        timeline: "main",
      });
    }
    if (url.pathname.endsWith("/generations"))
      return json({ enabled: false, items: [] });
    if (url.pathname.startsWith("/api/")) return json([]);
    try {
      const file = resolve(
        root,
        url.pathname === "/" ? "index.html" : `.${url.pathname}`,
      );
      if (!file.startsWith(root + "/")) {
        res.writeHead(403);
        return res.end();
      }
      let content = await readFile(file);
      if (url.pathname === "/sw.js")
        content = Buffer.concat([
          content,
          Buffer.from(`\n// fixture release ${release}\n`),
        ]);
      res.writeHead(200, {
        "Content-Type":
          {
            ".html": "text/html",
            ".js": "application/javascript",
            ".css": "text/css",
            ".webmanifest": "application/manifest+json",
            ".png": "image/png",
            ".woff2": "font/woff2",
            ".woff": "font/woff",
          }[extname(file)] || "application/octet-stream",
        "Cache-Control": "no-store",
      });
      res.end(content);
    } catch {
      res.writeHead(404);
      res.end("Not found");
    }
  })
  .listen(4176, "127.0.0.1");
