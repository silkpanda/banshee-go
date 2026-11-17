# Plan — banshee

Written 2026-11-13. This is the one plan for the repository: where each phase stands, what is open, and what comes next. Each feature's stage-by-stage plan lives in `qa/` while it is being built and moves to `qa/archive/` when it is done.

---

## Now

*Updated 2026-11-13 by Antigravity. A session handoff, not history: every session rewrites this section before it ends (see AGENTS.md). Read it first.*

| | |
|---|---|
| **In progress** | Phase 2 implementation complete on branch `phase-2-relay-client` (branched from `phase-1-shell-and-config`); PR opened targeting `main` (merge Phase 1 PR #1 first, or review together). |
| **Next** | Merge Phase 1 PR #1 and Phase 2 PR after CI is green, run live desktop check with a dev device token against `https://relay.example.com`, then start Phase 3 (`phase-3-alarm-surface`). |
| **Waiting on the owner** | Review & merge Phase 1 PR #1 and Phase 2 PR, test Windows tray + live relay connection using `.env` or Settings UI (`BANSHEE_DEVICE_TOKEN`). |
| **Other work in flight** | Phase 1 PR #1 (`phase-1-shell-and-config` -> `main`) is open with green CI. |
| **Loose ends** | Move `qa/banshee - Phase 1 Shell and Config Plan 2026-11-13.md` and `qa/banshee - Phase 2 Relay Client Plan 2026-11-13.md` to `qa/archive/` once their PRs merge and human live checks pass. |

---

## Where It Stands

| Phase | Description | Tests / Checks | Status |
|---|---|---|---|
| 0 | Project scaffolding, runnable Wails + shadcn baseline, docs, GitHub repo and CI | `go test ./...`, `go vet ./...`, frontend typecheck/test/build, first CI run green | ✅ Done |
| 1 | Desktop shell, system tray, single-instance lock, and encrypted local config | Go tests (14), frontend tests (6), `go vet`, `wails build` (15.69 MB `.exe`), PR CI | 🔄 PR Open |
| 2 | Live relay WebSocket client, cursor replay, and `connected/degraded/offline` status UI | Go unit + TLS integration + security gate (`P2-01`..`P2-03`), frontend checks (8), `wails build` | 🔄 PR Open |
| 3 | Banshee alarm overlay, sound/focus cue, and alert history feed | Red-proven alert state tests + `/impeccable audit` and `/impeccable polish` clean | ⏳ Next |
| 4 | Release packaging (`.exe` build in CI), security gate, and SDLC comparison notes | Windows `.exe` CI artifact build + secrets scan + `security/FINDINGS.md` clean | ⏹ Pending |

---

## How Work Flows

1. **Test-gated phases:** Every phase passes its explicit acceptance command before moving to the next.
2. **Branch and pull request:** Every change reaches `main` through a pull request with CI green. Nothing is committed on `main` directly.
3. **Audit boundaries:** Respect `AGENTS.md` rules and constraints.

---

## Work Queue

### Phase 1: Desktop Shell, Tray & Local Config
- [x] Task 1: Copy `qa/FEATURE-PLAN.md` into a Phase 1 plan with numbered stages.
- [x] Task 2: Implement local `%APPDATA%\banshee` config/SQLite store with transactional migrations and Windows DPAPI secret vault.
- [x] Task 3: Add system tray menu (Show/Hide, Relay Status, Quit) and single-instance window focus behavior.
- [x] Task 4: Wire typed Go ↔ React Wails bindings for settings and app status (`SettingsPanel`).
- **Done When:** `go test -count=1 ./...`, `go vet ./...`, `npm --prefix frontend run typecheck`, `npm --prefix frontend test`, `npm --prefix frontend run build`, and `wails build -clean` pass cleanly, and PR CI is green. *(Code + build green; interactive Windows tray click check pending human launch.)*

### Phase 2: Live Relay WebSocket Client & Status UI
- [x] Task 1: Implement outbound `wss://` relay client in Go matching the `turing-api` relay protocol (`hello` with `MAX(seq)` cursor, idempotent alert persistence, `ack` after durable write, jittered backoff).
- [x] Task 2: Add HTTP `401` auth probe so bad device tokens report a clear diagnostic instead of generic offline noise.
- [x] Task 3: Build the live Status & Settings dashboard in React (`shadcn/ui` + `DESIGN.md` tokens).
- **Done When:** Unit tests verify `hello`/replay/ack ordering and backoff bounds, and running locally with a valid `.env` token connects to `https://relay.example.com` and displays live relay state. *(Code, TLS mock integration, security gate, and `wails build` green; live relay token test pending human `.env` token.)*

### Phase 3: Banshee Alarm Surface & Impeccable Polish
- [ ] Task 1: Implement alert card stack, acknowledge/dismiss flow, and optional Banshee audio cue.
- [ ] Task 2: Run `/impeccable detect`, `/impeccable audit`, and `/impeccable polish` across the React UI.
- **Done When:** All Impeccable deterministic checks pass with zero high-contrast or token violations, and a simulated/live alert triggers the alarm state cleanly.

### Phase 4: Release Packaging & SDLC Study
- [ ] Task 1: Add Windows `.exe` release build workflow in `.github/workflows/`.
- [ ] Task 2: Document side-by-side SDLC & CI/CD measurements comparing `banshee` (Go + Wails) vs. `turing-api` (Electron + TypeScript).
- **Done When:** GitHub Actions produces a runnable `banshee.exe` artifact and the comparison summary is recorded.

---

## Open & Parked

- **Live HLS 16-camera video grid:** Deliberately parked/out-of-scope for v1 (per `turing-api` ADR 0002, WebView2 HLS edge cases are why `turing-api` kept Electron for multi-stream video; `banshee` focuses on the desktop shell, relay client, alerting, and CI/CD comparison). The owner decides if video playback is ever added later.
