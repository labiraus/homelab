#!/usr/bin/env python3
"""Exercise a real OpenAI-compatible embedding endpoint with synthetic text only."""
import argparse
import concurrent.futures
import json
import math
import time
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", required=True, help="Explicit endpoint ending in /v1")
    parser.add_argument("--model", default="BAAI/bge-large-en-v1.5")
    parser.add_argument("--dimensions", type=int, default=1024)
    args = parser.parse_args()
    address = urllib.parse.urlsplit(args.base_url)
    if address.scheme not in ("http", "https") or not address.hostname or address.username or address.password or address.query or address.fragment or address.path.rstrip("/") != "/v1":
        parser.error("expected an explicit HTTP(S) /v1 endpoint without credentials, query or fragment")
    endpoint = args.base_url.rstrip("/") + "/embeddings"

    def request(inputs):
        payload = json.dumps({"model": args.model, "input": inputs, "encoding_format": "float"}).encode()
        started = time.monotonic()
        req = urllib.request.Request(endpoint, data=payload, headers={"Content-Type": "application/json"})
        with urllib.request.build_opener(NoRedirect).open(req, timeout=45) as response:
            raw = response.read(2_000_001)
        assert len(raw) <= 2_000_000, "unbounded embedding response"
        result = json.loads(raw)
        assert result.get("model") == args.model, "unexpected model identity"
        rows = result["data"]
        assert len(rows) == len(inputs), "batch size changed"
        vectors = []
        for index, row in enumerate(rows):
            assert row["index"] == index, "batch identity changed"
            vector = row["embedding"]
            assert len(vector) == args.dimensions, "embedding dimensions differ from the index"
            assert all(type(x) in (int, float) and math.isfinite(x) for x in vector), "invalid vector values"
            norm = math.sqrt(sum(x*x for x in vector))
            assert 0.99 < norm < 1.01, "expected normalized vectors"
            vectors.append(vector)
        return vectors, time.monotonic() - started

    texts = [
        "How do I reset a forgotten password?",
        "To recover account access, choose Forgot password and follow the reset link sent by email.",
        "Tomatoes grow well in warm sunlight and well-drained garden soil.",
    ]
    vectors, batch_seconds = request(texts)
    cosine = lambda a, b: sum(x*y for x, y in zip(a, b))
    related, unrelated = cosine(vectors[0], vectors[1]), cosine(vectors[0], vectors[2])
    assert related > unrelated + 0.1, "synthetic semantic retrieval did not separate relevant text"
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        results = list(pool.map(lambda text: request([text]), [texts[0], texts[2]]))
    for (rows, _), original in zip(results, [vectors[0], vectors[2]]):
        assert max(abs(a-b) for a, b in zip(rows[0], original)) < 0.001, "concurrent request identity or output drift"
    try:
        request(["word " * 600])
    except urllib.error.HTTPError as error:
        assert error.code in (400, 413, 422), "unexpected oversized-input failure"
    else:
        raise AssertionError("oversized input was silently accepted/truncated")
    print(json.dumps({"status": "PASS", "model": args.model, "dimensions": args.dimensions,
                      "relatedCosine": round(related, 4), "unrelatedCosine": round(unrelated, 4),
                      "batchSeconds": round(batch_seconds, 3),
                      "concurrentSeconds": [round(result[1], 3) for result in results],
                      "oversizedInput": "refused"}))


if __name__ == "__main__":
    main()
