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
- local JSON store
- append-only session records
- deterministic handoff prompt rendering
- git working-tree signals captured at save time (when project is a git repo)
- `list --json` for scripting, plus `list --tool`, `list --project`, `list --since`, and `list --limit` filters for triage
- markdown + JSON bundle export
- SHA-256 checksum on JSON bundles with verification on import
- JSON bundle import for cross-machine/tool transfer

## Roadmap

- signed bundle verification (checksum + signer identity)
- TUI selector for previous sessions
- optional encrypted export bundles

## License

MIT
