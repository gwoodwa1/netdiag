#!/usr/bin/env python3
"""Fail when a relative Markdown link points at a missing repository path."""

from pathlib import Path
import re
import sys
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[2]
LINK = re.compile(r"!?\[[^\]]*\]\(([^)\s]+)(?:\s+[^)]*)?\)")
problems = []

for markdown in sorted(ROOT.rglob("*.md")):
    if ".git" in markdown.parts:
        continue
    for line_number, line in enumerate(markdown.read_text(encoding="utf-8").splitlines(), 1):
        for target in LINK.findall(line):
            if target.startswith(("http://", "https://", "mailto:", "#")):
                continue
            path = unquote(target.split("#", 1)[0])
            if path and not (markdown.parent / path).resolve().exists():
                problems.append(f"{markdown.relative_to(ROOT)}:{line_number}: missing {target}")

if problems:
    print("\n".join(problems), file=sys.stderr)
    raise SystemExit(1)

print("Markdown links resolve")
