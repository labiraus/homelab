import assert from "node:assert/strict";
import test from "node:test";
import { PutObjectCommand, S3Client } from "@aws-sdk/client-s3";
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
  const writer = new S3Client({endpoint: "http://127.0.0.2:8333", region: "us-east-1", forcePathStyle: true,
    credentials: {accessKeyId: "test-documents", secretAccessKey: "test-secret-documents"}});
  try {
    const original = await writer.send(new PutObjectCommand({Bucket: "documents", Key: "sdk/node-version.txt", Body: "queued original"}));
    await writer.send(new PutObjectCommand({Bucket: "documents", Key: "sdk/node-version.txt", Body: "newer source"}));
    assert.ok(original.VersionId && original.ETag);
    assert.equal((await storage.readDocument("documents", "sdk/node-version.txt", {versionMarker: original.VersionId, etag: original.ETag})).toString(), "queued original");
    assert.equal((await storage.readDocument("documents", "sdk/node-version.txt")).toString(), "newer source");
    await assert.rejects(storage.readDocument("documents", "sdk/node-version.txt", {etag: original.ETag}), {code: "PreconditionFailed"});
  } finally { writer.destroy(); }
});
