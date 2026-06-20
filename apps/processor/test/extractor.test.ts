import assert from "node:assert/strict";
import test from "node:test";

import { decodeHTMLEntities, extractDocumentContent } from "../src/extractor.js";

test("extracts visible text and heading metadata from html documents", () => {
	const extracted = extractDocumentContent(
		Buffer.from(
			[
				"<html><head><title>Ancient Tower</title></head>",
				"<body><nav>Ignore me</nav><main>",
				"<h1>Entrance</h1>",
				"<p>The <strong>brass door</strong> opens inward.</p>",
				"<img src=\"tower.png\" alt=\"tower diagram\">",
				"<h2>Basement</h2>",
				"<p>Stone stairs descend below the keep.</p>",
				"</main></body></html>",
			].join(""),
			"utf8",
		),
		"text/html; charset=utf-8",
	);

	assert.ok(extracted.segments.length >= 4);
	assert.equal(extracted.segments[0]?.text, "Ancient Tower");
	assert.equal(extracted.segments[1]?.metadata?.title, "Ancient Tower");
	assert.deepEqual(extracted.segments[1]?.metadata?.headingPath, ["Entrance"]);
	assert.match(extracted.segments.map((segment) => segment.text).join("\n"), /tower diagram/);
	assert.match(extracted.segments.map((segment) => segment.text).join("\n"), /Stone stairs descend/);
});

test("warns when html structure is incomplete but readable", () => {
	const extracted = extractDocumentContent(
		Buffer.from("<html><body><h1>Field Notes</h1><p>Partial markup", "utf8"),
		"text/html",
	);

	assert.ok((extracted.warnings?.length ?? 0) > 0);
	assert.equal(extracted.segments[0]?.metadata?.warnings?.length, extracted.warnings?.length);
});

test("decodes html entities for extracted text", () => {
	assert.equal(decodeHTMLEntities("Astra &amp; Samael &#62; Tiamat"), "Astra & Samael > Tiamat");
});
