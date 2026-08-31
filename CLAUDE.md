# CLAUDE.md

This repo IS the loop: the shared develop→review protocol and roles that
every wired project imports (see README.md). There is no code to build —
the deliverables are `protocol.md`, `developer.md`, `reviewer.md`,
`commands/task.md`, and the README.

A change here changes how every wired repo works. Treat edits accordingly:
keep the shared files repo-agnostic (anything repo-specific belongs in a
repo's Loop parameters block, not here), keep protocol/roles/README
consistent with each other, and record new lessons in the README field
notes with the reason, not just the rule.

After editing `commands/task.md`, copy it to `~/.claude/commands/task.md` —
the installed copy does not update itself.

## Review loop

@C:/Projects/yaaadabi/protocol.md

Loop parameters:
- Verify: none — prose-only repo; verification is reading every file the
  change touches plus each file that references it (protocol ↔ roles ↔
  README must not contradict each other).
- Yardstick docs: README.md (especially the field notes — decisions in this
  repo were bought with trial-run failures; don't undo one without a reason
  that beats the original).
- Review focus: repo-agnosticism of the shared files; consistency between
  protocol, roles, and README; unverified claims stated as fact.
