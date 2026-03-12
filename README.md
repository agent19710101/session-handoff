# session-handoff

Portable handoff bundles for AI coding sessions.

`session-handoff` is a Go CLI that records coding-session context and renders a clean handoff prompt so work can continue in another tool without losing intent.

## Problem

Agent workflows are fragmented (Codex, Claude Code, Cursor, Gemini CLI, CI bots). Session state often stays trapped in one tool, making handoff slow and lossy.

`session-handoff` keeps a local structured history and produces deterministic handoff bundles you can render, export, and now import on another machine.

## Install

```bash
go install github.com/agent19710101/session-handoff@latest
```

## Examples

```bash
# Save a session note
session-handoff save \
  --tool codex \
  --project ~/repos/my-app \
  --title "Refactor auth middleware" \
  --summary "Extracted token parser, tests still failing on refresh flow" \
  --next "Fix refresh test matrix" \
  --next "Run race tests"

# List handoffs (human or JSON)
session-handoff list
session-handoff list --tool codex --limit 5
session-handoff list --project ~/repos/my-app --json
session-handoff list --json --tool claude-code
session-handoff list --since 6h --json

# Render handoff prompt for another tool
session-handoff render --id latest --target claude-code

# Export as markdown (default)
session-handoff export --id latest --output handoff.md

# Export/import portable JSON bundle
session-handoff export --id latest --format json --output handoff.json
session-handoff import --input handoff.json
```

## Status

Current capabilities:
- modular CLI code layout (`main`, `store`, `filters`, `render`, `git` concerns split for easier iteration)
- local JSON store
- append-only session records with collision-safe unique handoff IDs
- deterministic handoff prompt rendering
- git working-tree signals captured at save time (when project is a git repo)
- `list --json` for scripting, plus `list --tool`, `list --project`, `list --since`, and `list --limit` filters for triage
- markdown + JSON bundle export
- SHA-256 checksum on JSON bundles with verification on import
- JSON bundle import for cross-machine/tool transfer

## Minimal release plan (v0.x)

- **v0.6.0 — reliability + CI gate**
  - robust git status parsing for spaces/renames ([#2](https://github.com/agent19710101/session-handoff/issues/2))
  - GitHub Actions for `gofmt` + `go test` on push/PR ([#3](https://github.com/agent19710101/session-handoff/issues/3))
- **v0.7.0 — command quality hardening**
  - expand tests for `save`/`render`/`export` edge cases ([#4](https://github.com/agent19710101/session-handoff/issues/4))
- **v0.8.0 — deeper modularization**
  - continue command/package split beyond current modular baseline ([#5](https://github.com/agent19710101/session-handoff/issues/5))
- **v0.9.0 — trust + UX improvements**
  - signed bundle verification (checksum + signer identity)
  - optional encrypted export bundles
  - TUI selector for prior handoffs

## License

MIT
