# banshee — [Feature] Plan (2026-11-13)

<!-- Copy this file to `qa/banshee - [Feature] Plan YYYY-MM-DD.md` for each
feature. Don't edit this copy. When every stage is done, move the plan to qa/archive/. -->

> **Status: Not started.** Record each stage's commit here.
>
> | Stage | Done |
> |---|---|
> | 1 | |
> | 2 | |

## Why

[The problem, in the owner's words where possible. What's wrong or missing today.]

## Stages

### Stage 1: [Name]
- **Changes:** [files, models, migrations, pages]
- **Tests, written first and seen failing:** [test names or what they check]
- **Red-proof:** [which control gets switched off, and which tests should turn red]
- **Done When:** [a concrete, checkable condition]

### Stage 2: [Name]
- **Changes:**
- **Tests, written first and seen failing:**
- **Red-proof:**
- **Done When:**

## Security gate

[Only if this feature touches permissions, input or a public URL; otherwise write
"n/a" and why.] Probes in `security/[area]/`, with IDs. Each probe is shown
EXPLOITED against a build with the control switched off, then HELD with it on.
Record them in `security/FINDINGS.md`.

## Out of scope

- [What this feature deliberately doesn't do, so nobody builds it by accident.]
