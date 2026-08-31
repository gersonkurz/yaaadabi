# yaaadabi

**Yet Another Attempt At Dialogue Between AIs.**

Claude Code writes the code. Codex reviews it. They argue until the reviewer
approves. You stop being the clipboard between them.

## TL;DR

- **What**: an autonomous develop→review→commit loop between two coding
  agents — Claude as developer, Codex as reviewer, driven as a subprocess.
- **Human involvement**: exactly three moments — a design fork before coding
  starts, something only you can supply, or a review deadlock. Everything
  else, including the commit, is automatic.
- **Adopt a repo**: run `yaaadabi.exe C:/path/to/repo` from an elevated
  shell, fill in two placeholder lines, start a fresh session, type
  `/task <what you want>`.
- **Cost**: no server, no daemon, no config files. One shared git clone,
  a few lines in each repo's CLAUDE.md.

## How it works

1. You type `/task fix the crash when scanning an empty folder`.
2. Claude implements it, runs the repo's build and tests until green.
3. Claude writes a structured handover (problem, chosen approach, rejected
   alternatives, what executed vs. what was only read) and pipes it to
   `codex exec` — the verdict lands straight back in its context.
4. Codex reviews design AND correctness, tags findings
   `[blocking]` / `[suggestion]` / `[task]`, ends with `VERDICT: APPROVED`
   or `VERDICT: NEEDS-WORK`.
5. NEEDS-WORK → fix, re-review in the same Codex session. Three rounds
   without agreement → both positions land on your desk.
6. APPROVED → final build + tests, commit, report: task, rounds, verdict,
   hash.

The result isn't just saved keystrokes. Both agents hold each other to a
written standard — forced comparison of alternatives, execution evidence
over reading confidence, findings that can't get lost in transcripts.

## Quick start

**Once per machine:**

1. Clone this repo to `C:\Projects\yaaadabi` (the protocol references
   `C:/Projects/yaaadabi/reviewer.md` by absolute path).
2. Build the wiring tool once: `go build -o yaaadabi.exe .` (self-contained
   exe; the Go toolchain is only needed for this step).
3. Copy `commands/task.md` to `~/.claude/commands/task.md`.
4. Requirements: Claude Code, codex-cli ≥ 0.151.0 on PATH.

**Once per repo:**

```
C:\Projects\yaaadabi\yaaadabi.exe C:/path/to/repo
```

Run it yourself, from an elevated (administrator) shell — granting an agent
permissions is a human act, and the tool enforces that: it refuses to run
non-elevated or inside an agent session, precisely so no agent can wire up
its own permissions. (Windows sudo is not a substitute in practice: without
inline mode it opens a new window whose output vanishes with it.)

The tool merges the loop's permission rules into
`.claude/settings.local.json`, appends a `## Review loop` block to
CLAUDE.md, creates an AGENTS.md stub if the repo has none, and installs the
user-level `/task` command. Idempotent — safe to re-run. Then fill in the
two `<placeholder>` lines it leaves in CLAUDE.md:

```markdown
Loop parameters:
- Verify: just build && just selftest
- Yardstick docs: docs/ROADMAP.md, tasks.md
- Review focus: C++ lifetime/UB, cross-platform Windows+macOS
```

`Verify` = the commands that must be green before and after review.
`Yardstick docs` = what defines "best" in this repo. `Review focus` = what
this codebase is most at risk of. C++ repos whose build needs the MSVC
environment: add a `build.cmd` that `call`s `VsDevCmd.bat` first and name
it in Verify (CMake presets pinning a Visual Studio generator don't need
this — CMake locates the toolchain itself).

Start a **fresh** session (not `/resume` — the protocol loads at session
start) and kick off with `/task <description>`.

## What's in the box

| File | Role |
|---|---|
| `protocol.md` | The review protocol, repo-agnostic. Imported into each repo's CLAUDE.md. |
| `developer.md` | Developer role: best-not-quickest, forced alternatives comparison, execution over reading, reporting discipline. |
| `reviewer.md` | Reviewer role: judges approach AND correctness, `[blocking]`/`[suggestion]`/`[task]` tags, evidence weighting, `VERDICT:` contract. |
| `commands/task.md` | The `/task` kickstart command (installed user-level, works in every wired repo). |
| `main.go` | The wiring tool. Zero flags, elevation-gated, idempotent. |

Repo-specific knowledge lives in each repo, not here: the Loop parameters
block in its CLAUDE.md, which the protocol reads and every handover copies
verbatim to the reviewer. Codex additionally reads the repo's own AGENTS.md,
as always.

## Field notes — why it is the way it is

Every rule below was bought with a real failure. Don't undo one without a
reason that beats the original.

- **Plain `codex exec`, not `codex exec review`**: the built-in review
  subcommand rejects `--uncommitted` combined with a prompt, and ignores
  the output contract in favor of its own fixed report format. Plain
  `codex exec` with the scope stated in the prompt honors the contract
  exactly.
- **Background always, `--json` as heartbeat**: reviews routinely exceed
  10 minutes; no output for ~15 minutes = hung → kill,
  `codex exec resume --last`.
- **Scratch files in `$TEMP`, never the working tree**: the review scope
  includes untracked files.
- **Round 2+ via `codex exec resume --last`**: the reviewer keeps context
  and verifies its own findings were addressed.
- **`-s read-only` on the reviewer**: a user-level codex config default of
  `workspace-write` would otherwise let the reviewer modify the tree
  mid-review.
- **Elevation, not environment checks, as the consent boundary**: a
  `CLAUDECODE` env guard and a `net session` subprocess check were both
  bypassed by the reviewer itself (scrubbed variable; fake `net.exe` on
  PATH). The tool queries the process token in-process via the Windows API.
  Corollary: run your agent sessions non-elevated, or you dissolve the
  boundary yourself.
- **Evidence rule, `[task]` tag, scope circuit breaker**: a 14-round manual
  session found something real every round, yet produced a large,
  plausible, entirely unexecuted change to destructive paths — the review
  peeled a defect class layer by layer while the diff grew more invasive.
  Reading is argument; execution is evidence. Findings drift is measured
  from the ticket that opened the loop, never from the previous round's
  fix, and deferred findings go in a task list a transcript can't swallow.
