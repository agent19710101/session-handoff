# session-handoff

Portable handoff bundles for AI coding sessions.

`session-handoff` is a Go CLI that records coding session context and renders a clean, tool-specific handoff prompt so work can continue in another agent/tool without losing intent.

## Why

Agent workflows are fragmented: Codex, Claude Code, Cursor, Gemini CLI, and CI bots all keep context differently.
This tool keeps a local, structured history and generates consistent handoff briefs.

## Install

```bash
go install github.com/agent19710101/session-handoff@latest
```

## Quick start

```bash
# save a session note
session-handoff save \
  --tool codex \
  --project ~/repos/my-app \
  --title "Refactor auth middleware" \
  --summary "Extracted token parser, tests still failing on refresh flow" \
  --next "Fix refresh test matrix" \
  --next "Run race tests"

# see history
session-handoff list

# render handoff for another tool
session-handoff render --id latest --target claude-code
```

## Status

v0 focuses on:
- local JSON store
- append-only session records
- deterministic handoff prompt rendering

Planned next:
- git-aware changed-files capture
- TUI for selecting prior sessions
- export/import bundles

## License

MIT
