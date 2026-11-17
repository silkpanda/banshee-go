# banshee

A Go learning project. I rebuilt the desktop alert client from an Electron + TypeScript app in **Go + Wails + React** to see how the two stacks compare on build, test, and CI.

It connects outbound to a WebSocket alert relay, stores alerts in SQLite, keeps tokens encrypted with Windows DPAPI, and raises a tray alarm when a selected alert type comes in. Go owns networking, storage, and secrets. React only draws the screen. CI runs both toolchains on every push.

Built with Claude Code and OpenCode, in phases I planned with an acceptance gate on each one, and any change touching tokens, input, or the relay URL had to pass probes in `security/` that were first shown failing with the control switched off.

## Prerequisites
- **Go:** `1.27.4`
- **Node.js:** `24.x` (with `npm`)
- **Wails CLI:** `v2.17.1` (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- **Windows WebView2 Runtime:** Built into Windows 11 / modern Windows 10

## Quickstart

1. **Install frontend dependencies:**
   ```powershell
   npm --prefix frontend ci
   ```

2. **Environment configuration:**
   ```powershell
   copy .env.example .env
   ```

3. **Run local dev mode (Vite on port `34115` + Go live reload):**
   ```powershell
   wails dev
   ```

4. **Run checks & tests:**
   ```powershell
   go vet ./...
   go test ./...
   npm --prefix frontend run typecheck
   npm --prefix frontend test
   npm --prefix frontend run build
   ```

5. **Build standalone Windows `.exe`:**
   ```powershell
   wails build
   ```

## Architecture & Directory Layout
```text
banshee/
├── main.go               # Wails window bootstrap & embedded frontend assets
├── app.go                # Go backend struct & typed bindings exposed to React
├── wails.json            # Wails project configuration
├── frontend/             # React + TypeScript + Vite (port 34115) + Tailwind v4 + shadcn/ui + craft
├── .github/workflows/    # Two-toolchain CI pipeline (Go + Node/Wails)
├── qa/                   # Stage-by-stage feature plans & QA findings
├── security/             # Security probes & findings register
├── deploy/               # Local Windows deployment & update guide
├── PRODUCT.md            # Durable product context & constraints
├── PLAN.md               # Phase roadmap, Done When gates & session handoff
├── DESIGN.md             # Raycast-inspired dark tokens & Banshee alarm palette
└── AGENTS.md             # Rules for Antigravity, Claude Code & OpenCode
```
