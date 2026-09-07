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
- Verify: all four of these, and the tests must actually EXECUTE — half the
  tool sits behind build tags, so vetting only the host build leaves the
  tagged-out half to break silently:
  1. `go vet ./...` — the host build.
  2. `go vet ./...` with `GOOS=windows` — the `elevate_windows.go` half.
  3. `go vet ./...` with `GOOS=darwin` — the `elevate_unix.go` half (`linux`
     selects the same file, either will do).
  4. `go test -count=1 ./...` — `-count=1` so no result is replayed.

  Setting `GOOS` is the one shell-specific part, and this repo is worked on
  from both kinds of machine: `GOOS=windows go vet ./...` in a POSIX shell,
  `$env:GOOS='windows'; go vet ./...` in PowerShell (then `Remove-Item
  Env:GOOS`). Name in the handover which form ran.

  Plus reading every prose file the change touches and each file that
  references it (protocol ↔ roles ↔ README must not contradict each other).
- Yardstick docs: README.md (especially the field notes — decisions in this
  repo were bought with trial-run failures; don't undo one without a reason
  that beats the original).
- Review focus: repo-agnosticism AND machine-agnosticism of the shared files
  (no path, platform, or shell assumption that is only true on one machine);
  consistency between protocol, roles, and README; unverified claims stated
  as fact.
