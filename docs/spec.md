# Steno — Behavioral Specification

This is the behavioral source of truth for Steno: what it does, what it deliberately does not do, the
event model it produces, and the CLI surface. It describes the **shipped** tool. Anything reserved for
future work is called out explicitly in [§9](#9-reserved-and-out-of-scope).

For the *why* behind the structure — package boundaries, contracts, design choices — see
[`architecture.md`](./architecture.md). For day-to-day usage, see the [README](../README.md).

## 1. Summary

`steno` launches a Chromium-family browser that a person clicks through by hand, records the
meaningful network calls and the navigation journey, keeps just enough detail (origin, path, method,
status, full request/response payloads, basic stats), and writes a durable, ordered timeline that the
same person can inspect from the CLI or hand to an LLM agent.

It owns two local, deterministic verbs and stays out of a third:

- **Record** (live, no LLM) — runs while the user tests, captures the browser plane, filters to
  signal, and writes the timeline plus payload side-files.
- **Inspect / render / replay** (offline, no LLM) — interrogate a recording, derive a sequence
  diagram, or reproduce a call as `curl`. All re-runnable against an existing recording.
- **Analyse** (downstream, out of scope) — an agent reads the artifacts to write a report or chase a
  failure into backend planes. Steno produces the raw material; it performs no analysis.

The record path never depends on an agent, and the artifacts are self-sufficient for the common cases
(audit, sequence diagram, failure localisation) without any backend round-trip.

## 2. Problem and motivation

Manual testing of a multi-service web flow leaves no faithful record of the front-of-house journey.
DevTools' network tab is per-tab, ephemeral, and buried under static assets. Backend observability
(logs, traces, queues) usually already exists; the gap is specifically the **browser plane** — a
low-noise, durable record of which systems were touched, in what order, what they returned, and where
something failed, captured live while a person tests, in a form both a human and an agent can read.

## 3. Audience

Developers and QA engineers who test web applications by hand and want an audit trail. Output targets
two consumers at once: a human skimming a timeline or sequence diagram, and an LLM agent ingesting the
record. The tool is general-purpose — any web app, any flow.

## 4. Goals

1. **Record the session.** Capture each meaningful call with a timestamp, request payload, response
   payload, and minimal metrics (duration, size). Follow same-tab navigations and cross-origin
   redirects.
2. **Produce a readable timeline.** Emit one canonical artifact that is both machine-ingestable
   (`timeline.jsonl`) and the source for a human-viewable sequence diagram (`flow.mmd`).
3. **Localise failures.** Make it obvious *where* a non-2xx response occurred — which origin, which
   call — with both payloads attached, without needing another tool.

## 5. Functional behavior (shipped)

### 5.1 Launch and capture

- **Launch** a managed Chromium instance over CDP and open `start_url` (or `--url`). Headful only —
  the user must see and drive the browser.
- A `browser` setting selects which Chromium binary to launch (`chrome`, `edge`, `brave`, …); same
  protocol, different executable.
- Capture is **single-tab**: the launched tab's `Network` and `Page` domains are observed. Recording
  runs until the user interrupts it (`Ctrl-C`).

### 5.2 Capture — lean by design

For each kept network call, one `call` event carries: **origin** (scheme + host), **path** (with
query), **method**, **status**, start/end-derived **duration**, transferred **size**, and references
to full **request** and **response** payloads. Document (HTML) bodies are skipped by default. Raw
request/response headers — including `Cookie` / `Set-Cookie` — are captured as side-files only when
`include_headers` is enabled.

### 5.3 Follow the journey

Top-frame navigations and redirects are captured as `navigation` events from CDP `Page` events
(sub-frames dropped; `about:blank` and other scheme-less URLs skipped). Navigation and `call` events
share one sequence counter, so the timeline preserves real order including redirect chains. No JS is
injected.

### 5.4 Filter to signal (config-driven, applied in order)

A CDP-free filter decides what to keep; the first failing rule drops the event:

1. **Resource-type allowlist** — keep `Document`, `XHR`, `Fetch`, `WebSocket`; drop
   `Script`, `Stylesheet`, `Image`, `Font`, `Media`, `Manifest`. The single largest noise cut.
2. **Host allowlist** — keep only configured origins; an empty list allows all. Supports exact match
   and a leading-`*.` wildcard suffix match (`*.example.com` matches `api.example.com` but **not** the
   bare `example.com`).
3. **Preflight drop** — drop `OPTIONS` requests when `drop_preflight` is true.
4. **Path denylist** — prefix match; drop paths that start with any denied prefix.

### 5.5 Persist in full

No masking, redaction, or truncation. Steno records manual test sessions on test data; full fidelity —
including test credentials, tokens, cookies, and complete payloads — is the point, so the user can
read and reproduce exactly what happened. Payloads live in side-files, so size is never a reason to
drop data.

### 5.6 Inspect a recording

`steno inspect` reads a run without rendering artifacts:

- **List view** — one line per kept event: `call` lines show seq, status, method, origin, path;
  `navigation` lines show seq and the navigated origin/path.
- **Filters** — `--origin` (case-insensitive substring) and `--status` (comma-separated codes or
  classes such as `404` or `4xx,5xx`), combined with AND.
- **Detail view** — `--seq <id>` shows one event in full: method, status, origin, path, duration,
  size, and the request/response headers and bodies read from the payload side-files.

### 5.7 Render artifacts

`steno render` derives `flow.mmd` — a Mermaid sequence diagram — from the canonical timeline.
Participants are the unique call origins; each `call` becomes one interaction line
(`User->>Host: METHOD /path (STATUS)`) and each `navigation` becomes a `Note over User`. Rendering is
pure, offline, and separate from recording so it can be re-run without re-recording.

### 5.8 Replay a call

`steno replay <run> <seq>` prints a copy-paste-ready `curl` reproducing one recorded `call`: URL from
`origin + path`, captured headers as `-H` lines (HTTP/2 pseudo-headers and `content-length` dropped),
and the request body as `--data-raw`. When no usable headers were captured, it still prints the
command but warns on stderr to re-record with `include_headers = true`.

## 6. Event model

One ordered stream of normalized events, ordered by capture sequence plus a wall-clock timestamp. Two
event kinds are emitted today:

- `navigation` — a top-frame page navigation or redirect; anchors the journey and the diagram.
- `call` — one HTTP exchange (request and response folded into a single record).

The canonical record is `timeline.jsonl`, one JSON object per line. A `call` looks like:

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
  "request_ref": "payloads/0042-req.txt",
  "response_ref": "payloads/0042-res.txt",
  "request_headers_ref": "payloads/0042-req-headers.json",
  "response_headers_ref": "payloads/0042-res-headers.json"
}
```

Request and response bodies are written in full to side-files and referenced via `request_ref` /
`response_ref` — never truncated. When `include_headers` is enabled, raw header maps are written
alongside as `*-req-headers.json` / `*-res-headers.json` and referenced via `request_headers_ref` /
`response_headers_ref`. The timeline stays compact because the bodies live beside it.

The full field list and the contract guarantees are in [`architecture.md`](./architecture.md#the-timeline-contract).

## 7. Outputs

All derived from one canonical timeline:

- **`timeline.jsonl`** — the canonical, ordered, machine-ingestable record. The contract every other
  output is generated from, and the most token-efficient form for an agent.
- **`payloads/`** — full request/response bodies (and headers, when enabled) as side-files.
- **`flow.mmd`** — a Mermaid sequence diagram of the cross-system journey, generated mechanically.

## 8. CLI

| Command | Behavior |
| --- | --- |
| `steno record --out <dir> [--url <u>] [--config <f>]` | Launch, capture until interrupted, write the timeline + payloads, print a summary. |
| `steno inspect <run> [--origin <s>] [--status <list>] [--seq <id>]` | List events, filter, or show one event in full. |
| `steno render <run>` | (Re)generate `flow.mmd` from the timeline. |
| `steno replay <run> <seq>` | Print a `curl` reproducing one call. |
| `steno version` | Print the build version. |

`record` and `render` are separate so rendering can be iterated without re-recording. `<run>` accepts
either a run directory or a path to a `timeline.jsonl`.

## 9. Reserved and out-of-scope

### Reserved — parsed but not yet wired

These config sections are accepted (the parser is forward-compatible) but currently have **no effect**.
The struct fields are left declared so a future slice can implement them without a schema change:

- `[target].mode = "attach"`, `[target].attach_url`, `[target].profile_dir` — **attach-to-running** was
  evaluated and dropped: Chrome 136+ refuses remote-debugging flags on the default profile, so attach
  can never target the user's everyday browser with real sessions. Launch covers the realistic
  workflow.
- `[correlation].header` — capturing a join key (e.g. `traceparent`) shared with backend planes. Cheap
  and general, so parked rather than cut; the `Event.Correlation` field stays in the schema.
- `[[extract]]` — config-driven named-fact extraction (endpoint + JSON path → named timeline value).
  Needs per-app config and duplicates the full payload side-files; revisit for a repeated-regression
  workflow.
- `[[supplemental]]` — polling an external HTTP source (e.g. a Mailpit OTP) and folding it into the
  timeline. The most complex piece, niche value.

The `console`, `fact`, and `supplemental` event kinds exist in the model but are not emitted yet.

### Out of scope (non-goals)

- **Chromium family only.** Steno talks CDP (Chrome, Edge, Brave, Opera, …). Non-Chromium engines
  (Firefox/WebDriver BiDi, Safari/WebKit) are out of scope; the recorder seam keeps that door open
  cheaply (see architecture).
- **Not a deep network profiler.** Origin, path, method, status, payloads, basic stats — not full
  timing waterfalls or HAR-level detail.
- **No literal user-input capture.** CDP cannot observe real clicks/keystrokes and Steno injects no
  scripts. The journey is reconstructed from navigation and network events.
- **Not an automation or replay-the-session tool.** It does not drive the browser or generate
  Playwright/Puppeteer scripts. The human drives; Steno observes.
- **Not a DOM/pixel session-replay recorder.** It is not rrweb.
- **No analysis / no LLM in the tool.** Analysis happens downstream in an agent.
- **No redaction or masking — ever.** Full fidelity is a deliberate, permanent design choice, not a
  missing feature.

## 10. Resolved decisions

- **Name:** Steno.
- **Protocol / browsers:** CDP, Chromium family only. Firefox/Safari out of scope.
- **Connection:** launch-managed only (attach dropped, see §9).
- **User actions:** no JS injection, no click/keystroke capture; journey from navigation + network.
- **Config format:** TOML, lenient on unknown keys.
- **Detail level:** origin, path, method, status, full payloads, basic stats — no full headers by
  default, no timing waterfalls.
- **Fidelity:** nothing masked, redacted, or truncated.
- **Local only:** the debug port binds to localhost; the tool never phones home.

## 11. Acceptance criteria

Run a real, multi-step, multi-service flow and confirm:

- The timeline is short and signal-only (tens of calls, not thousands of asset requests).
- The cross-system journey is reconstructable from the timeline alone.
- The generated sequence diagram is readable and matches what happened.
- Each call carries enough to be useful: origin, path, method, status, both payloads, duration, size.
- An injected non-2xx is clearly localised to an origin and call, with payloads attached, and findable
  via `steno inspect --status 4xx,5xx`.
