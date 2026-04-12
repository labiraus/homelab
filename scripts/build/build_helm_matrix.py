#!/usr/bin/env python3

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
HELM_ROOT = REPO_ROOT / "helm"


def git(*args: str) -> str:
    return subprocess.check_output(
        ["git", *args],
        cwd=REPO_ROOT,
        text=True,
    )


def git_object_exists(rev: str) -> bool:
    if not rev:
        return False

    result = subprocess.run(
        ["git", "rev-parse", "--verify", "--quiet", f"{rev}^{{commit}}"],
        cwd=REPO_ROOT,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        text=True,
    )
    return result.returncode == 0


def load_chart_type(chart_yaml: Path) -> str:
    for line in chart_yaml.read_text(encoding="utf-8").splitlines():
        if line.startswith("type:"):
            return line.split(":", 1)[1].strip().lower()
    return ""


def discover_installable_charts() -> list[dict[str, str]]:
    rows: list[dict[str, str]] = []
    for chart_yaml in sorted(HELM_ROOT.rglob("Chart.yaml")):
        chart_dir = chart_yaml.parent
        chart_rel = chart_dir.relative_to(HELM_ROOT).as_posix()
        if chart_rel.startswith("libraries/"):
            continue
        if load_chart_type(chart_yaml) == "library":
            continue

        ghcr_values = "values-ghcr.yaml" if (chart_dir / "values-ghcr.yaml").exists() else "values.yaml"
        rows.append({"chart": chart_rel, "ghcr_values": ghcr_values})
    return rows


def map_chart_file_to_dir(rel_path: str) -> str | None:
    path = REPO_ROOT / rel_path
    if not path.exists():
        return None

    current = path.parent if path.is_file() else path
    while current != REPO_ROOT and current != HELM_ROOT:
        if (current / "Chart.yaml").exists():
            return current.relative_to(HELM_ROOT).as_posix()
        current = current.parent
    return None


def changed_files(before_sha: str, current_sha: str) -> list[str] | None:
    if not git_object_exists(before_sha) or not git_object_exists(current_sha):
        return None

    output = git("diff", "--name-only", before_sha, current_sha)
    return [line for line in output.splitlines() if line]


def chart_uses_library(chart_rel: str, library_rel: str) -> bool:
    chart_yaml = HELM_ROOT / chart_rel / "Chart.yaml"
    library_name = library_rel.removeprefix("libraries/")
    needle = f"file://../../libraries/{library_name}"
    return needle in chart_yaml.read_text(encoding="utf-8")


def discover_publish_subset(rows: list[dict[str, str]], event_name: str, before_sha: str, current_sha: str) -> list[dict[str, str]]:
    publish_all = event_name != "push" or not before_sha or re.fullmatch(r"0+", before_sha) is not None
    changed_chart_dirs: set[str] = set()
    changed_app_sources: set[str] = set()
    changed_libraries: set[str] = set()
    files: list[str] = []

    if not publish_all:
        files = changed_files(before_sha, current_sha)
        if files is None:
            print(
                f"warning: could not diff {before_sha!r}..{current_sha!r}; falling back to publish_all",
                file=sys.stderr,
            )
            publish_all = True

    if not publish_all:
        if any(
            path.startswith(".github/actions/helm/")
            or path.startswith("scripts/build/")
            or path == ".github/workflows/helm-all.yml"
            for path in files
        ):
            publish_all = True

    if publish_all:
        return rows

    for rel_path in files:
        if rel_path.startswith("helm/"):
            chart_rel = map_chart_file_to_dir(rel_path)
            if chart_rel:
                changed_chart_dirs.add(chart_rel)
                if chart_rel.startswith("libraries/"):
                    changed_libraries.add(chart_rel)

        if rel_path.startswith("apps/"):
            parts = rel_path.split("/", 2)
            if len(parts) >= 2:
                changed_app_sources.add(parts[1])

    if changed_libraries:
        for row in rows:
            chart_rel = row["chart"]
            if any(chart_uses_library(chart_rel, library_rel) for library_rel in changed_libraries):
                changed_chart_dirs.add(chart_rel)

    publish_rows: list[dict[str, str]] = []
    for row in rows:
        chart_rel = row["chart"]
        if chart_rel not in changed_chart_dirs:
            continue

        if chart_rel.startswith("apps/"):
            app_name = chart_rel.split("/", 1)[1].split("/", 1)[0]
            if app_name in changed_app_sources:
                continue

        publish_rows.append(row)

    return publish_rows


def emit_output(name: str, value: str) -> None:
    output_path = os.environ.get("GITHUB_OUTPUT")
    if output_path:
        with open(output_path, "a", encoding="utf-8") as handle:
            handle.write(f"{name}={value}\n")
    else:
        print(f"{name}={value}")


def main() -> int:
    event_name = os.environ.get("GITHUB_EVENT_NAME", "")
    before_sha = os.environ.get("GITHUB_EVENT_BEFORE", "")
    current_sha = os.environ.get("GITHUB_SHA", "")

    rows = discover_installable_charts()
    publish_rows = discover_publish_subset(rows, event_name, before_sha, current_sha)

    emit_output("matrix", json.dumps(rows, separators=(",", ":")))
    emit_output("ghcr_matrix", json.dumps(publish_rows, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    sys.exit(main())
