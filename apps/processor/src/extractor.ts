import { decodeUtf8Text } from "./storage.js";
import type { ChunkMetadata, ExtractedSegment } from "./types.js";

export interface ExtractedDocument {
	segments: ExtractedSegment[];
	warnings?: string[];
}

export function extractDocumentContent(raw: Buffer, contentType: string): ExtractedDocument {
	if (normalizedMediaType(contentType) == "text/html") {
		return extractHTMLDocument(raw);
	}

	return {
		segments: [
			{
				text: decodeUtf8Text(raw),
				metadata: {
					sourceType: "text",
				},
			},
		],
	};
}

function extractHTMLDocument(raw: Buffer): ExtractedDocument {
	const source = decodeUtf8Text(raw);
	const warnings = detectHTMLWarnings(source);
	const title = htmlFragmentToText(extractFirstTagInner(source, "title"));
	let body = source;

	body = body.replace(/<!--[\s\S]*?-->/g, " ");
	body = stripContainerTag(body, "head");
	body = stripContainerTag(body, "script");
	body = stripContainerTag(body, "style");
	body = stripContainerTag(body, "noscript");
	body = stripContainerTag(body, "template");
	body = stripContainerTag(body, "svg");
	body = stripContainerTag(body, "canvas");
	body = stripContainerTag(body, "nav");
	body = stripContainerTag(body, "header");
	body = stripContainerTag(body, "footer");
	body = stripContainerTag(body, "aside");
	body = body.replace(/<img\b[^>]*>/gi, (value) => imageAltReplacement(value));
	body = body.replace(
		/<h([1-6])\b[^>]*>([\s\S]*?)<\/h\1>/gi,
		(_, level: string, inner: string) => `\n\n[[HEADING:${level}:${htmlFragmentToText(inner)}]]\n\n`,
	);
	body = body.replace(/<(br|hr)\b[^>]*\/?>/gi, "\n");
	body = body.replace(/<\/?(main|section|article|div|p|ul|ol|li|blockquote|pre|table|tr|td|th)\b[^>]*>/gi, "\n");
	body = body.replace(/<[^>]+>/g, " ");

	const lines = decodeHTMLEntities(body)
		.split(/\r?\n+/)
		.map((line) => collapseWhitespace(line))
		.filter(Boolean);

	const segments: ExtractedSegment[] = [];
	const headingPath: string[] = [];
	const normalizedTitle = collapseWhitespace(title);
	if (normalizedTitle) {
		segments.push({
			text: normalizedTitle,
			metadata: buildHTMLMetadata(normalizedTitle, [], warnings),
		});
	}

	for (const line of lines) {
		const headingMatch = line.match(/^\[\[HEADING:(\d):([\s\S]+)\]\]$/);
		if (headingMatch) {
			const level = Number(headingMatch[1]);
			const headingText = collapseWhitespace(headingMatch[2] ?? "");
			if (!headingText) {
				continue;
			}
			headingPath.splice(Math.max(level-1, 0));
			headingPath.push(headingText);
			segments.push({
				text: headingText,
				metadata: buildHTMLMetadata(normalizedTitle, headingPath, warnings),
			});
			continue;
		}

		segments.push({
			text: line,
			metadata: buildHTMLMetadata(normalizedTitle, headingPath, warnings),
		});
	}

	if (segments.length == 0) {
		throw new Error("html extraction produced no readable text");
	}

	return {
		segments,
		warnings: warnings.length > 0 ? warnings : undefined,
	};
}

function buildHTMLMetadata(title: string, headingPath: string[], warnings: string[]): ChunkMetadata {
	const metadata: ChunkMetadata = {
		sourceType: "html",
	};
	if (title) {
		metadata.title = title;
	}
	if (headingPath.length > 0) {
		metadata.headingPath = [...headingPath];
	}
	if (warnings.length > 0) {
		metadata.warnings = [...warnings];
	}
	return metadata;
}

function normalizedMediaType(contentType: string): string {
	return contentType.split(";", 1)[0]?.trim().toLowerCase() ?? "";
}

function stripContainerTag(source: string, tag: string): string {
	const pattern = new RegExp(`<${tag}\\b[^>]*>[\\s\\S]*?<\\/${tag}>`, "gi");
	return source.replace(pattern, " ");
}

function extractFirstTagInner(source: string, tag: string): string {
	const pattern = new RegExp(`<${tag}\\b[^>]*>([\\s\\S]*?)<\\/${tag}>`, "i");
	return pattern.exec(source)?.[1] ?? "";
}

function imageAltReplacement(tag: string): string {
	const quoted = tag.match(/\balt\s*=\s*(['"])([\s\S]*?)\1/i);
	if (quoted?.[2]) {
		return ` ${decodeHTMLEntities(quoted[2])} `;
	}
	const unquoted = tag.match(/\balt\s*=\s*([^\s>]+)/i);
	if (unquoted?.[1]) {
		return ` ${decodeHTMLEntities(unquoted[1])} `;
	}
	return " ";
}

function htmlFragmentToText(source: string): string {
	if (!source) {
		return "";
	}
	return collapseWhitespace(decodeHTMLEntities(source.replace(/<[^>]+>/g, " ")));
}

function collapseWhitespace(value: string): string {
	return value.replace(/\s+/g, " ").trim();
}

function detectHTMLWarnings(source: string): string[] {
	const warnings = new Set<string>();
	const normalized = source.toLowerCase();
	if (normalized.includes("<html") && !normalized.includes("</html>")) {
		warnings.add("html structure appears incomplete; extracted readable text from partial markup");
	}
	if (normalized.includes("<body") && !normalized.includes("</body>")) {
		warnings.add("html body appears incomplete; extracted readable text from partial markup");
	}
	if ((source.match(/</g) ?? []).length != (source.match(/>/g) ?? []).length) {
		warnings.add("html markup contains unmatched angle brackets; extracted readable text where possible");
	}
	return [...warnings];
}

const htmlEntities = new Map<string, string>([
	["amp", "&"],
	["apos", "'"],
	["gt", ">"],
	["lt", "<"],
	["nbsp", " "],
	["ndash", "-"],
	["mdash", "-"],
	["quot", "\""],
]);

export function decodeHTMLEntities(value: string): string {
	return value.replace(/&(#x?[0-9a-f]+|[a-z]+);/gi, (match, entity: string) => {
		const lower = entity.toLowerCase();
		if (lower.startsWith("#x")) {
			const codePoint = Number.parseInt(lower.slice(2), 16);
			return Number.isFinite(codePoint) ? String.fromCodePoint(codePoint) : match;
		}
		if (lower.startsWith("#")) {
			const codePoint = Number.parseInt(lower.slice(1), 10);
			return Number.isFinite(codePoint) ? String.fromCodePoint(codePoint) : match;
		}
		return htmlEntities.get(lower) ?? match;
	});
}
