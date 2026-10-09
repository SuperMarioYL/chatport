**English** | [简体中文](./README.md)

<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/hero-dark.svg" />
  <source media="(prefers-color-scheme: light)" srcset="assets/hero-light.svg" />
  <img src="assets/hero-light-static.svg" alt="ChatPort: an official export zip enters on the left, is parsed into the glowing Context Pack in the middle (conversations, projects, memory), then flows to the next Agent on the right" width="100%" />
</picture>

**ChatPort turns official AI chat exports into a Context Pack your next Agent can ingest.**

Three years of projects and decisions live in your ChatGPT history. Switching to Claude, Kimi, or a fresh Coding Agent should not mean starting from zero. ChatPort converts official exports into a Context Pack the next agent can swallow in one bite — move in, don't start over.

[![License: MIT](https://img.shields.io/badge/license-MIT-56803b?style=flat-square)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/badge/release-v0.1.0-56803b?style=flat-square)](https://github.com/SuperMarioYL/chatport/releases)
[![Local](https://img.shields.io/badge/local--only-no_telemetry-383c1d?style=flat-square)](#why-switching-hurts)

[Why](#why-switching-hurts) · [How it works](#how-it-works) · [Quickstart](#quickstart) · [Commands](#commands) · [Demo](#demo) · [Pricing](#pricing) · [Roadmap](#roadmap) · [Development](#development)

</div>

> **What v0.1.0 actually implements**: parsing of the official ChatGPT export zip, the Context Pack intermediate representation, terminal statistics, and a local placeholder preview page. The drag-and-drop preview and pack download land in v0.2, domestic-app adapters in v0.3 — see the [roadmap](#roadmap). This document only advertises what is already implemented.

## Why switching hurts

The real workflow of multi-assistant power users: grind a project in ChatGPT, take it to Claude to write it up, then open a Coding Agent to build it. Every switch means copying background and explaining decisions again, because:

- **Official exports are regulatory data-ownership artifacts, not interoperability formats.** A ChatGPT export easily runs to hundreds of megabytes of JSON laced with system fields — no project-knowledge upload entry can digest it.
- **No assistant ships an ingest primitive for another service's history.** Vendors are happy to satisfy data portability on the way out; they have no incentive to lower switching costs on the way in. The seam stays open.
- **Copy-paste is the worst channel.** Memory, project structure, and prior decisions are lost on the way across; the more you accumulated, the more the move costs you.

ChatPort builds the missing half: **ingest**. It reads the full official export zip, normalizes it into one Context Pack, prints statistics, and — from v0.2 — renders Markdown bundles that the target app's existing upload entry can actually accept. Fully local, single binary, zero model calls.

## How it works

One Go binary, three internal pipelines, no database, no server, no network:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/architecture-dark.svg" />
  <source media="(prefers-color-scheme: light)" srcset="assets/architecture-light.svg" />
  <img src="assets/architecture-light-static.svg" alt="Architecture: ingest (chatgpt.go wired, cn_export.go reserved) writes exports into the Context Pack IR (meta/threads/digest); render and web read the IR in later milestones" width="100%" />
</picture>

| Module | Job | Status |
| --- | --- | --- |
| `internal/ingest` | zip/json parsing into the Context Pack; `chatgpt.go` is wired, `cn_export.go` is reserved for domestic adapters | parsing implemented |
| `internal/ir` | Context Pack types: meta / threads / digest (dedup fingerprints, project groups, per-file size budget) | implemented |
| `internal/render` | target renderers: Claude Projects shard pack (v0.2), generic knowledge bundle (v0.3) | contract defined, see roadmap |
| `internal/web` | localhost preview page (embedded static assets, stdlib http) | placeholder page in v0.1 |

Every export parser writes into the same IR; every renderer reads from it. A domestic-app adapter is just one more ingest implementation — no new processes.

## Quickstart

Go 1.24+ required, build from source (or `go install`):

```bash
git clone https://github.com/SuperMarioYL/chatport
cd chatport
make build          # produces the single-file binary ./chatport
```

**First run** — the repo ships a synthetic sample export (format demo only, not real data):

```bash
./chatport parse examples/sample-chatgpt-export.zip --stats
```

**Move your own history**:

1. ChatGPT → Settings → Data controls → Export data; wait for the export email (the only slow step, and it happens outside the flow).
2. Download the export zip from the email.
3. Run:

```bash
./chatport parse your-export.zip --stats
```

Every user step stays under a minute. From v0.2, the `chatport open` local page takes over the full drag → select → download flow.

## Commands

| Command | What it does |
| --- | --- |
| `chatport parse <export.zip>` | Parse the export and print a summary (source, conversations, messages, time span, project grouping) |
| `chatport parse <export.zip> --stats` | Add a per-conversation breakdown: message counts, duplicates, project, time range |
| `chatport parse <export.zip> --json` | Emit the full Context Pack IR (JSON) |
| `chatport open` | Start the localhost preview page and open the browser (placeholder in v0.1; interactive preview in v0.2) |
| `chatport --version` | Version |

`--json` exposes exactly the representation the v0.2 renderer will consume:

```json
{
  "meta":   { "source_app": "chatgpt", "n_conversations": 4, "n_messages": 19 },
  "threads": [ { "id": "…", "title": "【项目】博客重构", "project_hint": "项目", "messages": [ … ] } ],
  "digest": { "dedup_index": {…}, "dup_messages": 1, "size_budget_per_file": 102400, "per_project_groups": {…} }
}
```

The design tradeoffs are stated plainly: **v0.1 conversion is deterministic rules, zero model calls.** Dedup matches only messages that are byte-identical after whitespace normalization (sha256 fingerprints); project grouping only reads title prefixes (`【…】`, `[…]`, `Project:`, `项目：`) — explainable and reproducible, which also means not clever; the rules evolve with v0.2's rendering needs. ChatGPT exports do not include memory or custom instructions, so the pack does not either; edited-away reply branches (anything off the current_node path) are not kept.

## Demo

<p align="center">
  <img src="assets/demo.gif" alt="Terminal recording: chatport --version, chatport parse on the sample export printing statistics, then --stats with the per-conversation breakdown" width="100%" />
</p>

A real terminal recording (script at [`assets/demo.tape`](assets/demo.tape), reproducible via `make demo`). The full reproduction record — input, commands, raw output, version, limitations — lives in [`docs/demo-results.json`](docs/demo-results.json). The sample zip in the recording is synthetic ([`examples/`](examples/README.md)); your own export zip produces the same statistics.

The graphical demo — drop the zip, select conversations, download `claude-projects-pack.zip` — will be recorded with v0.2.

## Where it sits

A division of responsibility rather than a feature contest:

| Stage | Who | Output |
| --- | --- | --- |
| Export | Each assistant's official export (or a browser export extension) | Raw archive: built for taking data *out*, designed as a data-ownership artifact |
| Convert | **ChatPort (this project)** | Context Pack: deduplicated, grouped by project, sharded to a budget — built to be *ingested* |
| Upload | The target app's existing project-knowledge / file-upload entry | Context in place |

Nobody is seriously doing the conversion stage today: it is unglamorous maintenance racing private export formats that churn without notice. v0.1 wires exactly one pair (ChatGPT); domestic adapters unlock pair-by-pair behind a Gate-0 audit (see the [roadmap](#roadmap)).

## Pricing

No subscription, no account, no uploads — a one-time buyout, matching a one-time move:

| | Free tier | Buyout ¥68 (early bird ¥39 / international $19) |
| --- | --- | --- |
| Conversion pairs | ChatGPT → Claude (one pair) | Every wired source and target |
| History range | Last 30 days of conversations | Full history |
| Context packs | Per-conversation packs | Project-grouped packs + full pack |
| Activation | — | `chatport activate <code>` — offline ed25519 verification, entirely on your machine, no network requests |

Purchase channels: Afdian / Mianbaoduo auto-delivery codes in China, Stripe Payment Link internationally. The links go live with the v0.2 / v0.3 releases (see the [roadmap](#roadmap)); this section updates first. v0.1 is in beta and its parsing and statistics are free to use; real move reports in the Issues tracker are very welcome.

## Roadmap

| Version | Scope | Status |
| --- | --- | --- |
| v0.1 (m1) | ChatGPT export parsing, Context Pack IR, terminal statistics, local placeholder preview page | **done** |
| v0.2 (m2) | Local preview page: drop zip → parse progress → select conversations → one-click download of the project-grouped, deduplicated, size-budgeted Claude Projects-friendly Markdown pack | in development |
| v0.3 (m3) | One Gate-0-audited domestic-app pair, generic knowledge-base renderer, macOS / Windows / Linux binaries, offline license verification | planned |

Two prerequisites, stated so they can be checked: a domestic adapter ships only if the Gate-0 audit passes — the source app must expose a parseable structured export and the target app an official upload / project-knowledge entry; and if any assistant vendor ships an official import, this tool's job is done and the project stops.

## Development

```bash
make build     # build (version stamped from VERSION)
make test      # go test ./...
make vet       # go vet ./...
make example   # rebuild the sample zip from examples/sample-export
make demo      # re-render assets/demo.gif with vhs
```

```
cmd/chatport/       CLI entry (cobra)
internal/ingest/    export parsers: chatgpt.go (wired), cn_export.go (v0.3 reserved)
internal/ir/        Context Pack: types, dedup fingerprints, project groups, stats
internal/render/    renderer contracts: claude.go (v0.2), generic.go (v0.3)
internal/web/       local preview: embedded static page + stdlib http
```

The only third-party dependency is cobra; zip, HTTP and static assets all come from the standard library. Golden-sample tests cover the parser's load-bearing behavior: current_node branch linearization, hidden system messages skipped, code parts preserved, cross-conversation dedup counting.

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
