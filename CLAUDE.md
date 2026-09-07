# CLAUDE.md

This repo IS the loop: the shared develop→review protocol and roles that
every wired project imports (see README.md). Deliverables: the shared prose
files (`protocol.md`, `developer.md`, `reviewer.md`, `commands/task.md`,
README.md) plus `main.go` — the Go wiring tool that configures a repo to
use them (it copies nothing but the user-level /task command; the shared
files stay here and are imported by reference).

A change here changes how every wired repo works. Treat edits accordingly:
keep the shared files repo-agnostic (anything repo-specific belongs in a
repo's Loop parameters block, not here), keep protocol/roles/README
consistent with each other, and record new lessons in the README field
notes with the reason, not just the rule.

After editing `commands/task.md`, copy it to `~/.claude/commands/task.md` —
the installed copy does not update itself.

## Review loop

@protocol.md

Loop parameters:
- Verify: `just verify` — vets the host build AND both build-tagged halves
  (half of `isElevated`/`dropPrivileges`/`invokingHome` is excluded by
  whichever platform you are on, and the excluded half breaks silently),
  then runs `go test -count=1 ./...` so nothing is replayed from the test
  cache. One command on either machine: the justfile holds the checks and
  the per-shell `GOOS` syntax, so this line does not restate them in the
  syntax of whichever shell its author happened to be using.

  Plus reading every prose file the change touches and each file that
  references it (protocol ↔ roles ↔ README must not contradict each other).
- Yardstick docs: README.md (especially the field notes — decisions in this
  repo were bought with trial-run failures; don't undo one without a reason
  that beats the original).
- Review focus: repo-agnosticism AND machine-agnosticism of the shared files
  (no path, platform, or shell assumption that is only true on one machine);
  consistency between protocol, roles, and README; unverified claims stated
  as fact.
