# AGENTS.md

Guidance for humans and agents working on Steno. Read `docs/spec.md` and `.pairing/PLAN.md` before
making changes.

## Collaboration

- Work in small, reviewable slices. Future sessions are pair-programming by default.
- Keep the product spec as the behavioral source of truth. Behavior changes update `docs/spec.md`.
- Keep `.pairing/PLAN.md` aligned with the current phase and next pairing slice.
- Prefer boring Go, standard library first, and one clear responsibility per package.
- Do not build broad product features without approval. Scaffold and implementation should stay easy
  to review.

## Architecture

- `cmd/steno` owns process startup only.
- `internal/cli` owns command parsing and command dispatch.
- `internal/config` owns the TOML-shaped configuration model and defaults.
- `internal/recorder` owns the protocol-agnostic recorder seam.
- `internal/timeline` owns normalized event types and JSONL I/O.
- `internal/inspect` reads recordings for CLI inspection.
- `internal/render` derives human artifacts from the canonical timeline.

CDP details must not leak past the recorder implementation into timeline, inspect, or render code.

## Testing And Verification

- Write tests before behavior code where practical.
- Every behavior slice ships with the smallest useful tests.
- Run checks through `just`: `just fmt`, `just test`, `just build`.
- Do not claim anything passes without running the command and reading the output.
- Agents never run browser smoke tests or launch browser/external GUI programs themselves. Ask the
  maintainer to run those checks and report the output. The boundary is executing the external
  browser program; unit tests, build, lint, and pure CLI commands stay agent-run.

## Documentation

- Multi-step procedures belong in `docs/` or `docs/runbooks/`, not only chat.
- Use Mermaid for diagrams.
- Do not put phase-status copy in the README; use `.pairing/PLAN.md`.
