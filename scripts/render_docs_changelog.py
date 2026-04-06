#!/usr/bin/env python3

import argparse
import json
import re
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Render the docs changelog page from GitHub releases JSON.")
    parser.add_argument("--releases-json", required=True, help="Path to a JSON array returned by the GitHub releases API")
    parser.add_argument("--output", required=True, help="Path to the markdown file to write")
    return parser.parse_args()


def strip_duplicate_heading(tag: str, body: str) -> str:
    text = (body or "").strip()
    if not text:
        return ""

    lines = text.splitlines()
    first_non_empty = next((index for index, line in enumerate(lines) if line.strip()), None)
    if first_non_empty is None:
        return ""

    heading_pattern = re.compile(rf"^#{{1,6}}\s+{re.escape(tag)}\s*$")
    if heading_pattern.match(lines[first_non_empty].strip()):
        lines = lines[first_non_empty + 1 :]
        while lines and not lines[0].strip():
            lines.pop(0)

    return "\n".join(lines).strip()


def render_release(release: dict[str, object]) -> list[str]:
    tag = str(release.get("tag_name") or release.get("name") or "Unversioned release")
    url = str(release.get("html_url") or "").strip()
    published_at = str(release.get("published_at") or "").strip()
    body = strip_duplicate_heading(tag, str(release.get("body") or ""))

    heading = f"## [{tag}]({url})" if url else f"## {tag}"
    lines = [heading, ""]

    if published_at:
        lines.append(f"Published: {published_at[:10]}")
        lines.append("")

    if body:
        lines.append(body)
    else:
        lines.append("Release notes are available on GitHub.")

    lines.append("")
    return lines


def main() -> None:
    args = parse_args()
    releases_path = Path(args.releases_json)
    output_path = Path(args.output)

    releases = json.loads(releases_path.read_text(encoding="utf-8"))
    published_releases = [release for release in releases if not release.get("draft")]

    lines = ["# Changelog", ""]
    if not published_releases:
        lines.append("Release notes are published on the GitHub Releases page as versions become available.")
        lines.append("")
        lines.append("- GitHub Releases: https://github.com/vriesdemichael/bitbucket-server-cli/releases")
    else:
        for release in published_releases:
            lines.extend(render_release(release))

    output_path.write_text("\n".join(lines).rstrip() + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()