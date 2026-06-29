# driftdoc

> Keep docs honest. `driftdoc` checks machine-verifiable **claims** about your codebase (declared in a sidecar `drift.yaml`) and **fails CI when a doc stops being true** — instead of letting it rot silently.

**Status:** 🟡 alpha — MVP `check` works (Tier-1 text matchers). Roadmap: structured/symbol/behavioral matchers + reverse-impact, AI harvest/fix, coverage. See [`DESIGN.md`](DESIGN.md).

## Why

Docs drift from code the moment someone changes the code and not the doc. Re-reading everything by hand doesn't scale, and pinning to `file:line` rots instantly. driftdoc makes each fact **checkable**: it grep-verifies a claim against the code, and a *miss is the alarm*.

Works for **any codebase** — Go, TypeScript, SQL, config, markdown — because matchers operate on file content, not on any one language.

## Quick start

```bash
go install github.com/thesatellite-ai/driftdoc/cmd/driftdoc@latest

driftdoc init          # scaffold a starter drift.yaml
driftdoc check         # verify every claim · exit 0 = current, 1 = drift, 2 = error
driftdoc check --json  # machine-readable (CI / editors / bots)
```

## The model

A **claim** is `(what we assert, where to look, what to expect)`:

```yaml
version: 1
defaults:
  root: .
claims:
  - id: apiadmin-versioned
    desc: the apiadmin GraphQL API is served at /api/admin/v1/query
    where: modules/apiadmin/boot.go
    match: contains
    expect: "/api/admin/v1/query"

  - id: robot-removed
    desc: the robot/system user was removed
    where: "modules/**/*.go"
    match: absent              # must appear in ZERO files
    expect: "EnsureSystemUser"
```

You scale by adding **rows**, never by writing per-fact code.

### Matchers (Tier 1 — text, language-agnostic)

| `match` | passes when |
|---|---|
| `contains` | the `expect` substring appears in ≥1 matched file |
| `regex` | the `expect` RE2 pattern matches in ≥1 file |
| `absent` | `expect` appears in **0** files (documents a deletion) |
| `count` | `expect` appears exactly `count:` times |

`where` is a file path or a `doublestar` glob (`modules/**/*.go`).

### Exit codes (the CI contract)

`0` = all current · `1` = drift found · `2` = config/IO error. Drop `driftdoc check` into CI and stale docs break the build.

## Roadmap

Tier 2 (structured: json/yaml/xml/html) · Tier 3 (symbol/AST via tree-sitter, rename-resilient) · Tier 4 (behavioral/live: HTTP, CLI, introspection) · `report` (stamp status into the doc) · `gen` (generate doc sections from code) · `fix` (AI repair) · **reverse impact** (a diff → which documented claims to review) · coverage score + PR bot. Full design + decisions in [`DESIGN.md`](DESIGN.md).
