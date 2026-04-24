#!/usr/bin/env python3
"""
Manage git worktrees for parallel implementation sessions.

Each implementation scenario gets its own worktree at /tmp/lp-work/<scenario-key>
so multiple sessions can work on different scenario branches simultaneously without
conflicting with the main checkout at /root/legalplant-api (on branch 'master').

Shared Go build cache: export GOCACHE=/root/.cache/go-build-shared
before running `task build` inside a worktree to avoid full rebuilds per session.

Usage:
  worktree.py add <scenario-key>            # create worktree + branch, print path
  worktree.py remove <scenario-key>         # remove worktree (--force)
  worktree.py path <scenario-key>           # print path (for shell capture)
  worktree.py list                          # list all active worktrees
"""
import sys
import os
import subprocess
import argparse

REPO_ROOT = "/root/legalplant-api"
WORKTREE_BASE = "/tmp/lp-work"
BRANCH_PREFIX = "scenario"
DEFAULT_BASE_BRANCH = "master"


def worktree_path(scenario_key):
    return os.path.join(WORKTREE_BASE, scenario_key)


def run(cmd, check=True, capture=False):
    kwargs = dict(cwd=REPO_ROOT, text=True)
    if capture:
        kwargs["capture_output"] = True
    return subprocess.run(cmd, check=check, **kwargs)


def cmd_add(args):
    key = args.scenario_key
    branch = f"{BRANCH_PREFIX}/{key}"
    path = worktree_path(key)

    # If the worktree already exists just print the path and exit (idempotent)
    result = run(["git", "worktree", "list", "--porcelain"], capture=True, check=False)
    if path in (result.stdout or ""):
        print(path)
        return

    # Create the scenario branch off master if it doesn't exist yet
    # (flow verify --init-branch should have already created it)
    run(
        ["git", "branch", branch, DEFAULT_BASE_BRANCH],
        check=False,  # harmless if branch already exists
        capture=True,
    )

    # Create the worktree directory
    os.makedirs(WORKTREE_BASE, exist_ok=True)

    # Add the worktree
    result = run(
        ["git", "worktree", "add", path, branch],
        check=False,
        capture=True,
    )
    if result.returncode != 0:
        err = (result.stderr or "").strip()
        # Already exists (e.g. from a previous run that wasn't cleaned up)
        if "already exists" in err or "already checked out" in err:
            print(path)
            return
        print(f"Error creating worktree: {err}", file=sys.stderr)
        sys.exit(1)

    print(path)


def cmd_remove(args):
    key = args.scenario_key
    path = worktree_path(key)

    result = run(
        ["git", "worktree", "remove", path, "--force"],
        check=False,
        capture=True,
    )
    if result.returncode != 0:
        err = (result.stderr or "").strip()
        if "is not a working tree" in err or "does not exist" in err:
            # Already gone — not an error
            print(f"Worktree {path} already removed (nothing to do).")
            return
        print(f"Error removing worktree: {err}", file=sys.stderr)
        sys.exit(1)

    print(f"Removed worktree: {path}")


def cmd_path(args):
    print(worktree_path(args.scenario_key))


def cmd_list(args):
    result = run(["git", "worktree", "list", "--porcelain"], capture=True, check=True)
    lines = (result.stdout or "").splitlines()

    entries = []
    current = {}
    for line in lines:
        if not line.strip():
            if current:
                entries.append(current)
                current = {}
        elif line.startswith("worktree "):
            current["path"] = line[len("worktree "):]
        elif line.startswith("HEAD "):
            current["head"] = line[len("HEAD "):][:8]
        elif line.startswith("branch "):
            current["branch"] = line[len("branch "):]
        elif line.strip() == "bare":
            current["branch"] = "(bare)"
    if current:
        entries.append(current)

    print(f"{'Path':<45} | {'Branch':<40} | {'HEAD'}")
    print("-" * 100)
    for e in entries:
        path = e.get("path", "")
        branch = e.get("branch", "(detached)").replace("refs/heads/", "")
        head = e.get("head", "")
        print(f"{path:<45} | {branch:<40} | {head}")


def main():
    parser = argparse.ArgumentParser(
        description="Manage git worktrees for parallel implementation sessions"
    )
    subparsers = parser.add_subparsers(dest="command")

    p_add = subparsers.add_parser("add", help="Create worktree + branch, print path")
    p_add.add_argument("scenario_key", help="Scenario key, e.g. s-archive-and-restore-cases")

    p_remove = subparsers.add_parser("remove", help="Remove worktree (force)")
    p_remove.add_argument("scenario_key", help="Scenario key")

    p_path = subparsers.add_parser("path", help="Print worktree path for a scenario key")
    p_path.add_argument("scenario_key", help="Scenario key")

    subparsers.add_parser("list", help="List all active worktrees")

    args = parser.parse_args()

    if args.command == "add":
        cmd_add(args)
    elif args.command == "remove":
        cmd_remove(args)
    elif args.command == "path":
        cmd_path(args)
    elif args.command == "list":
        cmd_list(args)
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
