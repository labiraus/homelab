import assert from "node:assert/strict";
import test, { type TestContext } from "node:test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { once } from "node:events";

import { createDocumentStorage, decodeUtf8Text, parseMinioEndpoint } from "../src/storage.js";

async function fixture(t: TestContext, handler: (request: IncomingMessage, response: ServerResponse) => void) {
	const server = createServer(handler);
	server.listen(0, "127.0.0.1");
	await once(server, "listening");
	t.after(() => { server.closeAllConnections(); server.close(); });
	const address = server.address();
	if (!address || typeof address === "string") throw new Error("missing fixture address");
	return `127.0.0.1:${address.port}`;
}

function storage(endpoint: string) {
	return createDocumentStorage({ minioEndpoint: endpoint, minioUseSSL: false,
		minioAccessKey: "adapter-test", minioSecretKey: "adapter-test-secret",
		minioRegion: "eu-west-2", minioBucket: "documents" });
}

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

test("rejects credentials, paths, queries and conflicting TLS configuration", () => {
	for (const endpoint of ["http://user:secret@storage.invalid", "http://storage.invalid/path", "storage.invalid?token=x", "storage.invalid#fragment", "https://storage.invalid"])
		assert.throws(() => parseMinioEndpoint(endpoint, false));
	assert.throws(() => parseMinioEndpoint("http://storage.invalid", true));
	assert.deepEqual(parseMinioEndpoint("https://storage.invalid:9443/", true), { endPoint: "storage.invalid", port: 9443 });
});

test("signs path-style reads with configured credentials and region, preserving opaque keys and bytes", async (t) => {
	const content = Buffer.from([0, 1, 0xff, 0xfe, 2]);
	let requests = 0;
	const endpoint = await fixture(t, (request, response) => {
		requests++;
		assert.equal(request.method, "GET");
		assert.equal(new URL(request.url!, "http://fixture").pathname, "/documents/folder/caf%C3%A9%20%2B%3F%23.bin");
		assert.match(request.headers.authorization!, /^AWS4-HMAC-SHA256 Credential=adapter-test\/\d{8}\/eu-west-2\/s3\/aws4_request,/);
		response.writeHead(200, { "content-length": content.length });
		response.end(content);
	});
	assert.deepEqual(await storage(endpoint).readDocument("", "folder/café +?#.bin"), content);
	assert.equal(requests, 1);
});

test("empty objects remain successful empty buffers", async (t) => {
	const endpoint = await fixture(t, (_request, response) => { response.writeHead(200, { "content-length": 0 }); response.end(); });
	assert.deepEqual(await storage(endpoint).readDocument("documents", "empty"), Buffer.alloc(0));
});

test("queued versions and ETags stay bound to the signed read, including literal null", async (t) => {
	for (const versionMarker of ["null", "opaque/+?version"]) {
		const endpoint = await fixture(t, (request, response) => {
			assert.equal(new URL(request.url!, "http://fixture").searchParams.get("versionId"), versionMarker);
			assert.equal(request.headers["if-match"], '"expected-etag"');
			response.end("pinned content");
		});
		assert.equal((await storage(endpoint).readDocument("documents", "history.txt", {
			versionMarker, etag: "expected-etag",
		})).toString(), "pinned content");
	}
});

test("changed source content fails without retrying an unpinned read", async (t) => {
	let requests = 0;
	const endpoint = await fixture(t, (request, response) => {
		requests++;
		assert.equal(request.headers["if-match"], '"old-etag"');
		response.writeHead(412, { "content-type": "application/xml" });
		response.end("<Error><Code>PreconditionFailed</Code><Message>source changed</Message></Error>");
	});
	await assert.rejects(storage(endpoint).readDocument("documents", "history.txt", {
		versionMarker: "null", etag: '"old-etag"',
	}), { code: "PreconditionFailed" });
	assert.equal(requests, 1);
});

test("missing objects and access denial preserve explicit S3 error codes", async (t) => {
	for (const [status, code] of [[404, "NoSuchKey"], [403, "AccessDenied"]] as const) {
		const endpoint = await fixture(t, (_request, response) => {
			response.writeHead(status, { "content-type": "application/xml" });
			response.end(`<Error><Code>${code}</Code><Message>fixture refusal</Message></Error>`);
		});
		await assert.rejects(storage(endpoint).readDocument("documents", "unavailable"), { code });
	}
});

test("does not forward signed reads to a redirect destination", async (t) => {
	let redirected = 0;
	const trap = await fixture(t, (_request, response) => { redirected++; response.end("unexpected"); });
	const endpoint = await fixture(t, (_request, response) => {
		response.writeHead(302, { location: `http://${trap}/private`, "content-type": "application/xml" });
		response.end("<Error><Code>PermanentRedirect</Code><Message>fixture redirect</Message></Error>");
	});
	await assert.rejects(storage(endpoint).readDocument("documents", "private"));
	assert.equal(redirected, 0);
});

test("truncated object streams fail instead of returning partial document content", async (t) => {
	const endpoint = await fixture(t, (_request, response) => {
		response.writeHead(200, { "content-length": 100 });
		response.write("partial");
		setImmediate(() => response.destroy());
	});
	await assert.rejects(storage(endpoint).readDocument("documents", "truncated"));
});
