#!/usr/bin/env python3
"""
pr_review.py — Orchestrate a non-destructive PR review workflow.

Steps:
  1. Check if a worktree for PR #N already exists (via `grove ls --json`).
  2. If not: run `grove fetch pr/{N}` to create the worktree.
  3. Locate the new worktree name in `grove ls --json`.
  4. Run `grove to <name> --peek` to switch without hooks or tmux.
  5. Emit a structured JSON result.

In --dry-run mode: prints the commands that would run without executing them.

Requirements:
  - grove CLI must be installed and in PATH
  - `gh` CLI must be installed (used internally by `grove fetch`)

Usage:
    python pr_review.py 42
    python pr_review.py 42 --repo owner/repo
    python pr_review.py 42 --dry-run

Exits 0 on success, 1 on failure.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from typing import Optional


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def run(cmd: list[str], *, capture: bool = True, timeout: int = 60) -> subprocess.CompletedProcess:
    """Run a command, returning the CompletedProcess. Raises on timeout."""
    return subprocess.run(
        cmd,
        capture_output=capture,
        text=True,
        timeout=timeout,
    )


def grove_ls() -> list[dict]:
    """Return parsed `grove ls --json` output.

    Raises RuntimeError on failure.
    """
    try:
        result = run(["grove", "ls", "--json"])
    except FileNotFoundError:
        raise RuntimeError("grove not found — install grove or add it to PATH")
    except subprocess.TimeoutExpired:
        raise RuntimeError("grove ls timed out")

    if result.returncode != 0:
        raise RuntimeError(
            result.stderr.strip() or f"grove ls exited {result.returncode}"
        )
    try:
        data = json.loads(result.stdout)
    except json.JSONDecodeError as e:
        raise RuntimeError(f"grove ls returned non-JSON: {e}")

    if isinstance(data, list):
        return data
    if isinstance(data, dict):
        # grove ls --json may return {"project": ..., "worktrees": [...]}
        if "worktrees" in data and isinstance(data["worktrees"], list):
            return data["worktrees"]
        raise RuntimeError(f"grove ls returned unexpected dict shape (keys: {list(data.keys())})")
    raise RuntimeError(f"grove ls output was not a list or dict: {type(data).__name__}")


def find_pr_worktree(worktrees: list[dict], pr_number: int) -> Optional[dict]:
    """Return the first worktree whose name contains pr-{N}, or None."""
    pattern = f"pr-{pr_number}"
    for wt in worktrees:
        # Support both snake_case (spec) and camelCase (actual grove output)
        name = wt.get("fullName") or wt.get("full_name") or wt.get("name") or ""
        if pattern in name:
            return wt
    return None


def fail(msg: str, output: dict) -> None:
    """Print error to stderr and exit 1 after dumping partial JSON."""
    print(f"error: {msg}", file=sys.stderr)
    output["error"] = msg
    print(json.dumps(output, indent=2))
    sys.exit(1)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(
        description=(
            "Orchestrate a non-destructive PR review workflow. "
            "Fetches the PR worktree with `grove fetch pr/N` if it does not "
            "already exist, then switches to it with `grove to <name> --peek` "
            "(no hooks, no tmux). Emits a JSON result."
        ),
        epilog=(
            "Examples:\n"
            "  python pr_review.py 42\n"
            "  python pr_review.py 42 --repo owner/repo\n"
            "  python pr_review.py 42 --dry-run"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "pr_number",
        type=int,
        metavar="PR_NUMBER",
        help="GitHub pull request number to review",
    )
    parser.add_argument(
        "--repo",
        metavar="OWNER/REPO",
        help="GitHub repository (e.g. acme/myapp). Passed to `grove fetch` as --repo.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print commands that would run without executing them.",
    )
    args = parser.parse_args()

    pr_number: int = args.pr_number
    dry_run: bool = args.dry_run

    output: dict = {
        "pr_number": pr_number,
        "worktree_name": None,
        "worktree_path": None,
        "action": None,
        "switched": False,
        "dry_run": dry_run,
        "next_steps": [],
    }

    # -----------------------------------------------------------------------
    # Step 1: Check for existing worktree
    # -----------------------------------------------------------------------
    if dry_run:
        print(f"[dry-run] would run: grove ls --json", file=sys.stderr)
        # Still run ls so we can report existing vs would_create
        try:
            worktrees = grove_ls()
        except RuntimeError as e:
            fail(str(e), output)
            return  # unreachable but satisfies type checker
    else:
        try:
            worktrees = grove_ls()
        except RuntimeError as e:
            fail(str(e), output)
            return

    existing = find_pr_worktree(worktrees, pr_number)

    # -----------------------------------------------------------------------
    # Step 2: Fetch if not exists
    # -----------------------------------------------------------------------
    fetch_cmd = ["grove", "fetch", f"pr/{pr_number}"]
    if args.repo:
        fetch_cmd += ["--repo", args.repo]

    if existing:
        action = "existing" if not dry_run else "would_use_existing"
        # Support both camelCase (actual grove) and snake_case (spec)
        wt_name = existing.get("fullName") or existing.get("full_name") or existing.get("name", "")
        wt_path = existing.get("path", "")
    else:
        action = "would_create" if dry_run else "created"
        wt_name = None
        wt_path = None

        if dry_run:
            print(f"[dry-run] would run: {' '.join(fetch_cmd)}", file=sys.stderr)
        else:
            try:
                result = run(fetch_cmd, timeout=120)
            except FileNotFoundError:
                fail("grove not found — install grove or add it to PATH", output)
                return
            except subprocess.TimeoutExpired:
                fail("grove fetch timed out after 120s", output)
                return

            if result.returncode != 0:
                err = result.stderr.strip() or f"grove fetch exited {result.returncode}"
                output["action"] = action
                fail(err, output)
                return

            # Re-query ls to find the new worktree
            try:
                worktrees = grove_ls()
            except RuntimeError as e:
                fail(str(e), output)
                return

            new_wt = find_pr_worktree(worktrees, pr_number)
            if new_wt is None:
                fail(
                    f"grove fetch succeeded but no worktree matching pr-{pr_number} found in grove ls",
                    output,
                )
                return

            wt_name = new_wt.get("fullName") or new_wt.get("full_name") or new_wt.get("name", "")
            wt_path = new_wt.get("path", "")

    output["action"] = action
    output["worktree_name"] = wt_name
    output["worktree_path"] = wt_path

    # -----------------------------------------------------------------------
    # Step 3: Switch with --peek (no hooks, no tmux)
    # -----------------------------------------------------------------------
    if wt_name:
        peek_cmd = ["grove", "to", wt_name, "--peek"]
    else:
        peek_cmd = ["grove", "to", f"pr-{pr_number}", "--peek"]

    if dry_run:
        print(f"[dry-run] would run: {' '.join(peek_cmd)}", file=sys.stderr)
        output["switched"] = False
    else:
        try:
            result = run(peek_cmd, timeout=30)
        except FileNotFoundError:
            fail("grove not found — install grove or add it to PATH", output)
            return
        except subprocess.TimeoutExpired:
            fail("grove to --peek timed out after 30s", output)
            return

        if result.returncode != 0:
            err = result.stderr.strip() or f"grove to exited {result.returncode}"
            fail(err, output)
            return

        output["switched"] = True

    # -----------------------------------------------------------------------
    # Step 4: Build next_steps
    # -----------------------------------------------------------------------
    path_display = wt_path or f"(worktree for pr-{pr_number})"
    rm_target = wt_name or f"<worktree-name>"

    if dry_run:
        output["next_steps"] = [
            f"Run without --dry-run to create and switch to the PR worktree",
            f"After review, run: grove rm {rm_target}",
        ]
    else:
        output["next_steps"] = [
            f"Review changes in {path_display}",
            f"Run `grove rm {rm_target}` when done",
        ]

    print(json.dumps(output, indent=2))


if __name__ == "__main__":
    main()
