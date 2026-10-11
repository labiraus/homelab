import assert from "node:assert/strict";
import test from "node:test";
import { createDocumentStorage } from "../src/storage.js";

test("actual processor SDK reads SeaweedFS bytes and enforces bucket isolation", {
  skip: process.env.SEAWEEDFS_CLIENT_TEST !== "1",
}, async () => {
  const storage = createDocumentStorage({
    minioEndpoint: "127.0.0.2:8333", minioUseSSL: false, minioRegion: "us-east-1",
    minioAccessKey: "test-documents", minioSecretKey: "test-secret-documents", minioBucket: "documents",
  });
  assert.equal((await storage.readDocument("documents", "sdk/node.txt")).toString("utf8"), "Go → Node: café 🌍");
  assert.equal((await storage.readDocument("", "sdk/node.txt")).toString("utf8"), "Go → Node: café 🌍");
  await assert.rejects(storage.readDocument("cnpg-backups", "probe"), { code: "AccessDenied" });
  await assert.rejects(storage.readDocument("documents", "sdk/history.txt"));
});
