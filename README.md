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
session-handoff list --query "refresh" --json
session-handoff list --since 6h --json

# Render handoff prompt for another tool
session-handoff render --id latest --target claude-code

# Export as markdown (default), optionally tailored for target tool
session-handoff export --id latest --target claude-code --output handoff.md

# Export/import portable JSON bundle
session-handoff export --id latest --format json --output handoff.json
session-handoff import --input handoff.json
```

## Status

Current capabilities:
- standard Go modular layout with `cmd/session-handoff`, `internal/handoff`, and reusable `pkg/handoffid`
- local JSON store
- append-only session records with collision-safe unique handoff IDs
- deterministic handoff prompt rendering
- git working-tree signals captured at save time (when project is a git repo)
- `list --json` for scripting, plus `list --tool`, `list --project`, `list --query`, `list --since`, and `list --limit` filters for triage
- markdown + JSON bundle export (`export --target <tool>` tailors markdown handoff context)
- SHA-256 checksum on JSON bundles with verification on import
- JSON bundle import for cross-machine/tool transfer
- RFC3339 UTC timestamp validation for stored/imported records
- crash-safe atomic store updates with lock-based concurrent write protection

## Store location and portability

By default, records are stored in:

- Linux: `${XDG_CONFIG_HOME:-~/.config}/session-handoff/handoffs.json`

For portability, use JSON export/import bundles:

```bash
session-handoff export --id latest --format json --output handoff.json
session-handoff import --input handoff.json
```

## Import trust model

Imports are local and explicit (`--input <file>`). For safety:

- v2 JSON bundles require a SHA-256 checksum match.
- Imported records must have valid RFC3339 UTC (`Z`) timestamps.
- `session-handoff` does not execute imported content; it stores and renders text fields only.
- Treat bundles from untrusted sources as untrusted text data.

## Minimal release plan (v0.x)

- **v0.6.4 — list triage UX pass** ✅
  - added `list --query <text>` filter across title/summary/next plus tests
- **v0.7.0 — trust + UX improvements**
  - signed bundle verification (checksum + signer identity)
  - optional encrypted export bundles
  - TUI selector for prior handoffs

## License

MIT
