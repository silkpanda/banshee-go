# Security findings — banshee

Every security finding and how it was fixed. Log it the moment it's found, even
before there's a fix.

**IDs:** `P[phase]-[nn]` for a finding from a phase's security gate, `REVIEW-[nn]`
for one from a code review.

**Status:** OPEN, FIXED (with the commit and the probe or test that proves it),
or ACCEPTED (with the reason and who accepted it).

**The gate:** each probe in `security/` prints EXPLOITED or HELD. A probe counts
only once it has been shown EXPLOITED against a build with the control switched
off, then HELD with the control back on. Only point probes at local addresses.

## Phase 1 Gate Checks (2026-11-13)

No open security findings in Phase 1. Verified controls via Go + frontend unit tests:
- **HTTPS-only `relay_url`:** `TestSettingsEnforcesHTTPSRelayURL` and `validation.test.ts` verify `http://`, `ws://`, and malformed URLs are rejected.
- **Allowlisted keys & secret names:** `TestSettingsRejectsUnknownOrSecretRefKeys` and `TestSecretsRejectsUnknownNamesAndEmptyOrOversizedTokens` verify refusal by allowlist.
- **No plaintext fallback or on-disk leakage:** `TestSecretsRefusesPlaintextWhenCipherUnavailable` and `TestSecretsDPAPIRoundTripLeavesZeroPlaintextOnDisk` verify Windows DPAPI encryption (`CryptProtectData` / `CryptUnprotectData`) and zero plaintext bytes in the `.sqlite` file.
- **No secret echo to UI:** `TestAppSettingsAndSecretBindingsNeverExposePlaintext` verifies `GetSecretsStatus` and `GetSettings` JSON responses never include plaintext tokens.

## Phase 2 Gate Checks (2026-11-13)

No open security findings in Phase 2. Verified controls via `security/relay/probe_test.go`:
- **`P2-01` (HTTPS-only `wss://` derivation):** `TestProbeP201RelayURLRequiresHTTPS` — shown `EXPLOITED` when `http://` was temporarily allowed in `RelayEndpoints`, then `HELD` with strict `https://` and no-userinfo enforcement restored.
- **`P2-02` (Bearer header-only auth):** `TestProbeP202BearerTokenHeaderOnlyNeverQueryString` — `HELD`; device token is sent only in `Authorization: Bearer <token>` on WebSocket upgrade and never in the URL query string.
- **`P2-03` (Zero token, media URL, or PII leakage):** `TestProbeP203ZeroTokenAndMediaLeakage` — `HELD`; relay logs, `RelayStatusSnapshot`, and raw `.sqlite` disk bytes contain zero device tokens, `mediums` URLs, or `metadata` payloads.
