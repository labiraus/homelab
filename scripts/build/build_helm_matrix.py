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
WORKFLOWS_ROOT = REPO_ROOT / ".github" / "workflows"
# Changes under these paths can affect every chart job, so we stop trying to
# be clever and rebuild the full matrix.
FULL_REBUILD_PREFIXES = (".github/actions/helm/", "scripts/build/")
FULL_REBUILD_FILES = {".github/workflows/helm-all.yml"}


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


def chart_app_name(chart_rel: str) -> str | None:
    # App charts live at helm/apps/<app>; extracting the app name in one place
    # keeps the publish/filter logic consistent.
    if not chart_rel.startswith("apps/"):
        return None
    return chart_rel.split("/", 2)[1]


def source_app_name(rel_path: str) -> str | None:
    if not rel_path.startswith("apps/"):
        return None

    parts = rel_path.split("/", 2)
    if len(parts) < 2:
        return None
    return parts[1]


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


def should_publish_all(event_name: str, before_sha: str, current_sha: str) -> tuple[bool, list[str]]:
    # Non-push events and initial branch pushes do not have a reliable "changed
    # files" window, so they always get the complete matrix.
    if event_name != "push" or not before_sha or re.fullmatch(r"0+", before_sha) is not None:
        return True, []

    files = changed_files(before_sha, current_sha)
    if files is None:
        print(
            f"warning: could not diff {before_sha!r}..{current_sha!r}; falling back to publish_all",
            file=sys.stderr,
        )
        return True, []

    if any(path.startswith(FULL_REBUILD_PREFIXES) or path in FULL_REBUILD_FILES for path in files):
        return True, files

    return False, files


def collect_changes(files: list[str]) -> tuple[set[str], set[str], set[str]]:
    changed_chart_dirs: set[str] = set()
    changed_app_sources: set[str] = set()
    changed_libraries: set[str] = set()

    for rel_path in files:
        if rel_path.startswith("helm/"):
            # Any file under a chart directory should count as a chart change,
            # not just direct edits to Chart.yaml.
            chart_rel = map_chart_file_to_dir(rel_path)
            if chart_rel:
                changed_chart_dirs.add(chart_rel)
                if chart_rel.startswith("libraries/"):
                    changed_libraries.add(chart_rel)

        app_name = source_app_name(rel_path)
        if app_name:
            changed_app_sources.add(app_name)

    return changed_chart_dirs, changed_app_sources, changed_libraries


def expand_library_changes(rows: list[dict[str, str]], changed_chart_dirs: set[str], changed_libraries: set[str]) -> None:
    if not changed_libraries:
        return

    # Library charts are not published directly, but downstream charts should be
    # rebuilt when their local file:// library dependency changes.
    for row in rows:
        chart_rel = row["chart"]
        if any(chart_uses_library(chart_rel, library_rel) for library_rel in changed_libraries):
            changed_chart_dirs.add(chart_rel)


def should_publish_chart(row: dict[str, str], changed_chart_dirs: set[str], changed_app_sources: set[str]) -> bool:
    chart_rel = row["chart"]
    if chart_rel not in changed_chart_dirs:
        return False

    # App source changes are handled by the dedicated app-* workflows, so the
    # Helm publish job should skip those matching app charts here.
    app_name = chart_app_name(chart_rel)
    if app_name and app_name in changed_app_sources:
        return False

    return True


def discover_publish_subset(rows: list[dict[str, str]], event_name: str, before_sha: str, current_sha: str) -> list[dict[str, str]]:
    publish_all, files = should_publish_all(event_name, before_sha, current_sha)
    if publish_all:
        return rows

    changed_chart_dirs, changed_app_sources, changed_libraries = collect_changes(files)
    expand_library_changes(rows, changed_chart_dirs, changed_libraries)
    return [row for row in rows if should_publish_chart(row, changed_chart_dirs, changed_app_sources)]


def split_publish_rows(rows: list[dict[str, str]]) -> tuple[list[dict[str, str]], list[dict[str, str]]]:
    non_app_rows: list[dict[str, str]] = []
    app_rows: list[dict[str, str]] = []

    for row in rows:
        chart_rel = row["chart"]
        app_name = chart_app_name(chart_rel)
        if app_name:
            workflow = f"app-{app_name}.yml"
            # When an app has its own workflow, dispatch that pipeline instead of
            # publishing the chart directly from helm-all.yml.
            if (WORKFLOWS_ROOT / workflow).exists():
                app_rows.append(
                    {
                        "chart": chart_rel,
                        "ghcr_values": row["ghcr_values"],
                        "app": app_name,
                        "workflow": workflow,
                    }
                )
            else:
                non_app_rows.append(row)
        else:
            non_app_rows.append(row)

    return non_app_rows, app_rows


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
    non_app_rows, app_rows = split_publish_rows(publish_rows)

    emit_output("matrix", json.dumps(rows, separators=(",", ":")))
    emit_output("ghcr_matrix", json.dumps(non_app_rows, separators=(",", ":")))
    emit_output("app_matrix", json.dumps(app_rows, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    sys.exit(main())
