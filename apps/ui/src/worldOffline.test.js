import "fake-indexeddb/auto";
import { beforeEach, afterEach, describe, it, expect, vi } from "vitest";
import { openDB } from "idb";
import {
  bindDeviceAccount,
  enableOffline,
  clearDevice,
  worldRequest,
  cacheRead,
  deviceQueue,
  lockDevice,
  offlineIdentity,
  saveLocalDraft,
  loadLocalDraft,
  syncDevice,
  exportDevice,
  importDevice,
  registerLocalWriter,
  flushLocalWriting,
} from "./worldOffline.js";
const offline = async () => {
  throw Object.assign(Error("Disconnected"), { networkFailure: true });
};
const body = () => ({
  id: crypto.randomUUID(),
  baseRevision: 4,
  reason: "Observed collapse",
  operations: [
    {
      action: "put",
      id: "x",
      record: { id: "x", kind: "source", label: "Notes" },
    },
  ],
});
beforeEach(async () => {
  await bindDeviceAccount("author@example.test");
  await clearDevice();
  await bindDeviceAccount("author@example.test");
  await enableOffline();
});
afterEach(() => vi.unstubAllGlobals());
describe("Durable account-separated authoring", () => {
  it("preserves explicit saves and exact retries without automatically queuing AI", async () => {
    const request = body();
    await worldRequest("/w/changes", request, {}, offline);
    await worldRequest("/w/changes", request, {}, offline);
    expect(await deviceQueue()).toHaveLength(1);
    await expect(
      worldRequest(
        "/w/changes",
        { ...request, reason: "Changed" },
        {},
        offline,
      ),
    ).rejects.toThrow("immutable");
    await expect(
      worldRequest("/w/reason", { question: "Generate" }, {}, offline),
    ).rejects.toThrow("connection");
    expect((await deviceQueue())[0].body).toEqual(request);
  });
  it("commits the story queue, provisional version and receipt together and rejects stale writes", async () => {
    await cacheRead("/w/sessions/s", { id: "s", version: 0, state: {} });
    const turn = {
      requestId: crypto.randomUUID(),
      expectedVersion: 0,
      narration: "Walk",
      operations: [],
    };
    await worldRequest("/w/sessions/s/turns", turn, {}, offline);
    await worldRequest("/w/sessions/s/turns", turn, {}, offline);
    expect(
      await worldRequest("/w/sessions/s", undefined, {}, offline),
    ).toMatchObject({ version: 1 });
    expect(
      await worldRequest("/w/sessions/s/turns", undefined, {}, offline),
    ).toHaveLength(1);
    await expect(
      worldRequest(
        "/w/sessions/s/turns",
        { ...turn, requestId: crypto.randomUUID() },
        {},
        offline,
      ),
    ).rejects.toThrow("version");
    expect(await deviceQueue()).toHaveLength(1);
  });
  it("hides retained work after sign-out and from a different account", async () => {
    await saveLocalDraft("essay", { text: "Private" });
    lockDevice();
    expect(offlineIdentity()).toBeNull();
    expect(await loadLocalDraft("essay")).toBeNull();
    await bindDeviceAccount("reader@example.test");
    await enableOffline();
    expect(await loadLocalDraft("essay")).toBeUndefined();
    await bindDeviceAccount("author@example.test");
    await enableOffline();
    expect(await loadLocalDraft("essay")).toEqual({ text: "Private" });
  });
  it("does not substitute cached content for authorization refusals", async () => {
    await cacheRead("/w", { secret: "Stored" });
    const denied = async () => {
      throw Object.assign(Error("Sign in"), { status: 401 });
    };
    await expect(worldRequest("/w", undefined, {}, denied)).rejects.toThrow(
      "Sign in",
    );
  });
  it("retries a lost acknowledgement unchanged and serializes concurrent synchronizers", async () => {
    const request = body();
    await worldRequest("/w/changes", request, {}, offline);
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: true,
        json: async () => ({ valid: true, email: "author@example.test" }),
      })),
    );
    const delivered = [];
    let lost = true;
    const online = async (path, payload) => {
      if (!payload) return { capabilities: { offlineDrafts: true } };
      delivered.push(structuredClone(payload));
      if (lost) {
        lost = false;
        throw Error("Lost acknowledgement");
      }
      return { id: payload.id };
    };
    await syncDevice(online);
    expect((await deviceQueue())[0].status).toBe("queued");
    await Promise.all([syncDevice(online), syncDevice(online)]);
    expect(delivered).toEqual([request, request]);
    expect((await deviceQueue())[0].status).toBe("synced");
  });
  it("pauses divergent story turns and dependencies without changing versions", async () => {
    await cacheRead("/w/sessions/s", { id: "s", version: 0, state: {} });
    const clock = vi.spyOn(Date, "now").mockReturnValue(1000);
    for (let version = 0; version < 2; version++)
      await worldRequest(
        "/w/sessions/s/turns",
        {
          requestId: crypto.randomUUID(),
          expectedVersion: version,
          narration: "Walk",
          operations: [],
        },
        {},
        offline,
      );
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: true,
        json: async () => ({ valid: true, email: "author@example.test" }),
      })),
    );
    clock.mockRestore();
    const sent = [];
    await syncDevice(async (path, payload) => {
      if (!payload) return { capabilities: { offlineDrafts: true } };
      sent.push(payload);
      throw Object.assign(Error("Version changed"), { status: 409 });
    });
    expect(sent).toHaveLength(1);
    expect(sent[0].expectedVersion).toBe(0);
    expect((await deviceQueue()).map((q) => q.status)).toEqual([
      "conflict",
      "queued",
    ]);
  });
  it("roundtrips work and rolls back a corrupt import atomically", async () => {
    const request = body();
    await worldRequest("/w/changes", request, {}, offline);
    await saveLocalDraft("essay", { text: "Keep" });
    const backup = await exportDevice();
    await clearDevice();
    await bindDeviceAccount("author@example.test");
    await enableOffline();
    await importDevice(backup);
    expect((await deviceQueue())[0].body).toEqual(request);
    const bad = JSON.parse(backup);
    bad.queue = [
      { ...bad.queue[0], id: "fresh", body: { ...request, id: "fresh" } },
      { ...bad.queue[0], account: "other@example.test" },
    ];
    await expect(importDevice(JSON.stringify(bad))).rejects.toThrow();
    expect(await deviceQueue()).toHaveLength(1);
  });
  it("awaits local writing before update and propagates quota failure", async () => {
    const stop = registerLocalWriter(async () => {
      throw new DOMException("Storage full", "QuotaExceededError");
    });
    await expect(flushLocalWriting()).rejects.toThrow("Storage full");
    stop();
    const put = vi
      .spyOn(IDBObjectStore.prototype, "put")
      .mockImplementation(() => {
        throw new DOMException("Storage full", "QuotaExceededError");
      });
    await expect(saveLocalDraft("quota", { text: "Keep" })).rejects.toThrow(
      "Storage full",
    );
    await expect(
      worldRequest("/w/changes", body(), {}, offline),
    ).rejects.toThrow("Storage full");
    put.mockRestore();
    expect(await deviceQueue()).toHaveLength(0);
  });
});
