# banshee — Phase 1 Shell and Config Plan (2026-11-13)

> **Status: All 4 stages implemented and verified locally; PR open on `phase-1-shell-and-config`.**
>
> | Stage | Done |
> |---|---|
> | 1 | `internal/store/db.go` + `db_test.go` (3 tests green) |
> | 2 | `internal/store/settings.go` + `internal/secrets/` (6 tests green, live DPAPI round-trip) |
> | 3 | `internal/lifecycle/` + `internal/tray/` + `main.go` + `app.go` (`wails build -clean` 15.69 MB `.exe`) |
> | 4 | `frontend/` SettingsPanel + validation + Go bindings (6 frontend tests + `App` tests green) |

## Why

Phase 0 delivered the runnable Go + Wails + React baseline. Before `banshee` can connect live to the Turing relay in Phase 2, the desktop shell needs the same foundational plumbing as `turing-api` Phase 2: durable local storage in `%APPDATA%\banshee`, Windows DPAPI-protected token storage (no plaintext secrets), system tray hide/show/quit + single-instance window behavior, and a clean Settings & Status UI.

## Stages

### Stage 1: SQLite Store & Transactional Migrations (`internal/store`)
- **Changes:** `internal/store/db.go`, `internal/store/db_test.go` using pure-Go `modernc.org/sqlite`.
- **Tests:** `TestApplyMigrationsCreatesSchema`, `TestApplyMigrationsIdempotentOnReopen`, `TestApplyMigrationsRollsBackOnError` — all passing.
- **Done When:** Migrations `0001-settings`, `0002-secrets`, `0003-alerts` apply transactionally, re-runs apply `0`, and failing migrations roll back cleanly without leaving partial state.

### Stage 2: Allowlisted Settings & DPAPI Secret Store (`internal/store/settings.go`, `internal/secrets`)
- **Changes:** `internal/store/settings.go`, `internal/store/settings_test.go`, `internal/secrets/store.go`, `internal/secrets/dpapi_windows.go`, `internal/secrets/dpapi_other.go`, `internal/secrets/store_test.go`.
- **Tests:** `TestSettingsRejectsUnknownOrSecretRefKeys`, `TestSettingsEnforcesHTTPSRelayURL`, `TestSettingsRoundTripsAndToleratesCorruptRows`, `TestSecretsRefusesPlaintextWhenCipherUnavailable`, `TestSecretsRejectsUnknownNamesAndEmptyOrOversizedTokens`, `TestSecretsDPAPIRoundTripLeavesZeroPlaintextOnDisk` — all passing.
- **Done When:** All store and secret tests pass green, including a live Windows DPAPI round-trip test.

### Stage 3: System Tray Lifecycle & Single-Instance Lock (`internal/tray`, `main.go`, `app.go`)
- **Changes:** `internal/lifecycle/policy.go`, `internal/lifecycle/policy_test.go`, `internal/tray/tray.go`, `main.go`, `app.go`.
- **Tests:** `TestLifecycleHideVsQuit` passing; `wails build -clean` produces `build/bin/banshee.exe` (15.69 MB).
- **Done When:** `wails build` compiles cleanly with `HideWindowOnClose` / `OnBeforeClose`, `SingleInstanceLock`, and `energye/systray` menu (`Show Banshee`, `Relay: Standby`, `Quit Banshee`).

### Stage 4: Typed Go ↔ React Bindings & Settings/Secrets UI (`frontend/`)
- **Changes:** `app.go` bindings (`GetSettings`, `UpdateSetting`, `GetSecretsStatus`, `SaveSecret`, `ClearSecret`, `HideToTray`), `frontend/src/components/SettingsPanel.tsx`, `frontend/src/App.tsx`, `frontend/src/lib/validation.ts`, `frontend/src/lib/validation.test.ts`, plus `shadcn/ui` input/field/switch/toggle-group/alert components.
- **Tests:** `validation.test.ts` (4 tests), `status.test.ts` (2 tests), `TestAppSettingsAndSecretBindingsNeverExposePlaintext` — all passing; `impeccable.exe detect frontend/src --no-advisory` reported 0 static findings across 21 files.
- **Done When:** `go vet ./...`, `go test -count=1 ./...`, `npm --prefix frontend run typecheck`, `npm --prefix frontend test`, `npm --prefix frontend run build`, and `wails build -clean` all pass, and PR is green on GitHub Actions.

## Security gate

Verified by unit tests in `internal/store/settings_test.go`, `internal/secrets/store_test.go`, and `app_test.go`:
1. Non-`https://` `relay_url` rejected at both React validation (`validation.ts`) and Go backend (`settings.go`) boundaries.
2. Unknown setting keys, `*_secret_ref` keys, and unknown secret names rejected by allowlist.
3. DPAPI secret storage refuses to save when encryption is unavailable, and raw SQLite file bytes contain zero plaintext tokens.
4. `GetSecretsStatus` / `GetSettings` serialized JSON never contains plaintext token strings.
Note: Interactive desktop UI + real tray click test remains for human smoke testing on Windows.

## Out of scope

- Live `wss://` relay connection & event replay (Phase 2).
- Banshee audio siren & overlay alert card stack (Phase 3).
- Live HLS camera streaming (parked).
