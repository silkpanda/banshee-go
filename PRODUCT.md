# Product

<!-- impeccable:product-schema 1 -->

## Platform

Windows Desktop Application (Go + Wails v2 + native Windows WebView2, paired with a React + TypeScript UI).

## Stack

- **Desktop Runtime & Backend:** Go `1.27.4`, Wails `v2.17.1`
- **Frontend UI:** React `19`, TypeScript, Vite (dev server on port `34115`)
- **Design & Styling:** Tailwind CSS v4, `shadcn/ui` (Base UI primitives), `craft` layout primitives, `lucide-react` icons, `impeccable` quality gates
- **Local Persistence (planned):** SQLite + Windows DPAPI secret storage
- **Live Cloud Integration:** Outbound HTTPS / WebSocket (`wss://`) to the `turing-api` relay (`https://relay.example.com`)

## Users

- **Owner / Learner:** Uses `banshee` both as a practical, lightweight Windows desktop alarm/relay monitor and as a hands-on engineering lab to compare a **Go + Wails** SDLC and CI/CD pipeline against `turing-api`'s **Electron + TypeScript** desktop shell.
- **Office / Operator Workstation:** Needs glanceable relay health, stored alert history, and instant visual/audio alarm visibility without Electron's ~100 MB+ Chromium overhead.

## Product Purpose

Deliver a fast, lightweight, tactile Windows desktop alerting shell ("Banshee" — pairing Wails with an unmistakable alarm identity) that connects to the live Turing relay, persists alerts and settings safely on the local machine, and serves as a side-by-side reference for Go + Wails build, test, security, and release pipelines.

## Positioning

`banshee` re-implements the Phase 2 desktop shell of `turing-api` in Go + Wails rather than Electron + Node.js. Instead of bundling a full Chromium browser, it renders the React interface inside Windows' built-in WebView2 engine and moves system tray, storage, networking, and OS integration into compiled Go code.

## Operating Context

- Runs on Windows 10/11 desktop workstations with WebView2 (version 157+ verified locally).
- Connects **outbound only** to `https://relay.example.com` (never exposes inbound ports on the office PC).
- Device tokens and Turing API tokens are sensitive credentials: they live only in gitignored `.env` files during local development or OS-protected local storage in production, never in source code, git history, logs, or frontend bundles.

## Capabilities and Constraints

- **System Tray & Window Lifecycle:** Hide to tray on close, reveal on second launch (single-instance behavior), explicit Quit from tray.
- **Live Relay Connectivity:** Authenticated WebSocket connection (`hello` + monotonic `seq` replay cursor) with `connected / degraded / offline` health reporting and bounded reconnect backoff.
- **Banshee Alarm Surface:** High-clarity, low-clutter dark command UI styled via `DESIGN.md` (`raycast`-inspired dark surfaces + crisp crimson/amber alarm telemetry).
- **Hard Constraint — Zero Secrets in Repo:** Never commit `.env`, SQLite databases, bearer tokens, or relay secrets. CI uses throwaway placeholder env vars only.
- **Hard Constraint — Branch & PR Discipline:** Never commit directly to `main` after the initial project-prep baseline; every change flows through a feature branch and green CI pull request.

## Brand Commitments

- **Name & Voice:** *Banshee* — sharp, immediate, calm until an alert fires, unmistakable when it does.
- **Visual Direction:** Near-black tactical surfaces (`#07080a` → `#121212`), hairline `1px` borders (`#242728`), `Hanken Grotesk` + `JetBrains Mono` typography, emerald for healthy relay heartbeat, amber for degraded states, and crimson (`#ff5757` / `#ff6161`) reserved strictly for active alarms.

## Evidence on Hand

- Reference Electron implementation and frozen relay WebSocket protocol documented in the private `turing-api` repo (`docs/sdlc/02-architecture.md`).
- Local toolchain verified: Go `1.27.4`, Wails CLI `v2.17.1`, Node `v24.16.0`, WebView2 `157.0.4341.46`.
