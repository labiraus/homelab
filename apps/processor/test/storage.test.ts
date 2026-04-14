import assert from "node:assert/strict";
import test from "node:test";

import { decodeUtf8Text, parseMinioEndpoint } from "../src/storage.js";

test("parses MinIO host and port from a bare endpoint", () => {
	assert.deepEqual(parseMinioEndpoint("svartalfheim:9000", false), {
		endPoint: "svartalfheim",
		port: 9000,
	});
});

test("decodes UTF-8 document text", () => {
	assert.equal(decodeUtf8Text(Buffer.from("hello world", "utf8")), "hello world");
});

test("rejects invalid UTF-8 document text", () => {
	assert.throws(() => decodeUtf8Text(Buffer.from([0xff, 0xfe])));
});
