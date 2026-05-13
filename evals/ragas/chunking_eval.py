#!/usr/bin/env python3
"""Evaluate processed document chunk quality with RAGAS retrieval metrics."""

from __future__ import annotations

import argparse
import asyncio
import json
import math
import os
import re
import urllib.error
import urllib.parse
import urllib.request
import warnings
from dataclasses import dataclass
from pathlib import Path
from typing import Any

try:
    import psycopg
except ImportError as error:  # pragma: no cover - exercised by operator setup.
    raise SystemExit(
        "Missing dependency psycopg. Install with: "
        "python3 -m pip install -r evals/ragas/requirements.txt"
    ) from error

try:
    warnings.filterwarnings(
        "ignore",
        message=r"Importing .* from 'ragas\.metrics' is deprecated.*",
        category=DeprecationWarning,
    )
    from ragas.dataset_schema import SingleTurnSample
    from ragas.metrics import (
        IDBasedContextPrecision,
        IDBasedContextRecall,
        NonLLMContextPrecisionWithReference,
        NonLLMContextRecall,
    )
except ImportError as error:  # pragma: no cover - exercised by operator setup.
    raise SystemExit(
        "Missing dependency ragas. Install with: "
        "python3 -m pip install -r evals/ragas/requirements.txt"
    ) from error


LOCAL_EMBEDDING_MODEL = "local-embeddings"
LOCAL_EMBEDDING_DIMENSIONS = 384
DEFAULT_LIMIT = 8


@dataclass(frozen=True)
class EvalCase:
    case_id: str
    query: str
    limit: int
    document_id: str
    prefix: str
    metadata: dict[str, Any]
    reference_context_ids: list[str]
    reference_contexts: list[str]


@dataclass(frozen=True)
class RetrievedChunk:
    context_id: str
    document_id: str
    source_uri: str
    object_key: str
    chunk_index: int
    chunk_text: str
    distance: float
    similarity: float


async def main() -> int:
    args = parse_args()
    load_env_files(args.env_file)
    cases = load_cases(args.cases, args.limit)
    dsn = postgres_dsn(args.postgres_dsn)
    embedding_endpoint = args.embedding_endpoint or os.environ.get("EMBEDDING_ENDPOINT", "").strip()
    embedding_model = args.embedding_model or os.environ.get("EMBEDDING_MODEL", "").strip() or LOCAL_EMBEDDING_MODEL

    results = []
    try:
        with psycopg.connect(dsn) as connection:
            for case in cases:
                embedding, model = fetch_embedding(case.query, embedding_model, embedding_endpoint)
                chunks = retrieve_chunks(connection, case, embedding, model)
                scores = await score_case(case, chunks)
                results.append(format_case_result(case, chunks, model, scores, args.include_contexts))
    except psycopg.OperationalError as error:
        raise SystemExit(
            "Could not connect to Postgres. Run through `make ragas-chunking-eval` so the "
            "target can refresh local DB env and open the app-db port-forward, or pass a "
            f"working --postgres-dsn. psycopg said: {error}"
        ) from error

    summary = summarize(results, args)
    payload = {"summary": summary, "cases": results}
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")

    if args.format == "json":
        print(json.dumps(payload, indent=2))
    else:
        print_text_report(payload)

    return 0 if summary["passed"] else 1


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Score live Postgres-backed RAG chunks with RAGAS retrieval metrics.",
    )
    parser.add_argument(
        "--cases",
        type=Path,
        default=Path("evals/ragas/chunking_cases.jsonl"),
        help="JSONL file containing gold retrieval cases.",
    )
    parser.add_argument(
        "--env-file",
        type=Path,
        action="append",
        default=[],
        help="Optional env file to load before connecting. Can be repeated.",
    )
    parser.add_argument(
        "--postgres-dsn",
        default="",
        help="Postgres DSN. Defaults to POSTGRES_CONNECTION_STRING, DB_*, POSTGRES_*, or PG* env vars.",
    )
    parser.add_argument(
        "--embedding-model",
        default="",
        help="Embedding model to query. Defaults to EMBEDDING_MODEL or local-embeddings.",
    )
    parser.add_argument(
        "--embedding-endpoint",
        default="",
        help="OpenAI-compatible embedding endpoint. Empty with local-embeddings uses the repo local embedding function.",
    )
    parser.add_argument("--limit", type=int, default=DEFAULT_LIMIT, help="Default top-k retrieval limit.")
    parser.add_argument("--min-id-precision", type=float, default=0.0)
    parser.add_argument("--min-id-recall", type=float, default=0.0)
    parser.add_argument("--min-context-precision", type=float, default=0.0)
    parser.add_argument("--min-context-recall", type=float, default=0.0)
    parser.add_argument("--output", type=Path, help="Optional JSON report output path.")
    parser.add_argument("--format", choices=("text", "json"), default="text")
    parser.add_argument(
        "--include-contexts",
        action="store_true",
        help="Include retrieved chunk text in the JSON output.",
    )
    return parser.parse_args()


def load_env_files(paths: list[Path]) -> None:
    defaults = [Path(".devcontainer/.env"), Path("/home/vscode/.env")]
    for path in [*defaults, *paths]:
        if not path.exists():
            continue
        for line in path.read_text(encoding="utf-8").splitlines():
            stripped = line.strip()
            if not stripped or stripped.startswith("#") or "=" not in stripped:
                continue
            key, value = stripped.split("=", 1)
            key = key.strip()
            value = value.strip().strip("'\"")
            if key and key not in os.environ:
                os.environ[key] = value


def load_cases(path: Path, default_limit: int) -> list[EvalCase]:
    if default_limit < 1:
        raise SystemExit("--limit must be greater than zero")
    if not path.exists():
        raise SystemExit(
            f"Case file not found: {path}. Copy evals/ragas/chunking_cases.example.jsonl "
            "to evals/ragas/chunking_cases.jsonl and replace it with real gold cases."
        )

    cases: list[EvalCase] = []
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if not line.strip():
            continue
        try:
            raw = json.loads(line)
        except json.JSONDecodeError as error:
            raise SystemExit(f"{path}:{line_number}: invalid JSON: {error}") from error
        cases.append(parse_case(raw, default_limit, path, line_number))

    if not cases:
        raise SystemExit(f"No cases found in {path}")
    return cases


def parse_case(raw: dict[str, Any], default_limit: int, path: Path, line_number: int) -> EvalCase:
    case_id = str(raw.get("id") or f"{path.name}:{line_number}").strip()
    query = str(raw.get("query") or "").strip()
    if not query:
        raise SystemExit(f"{path}:{line_number}: query is required")

    limit = int(raw.get("limit") or default_limit)
    if limit < 1:
        raise SystemExit(f"{path}:{line_number}: limit must be greater than zero")

    metadata = raw.get("metadata") or {}
    if not isinstance(metadata, dict):
        raise SystemExit(f"{path}:{line_number}: metadata must be an object")

    reference_context_ids = string_list(raw.get("reference_context_ids") or [])
    reference_contexts = string_list(raw.get("reference_contexts") or [])
    if not reference_context_ids and not reference_contexts:
        raise SystemExit(
            f"{path}:{line_number}: provide reference_context_ids, reference_contexts, or both"
        )

    return EvalCase(
        case_id=case_id,
        query=query,
        limit=limit,
        document_id=str(raw.get("documentId") or raw.get("document_id") or "").strip(),
        prefix=normalize_prefix(str(raw.get("prefix") or "")),
        metadata=metadata,
        reference_context_ids=reference_context_ids,
        reference_contexts=reference_contexts,
    )


def string_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        raise SystemExit("reference context fields must be arrays")
    return [str(item).strip() for item in value if str(item).strip()]


def normalize_prefix(value: str) -> str:
    return value.strip().lstrip("/")


def postgres_dsn(explicit_dsn: str) -> str:
    if explicit_dsn.strip():
        return explicit_dsn.strip()

    candidates = (
        ("DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASS", "DB_SSLMODE"),
        ("POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_DATABASE", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_SSLMODE"),
        ("PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGSSLMODE"),
    )
    for host_key, port_key, db_key, user_key, pass_key, ssl_key in candidates:
        host = os.environ.get(host_key, "").strip()
        database = os.environ.get(db_key, "").strip()
        user = os.environ.get(user_key, "").strip()
        password = os.environ.get(pass_key, "")
        if host and database and user:
            port = os.environ.get(port_key, "").strip() or "5432"
            sslmode = os.environ.get(ssl_key, "").strip() or "disable"
            auth = urllib.parse.quote(user)
            if password:
                auth += ":" + urllib.parse.quote(password)
            return f"postgresql://{auth}@{host}:{port}/{urllib.parse.quote(database)}?sslmode={sslmode}"

    for key in ("POSTGRES_CONNECTION_STRING", "DATABASE_URL"):
        value = os.environ.get(key, "").strip()
        if value:
            return value

    raise SystemExit(
        "Postgres connection details are missing. Run make refresh-postgres-env, start the "
        "app-db port-forward, or pass --postgres-dsn."
    )


def fetch_embedding(query: str, model: str, endpoint: str) -> tuple[list[float], str]:
    if not endpoint and model == LOCAL_EMBEDDING_MODEL:
        return embed_text(query), model
    if not endpoint:
        raise SystemExit("EMBEDDING_ENDPOINT is required for non-local embedding models")

    body = json.dumps({"model": model, "input": query}).encode("utf-8")
    request = urllib.request.Request(
        endpoint,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            payload = json.loads(response.read().decode("utf-8"))
    except urllib.error.URLError as error:
        raise SystemExit(f"Embedding request failed: {error}") from error

    vector = ((payload.get("data") or [{}])[0]).get("embedding")
    if not vector:
        raise SystemExit("Embedding response did not include a vector")
    return [float(value) for value in vector], str(payload.get("model") or model)


def embed_text(input_text: str) -> list[float]:
    vector = [0.0] * LOCAL_EMBEDDING_DIMENSIONS
    tokens = re.findall(r"[a-z0-9]+", input_text.lower())
    if not tokens:
        tokens = [input_text.strip().lower()]

    for token in tokens:
        token_hash = fnv1a(token)
        index = token_hash % LOCAL_EMBEDDING_DIMENSIONS
        vector[index] += -1.0 if token_hash & 0x80000000 else 1.0

    magnitude = math.sqrt(sum(value * value for value in vector))
    if magnitude == 0:
        return vector
    return [value / magnitude for value in vector]


def fnv1a(value: str) -> int:
    token_hash = 2166136261
    for byte in value.encode("utf-8"):
        token_hash ^= byte
        token_hash = (token_hash * 16777619) & 0xFFFFFFFF
    return token_hash


def retrieve_chunks(
    connection: psycopg.Connection[Any],
    case: EvalCase,
    embedding: list[float],
    model: str,
) -> list[RetrievedChunk]:
    query = [
        """
SELECT
    d.document_id,
    d.source_uri,
    COALESCE(d.object_key, ''),
    c.chunk_index,
    c.chunk_text,
    e.vector <=> %s::vector AS distance
FROM rag.embeddings e
JOIN rag.chunks c ON c.id = e.chunk_id
JOIN rag.documents d ON d.id = c.document_pk
WHERE d.status = 'processed'
    AND e.model = %s
    AND c.processing_version = d.current_processing_version
    AND e.vector IS NOT NULL
""",
    ]
    args: list[Any] = [vector_literal(embedding), model]

    if case.document_id:
        query.append("    AND d.document_id = %s\n")
        args.append(case.document_id)
    if case.prefix:
        query.append("    AND d.object_key LIKE %s\n")
        args.append(case.prefix + "%")
    if case.metadata:
        query.append("    AND d.metadata @> %s::jsonb\n")
        args.append(json.dumps(case.metadata))

    query.append("ORDER BY e.vector <=> %s::vector\nLIMIT %s")
    args.extend([vector_literal(embedding), case.limit])

    chunks: list[RetrievedChunk] = []
    with connection.cursor() as cursor:
        cursor.execute("".join(query), args)
        for row in cursor.fetchall():
            document_id, source_uri, object_key, chunk_index, chunk_text, distance = row
            distance = float(distance)
            chunks.append(
                RetrievedChunk(
                    context_id=f"{source_uri or document_id}#chunk-{chunk_index}",
                    document_id=document_id,
                    source_uri=source_uri,
                    object_key=object_key,
                    chunk_index=int(chunk_index),
                    chunk_text=chunk_text,
                    distance=distance,
                    similarity=max(0.0, min(1.0, 1.0 - distance)),
                )
            )
    return chunks


def vector_literal(values: list[float]) -> str:
    return "[" + ",".join(format(value, ".12g") for value in values) + "]"


async def score_case(case: EvalCase, chunks: list[RetrievedChunk]) -> dict[str, float | None]:
    sample = SingleTurnSample(
        user_input=case.query,
        retrieved_contexts=[chunk.chunk_text for chunk in chunks],
        reference_contexts=case.reference_contexts or None,
        retrieved_context_ids=[chunk.context_id for chunk in chunks],
        reference_context_ids=case.reference_context_ids or None,
    )

    scores: dict[str, float | None] = {
        "id_precision": None,
        "id_recall": None,
        "context_precision": None,
        "context_recall": None,
    }
    if case.reference_context_ids:
        scores["id_precision"] = float(await IDBasedContextPrecision().single_turn_ascore(sample))
        scores["id_recall"] = float(await IDBasedContextRecall().single_turn_ascore(sample))
    if case.reference_contexts:
        scores["context_precision"] = float(
            await NonLLMContextPrecisionWithReference().single_turn_ascore(sample)
        )
        scores["context_recall"] = float(await NonLLMContextRecall().single_turn_ascore(sample))
    return scores


def format_case_result(
    case: EvalCase,
    chunks: list[RetrievedChunk],
    embedding_model: str,
    scores: dict[str, float | None],
    include_contexts: bool,
) -> dict[str, Any]:
    retrieved = []
    for chunk in chunks:
        row: dict[str, Any] = {
            "context_id": chunk.context_id,
            "document_id": chunk.document_id,
            "object_key": chunk.object_key,
            "chunk_index": chunk.chunk_index,
            "distance": chunk.distance,
            "similarity": chunk.similarity,
        }
        if include_contexts:
            row["chunk_text"] = chunk.chunk_text
        retrieved.append(row)

    return {
        "id": case.case_id,
        "query": case.query,
        "embedding_model": embedding_model,
        "filters": {
            "documentId": case.document_id,
            "prefix": case.prefix,
            "metadata": case.metadata,
            "limit": case.limit,
        },
        "scores": scores,
        "retrieved_context_ids": [chunk.context_id for chunk in chunks],
        "reference_context_ids": case.reference_context_ids,
        "retrieved": retrieved,
    }


def summarize(results: list[dict[str, Any]], args: argparse.Namespace) -> dict[str, Any]:
    thresholds = {
        "id_precision": args.min_id_precision,
        "id_recall": args.min_id_recall,
        "context_precision": args.min_context_precision,
        "context_recall": args.min_context_recall,
    }
    means = {}
    for metric in thresholds:
        values = [
            result["scores"][metric]
            for result in results
            if result["scores"].get(metric) is not None
        ]
        means[metric] = sum(values) / len(values) if values else None

    failures = []
    for metric, threshold in thresholds.items():
        mean = means[metric]
        if mean is not None and mean < threshold:
            failures.append({"metric": metric, "mean": mean, "threshold": threshold})

    return {
        "case_count": len(results),
        "means": means,
        "thresholds": thresholds,
        "failures": failures,
        "passed": not failures,
    }


def print_text_report(payload: dict[str, Any]) -> None:
    summary = payload["summary"]
    print("RAGAS chunking evaluation")
    print(f"cases: {summary['case_count']}")
    for metric, value in summary["means"].items():
        rendered = "n/a" if value is None else f"{value:.3f}"
        threshold = summary["thresholds"][metric]
        print(f"{metric}: {rendered} (min {threshold:.3f})")
    print(f"passed: {summary['passed']}")
    print()

    for result in payload["cases"]:
        scores = {
            metric: ("n/a" if value is None else f"{value:.3f}")
            for metric, value in result["scores"].items()
        }
        print(f"- {result['id']}: {result['query']}")
        print(
            "  "
            + ", ".join(
                f"{metric}={value}"
                for metric, value in scores.items()
            )
        )
        if result["retrieved_context_ids"]:
            print(f"  top: {result['retrieved_context_ids'][0]}")


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
