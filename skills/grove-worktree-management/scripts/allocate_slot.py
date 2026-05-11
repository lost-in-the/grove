#!/usr/bin/env python3
"""
allocate_slot.py — Find the lowest available isolated Docker slot number.

Queries `grove ps --json` to see which agent/Docker slots are currently in use,
then recommends the lowest positive integer slot not yet occupied.

Slots are integers 1..N (default max: 8). A slot is "in use" if its `slot`
field appears in the `grove ps --json` output.

Usage:
    python allocate_slot.py
    python allocate_slot.py --max-slots 16
    python allocate_slot.py --dry-run

Output JSON (slot available):
    {
      "recommended_slot": 2,
      "active_slots": [1, 3],
      "available": true
    }

Output JSON (all slots in use):
    {
      "recommended_slot": null,
      "active_slots": [1, 2, 3, 4, 5, 6, 7, 8],
      "available": false,
      "error": "all slots in use (max: 8)"
    }

Exits 0 on success, 1 if grove ps fails.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys


def run_grove_ps() -> list[dict]:
    """Run `grove ps --json` and return the parsed list.

    Raises RuntimeError with a human-readable message on failure.
    """
    try:
        result = subprocess.run(
            ["grove", "ps", "--json"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode != 0:
            stderr = result.stderr.strip()
            raise RuntimeError(
                stderr or f"grove ps exited with code {result.returncode}"
            )

        # grove ps may write JSON to stderr and nothing to stdout when agent
        # stack is not configured (error case but exit 0). Try stdout first,
        # then stderr.
        raw = result.stdout.strip()
        if not raw:
            raw = result.stderr.strip()
        if not raw:
            # No output at all — treat as empty slot list
            return []

        try:
            data = json.loads(raw)
        except json.JSONDecodeError as e:
            raise RuntimeError(f"grove ps returned non-JSON output: {e}") from e

        if isinstance(data, list):
            return data
        if isinstance(data, dict):
            # grove ps may return {"error": true, "message": "..."} when agent
            # stack is not configured — treat as no active slots (not a fatal error)
            if data.get("error"):
                return []
            # May wrap in {"agents": [...]} or similar
            for key in ("agents", "slots", "worktrees"):
                if key in data and isinstance(data[key], list):
                    return data[key]
            # If no known list key and no error, return empty (no agents running)
            return []
        raise RuntimeError(f"unexpected grove ps output shape: {type(data).__name__}")
    except FileNotFoundError:
        raise RuntimeError("grove not found — install grove or add it to PATH")
    except subprocess.TimeoutExpired:
        raise RuntimeError("grove ps timed out after 10s")
    except json.JSONDecodeError as e:
        raise RuntimeError(f"grove ps returned non-JSON output: {e}")


def find_lowest_available(active: set[int], max_slots: int) -> int | None:
    """Return the lowest positive integer not in `active`, up to max_slots."""
    for slot in range(1, max_slots + 1):
        if slot not in active:
            return slot
    return None


def main() -> None:
    parser = argparse.ArgumentParser(
        description=(
            "Find the lowest available isolated Docker slot number by checking "
            "which slots are currently in use via `grove ps --json`. "
            "Exits 0 on success, 1 if grove ps cannot be run."
        ),
        epilog=(
            "Examples:\n"
            "  python allocate_slot.py\n"
            "  python allocate_slot.py --max-slots 16\n"
            "  python allocate_slot.py --dry-run"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--max-slots",
        type=int,
        default=8,
        metavar="N",
        help="Maximum number of slots to consider (default: 8)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Compute and print the result without reserving anything (always true — this script is read-only)",
    )
    args = parser.parse_args()

    if args.max_slots < 1:
        print("error: --max-slots must be at least 1", file=sys.stderr)
        sys.exit(1)

    try:
        agents = run_grove_ps()
    except RuntimeError as e:
        print(f"error: {e}", file=sys.stderr)
        sys.exit(1)

    active_slots: list[int] = sorted(
        {
            int(entry["slot"])
            for entry in agents
            if "slot" in entry and entry["slot"] is not None
        }
    )
    active_set = set(active_slots)

    recommended = find_lowest_available(active_set, args.max_slots)

    output: dict = {
        "recommended_slot": recommended,
        "active_slots": active_slots,
        "available": recommended is not None,
    }

    if recommended is None:
        output["error"] = f"all slots in use (max: {args.max_slots})"

    if args.dry_run:
        output["dry_run"] = True

    print(json.dumps(output, indent=2))


if __name__ == "__main__":
    main()
