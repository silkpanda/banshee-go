# AGENTS.md — banshee

Instructions for every coding agent working in this repo (**Antigravity**, **Claude Code**, and **OpenCode**). Keep this file short: it points to the real docs rather than repeating them. If something here disagrees with those docs, the docs win; fix this file.

## What this is
Banshee is a lightweight Windows desktop alerting and relay-monitoring shell modeled on `turing-api`'s Phase 2 desktop architecture, built to compare a Go + Wails SDLC and CI/CD workflow against Electron. Stack: Go `1.27.4`, Wails `v2.17.1`, React `19`, TypeScript, Vite (port `34115`), Tailwind CSS v4, `shadcn/ui` (Base UI), `craft`, and `impeccable`.

- `README.md`: setup and how to run it. Read it first.
- `PRODUCT.md`: who it's for and what it must do.
- `PLAN.md`: where each phase stands. Its **Now** section says what's happening right now.
- `qa/`: one plan per feature, built in stages. Finished plans move to `qa/archive/`.
- `DESIGN.md`: color tokens, typography, surface hierarchy, and component rules. Read it before building any screen.
- `qa/FINDINGS.md`, `security/FINDINGS.md`, `TECH_DEBT.md`: the registers. Log a finding the moment it's found.
- `deploy/README.md`: where it runs, and how to update, back up and restore it.

## Commands (run from the project root)
```powershell
# Dev server (Wails + Vite frontend on http://localhost:34115)
wails dev

# Go targeted & full tests + static analysis
go test ./internal/config -run TestName
go test ./...
go vet ./...

# Frontend typecheck, unit tests, and production bundle (from frontend/)
npm --prefix frontend run typecheck
npm --prefix frontend test
npm --prefix frontend run build

# Full production Windows executable build
wails build
```

## Who does what
- **Antigravity / Claude Code:** Architecture, `PLAN.md` and `qa/` feature planning, Go backend and Wails bindings, security & code review, and Impeccable UI audits.
- **OpenCode:** Scoped stage implementation on dedicated branches/worktrees following an approved `qa/` feature plan.
- Any agent picking up a session must leave `PLAN.md`'s **Now** table accurate for the next agent.

## Start and end of every session
- **Start:** read the **Now** section near the top of `PLAN.md` before anything else. It says what's in progress, what's next, and what's waiting on the owner. If it disagrees with git (`git status`, `git worktree list`, open PRs), trust git and say so.
- **End:** before you finish, rewrite the Now section so the next session (any agent) can pick up from it alone. Replace it; don't append. Date it and name yourself. Keep it to the table: in progress, next, waiting on the owner, other work in flight, loose ends. Commit it on your branch with your work, so it reaches `main` with your PR. If you made no other change, put it on a `plan/<yyyy-mm-dd>-now` branch and open a PR, never a commit on `main`.
- Short questions that change nothing don't need an update.

## How work is done here
1. **Plan first.** A feature gets a plan in `qa/`, copied from `qa/FEATURE-PLAN.md`, with numbered stages. Build one stage at a time, and record what each stage actually did in the plan's status table.
2. **Tests prove the rule.** For each new rule or control, write the test, switch the control off, confirm the test goes red, and switch it back on. The plan's status table says which tests turned red.
3. **Check it in the app / browser** for anything a user sees (launch `wails dev` on port `34115` and run `/impeccable audit` for UI work).
4. **Security gate.** Anything that touches permissions, input, local secret storage, or a public URL gets probes in `security/` that are red-proven first and then HELD, and an entry in `security/FINDINGS.md`.
5. **Commits** look like `<Feature> Stage 2: <what changed, in plain words>`, or `Plan: ...` for plan-only changes. One branch per feature, merged into `main` by pull request once CI is green. Fill in every section of the PR template.

## Rules that are easy to break
- **Nothing is committed on `main`, not even a `Plan:` change.** The baseline commit from project prep is the only exception. Every change, docs and plans included, goes on a branch and reaches `main` only through a pull request on GitHub. Don't merge into `main` locally and don't push to it. If you're on `main` when it's time to commit, make a branch first (`plan/<yyyy-mm-dd>-<topic>` for plan-only changes), commit there, open a PR, and say that's what you did. This holds when the owner says "commit" or "push" while on `main`: he wants the branch.
- **Never commit secrets:** `.env`, local databases, credentials and tokens stay gitignored. Private machine details (server addresses, local paths, SSH aliases) go in `CLAUDE.local.md`, which is gitignored. `AGENTS.md` and `CLAUDE.md` are committed.
- **Never leave the baseline red:** every task ends with both Go tests (`go test ./...`) and frontend checks (`typecheck`, `test`, `build`) green.
- **Respect Wails + `shadcn/ui` boundaries:** Go owns networking, secrets, SQLite, and OS tray/window control; React owns presentation only and calls typed Wails bindings (never embed secrets in `frontend/`). Follow `DESIGN.md` and `shadcn` rules (`gap-*` instead of `space-y-*`, semantic tokens, no nested cards).

## Working alongside other agents
More than one agent may work on this repo. To avoid overwriting each other:
- Work only on the branch or task you were given, ideally in your own worktree.
- Don't edit files outside your task's area. If you need to, stop and say so.
- Don't rewrite, reformat or "tidy" code you weren't asked to touch.
- If the working tree has uncommitted changes you didn't make, leave them alone.
