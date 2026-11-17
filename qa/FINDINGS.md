# QA findings — banshee

Whether the app works for people. Security findings go in `security/FINDINGS.md`,
and code-health items in `TECH_DEBT.md`.

Log each finding the moment it's found, with an ID (`QA-01`, `QA-02`, ...) and a
status in its heading: OPEN, FIXED (with the commit), or ACCEPTED (with the reason).

Template for an entry:

### QA-01 — [short description] — OPEN
- **Steps:** [as which user, what you did]
- **Expected:** [what should happen]
- **Actual:** [what happened]
- **Where:** [file:line, page or URL]
- **Fix:** [what changed, the commit, and the test that covers it]
