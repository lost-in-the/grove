#!/usr/bin/env python3
"""
strip_directives.py — Filter grove shell-integration directive lines from stdin.

Grove commands emit special directive lines (cd:, tmux-attach:, env:, etc.)
that are intercepted by the shell wrapper. When running grove outside the
shell wrapper (e.g., in scripts or pipelines), these directives appear as
noise on stdout. This script strips them.

In normal mode (default): passes through all non-directive lines to stdout.
In --show-directives mode: prints ONLY the directive lines as a JSON array.

Directive prefixes recognized:
    cd:               → type "cd"
    tmux-attach:      → type "tmux-attach"
    tmux-attach-cc:   → type "tmux-attach-cc"
    env:              → type "env"

Usage:
    grove to feature 2>&1 | python strip_directives.py
    grove to feature 2>&1 | python strip_directives.py --show-directives
    echo "cd:/some/path" | python strip_directives.py --show-directives

Exits 0 always.
"""

from __future__ import annotations

import argparse
import json
import sys

# Ordered longest-first so "tmux-attach-cc:" is matched before "tmux-attach:"
DIRECTIVE_PREFIXES = [
    ("tmux-attach-cc:", "tmux-attach-cc"),
    ("tmux-attach:", "tmux-attach"),
    ("cd:", "cd"),
    ("env:", "env"),
]


def classify_line(line: str) -> tuple[str, str] | None:
    """Return (type, value) if line is a directive, else None."""
    stripped = line.rstrip("\n")
    for prefix, directive_type in DIRECTIVE_PREFIXES:
        if stripped.startswith(prefix):
            value = stripped[len(prefix):]
            return directive_type, value
    return None


def main() -> None:
    parser = argparse.ArgumentParser(
        description=(
            "Filter grove shell-integration directive lines from stdin. "
            "In normal mode, passes through non-directive lines unchanged. "
            "With --show-directives, prints only directive lines as a JSON array."
        ),
        epilog=(
            "Examples:\n"
            "  grove to feature 2>&1 | python strip_directives.py\n"
            "  grove to feature 2>&1 | python strip_directives.py --show-directives"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--show-directives",
        action="store_true",
        help=(
            "Instead of stripping directives, print ONLY the directive lines "
            "as a JSON array of {type, value} objects."
        ),
    )
    args = parser.parse_args()

    directives: list[dict] = []
    normal_lines: list[str] = []

    for line in sys.stdin:
        result = classify_line(line)
        if result is not None:
            directive_type, value = result
            directives.append({"type": directive_type, "value": value})
        else:
            normal_lines.append(line)

    if args.show_directives:
        print(json.dumps(directives, indent=2))
    else:
        for line in normal_lines:
            sys.stdout.write(line)


if __name__ == "__main__":
    main()
