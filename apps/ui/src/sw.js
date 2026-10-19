import {
  precache,
  matchPrecache,
  cleanupOutdatedCaches,
} from "workbox-precaching";
import { registerRoute } from "workbox-routing";

// Only the build's explicit static asset manifest is precached. No API caching,
// credential persistence, mutation replay, OAuth or MCP route interception.
precache(self.__WB_MANIFEST);
registerRoute(
  ({ request, url }) =>
    request.method === "GET" &&
    request.mode !== "navigate" &&
    url.origin === self.location.origin &&
    (/^\/assets\//.test(url.pathname) ||
      /^\/worldbuilder-(192|512)\.png$/.test(url.pathname)),
  async ({ request }) => (await matchPrecache(request.url)) || fetch(request),
);
cleanupOutdatedCaches();
registerRoute(
  ({ request, url }) =>
    request.mode === "navigate" &&
    url.origin === self.location.origin &&
    (url.pathname === "/" || url.pathname === "/index.html"),
  async ({ request }) => {
    try {
      return await fetch(request);
    } catch {
      return (await matchPrecache("/index.html")) || Response.error();
    }
  },
);
self.addEventListener("message", (event) => {
  if (event.data?.type === "SKIP_WAITING") self.skipWaiting();
});
