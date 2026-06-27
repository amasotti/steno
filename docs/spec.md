# Steno — product spec

> A headful Chromium session recorder & inspector. It turns a manual test flow into a clean, signal-only, dual-audience timeline. Records everything
> that matters, judges nothing.

## 1. One-line summary

`steno` attaches to (or launches) a Chromium browser you click through by hand, records the meaningful network calls and the navigation journey across
tabs and redirects, keeps just enough detail (origin, path, method, request/response payloads, basic stats), and writes a durable timeline you can
inspect yourself or hand to an LLM agent.

## 2. Problem & motivation

When testing a web application manually — especially a flow that spans several microservices, redirects between origins, and occasionally opens new
tabs — there is no good record of *what actually happened*. The browser DevTools network tab is per-tab, ephemeral, and drowned in static assets (JS
bundles, CSS, fonts, images, analytics, telemetry). After a manual test you are left with no reproducible audit of which systems were touched, in what
order, what they returned, and where (if anywhere) something failed.

The gap is specifically in the **browser plane**. Backend observability (logs, traces, message queues, databases) usually already exists or can be
queried after the fact. What is missing is a faithful, low-noise record of the front-of-house journey, captured live while a person tests, in a form
that both a human and an agent can read. Steno fills exactly that gap and nothing more.

## 3. Who it is for

Developers and QA engineers who test web applications manually and want an audit trail of the session. Output is designed for two consumers at once: a
human skimming a timeline / sequence diagram, and an LLM agent (Claude Code, Codex, etc.) ingesting the record to write a report, reconstruct the
flow, or debug a failure. The tool is general-purpose — any web app, any flow — not tied to one application.

## 4. Goals

1. **Record the session.** While the user tests manually, capture each meaningful step with a timestamp, the request payload, the response payload,
   and minimal per-call metrics (duration, size). Follow the user across same-tab navigations, cross-origin redirects, and additional tabs.
2. **Produce a dual-audience timeline.** Emit one artifact that is both machine-ingestable and human-viewable, describing what was done on which
   systems. It serves as (a) an audit of the test — the ordered timeline, plus named facts extracted from responses such as the UUID assigned to a
   newly created user — and (b) the raw material from which a sequence diagram and a user-journey view can be drawn.
3. **Localise failures.** When something goes wrong, the record must make it obvious *where* the non-2xx response occurred — which origin, which call,
   with the request and response payloads attached — without needing any other tool.

## 5. Non-goals (YAGNI boundary)

- **Chromium family only.** Steno talks CDP, which covers Chrome, Edge, Brave, Opera and any other Chromium browser. Non-Chromium engines (Firefox via
  WebDriver BiDi, Safari via WebKit) are explicitly out of scope for v1. See §13 for the seam that keeps that door open cheaply.
- **Not a deep network profiler.** It captures origin, path, method, status, payloads and basic stats — not full headers, timing waterfalls, or
  HAR-level detail. "Enough to understand and audit the flow", not "everything DevTools knows".
- **No literal user-input capture.** It does not inject scripts to record individual clicks or keystrokes. CDP can only *dispatch* synthetic input,
  never observe real input, so capturing literal clicks would require JS injection — deliberately avoided. The journey is reconstructed from
  navigation and network events instead, which is sufficient for the audit, the sequence diagram, and failure localisation.
- **Not an automation or replay tool.** It does not drive the browser, generate Playwright/Puppeteer scripts, or replay sessions. The human drives;
  Steno only observes. If you need a script, use Playwright — that is its job, not this one's.
- **Not a session-replay / DOM-pixel recorder.** It is not rrweb. It records a structured event timeline, not a frame-by-frame DOM reconstruction.
- **No analysis / no LLM in the tool.** The recorder contains no LLM and makes no judgements. Analysis (reports, root-causing) happens downstream in
  an agent. Steno is a stenographer, not an investigator.

## 6. Core concept — two verbs, then a downstream phase

Steno owns two local, deterministic verbs and stays out of the third:

- **Record** (live, no LLM) — runs while the user tests, captures the browser plane, filters and extracts, writes the timeline.
- **Inspect** (offline, no LLM) — examine and render a recorded session: list the calls, view a payload, filter by origin or status, generate the
  report and sequence diagram. Fully re-runnable against an existing recording.
- **Analyse** (downstream, out of scope) — an agent reads the artifacts to write a report or, when chasing a failure, pivots to backend planes (
  Datadog, Kafka, service/CRM APIs) using the join keys the timeline already holds.

The record path must never depend on an agent, and the artifacts must be self-sufficient for the common cases (audit, sequence diagram, failure
localisation) without any backend round-trip.

## 7. Functional requirements

### 7.1 Attach or launch (both first-class)

- **Attach** to an already-running Chromium debug instance via a browser URL (`http://localhost:9222`).
- **Launch** a managed Chromium instance with a dedicated `--user-data-dir`. Because of the Chrome 136+ security change, `--remote-debugging-port` is
  not honoured on the default profile, so a dedicated profile is mandatory for the launch path.
- A `--browser` flag selects which Chromium binary to launch/expect (chrome, edge, brave, …) — same protocol, different executable.
- Headful only. The user must see and drive the browser.

### 7.2 Follow the whole journey

- Auto-attach to every target (tab, popup, cross-origin navigation) via `Target.setAutoAttach`, so a multi-tab or redirect-heavy flow is captured as
  one continuous, ordered session — not one log per tab.
- The journey (which origins were visited, in what order, including redirects) is derived from CDP `Page`/`Target` navigation events. No injected
  listeners.

### 7.3 Capture — lean by design

For each kept network call, one record carrying: **origin** (scheme + host), **path**, **method**, **status**, **request payload**, **response payload
**, start/end timestamps, **duration**, and **transferred size**. Headers are not captured by default; only explicitly configured headers (e.g. a
correlation header) are kept. Optionally, with `[payload] include_headers = true`, the full raw request and response headers (including `Cookie` /
`Set-Cookie`) are written as JSON side-files and referenced via `request_headers_ref` / `response_headers_ref` — off by default to keep recordings lean. Console
capture: `error` and `warn` by default, plus messages under a configurable application log namespace, and uncaught exceptions.

### 7.4 Filter to signal (config-driven, applied in order)

- **Resource-type cut** — keep `Document`, `XHR`, `Fetch`, `WebSocket`; drop `Script`, `Stylesheet`, `Image`, `Font`, `Media`, `Manifest`. The single
  largest noise reduction.
- **Host allowlist** — keep only first-party / system-of-interest origins; drop third-party analytics, CDNs, telemetry.
- **Path / method denylist** — drop `OPTIONS` preflights (or collapse into the matching call), health checks, favicon, telemetry endpoints.

### 7.5 Persist in full

- No masking, redaction, or truncation. Steno records manual test sessions on test data; full fidelity —
  including test credentials, tokens, cookies, and complete payloads — is the point, so the user can read
  and reproduce exactly what happened. Payloads live in side-files, so size is not a reason to drop data.

### 7.6 Extract named facts (config-driven)

- A rule maps an endpoint + response JSON path to a named fact, e.g. `confirm.response → $.user.id` becomes `user_uuid`. Facts are first-class
  timeline entries: both the audit details ("user `a3f…` created") and the join keys for the downstream analyse phase.

### 7.7 Capture correlation keys

- For each kept call, capture `traceparent` (W3C) and/or a configurable correlation header (`X-Request-ID`), so the browser timeline shares join keys
  with backend planes.

### 7.8 Supplemental HTTP sources (general; OTP is the canonical example)

- Optionally poll an external HTTP source during the run and fold its data into the timeline as synthetic events. Motivating case: a flow needing an
  emailed OTP caught by a local mail sink (e.g. Mailpit) with an HTTP API — Steno fetches the code and records "OTP received → entered", completing
  the cross-service narrative the network tab never shows. Designed generically for any pollable HTTP source.

### 7.9 Inspect a recording

- Examine a recorded session without rendering artifacts: list calls, show one call's request/response payloads **and headers** by sequence id
  (`--seq`), filter by origin (`--origin`) or status (`--status 4xx,5xx`, e.g. only non-2xx). This is the "inspect" half of the tool — turning the
  durable timeline into something you can interrogate from the CLI.

### 7.10 Emit artifacts

All derived from one canonical timeline:

- `timeline.jsonl` — the canonical, ordered, machine-ingestable record (also the most token-efficient form for an agent).
- `report.html` — a human-readable rendering of the timeline.
- `flow.mmd` — a Mermaid sequence diagram of the cross-system journey.

## 8. The event model

One ordered stream of normalized events, ordered by capture sequence plus a wall-clock timestamp. Event kinds:

- `navigation` — a page navigation / redirect (anchors the journey and the sequence diagram).
- `call` — one HTTP exchange (request + response folded into a single record).
- `console` — an error, warning, namespaced log, or uncaught exception.
- `fact` — an extracted named value (`user_uuid`, `account_id`).
- `supplemental` — a folded-in external event (e.g. the OTP).

A `call` looks roughly like:

```json
{
  "seq": 42,
  "ts": "2026-06-27T14:02:40.118Z",
  "kind": "call",
  "origin": "https://account.int.example",
  "method": "POST",
  "path": "/api/v1/profile",
  "status": 201,
  "duration_ms": 142,
  "size_bytes": 3380,
  "correlation": "00-4bf9...-01",
  "request_ref": "payloads/0042-req.json",
  "response_ref": "payloads/0042-res.json",
  "request_headers_ref": "payloads/0042-req-headers.json",
  "response_headers_ref": "payloads/0042-res-headers.json"
}
```

Request and response payloads are written in full to side-files and referenced (`request_ref` / `response_ref`) — never truncated. When
`include_headers` is enabled, raw header maps are written alongside as `*-req-headers.json` / `*-res-headers.json` and referenced via
`request_headers_ref` / `response_headers_ref`. The timeline itself stays compact because the bodies live beside it.

## 9. Outputs in detail

- **`timeline.jsonl`** — one event per line, the schema above. The contract every other output is generated from; stable enough that an agent can rely
  on it.
- **`report.html`** — a single self-contained file rendering the timeline chronologically, grouped by origin, with non-2xx calls highlighted and
  extracted facts surfaced at the top. No server, no build step.
- **`flow.mmd`** — generated mechanically from the timeline; each cross-origin call becomes one line, statuses annotated, the user as actor. Example
  shape:

```
sequenceDiagram
  You->>Registration: POST /verify-code (200)
  Mailpit-->>You: OTP received
  You->>Registration: POST /confirm (200, user=a3f…)
  You->>Account: POST /profile (201, account=acc_88…)
  You->>Product: GET /dashboard (200)
```

## 10. Configuration

A single TOML file declares the target, the filters, the extraction rules, the correlation header, and any supplemental sources. Sketch:

```toml
[target]
mode = "launch"            # or "attach"
browser = "chrome"         # chrome | edge | brave | ...
attach_url = "http://localhost:9222"   # used when mode = "attach"
profile_dir = "~/.steno/profile"       # used when mode = "launch"
start_url = "https://app.example.test"

[filter]
resource_types = ["Document", "XHR", "Fetch", "WebSocket"]
allow_hosts = ["*.int.example.test", "auth.example.test"]
deny_paths = ["/health", "/favicon.ico"]
drop_preflight = true

[payload]
include_documents = false  # capture document (HTML) bodies as side-files
include_headers = false    # capture raw request/response headers + cookies as side-files

[[extract]]
when_path = "/confirm"
from = "response"
json_path = "$.user.id"
as = "user_uuid"

[correlation]
header = "traceparent"

[[supplemental]]
name = "mailpit-otp"
url = "http://localhost:8025/api/v1/messages"
extract_otp = "\\b\\d{6}\\b"
```

## 11. CLI

- `steno record --config flow.toml --out ./runs/<timestamp>/` — attach or launch (per config/flags), record until interrupted, write
  `timeline.jsonl` + payload side-files. Flags: `--browser`, `--attach <url>`, `--launch`.
- `steno inspect ./runs/<timestamp>/` — examine a recording: list calls, show a call's payloads + headers by `--seq`, filter by origin or status (
  `--status 4xx,5xx`, `--origin account.int.example`).
- `steno render ./runs/<timestamp>/` — (re)generate `report.html` and `flow.mmd` from an existing timeline. Pure, offline, re-runnable.

Recording and rendering are separate so rendering can be iterated without re-recording.

## 12. Stack & distribution

- **Language: Go.** Chosen for three tool-specific reasons: a mature CDP client ecosystem (`chromedp` / `go-rod`); a concurrency model where merging
  several concurrent tab event-streams into one ordered timeline is idiomatic (one channel, one draining goroutine); and single static binaries with
  trivial cross-compilation. Covers the whole Chromium family via CDP.
- **Key libraries:** `chromedp` or `go-rod` (CDP); `tidwall/gjson` (extract paths from arbitrary response JSON without predefining structs); stdlib
  `encoding/json` (JSONL), `html/template` (report), `text/template`/`fmt` (Mermaid).
- **Tooling:** `gofmt` + `staticcheck` from day one.
- **Distribution (the headline requirement — colleagues just run it):** GoReleaser publishes cross-platform binaries (macOS/Linux/Windows,
  arm64/amd64) to GitHub Releases — download one file, run it, no runtime installed. Also `go run github.com/<user>/steno@latest` and `go install`. (
  `npx`-style invocation is Node-only and explicitly not a target; the single binary is the distribution story.)

## 13. Constraints & considerations

- **Chrome 136+** ignores remote-debugging flags on the default profile; a dedicated `--user-data-dir` is required for the launch path, and an
  attached instance must have been started with the debug port + a non-default profile. This is also correct test hygiene — no personal cookies in the
  trace.
- **Protocol-agnostic recorder seam (cheap insurance).** The capture core sits behind a `Recorder` interface that emits *normalized* events; no CDP
  type leaks past it into the filter/extract/timeline/inspect/render layers. CDP is one implementation. This costs almost nothing now and is the exact
  seam a future WebDriver BiDi backend (Firefox) would slot into — without committing to building it.
- **Local only.** The debug port binds to localhost; nothing is sent anywhere. The tool never phones home.
- **Full fidelity.** Nothing is masked or redacted; test credentials, tokens, and cookies are persisted verbatim so the session is faithfully reproducible. The trace is a local artifact the user owns.
- **Cross-platform.** Must run on macOS, Linux, Windows.

## 14. MVP — the quick first result (validate the idea fast)

The first build proves the core idea with the least code. Walking skeleton:

- Launch *or* attach a Chromium instance over CDP.
- Single tab only (no auto-attach yet).
- Capture `call` events (origin, path, method, status, payloads, duration, size) with the resource-type cut and host allowlist applied.
- Write `timeline.jsonl` with full payloads in side-files.
- A minimal `steno inspect` that prints the timeline.
- A trivial `flow.mmd`.

**MVP done looks like:** run `steno record` against a real multi-step flow in one tab, click through it, stop, and get a short `timeline.jsonl` of
only the meaningful calls — each with its payloads and stats — that you can `steno inspect`, plus a readable sequence diagram. If that already feels
useful, the idea is validated and the rest is worth building.

## 15. Phased roadmap (for the planning agent to expand)

- **Phase 0 — MVP:** as in §14.
- **Phase 1 — full capture:** auto-attach across tabs/redirects; `navigation` events for the journey; console; path/method denylist; payload
  side-files (full payloads); full `inspect` (filter by origin/status, show payload by seq). Establish the `Recorder` interface seam here.
- **Phase 2 — audit value:** config-driven fact extraction; correlation-key capture.
- **Phase 3 — outputs & supplemental:** polished `report.html`; richer `flow.mmd`; supplemental HTTP sources (OTP/Mailpit).
- **Phase 4 — later / maybe:** expose Steno as an MCP server so an agent reads the live session; a failure-pivot helper that fans out to backend
  planes by join key; (only if ever needed) a BiDi backend behind the existing seam.

## 16. Acceptance criteria (how to judge whether the idea is good)

Run a real, multi-step, multi-service flow and confirm:

- The timeline is short and signal-only (tens of calls, not thousands of asset requests).
- The cross-system journey is reconstructable from the timeline alone.
- The generated sequence diagram is readable and matches what happened.
- Each call carries enough to be useful: origin, path, method, status, both payloads, duration, size.
- Named facts (e.g. the new user's UUID) are present and correct.
- An injected non-2xx is clearly localised to an origin and call, with payloads attached, and findable via `steno inspect --status 4xx,5xx`.

## 17. Decisions (resolved)

- **Name:** Steno.
- **Protocol / browsers:** CDP, Chromium family only (Chrome, Edge, Brave, Opera, …). Firefox/Safari out of scope for v1.
- **Connection:** support both attach-to-running and launch-managed.
- **User actions:** no JS injection, no literal click/keystroke capture; journey reconstructed from navigation + network.
- **Config format:** TOML.
- **Detail level:** origin, path, method, status, payloads, basic stats — no full headers / waterfalls.

## 18. Open questions (for Tony / the planning agent)

- **Payload capture policy:** content-type allowlist (capture JSON/form, skip binary?). No size cap or truncation — payloads persist in full.
- **WebSocket handling:** connection event only, or capture frames? (Lean toward connection-only for v1, frames deferred.)
- **`inspect` ergonomics:** plain text output, or a TUI later? (Plain text for v1.)
