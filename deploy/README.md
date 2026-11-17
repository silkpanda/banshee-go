# Deploy — banshee

> **Local Windows desktop application.** Built as a standalone Windows `.exe` via Wails (`wails build`) and shipped via GitHub Releases / local installer. Not a remote server service.

## Where it runs

- **Target machine:** Windows 10 / 11 desktop PC (with WebView2 runtime installed — standard on modern Windows).
- **Cloud dependency:** Connects outbound over HTTPS/WSS to the live `turing-api` relay (`https://relay.example.com`).
- **Local storage:** SQLite database and encrypted secrets stored under the user's local `%APPDATA%\banshee\` directory (never in the repo).
- **Settings:** Runtime configuration via `.env` (gitignored) or in-app settings UI.

Private details (device tokens, local paths) stay in `.env` or `CLAUDE.local.md`, which are gitignored.

## Release order

1. Merge feature PR into `main` only after CI (`tests.yml`) is green.
2. Tag or build release binary via `wails build` (Windows `.exe` output in `build/bin/banshee.exe`).
3. Verify the packaged `.exe` boots cleanly, connects to the relay, and sits in the system tray before replacing the running binary.

## Updating to a new version

1. Quit the running `banshee.exe` from the system tray.
2. Back up `%APPDATA%\banshee\banshee.sqlite` before schema upgrades.
3. Replace `banshee.exe` with the new build.
4. Launch `banshee.exe` and verify the relay status badge turns green (`connected`).

## Backups & Rollback

- **Backups:** Copy `%APPDATA%\banshee\banshee.sqlite` before major updates.
- **Rollback:** Quit `banshee.exe`, restore the previous `banshee.exe` and backup `.sqlite` file, and relaunch.
