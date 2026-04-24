#!/usr/bin/env python3
"""
strip_bad_props.py — Remove forbidden property keys from existing graph objects.

Forbidden keys: domain, change_type, new_file, status (in properties dict),
                service, handler (type-specific extras)

Runs against the plan/next-gen branch. Dry-run by default; pass --apply to write.

Usage:
  python3 strip_bad_props.py           # dry-run: show what would change
  python3 strip_bad_props.py --apply   # apply changes
"""

import sys
import json
import subprocess
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from flow_config import load_config as _load_config

_cfg = _load_config()
BRANCH = _cfg["plan_branch_id"]

APPLY = "--apply" in sys.argv

# Keys to strip from ALL types
GLOBAL_FORBIDDEN = {"domain", "change_type", "new_file", "status"}

# Extra keys to strip per type
TYPE_EXTRA: dict[str, set[str]] = {
    "ServiceMethod": {"service"},
    "UIComponent":   {"handler"},
}

TYPES_TO_AUDIT = ["UIComponent", "APIEndpoint", "ServiceMethod", "SQLQuery", "SourceFile"]


def list_objects(obj_type: str) -> list[dict]:
    r = subprocess.run(
        ["memory", "graph", "objects", "list",
         "--type", obj_type, "--branch", BRANCH, "--output", "json"],
        capture_output=True, text=True,
    )
    if r.returncode != 0:
        print(f"ERROR listing {obj_type}: {r.stderr.strip()}", file=sys.stderr)
        return []
    return json.loads(r.stdout.strip()).get("items", [])


def strip_object(entity_id: str, bad_keys: list[str]) -> bool:
    """Null out the forbidden keys via merge (null = delete on the server). Returns True on success."""
    null_patch = {k: None for k in bad_keys}
    r = subprocess.run(
        ["memory", "graph", "objects", "update", entity_id,
         "--properties", json.dumps(null_patch),
         "--branch", BRANCH],
        capture_output=True, text=True,
    )
    if r.returncode != 0:
        print(f"  ERROR: {r.stderr.strip()}", file=sys.stderr)
        return False
    return True


def main() -> None:
    total_dirty = 0
    total_fixed = 0

    for obj_type in TYPES_TO_AUDIT:
        forbidden = GLOBAL_FORBIDDEN | TYPE_EXTRA.get(obj_type, set())
        items = list_objects(obj_type)
        dirty = [(i["entity_id"], i.get("key", ""), i.get("properties", {}))
                 for i in items
                 if any(k in i.get("properties", {}) for k in forbidden)]

        if not dirty:
            print(f"{obj_type:20s}  ✓  all clean")
            continue

        print(f"\n{obj_type:20s}  {len(dirty)} object(s) to fix:")
        for entity_id, key, props in dirty:
            bad_keys = [k for k in props if k in forbidden]
            print(f"  {'APPLY' if APPLY else 'DRY  '} {entity_id[:8]}  {key!r:52s}  strip {bad_keys}")
            total_dirty += 1
            if APPLY:
                ok = strip_object(entity_id, bad_keys)
                if ok:
                    total_fixed += 1

    print()
    if APPLY:
        print(f"Done. Fixed {total_fixed}/{total_dirty} objects.")
    else:
        print(f"Dry run complete. {total_dirty} objects would be updated. Pass --apply to write.")


if __name__ == "__main__":
    main()
