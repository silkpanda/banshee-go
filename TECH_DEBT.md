# Tech debt — banshee

This register lists the code-health problems that aren't bugs or security findings.
Security findings go in `security/FINDINGS.md`, and feature work goes in `PLAN.md`.

**Severity**
- **High:** costs something on every request, or will mislead the next person to change the code.
- **Medium:** slows work down or lets problems through without anyone noticing.
- **Low:** tidy-up.

**Status:** OPEN → FIXED (with the commit) or ACCEPTED (with the reason).

When you fix an item, update its row and add the commit to its section. An item that
adds or changes a rule follows the usual practice: write the test, show it going red,
then fix.

## Summary

| ID | Severity | Area | Item | Effort | Status |
|---|---|---|---|---|---|
| | | | | | |

Effort: **S** takes under half a day, **M** takes one to three days, **L** takes longer.

---

<!-- One section per item:

## TD-01: [short description]

**Where:** [files]

**What:** [the problem, with numbers if it was measured]

**Fix:** [what to change]

**Verify:** [the test or check that proves it's fixed]
-->
