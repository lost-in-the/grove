#!/usr/bin/env python3
"""
audit_hooks.py — Summarize what hooks would run for each grove event.

Reads `.grove/hooks.toml` and `.grove/config.toml` from the target directory
and produces a structured JSON summary of every hook, grouped by event.

Assigns a risk level based on command patterns:
    none   — no hooks defined
    low    — read-only operations (cp, ln, echo, mkdir, touch)
    medium — unrecognized commands
    high   — destructive or network-fetching commands (rm -rf, curl, wget, sh -c, subshells)

Usage:
    python audit_hooks.py
    python audit_hooks.py --dir /path/to/repo
    python audit_hooks.py --dir /path/to/repo | jq .events.post_create

Exits 0 always — error state is represented in the JSON output.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys

# ---------------------------------------------------------------------------
# TOML parsing
# ---------------------------------------------------------------------------

try:
    import tomllib  # Python 3.11+
except ImportError:
    try:
        import tomli as tomllib  # type: ignore[no-redef]
    except ImportError:
        tomllib = None  # type: ignore[assignment]


def _parse_toml_basic(text: str) -> dict:
    """Minimal line-scanner for the subset of TOML used in hooks.toml / config.toml.

    Supports:
    - [section] and [[array_of_tables]] headers
    - key = "string value" pairs
    - key = integer pairs
    - # comments
    - Inline strings only (no multi-line)

    This is intentionally limited — just enough for grove's config files.
    """
    result: dict = {}
    current_path: list[str] = []
    current_array_key: str | None = None
    current_obj: dict | None = None

    def resolve_path(root: dict, path: list[str]) -> dict:
        node = root
        for key in path:
            if key not in node:
                node[key] = {}
            node = node[key]
        return node

    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue

        # [[array_of_tables]]
        m = re.match(r'^\[\[(.+?)\]\]$', line)
        if m:
            key = m.group(1).strip()
            current_array_key = key
            current_path = []
            # Push a new object into the array
            parts = key.split(".")
            node = result
            for part in parts[:-1]:
                node = node.setdefault(part, {})
            arr = node.setdefault(parts[-1], [])
            current_obj = {}
            arr.append(current_obj)
            continue

        # [section]
        m = re.match(r'^\[(.+?)\]$', line)
        if m:
            current_array_key = None
            current_obj = None
            current_path = [k.strip() for k in m.group(1).split(".")]
            continue

        # key = value
        m = re.match(r'^(\w+)\s*=\s*(.+)$', line)
        if m:
            key = m.group(1)
            raw_val = m.group(2).strip()

            # Quoted string
            if raw_val.startswith('"') and raw_val.endswith('"'):
                val: str | int | bool = raw_val[1:-1]
            elif raw_val.startswith("'") and raw_val.endswith("'"):
                val = raw_val[1:-1]
            elif raw_val in ("true", "false"):
                val = raw_val == "true"
            else:
                try:
                    val = int(raw_val)
                except ValueError:
                    val = raw_val  # leave as string

            if current_obj is not None:
                current_obj[key] = val
            else:
                target = resolve_path(result, current_path)
                target[key] = val

    return result


def load_toml(path: str) -> tuple[dict | None, str | None]:
    """Load a TOML file. Returns (data, None) or (None, error_message)."""
    if not os.path.exists(path):
        return None, f"{path} not found"
    try:
        with open(path, "rb") as fh:
            raw = fh.read()
    except OSError as e:
        return None, str(e)

    if tomllib is not None:
        try:
            return tomllib.loads(raw.decode("utf-8")), None
        except Exception as e:
            return None, f"TOML parse error: {e}"
    else:
        try:
            return _parse_toml_basic(raw.decode("utf-8")), None
        except Exception as e:
            return None, f"basic TOML parse error: {e}"


# ---------------------------------------------------------------------------
# Risk assessment
# ---------------------------------------------------------------------------

HIGH_PATTERNS = [
    r"\brm\s+-rf\b",
    r"\bcurl\b",
    r"\bwget\b",
    r"\bsh\s+-c\b",
    r"\bbash\s+-c\b",
    r"`[^`]+`",          # backtick subshells
    r"\$\([^)]+\)",      # $(subshell)
    r"\beval\b",
    r"\bexec\b",
    r"\bsudo\b",
]

LOW_COMMANDS = {
    "cp", "ln", "echo", "mkdir", "touch", "printf", "cat",
    "ls", "pwd", "true", "false", "test", "[",
}


def assess_command_risk(cmd: str) -> str:
    """Return "high", "medium", or "low" for a single command string."""
    for pattern in HIGH_PATTERNS:
        if re.search(pattern, cmd):
            return "high"
    # Extract the base command name
    base = cmd.strip().split()[0] if cmd.strip() else ""
    base_name = os.path.basename(base)
    if base_name in LOW_COMMANDS:
        return "low"
    return "medium"


def overall_risk(events: dict[str, list[dict]]) -> str:
    all_hooks = [h for hooks in events.values() for h in hooks]
    if not all_hooks:
        return "none"
    levels = [assess_command_risk(h.get("command", "")) for h in all_hooks]
    if "high" in levels:
        return "high"
    if "medium" in levels:
        return "medium"
    return "low"


# ---------------------------------------------------------------------------
# Hook extraction
# ---------------------------------------------------------------------------

KNOWN_EVENTS = [
    "post_create",
    "pre_switch",
    "post_switch",
    "pre_remove",
    "post_remove",
]


def extract_hooks(hooks_data: dict) -> dict[str, list[dict]]:
    """Extract hooks from parsed hooks.toml data, grouped by event."""
    events: dict[str, list[dict]] = {e: [] for e in KNOWN_EVENTS}

    raw_hooks = hooks_data.get("hooks", [])
    if not isinstance(raw_hooks, list):
        return events

    for hook in raw_hooks:
        if not isinstance(hook, dict):
            continue
        event = hook.get("event")
        if not event:
            continue
        entry = {k: v for k, v in hook.items() if k != "event"}
        if event not in events:
            events[event] = []
        events[event].append(entry)

    return events


def extract_plugin_docker_actions(config_data: dict, events: dict[str, list[dict]]) -> None:
    """Pull any docker plugin action hooks into the events dict (in-place)."""
    plugins = config_data.get("plugins", {})
    docker = plugins.get("docker", {})
    actions = docker.get("actions", {})
    if not isinstance(actions, dict):
        return

    # Map docker plugin action names to grove events where possible
    action_event_map = {
        "post_create": "post_create",
        "pre_remove": "pre_remove",
        "post_remove": "post_remove",
        "pre_switch": "pre_switch",
        "post_switch": "post_switch",
    }
    for action_name, event_name in action_event_map.items():
        action = actions.get(action_name)
        if not action:
            continue
        if isinstance(action, str):
            entry = {"command": action, "source": "plugins.docker"}
        elif isinstance(action, dict):
            entry = {**action, "source": "plugins.docker"}
        else:
            continue
        if event_name not in events:
            events[event_name] = []
        events[event_name].append(entry)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(
        description=(
            "Read .grove/hooks.toml and .grove/config.toml from a directory "
            "and summarize what hooks would run for each grove event. "
            "Assigns a risk level based on command patterns. "
            "Always exits 0 — errors are represented in the JSON output."
        ),
        epilog=(
            "Examples:\n"
            "  python audit_hooks.py\n"
            "  python audit_hooks.py --dir /path/to/repo\n"
            "  python audit_hooks.py | jq .events.post_create"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--dir",
        default=".",
        metavar="PATH",
        help="Directory to inspect (default: current working directory)",
    )
    args = parser.parse_args()

    base = os.path.abspath(args.dir)
    hooks_path = os.path.join(base, ".grove", "hooks.toml")
    config_path = os.path.join(base, ".grove", "config.toml")

    notes: list[str] = []

    hooks_data, hooks_err = load_toml(hooks_path)
    config_data, config_err = load_toml(config_path)

    grove_dir = os.path.join(base, ".grove")
    if not os.path.isdir(grove_dir):
        notes.append("no grove config found — .grove/ directory does not exist")

    if hooks_data is None:
        notes.append(f"hooks.toml not found — no custom hooks defined")
        hooks_data = {}
    if config_data is None:
        notes.append(f"config.toml not found")
        config_data = {}

    if hooks_err and "not found" not in hooks_err:
        notes.append(f"hooks.toml parse error: {hooks_err}")
    if config_err and "not found" not in config_err:
        notes.append(f"config.toml parse error: {config_err}")

    events = extract_hooks(hooks_data)
    extract_plugin_docker_actions(config_data, events)

    risk = overall_risk(events)

    output = {
        "hooks_file": os.path.join(".grove", "hooks.toml"),
        "config_file": os.path.join(".grove", "config.toml"),
        "events": events,
        "risk_level": risk,
        "notes": notes,
    }

    print(json.dumps(output, indent=2))


if __name__ == "__main__":
    main()
