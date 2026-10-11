"""Read-only retrieval adapter for the authenticated external search API."""
import json
import os
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def retrieve(base_url, case):
    target = urllib.parse.urlsplit(base_url)
    if (target.scheme != "https" and not (target.scheme == "http" and target.hostname in {"127.0.0.1", "localhost", "::1"})) or not target.hostname or target.username or target.password or target.query or target.fragment or target.path not in {"", "/"}:
        raise ValueError("API root must be HTTPS (HTTP allowed only on loopback), without credentials/path/query")
    body = {"query": case.query, "limit": case.limit, "documentId": case.document_id, "prefix": case.prefix, "metadata": case.metadata}
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    # Explicit operator session only. Never manufacture forwarded identity headers.
    if os.environ.get("RAGAS_API_COOKIE"):
        headers["Cookie"] = os.environ["RAGAS_API_COOKIE"]
    if os.environ.get("RAGAS_API_AUTHORIZATION"):
        headers["Authorization"] = os.environ["RAGAS_API_AUTHORIZATION"]
    request = urllib.request.Request(base_url.rstrip("/") + "/api/documents/search", data=json.dumps(body).encode(), headers=headers, method="POST")
    with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
        raw = response.read(2_000_001)
    if len(raw) > 2_000_000:
        raise ValueError("search response exceeds evaluation budget")
    return validate_hits(json.loads(raw), case.limit)


def validate_hits(payload, limit):
    if not isinstance(payload, dict) or not isinstance(payload.get("hits"), list) or len(payload["hits"]) > limit:
        raise ValueError("invalid search response")
    for hit in payload["hits"]:
        if not isinstance(hit, dict) or not isinstance(hit.get("chunkText"), str) or not isinstance(hit.get("documentId"), str):
            raise ValueError("search hit lacks document or chunk text")
        citation = hit.get("citation")
        if not isinstance(citation, dict) or not isinstance(citation.get("id"), str) or not citation["id"]:
            raise ValueError("search hit lacks a stable citation")
        for key in ["chunkIndex", "processingVersion"]:
            if type(hit.get(key)) is not int or hit[key] < 0 or citation.get(key) != hit[key]:
                raise ValueError("search hit citation version/index mismatch")
    return payload["hits"]
