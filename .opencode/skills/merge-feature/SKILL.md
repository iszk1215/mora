---
name: merge-feature
description: Merge a completed feature branch into main (local only, no push). Optionally close a related Gitea issue via MCP.
---

## When to use
Load this skill when the user requests merging a completed feature branch into main.
If the work was associated with a Gitea issue, the user may specify the issue number (e.g., "issue #42 をマージしてください").

## Constraints
- **Never push to any remote.** Only local git operations.
- Never modify the local `main` branch (no pull, no fetch of main).
- If a linked Gitea issue is specified, close it via Gitea MCP after the merge.

## Workflow

1. **Issue check**: If the user specified an issue number, record it. Otherwise, skip issue closing.
2. **Branch check**: Verify the current branch starts with `feature/`. Extract the branch name (e.g., `feature/add-login`).
3. **Fetch (read-only)**: Run `git fetch --all --prune`. This is informational only — local branches are never updated by fetch.
4. **Rebase onto local main**: `git rebase main`. If conflicts occur, abort and ask the user to resolve them manually. After resolution, re-run the skill.
5. **Verify**: Run `make test-all && make lint-all`. If any failure, stop and ask the user to fix it before continuing.
6. **Switch to main**: `git switch main`.
7. **Merge**: `git merge --no-ff feature/<name>`. If conflicts occur, stop and ask the user to resolve them.
8. **Close issue (optional)**: If an issue number was specified, use `gitea-mcp_issue_write` with `method=update`, `state=closed` (no comment).
9. **Notify**: Tell the user the merge is complete locally. Remind them that push is not done and must be performed manually if needed.
