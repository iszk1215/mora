---
name: proceed-implementation
description: Execute an approved implementation plan: create feature branch, write code/tests, lint, build, and commit
---

## When to use
Load this skill when the user has reviewed your plan and explicitly said to proceed ("進んでください").

## Workflow
1. **Create feature branch**: `feature/<short-description>` (kebab-case) from current branch
2. **Implement**: Code changes per the approved plan
3. **Test & lint**: Write/update tests, run `make test-all` and `make lint-all`, fix all failures
4. **Build**: `make build-all` -> builds frontend (`server/static/public/`) + Go binary (`bin/mora`)
5. **Note on build artifacts**:
   - `server/static/public/` is embedded in the binary -> **committed**
   - `bin/mora` is in `.gitignore` -> **not committed**
6. **Docs**: Update `docs/` if needed
7. **Commit**: Source + frontend artifacts + docs (not `bin/mora`)
8. **Notify**: Tell user `bin/mora` has been rebuilt with the changes
