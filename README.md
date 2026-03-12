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
# Save a session note (project path should be a git worktree)
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
session-handoff list --id 20260312 --json
session-handoff list --since 6h --json
session-handoff list --latest --json

# Render handoff prompt for another tool (defaults target to generic)
session-handoff render --id latest
session-handoff render --id latest --target claude-code

# Export as markdown (default), optionally tailored for target tool
session-handoff export --id latest --target claude-code --output handoff.md

# Export/import portable JSON bundle (legacy checksum-only)
session-handoff export --id latest --format json --output handoff.json
session-handoff import --input handoff.json --allow-unsigned

# Signed + encrypted bundle workflow (recommended)
session-handoff export --id latest --format json \
  --sign-key ~/.config/session-handoff/signing-key.pem \
  --signer "wregen" \
  --passphrase "$HANDOFF_PASSPHRASE" \
  --output handoff.secure.json
session-handoff import --input handoff.secure.json --passphrase "$HANDOFF_PASSPHRASE"

# Interactive selector / script-friendly output
session-handoff select
session-handoff select --query "refresh" --print-id
session-handoff select --id 20260312 --since 6h --print-id

# Control ID conflicts during import
session-handoff import --input handoff.json --on-conflict skip --allow-unsigned
session-handoff import --input handoff.json --on-conflict replace --allow-unsigned
```

## Status

Current capabilities:
- standard Go modular layout with `cmd/session-handoff`, `internal/handoff`, and reusable `pkg/handoffid`
- local JSON store
- append-only session records with collision-safe unique handoff IDs
- `save --next` validation rejects empty/whitespace-only items to keep action lists clean
- deterministic handoff prompt rendering (`render` now defaults to `target=generic` for quicker copy/paste flows)
- git working-tree signals captured at save time with surfaced git-status errors (invalid/non-git project paths now fail fast)
- `list --json` for scripting, plus `list --id`, `list --tool`, `list --project`, `list --query`, `list --since`, `list --latest`, and `list --limit` filters for triage
- markdown + JSON bundle export (`export --target <tool>` tailors markdown handoff context)
- SHA-256 checksum on JSON bundles plus optional signer identity metadata + ed25519 signature verification
- optional encrypted JSON bundles (`--passphrase`) for transfer-at-rest protection
- interactive handoff selector (`select`) with script-friendly `--print-id` mode and time/id filters (`--since`, `--id`)
- JSON bundle import for cross-machine/tool transfer with conflict handling (`--on-conflict fail|skip|replace`)
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

- v3 signed bundles verify checksum + signer metadata + ed25519 signature.
- v2 checksum-only bundles are legacy and require explicit `--allow-unsigned` during import.
- encrypted bundles require `--passphrase`; wrong passphrase fails import.
- Imported records must have valid RFC3339 UTC (`Z`) timestamps.
- `session-handoff` does not execute imported content; it stores and renders text fields only.
- Treat bundles from untrusted sources as untrusted text data.

## Minimal release plan (v0.x)

- **v0.6.4 — list triage UX pass** ✅
  - added `list --query <text>` filter across title/summary/next plus tests
- **v0.6.8 — import conflict policy UX pass** ✅
  - added `import --on-conflict fail|skip|replace` for safer sync workflows
  - added conflict-policy tests for skip/replace/validation paths
- **v0.7.0 — trust + UX improvements** ✅
  - signed bundle verification (checksum + signer identity)
  - optional encrypted export/import bundles
  - interactive selector for prior handoffs (`select`)
- **v0.7.1 — selector filtering parity** ✅
  - added `select --id <prefix>` and `select --since <duration>` for faster scriptable pick flows
  - aligned `select` validation with `list` (`--limit >= 0`, strict duration parsing)
- **v0.7.2 — save-path reliability + maintainability** ✅
  - split CLI command handlers into dedicated internal files to reduce `main.go` complexity
  - `save` now rejects empty `--next` entries after trim
  - `save` now returns git-status failures directly instead of silently swallowing them
  - CI now pins staticcheck version for reproducible lint runs
- **v0.7.3 — render UX fallback** ✅
  - `render` now defaults `--target` to `generic` (matching `export` behavior)
  - updated usage/docs and added regression test for generic fallback rendering
- **v0.7.4 — list quick-pick UX** ✅
  - added `list --latest` to quickly fetch the newest handoff without manual `--limit` tuning
  - added validation to reject ambiguous `list --latest --limit <n>` combinations
  - added regression tests for latest-selection and flag validation paths

## License

MIT
