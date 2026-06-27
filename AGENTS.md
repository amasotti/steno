# AGENTS.md

Guidance for humans and AI agents working on Steno. Steno is a shipped tool, not a scaffold — this
file is your companion for changing it safely.

**Read first:** [`docs/spec.md`](docs/spec.md) (what the tool does and refuses to do) and
[`docs/architecture.md`](docs/architecture.md) (how it is built and why). Track in-flight slices in
[`.pairing/PLAN.md`](.pairing/PLAN.md).

## What Steno is (in one paragraph)

A single Go binary that launches a headful Chromium browser, watches it over CDP while a person tests
by hand, and writes a signal-only timeline (`timeline.jsonl` + payload side-files) that a human or an
LLM agent can read. It records; it never analyses. See the spec for the full contract.

## How to work here

- **Small, reviewable slices.** Every change is reviewed by the maintainer before the next one.
  Default to pair-programming: propose the slice, get a nod, then implement. Do not build broad product
  features without approval.
- **The spec is the behavioral source of truth.** Any change to behavior updates `docs/spec.md` in the
  same change. Structural changes update `docs/architecture.md`. Keep `.pairing/PLAN.md` aligned with
  the current slice.
- **Boring Go, standard library first.** One clear responsibility per package. Reach for a dependency
  only when the stdlib genuinely falls short.
- **Tests before behavior where practical.** Every behavior slice ships with the smallest useful test.

## Invariants — do not break these

These are the load-bearing constraints. Violating one is a bug even if it compiles and tests pass.

1. **No CDP type leaks past `internal/recorder/cdp`.** Everything downstream speaks `timeline.Event`
   and plain strings. This seam is what keeps `filter`, `inspect`, `render`, and `replay` testable
   without a browser, and is where a future non-Chromium backend would slot in. If you find yourself
   importing `chromedp`/`cdproto` outside that package, stop and rethink.
2. **No redaction, masking, or truncation.** Full fidelity (including test credentials, tokens,
   cookies, complete payloads) is a deliberate, permanent design choice. It is not a missing feature;
3. **The `timeline.jsonl` schema is additive.** New optional fields are fine; never repurpose or remove
   an existing field's meaning. Reserved features keep their fields declared rather than reshaping the
   record later.
4. **The record path stays LLM-free and self-sufficient.** No network calls home, no analysis in the
   binary. The debug port binds to localhost.
5. **One responsibility per package**, dependencies pointing inward to `timeline`. If a new concern
   doesn't fit an existing package, add one rather than overloading another.

## What an agent may and may not run

**May run freely** (these are the verification surface):

- Unit tests, build, lint, vuln: `just fmt`, `just test`, `just build`, `just lint`, `just vuln`, or
  `just ci-local` for everything.
- Pure CLI commands against an *existing* recording: `steno inspect`, `steno render`, `steno replay`.

**Must NOT run — ask the maintainer instead:**

- Anything that launches a browser or an external GUI program — including `steno record`, which starts
  Chromium. The boundary is executing the external browser. Write the code and the unit tests, then ask
  the maintainer to run the browser smoke test and report the output.

**Never claim something passes without running the command and reading its output.** "Tests pass" means
you ran `just test` and saw it succeed.

## Conventions

- **Commits drive releases.** `release.yml` computes the next semver from
  [Conventional Commits](https://www.conventionalcommits.org/) on `main`: `feat`/`feature`/`security`
  bump the minor; `fix`/`bugfix`/`perf`/`refactor`/`test`/`chore`/`ci`/`build` bump the patch. Write
  commit subjects accordingly.
- **Formatting and linting are enforced.** `gofmt` via `just fmt`; `golangci-lint` via `just lint`
  (config in `.golangci.yml`). CI runs lint, test (with `-race`), build, and `govulncheck`.
- **Multi-step procedures belong in `docs/` or `docs/runbooks/`,** not only in a chat reply. Use
  Mermaid for diagrams.
- **Keep phase/status copy out of the README.** It lives in `.pairing/PLAN.md`.

## Where things live

| Area | Package |
| --- | --- |
| Startup | `cmd/steno` |
| Command parsing/dispatch | `internal/cli` |
| Config model + defaults | `internal/config` |
| Recorder seam | `internal/recorder` |
| CDP capture (browser-touching) | `internal/recorder/cdp` |
| Signal filtering | `internal/filter` |
| Event types + JSONL I/O | `internal/timeline` |
| Payload side-file store | `internal/payload` |
| CLI inspection | `internal/inspect` |
| Mermaid rendering | `internal/render` |
| curl replay | `internal/replay` |
| Session summary | `internal/summary` |

Reserved-but-unwired surfaces (attach mode, `[correlation]`, `[[extract]]`, `[[supplemental]]`, and the
`console`/`fact`/`supplemental` event kinds) are documented in [`docs/spec.md`](docs/spec.md#9-reserved-and-out-of-scope).
If you implement one, wire it behind its existing config/struct fields and update the spec.
