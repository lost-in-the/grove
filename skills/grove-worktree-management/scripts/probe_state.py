#!/usr/bin/env python3
"""
probe_state.py — Emit a normalized combined grove repo status object.

Runs `grove here --json` and `grove ls --json` and combines the results
into a single JSON document. Useful for agents to understand full repo
state before taking action.

Exits 0 always — error state is represented in the JSON output.

Usage:
    python probe_state.py
    python probe_state.py | jq .current.branch
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys


def run_grove(args: list[str]) -> tuple[dict | list, str | None]:
    """Run a grove command and return (parsed_json, error_message).

    Returns (data, None) on success, (None, error_msg) on failure.
    """
    try:
        result = subprocess.run(
            ["grove"] + args,
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode != 0:
            stderr = result.stderr.strip()
            return None, stderr or f"grove {' '.join(args)} exited {result.returncode}"
        return json.loads(result.stdout), None
    except FileNotFoundError:
        return None, "grove not found — install grove or add it to PATH"
    except subprocess.TimeoutExpired:
        return None, f"grove {' '.join(args)} timed out after 10s"
    except json.JSONDecodeError as e:
        return None, f"grove {' '.join(args)} returned non-JSON output: {e}"


def main() -> None:
    parser = argparse.ArgumentParser(
        description=(
            "Probe the current grove repo state by running `grove here --json` "
            "and `grove ls --json`, then emit a combined normalized status object. "
            "Always exits 0; error conditions are encoded in the JSON output."
        ),
        epilog="Example: python probe_state.py | jq .current.branch",
    )
    parser.parse_args()

    here_data, here_err = run_grove(["here", "--json"])
    ls_data, ls_err = run_grove(["ls", "--json"])

    grove_available = here_err is None or ls_err is None

    # If neither command succeeded, check specifically for "not installed"
    if here_err and "not found" in here_err:
        grove_available = False

    if not grove_available or (here_data is None and ls_data is None):
        error_msg = here_err or ls_err or "grove commands failed"
        output = {
            "current": None,
            "worktrees": [],
            "grove_available": False,
            "error": error_msg,
        }
    else:
        errors = []
        if here_err:
            errors.append(f"grove here: {here_err}")
        if ls_err:
            errors.append(f"grove ls: {ls_err}")

        # grove ls --json returns either a bare list or {"worktrees": [...]}
        if isinstance(ls_data, list):
            worktrees = ls_data
        elif isinstance(ls_data, dict):
            worktrees = ls_data.get("worktrees", [])
        else:
            worktrees = []

        output: dict = {
            "current": here_data,
            "worktrees": worktrees,
            "grove_available": True,
        }
        if errors:
            output["errors"] = errors

    print(json.dumps(output, indent=2))


if __name__ == "__main__":
    main()
