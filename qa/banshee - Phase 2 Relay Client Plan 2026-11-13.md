# banshee — Phase 2 Relay Client Plan (2026-11-13)

> **Status: All 4 stages implemented and verified locally on branch `phase-2-relay-client`.**
>
> | Stage | Done |
> |---|---|
> | 1 | `internal/relay/protocol.go`, `backoff.go`, `internal/store/alerts.go` + tests (red-proven stale guard) |
> | 2 | `internal/relay/transport.go`, `client.go` + unit & TLS `httptest` integration tests (12 tests green; red-proven 401 halt) |
> | 3 | `app.go`, `main.go`, `internal/env/dotenv.go` + App relay/tray wiring tests (red-proven `SaveSecret` restart) |
> | 4 | `frontend/` live Relay Link telemetry card, `security/relay/probe_test.go` (`P2-01`..`P2-03` HELD), `security/FINDINGS.md`, `impeccable detect` (0 findings) |

## Why

Phase 1 delivered the desktop shell, system tray lifecycle, DPAPI secret store, and SQLite settings/alerts schema. Phase 2 connects `banshee` live to the `turing-api` relay (`https://relay.example.com`) over outbound `wss://` so the workstation maintains an authenticated WebSocket stream, replays missed events monotonically from `MAX(seq)` on reconnect, persists alerts idempotently before sending `ack`, diagnoses invalid device tokens via HTTP `401` probes, and surfaces live `connected / degraded / offline` telemetry in the desktop UI.

## Stages

### Stage 1: Relay Protocol Frames, Jittered Backoff & Idempotent Alerts Store
- **Changes:** `internal/relay/protocol.go`, `internal/relay/protocol_test.go`, `internal/relay/backoff.go`, `internal/relay/backoff_test.go`, `internal/store/alerts.go`, `internal/store/alerts_test.go`.
- **Tests, written first and seen failing:**
  - `TestProtocolParseAndEncodeFrames` / `TestProtocolRejectsMalformedOrInvalidFrames`
  - `TestBackoffBoundsAndReset`
  - `TestAlertsStoreIdempotencyPreservesLocalStateAndPrivacy`
- **Red-proof:** Temporarily disable the `seq <= stored.seq` / local-state preservation check in `UpsertFromRelay` and negative-`seq` validation in `protocol.go`, verify tests turn red, then restore.
- **Done When:** All protocol, backoff, and alerts store unit tests pass with `go test -count=1 ./internal/relay ./internal/store`.

### Stage 2: Outbound WSS Relay Client, HTTP 401 Auth Probe & Heartbeat Watchdog
- **Changes:** `internal/relay/transport.go`, `internal/relay/transport_test.go`, `internal/relay/client.go`, `internal/relay/client_test.go`, `go.mod` (promote `github.com/gorilla/websocket` to direct dependency).
- **Tests, written first and seen failing:**
  - `TestRelayEndpointsEnforcesHTTPS`
  - `TestHTTPAuthProbeStatusMapping`
  - `TestRelayClientReplayOrderingAndAckAfterStore`
  - `TestRelayClientStoreFailureSkipsAckAndCloses`
  - `TestRelayClientAuthProbe401StopsReconnect`
  - `TestRelayClientHeartbeatTimeoutAndBackoff`
  - `TestRelayClientLiveMockWebSocketIntegration` (real `httptest.Server` + `gorilla/websocket` handshake, `hello` cursor, replay, `ack`, `ping`/`pong`, and HTTP `401` upgrade rejection)
- **Red-proof:** Temporarily send `ack` before `UpsertFromRelay` or reconnect after HTTP `401`, confirm tests go red, then restore.
- **Done When:** `go test -count=1 ./internal/relay/...` and `go vet ./...` pass cleanly.

### Stage 3: App Lifecycle Wiring, Live Tray Status, `.env` Dev Fallback & Wails Bindings
- **Changes:** `app.go`, `main.go`, `app_test.go`, `internal/tray/tray.go`, `internal/env/dotenv.go`, `internal/env/dotenv_test.go`.
- **Tests, written first and seen failing:**
  - `TestAppRelayStatusBindingAndSecretRedaction`
  - `TestAppReconfiguresRelayOnURLOrTokenChange`
  - `TestDotenvLoadsUnsetBansheeVarsOnly`
- **Red-proof:** Temporarily omit token redaction or ignore `SaveSecret` restart, confirm test fails red, then restore.
- **Done When:** `go test -count=1 ./...`, `go vet ./...`, and `wails generate module` / `wails build -clean` pass cleanly.

### Stage 4: Live Relay Status Dashboard (`DESIGN.md` Tokens), Security Gate & Impeccable Audit
- **Changes:** `frontend/src/style.css`, `frontend/src/lib/status.ts`, `frontend/src/lib/status.test.ts`, `frontend/src/App.tsx`, `frontend/src/components/SettingsPanel.tsx`, `frontend/wailsjs/...`, `security/relay/probe_test.go`, `security/FINDINGS.md`, `PLAN.md`.
- **Tests, written first and seen failing:**
  - Updated `frontend/src/lib/status.test.ts` covering `connected`, `degraded`, `offline`, `standby`, and error/timestamp formatting.
  - `security/relay/probe_test.go` (red-proven probes for HTTPS-only `wss://` derivation, bearer header-only auth, zero token leakage in status/logs, and zero media URL persistence in SQLite `alerts`).
- **Red-proof:** Temporarily allow `http://` in `RelayEndpoints` or put token in query string, run security probe to confirm `EXPLOITED`, restore control and confirm `HELD`.
- **Done When:** `go vet ./...`, `go test -count=1 ./...`, `npm --prefix frontend run typecheck`, `npm --prefix frontend test`, `npm --prefix frontend run build`, `wails build -clean`, and `impeccable` checks all pass cleanly.

## Security gate

Touches outbound TLS/WebSocket endpoints, bearer device token authentication, and untrusted relay JSON payloads (`security/relay/probe_test.go` + `security/FINDINGS.md`). Controls:
1. Strict `https://` -> `wss://` endpoint derivation (reject `http://`, `ws://`, malformed URLs).
2. Device bearer token sent exclusively via `Authorization: Bearer <token>` HTTP upgrade header, never in the URL query string.
3. Strict allowlist/JSON validation of inbound server frames; malformed frames dropped without crash or secret echo.
4. Zero persistence of `mediums` URLs or `metadata` in SQLite `alerts` (`snapshot_url`, `clip_url`, `metadata_summary` remain `NULL`).

## Out of scope

- Banshee alarm overlay card stack, dismiss/acknowledge UI, and Web Audio / OS siren playback (Phase 3).
- Turing OpenAPI camera list / HLS playback (parked).
- Windows `.exe` CI release artifact workflow and SDLC comparison report (Phase 4).
