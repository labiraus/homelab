import { openDB } from "idb";
import { parseWorldJSON } from "./worldTicks.js";

let actor = "";
let database;
const writers = new Set();
export function registerLocalWriter(write) {
  writers.add(write);
  return () => writers.delete(write);
}
export async function flushLocalWriting() {
  await Promise.all([...writers].map((write) => write()));
}
let installation;
if (typeof window !== "undefined")
  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault();
    installation = e;
    changed();
  });
export function installationPrompt() {
  return installation;
}
export function consumedInstallation() {
  installation = null;
  changed();
}

const listeners = new Set();
const deviceKey = "worldbuilder-device";
const channel =
  typeof BroadcastChannel !== "undefined"
    ? new BroadcastChannel("worldbuilder-device")
    : null;
export const onDeviceChange = (fn) => {
  listeners.add(fn);
  return () => listeners.delete(fn);
};
function changed() {
  for (const fn of listeners) fn();
}
channel?.addEventListener("message", (e) => {
  if (
    e.data?.locked ||
    (e.data?.account && actor && e.data.account !== actor)
  ) {
    actor = "";
    window.dispatchEvent(new Event("worldbuilder-signed-out"));
  }
  changed();
});
const db = () =>
  (database ||= openDB("worldbuilder-device-v1", 1, {
    upgrade(d) {
      for (const name of ["meta", "responses", "drafts", "queue", "leases"])
        d.createObjectStore(name);
    },
  }));
export function setDeviceActor(email) {
  actor = email?.trim().toLowerCase() || "";
}
export function deviceActor() {
  return actor;
}
function device() {
  try {
    return JSON.parse(localStorage.getItem(deviceKey) || "{}");
  } catch {
    return {};
  }
}
export async function bindDeviceAccount(email) {
  setDeviceActor(email);
  const state = device();
  if (state.account && state.account !== actor)
    channel?.postMessage({ account: actor });
  localStorage.setItem(
    deviceKey,
    JSON.stringify({
      ...state,
      enabled: state.account === actor && state.enabled === true,
      account: actor,
      locked: false,
    }),
  );
  changed();
}
export function lockDevice() {
  actor = "";
  window.dispatchEvent(new Event("worldbuilder-signed-out"));
  localStorage.setItem(
    deviceKey,
    JSON.stringify({ ...device(), locked: true }),
  );
  channel?.postMessage({ locked: true });
  changed();
}
export function offlineIdentity() {
  const state = device();
  return state.enabled && !state.locked && state.account
    ? {
        valid: true,
        email: state.account,
        offline: true,
        mode: "offline device",
      }
    : null;
}
export function offlineEnabled() {
  const state = device();
  return (
    !!actor &&
    state.enabled === true &&
    state.account === actor &&
    !state.locked
  );
}
export async function enableOffline() {
  if (!actor) throw Error("Sign in before enabling offline access.");
  await db();
  localStorage.setItem(
    deviceKey,
    JSON.stringify({ account: actor, enabled: true, locked: false }),
  );
  await navigator.storage?.persist?.();
  changed();
}
function key(path, account = actor) {
  return `${account}\n${path}`;
}
export async function cacheRead(path, value) {
  if (!offlineEnabled()) return;
  const account = actor;
  const d = await db();
  if (account !== actor) throw Error("Account changed.");
  await d.put("responses", { value, savedAt: Date.now() }, key(path, account));
}
export async function saveLocalDraft(name, value) {
  if (!offlineEnabled()) throw Error("Offline writing is not enabled.");
  const account = actor;
  const d = await db();
  if (account !== actor) throw Error("Account changed.");
  await d.put("drafts", value, key(name, account));
  changed();
}
export async function loadLocalDraft(name) {
  if (!offlineEnabled()) return null;
  const account = actor;
  const value = await (await db()).get("drafts", key(name, account));
  if (account !== actor) throw Error("Account changed.");
  return value;
}
export async function deviceQueue() {
  if (!offlineEnabled()) return [];
  const account = actor;
  const d = await db();
  const keys = await d.getAllKeys("queue");
  const rows = (
    await Promise.all(
      keys
        .filter((k) => k.startsWith(`${account}\n`))
        .map((k) => d.get("queue", k)),
    )
  ).sort((a, b) => (a.sequence ?? a.createdAt) - (b.sequence ?? b.createdAt));
  if (account !== actor) throw Error("Account changed.");
  return rows;
}
export async function removeQueued(id) {
  await (await db()).delete("queue", key(id));
  changed();
}
function validQueuePath(path) {
  return (
    /^\/[^/?]+\/changes$/.test(path) ||
    /^\/[^/?]+\/sessions\/[^/?]+\/turns$/.test(path)
  );
}
async function offlineRead(path) {
  const d = await db();
  const exact = await d.get("responses", key(path));
  if (exact)
    return /\/canon(?:\?|$)/.test(path)
      ? { ...exact.value, offline: true, downloadedAt: exact.savedAt }
      : exact.value;
  const url = new URL(`https://offline.invalid${path || "/"}`);
  const parts = url.pathname.split("/").filter(Boolean);
  if (
    [...url.searchParams.keys()].every((k) => ["limit", "offset"].includes(k))
  ) {
    const cached = await d.get("responses", key(url.pathname));
    if (Array.isArray(cached?.value))
      return cached.value.slice(
        Number(url.searchParams.get("offset") || 0),
        Number(url.searchParams.get("offset") || 0) +
          Number(url.searchParams.get("limit") || 100),
      );
  }
  if (!parts.length) {
    const entry = await d.get("responses", key(""));
    if (entry) return entry.value;
  }
  if (parts.length === 2 && ["canon", "search"].includes(parts[1])) {
    const timeline = url.searchParams.get("timeline");
    const revision = url.searchParams.get("revision");
    const all = await d.getAll("responses");
    const found = all
      .filter(
        (e) =>
          e.account === actor &&
          e.snapshot &&
          e.value.world.id === parts[0] &&
          e.value.timeline === timeline &&
          (!revision || String(e.value.revision) === revision),
      )
      .sort((a, b) => b.value.revision - a.value.revision)[0];
    if (found) {
      const v = found.value;
      const q = (url.searchParams.get("q") || "").toLowerCase();
      const kinds = (url.searchParams.get("kind") || "")
        .split(",")
        .filter(Boolean);
      const at = url.searchParams.get("at");
      const rows = v.records.filter(
        (r) =>
          (!q || JSON.stringify(r).toLowerCase().includes(q)) &&
          (!kinds.length || kinds.includes(r.kind)) &&
          (at === null ||
            ((r.from == null || BigInt(r.from) <= BigInt(at)) &&
              (r.until == null || BigInt(r.until) > BigInt(at)))),
      );
      const offset = Number(url.searchParams.get("offset") || 0),
        limit = Number(url.searchParams.get("limit") || 100);
      return {
        ...v,
        records: rows.slice(offset, offset + limit),
        total: rows.length,
        offline: true,
        downloadedAt: found.savedAt,
      };
    }
  }
  if (parts.length === 3 && parts[1] === "records") {
    const all = await offlineRead(
      `/${parts[0]}/canon?${url.searchParams}&limit=5000`,
    );
    const record = all.records.find((r) => r.id === parts[2]);
    if (record)
      return {
        record,
        citationId: `${parts[0]}:${all.revision}:${all.timeline}:${record.id}`,
      };
  }
  throw Error(
    "This view has not been downloaded. Reconnect to load its exact scope.",
  );
}
async function enqueue(path, body) {
  if (!validQueuePath(path))
    throw Error(
      "This action requires a connection. Your writing can be saved locally.",
    );
  const account = actor;
  const id = body.id || body.requestId;
  if (!id) throw Error("A stable request ID is required.");
  const d = await db();
  if (account !== actor) throw Error("Account changed.");
  const tx = d.transaction(["queue", "responses", "meta"], "readwrite");
  try {
    const k = key(id, account),
      existing = await tx.objectStore("queue").get(k);
    if (
      existing &&
      (existing.path !== path ||
        JSON.stringify(existing.body) !== JSON.stringify(body))
    )
      throw Error(
        "This saved request is immutable; save edited work as a new draft.",
      );
    if (existing) {
      await tx.done;
      return existing.receipt;
    }
    const responses = tx.objectStore("responses");
    let receipt;
    if (path.endsWith("/turns")) {
      const sessionPath = path.slice(0, -6);
      const session = (await responses.get(key(sessionPath, account)))?.value;
      if (!session || session.version !== body.expectedVersion)
        throw Error(
          "Offline story version changed or is unavailable. Inspect the saved session.",
        );
      const state = { ...session.state };
      for (const op of body.operations || []) {
        if (op.action !== "put")
          throw Error(
            "Offline story transitions only support provisional puts.",
          );
        state[op.id || op.record.id] = op.record;
      }
      const next = {
        ...session,
        version: body.expectedVersion + 1,
        state,
        scene: body.scene || session.scene,
      };
      receipt = {
        version: next.version,
        state,
        scene: next.scene,
        local: true,
      };
      await responses.put(
        { value: next, savedAt: Date.now() },
        key(sessionPath, account),
      );
      const turns = (await responses.get(key(path, account)))?.value || [];
      await responses.put(
        {
          value: [{ ...body, version: next.version, local: true }, ...turns],
          savedAt: Date.now(),
        },
        key(path, account),
      );
    } else {
      receipt = {
        ...body,
        status: "draft",
        author: account,
        local: true,
        approvals: [],
        reviews: [],
      };
      const cached = (await responses.get(key(path, account)))?.value || [];
      await responses.put(
        {
          value: [receipt, ...cached.filter((v) => v.id !== id)],
          savedAt: Date.now(),
        },
        key(path, account),
      );
      await responses.put(
        { value: receipt, savedAt: Date.now() },
        key(`${path}/${id}`, account),
      );
    }
    const sequence =
      ((await tx.objectStore("meta").get(key("sequence", account))) || 0) + 1;
    await tx.objectStore("meta").put(sequence, key("sequence", account));
    await tx.objectStore("queue").put(
      {
        id,
        path,
        body: structuredClone(body),
        receipt,
        account,
        status: "queued",
        createdAt: Date.now(),
        sequence,
      },
      k,
    );
    await tx.done;
    changed();
    channel?.postMessage({ changed: true });
    return receipt;
  } catch (e) {
    try {
      tx.abort();
    } catch {}
    await tx.done.catch(() => {});
    throw e;
  }
}
export async function worldRequest(path, body, options, online) {
  const account = actor;
  if (body !== undefined && offlineEnabled() && validQueuePath(path)) {
    const receipt = await enqueue(path, body);
    // Persist before any delivery. Foreground replay owns authentication, ordering
    // and exact retries even when the first save happens while connected.
    try {
      await syncDevice(online);
    } catch (error) {
      if (error.name !== "TypeError" && !error.networkFailure) throw error;
    }
    if (account !== actor)
      throw Error(
        "Sign in with the same account to synchronize saved writing.",
      );
    const item = await (
      await db()
    ).get("queue", key(body.id || body.requestId, account));
    if (["conflict", "invalid", "denied"].includes(item?.status))
      throw Object.assign(Error(item.error), {
        status:
          item.status === "conflict"
            ? 409
            : item.status === "invalid"
              ? 422
              : 403,
      });
    return item?.status === "synced" ? item.receipt : receipt;
  }
  try {
    const value = await online(path, body, options);
    if (account !== actor) throw Error("Account changed.");
    if (body === undefined) await cacheRead(path, value);
    return value;
  } catch (error) {
    if (
      account !== actor ||
      !error.networkFailure ||
      error.status ||
      error.name === "AbortError" ||
      !offlineEnabled()
    )
      throw error;
    const value =
      body === undefined ? await offlineRead(path) : await enqueue(path, body);
    if (account !== actor) throw Error("Account changed.");
    return value;
  }
}
export async function downloadWorld(worldId, timeline, online) {
  if (!offlineEnabled()) throw Error("Enable offline access first.");
  const account = actor;
  const world = await online(`/${worldId}`);
  const revision = world.revision;
  const records = [];
  let page;
  do {
    page = await online(
      `/${worldId}/canon?timeline=${encodeURIComponent(timeline)}&revision=${revision}&limit=500&offset=${records.length}`,
    );
    records.push(...page.records);
    if (page.records.length === 0 && records.length < page.total)
      throw Error("Incomplete download.");
  } while (records.length < page.total);
  if (records.length !== page.total || account !== actor)
    throw Error("Download scope changed.");
  const d = await db();
  if (account !== actor) throw Error("Account changed.");
  const snapshot = {
    ...page,
    records,
    world,
    revision,
    timeline,
    complete: true,
  };
  await d.put(
    "responses",
    { account, snapshot: true, value: snapshot, savedAt: Date.now() },
    key(`snapshot/${worldId}/${timeline}/${revision}`, account),
  );
  for (const path of [
    "",
    `/${worldId}`,
    `/${worldId}/changes`,
    `/${worldId}/sessions`,
    `/${worldId}/conversations`,
    `/${worldId}/story-definitions`,
  ]) {
    const value = await online(path);
    if (account !== actor) throw Error("Account changed.");
    await cacheRead(path, value);
  }
  const collect = async (path) => {
    const all = [];
    let page;
    do {
      page = await online(`${path}?limit=100&offset=${all.length}`);
      all.push(...page);
      if (account !== actor) throw Error("Account changed.");
    } while (page.length === 100);
    await cacheRead(path, all);
    return all;
  };
  for (const kind of ["changes", "story-definitions"])
    await collect(`/${worldId}/${kind}`);
  const sessions = await collect(`/${worldId}/sessions`);
  for (const session of sessions) {
    const path = `/${worldId}/sessions/${session.id}`;
    const value = await online(path);
    if (account !== actor) throw Error("Account changed.");
    await cacheRead(path, value);
    await collect(`${path}/turns`);
  }
  const conversations = await collect(`/${worldId}/conversations`);
  for (const c of conversations)
    await collect(`/${worldId}/conversations/${c.id}/messages`);
  const accountsPath = `/${worldId}/established-accounts?revision=${revision}&timeline=${encodeURIComponent(timeline)}`;
  const accounts = await online(accountsPath);
  if (account !== actor) throw Error("Account changed.");
  await cacheRead(accountsPath, accounts);
  changed();
  return records.length;
}
export async function syncDevice(online) {
  if (!offlineEnabled()) return;
  const account = actor;
  const leaseOwner = crypto.randomUUID();
  const d = await db();
  const tx = d.transaction("leases", "readwrite");
  const lease = await tx.store.get(account);
  if (lease?.until > Date.now()) {
    await tx.done;
    return;
  }
  await tx.store.put({ owner: leaseOwner, until: Date.now() + 30000 }, account);
  await tx.done;
  try {
    const auth = await fetch("/api/auth/status", {
      cache: "no-store",
      redirect: "error",
    });
    if (!auth.ok) {
      if (auth.status === 401 || auth.status === 403) {
        for (const item of await deviceQueue())
          if (item.status !== "synced")
            await d.put(
              "queue",
              { ...item, status: "sign-in", error: "Sign in to synchronize." },
              key(item.id, account),
            );
        lockDevice();
      }
      throw Error("Sign in to synchronize.");
    }
    const identity = await auth.json();
    if (!identity.valid || identity.email?.toLowerCase() !== account) {
      lockDevice();
      return;
    }
    const blocked = new Set();
    for (const item of await deviceQueue()) {
      if (account !== actor) break;
      if (item.status === "synced") continue;
      const world = item.path.split("/")[1];
      const chain = item.path.includes("/sessions/") ? item.path : world;
      if (
        blocked.has(chain) ||
        ["conflict", "denied", "invalid"].includes(item.status)
      ) {
        blocked.add(chain);
        continue;
      }
      const claim = d.transaction("leases", "readwrite");
      const current = await claim.store.get(account);
      if (current?.owner !== leaseOwner) {
        await claim.done;
        break;
      }
      await claim.store.put(
        { owner: leaseOwner, until: Date.now() + 30000 },
        account,
      );
      await claim.done;
      try {
        const worldState = await online(`/${world}`);
        if (worldState.archived)
          throw Object.assign(Error("World is archived."), { status: 409 });
        if (!worldState.capabilities?.offlineDrafts)
          throw Object.assign(
            Error("Server does not support offline synchronization."),
            { status: 503 },
          );
        if (account !== actor) break;
        const renew = d.transaction("leases", "readwrite");
        const held = await renew.store.get(account);
        if (held?.owner !== leaseOwner || held.until < Date.now()) {
          await renew.done;
          break;
        }
        await renew.store.put(
          { owner: leaseOwner, until: Date.now() + 30000 },
          account,
        );
        await renew.done;
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 15000);
        let receipt;
        try {
          receipt = await online(item.path, item.body, {
            signal: controller.signal,
          });
        } finally {
          clearTimeout(timer);
        }
        await d.put(
          "queue",
          { ...item, receipt, status: "synced", error: "" },
          key(item.id, account),
        );
      } catch (error) {
        const status =
          error.status === 409
            ? "conflict"
            : error.status === 401
              ? "sign-in"
              : error.status === 403 || error.status === 404
                ? "denied"
                : error.status === 422
                  ? "invalid"
                  : "queued";
        await d.put(
          "queue",
          { ...item, status, error: error.message },
          key(item.id, account),
        );
        blocked.add(chain);
        if (status === "sign-in") {
          lockDevice();
          break;
        }
      }
    }
  } finally {
    const release = d.transaction("leases", "readwrite");
    if ((await release.store.get(account))?.owner === leaseOwner)
      await release.store.delete(account);
    await release.done;
    changed();
  }
}
export async function exportDevice() {
  const account = actor;
  if (!offlineEnabled()) throw Error("Sign in to export local writing.");
  const d = await db();
  const result = {
    version: 1,
    account,
    queue: await deviceQueue(),
    drafts: [],
  };
  for (const k of await d.getAllKeys("drafts"))
    if (k.startsWith(`${account}\n`))
      result.drafts.push([k, await d.get("drafts", k)]);
  if (account !== actor) throw Error("Account changed.");
  return JSON.stringify(result, null, 2);
}
export async function importDevice(text) {
  const account = actor;
  if (!offlineEnabled())
    throw Error("Enable offline writing before importing.");
  const value = parseWorldJSON(text);
  if (
    value.version !== 1 ||
    value.account !== account ||
    !Array.isArray(value.queue) ||
    !Array.isArray(value.drafts)
  )
    throw Error(
      "Import must belong to this account and use the supported format.",
    );
  const d = await db();
  const tx = d.transaction(["queue", "drafts", "meta"], "readwrite");
  try {
    for (const item of value.queue) {
      if (
        item.account !== account ||
        !validQueuePath(item.path) ||
        item.id !== (item.body?.id || item.body?.requestId)
      )
        throw Error("Invalid saved request.");
      const k = key(item.id, account);
      const old = await tx.objectStore("queue").get(k);
      if (old && JSON.stringify(old.body) !== JSON.stringify(item.body)) {
        tx.abort();
        throw Error("Conflicting request ID in import.");
      }
      if (!old) {
        const sequence =
          ((await tx.objectStore("meta").get(key("sequence", account))) || 0) +
          1;
        await tx.objectStore("meta").put(sequence, key("sequence", account));
        await tx.objectStore("queue").put(
          {
            ...item,
            sequence,
            status: item.status === "synced" ? "synced" : "queued",
          },
          k,
        );
      }
    }
    for (const [k, v] of value.drafts) {
      if (!k.startsWith(`${account}\n`)) {
        tx.abort();
        throw Error("Wrong draft account.");
      }
      await tx.objectStore("drafts").put(v, k);
    }
    if (account !== actor) throw Error("Account changed.");
    await tx.done;
    changed();
  } catch (e) {
    try {
      tx.abort();
    } catch {}
    await tx.done.catch(() => {});
    throw e;
  }
}
export async function clearDevice() {
  const account = actor;
  const d = await db();
  for (const store of ["queue", "drafts", "responses", "leases", "meta"]) {
    const tx = d.transaction(store, "readwrite");
    for (const k of await tx.store.getAllKeys())
      if (k === account || k.startsWith(`${account}\n`))
        await tx.store.delete(k);
    await tx.done;
  }
  if (account === actor) localStorage.removeItem(deviceKey);
  changed();
}
