# yaaadabi

**Yet Another Attempt At Dialogue Between AIs.**

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo_dark.png">
  <source media="(prefers-color-scheme: light)" srcset="assets/logo_light.png">
  <img alt="yaaadabi logo" src="assets/logo_light.png">
</picture>

Claude Code writes the code. Codex reviews it. They argue until the reviewer
approves. You stop being the clipboard between them.

## TL;DR

- **What**: an autonomous develop→review→commit loop between two coding
  agents — Claude as developer, Codex as reviewer, driven as a subprocess.
- **Human involvement**: exactly three moments — a design fork before coding
  starts, something only you can supply, or a review deadlock. Everything
  else, including the commit, is automatic.
- **Adopt a repo**: run `yaaadabi.exe C:/path/to/repo` from an elevated
  shell, fill in the Loop parameters it leaves in CLAUDE.md, start a fresh
  session, type `/task <what you want>`.
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
   without agreement → both positions land on your desk. Only a review that
   keeps finding new defects, each accepted and each fix holding, goes on
   past three; five rounds stops regardless.
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
`<placeholder>` lines it leaves in CLAUDE.md:

```markdown
Loop parameters:
- Verify: just build && just selftest
- Yardstick docs: docs/ROADMAP.md, tasks.md
- Review focus: C++ lifetime/UB, cross-platform Windows+macOS
- Task list: JIRA project ACME — file each [task] finding as an issue
```

`Verify` = the commands that must be green before and after review, in the
form whose tests actually execute — name the variant that defeats the test
runner's result cache (`go test -count=1 ./...`, not `go test ./...`),
because a replayed green is not evidence. Compilation caches are fine.
`Yardstick docs` = what defines "best" in this repo. `Review focus` = what
this codebase is most at risk of. `Task list` = where deferred `[task]`
findings go; delete the line and they go to `TODO.md` at the repo root. C++
repos whose build needs the MSVC environment: add a `build.cmd` that
`call`s `VsDevCmd.bat` first and name it in Verify (CMake presets pinning
a Visual Studio generator don't need this — CMake locates the toolchain
itself).

Start a **fresh** session (not `/resume` — the protocol loads at session
start) and kick off with `/task <description>`.

## Local conventions across several repos

Conventions shared by a group of your repos but nobody else's — a tracker
all of them file into, a house commit style — do not belong in this repo,
and repeating them in every CLAUDE.md rots. Put them in one file outside
every project and import it next to the protocol:

```markdown
## Review loop

@C:/Projects/yaaadabi/protocol.md
@C:/Projects/acme-conventions.md

Loop parameters:
- Verify: just build && just selftest
- Yardstick docs: docs/ROADMAP.md, tasks.md
- Review focus: C++ lifetime/UB, cross-platform Windows+macOS
```

The imported file supplies Loop parameters lines like any other — here a
`Task list` naming your tracker, so no wired repo needs a `TODO.md`. Any line
can travel that way; in this example only `Task list` does, because `Verify`,
`Yardstick docs` and `Review focus` differ per repo, so there is nothing to
share. Precedence: a line the repo states itself wins over the imported
default, so a shared default is worth setting even where one repo differs.
The file is yours, stays out of every repo, and this repo never learns it
exists.

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
  10 minutes; no output for ~15 minutes = hung → kill and re-run the exact
  command that hung, verbatim. Not a promptless `codex exec resume` as the
  recovery: it is rejected outright, and a hung first round has no verdict
  to respond to anyway.
- **Scratch files outside the working tree, in a per-session directory**:
  the review scope includes untracked files, so nothing may land in the
  repo — and `$TEMP/handover.md` is shared by every session on the machine,
  so two concurrent loops would overwrite each other's handover and verdict.
  Each session writes into the scratchpad directory Claude Code gives it.
- **Round 2+ via `codex exec resume <thread_id>`, never `--last`**: the
  reviewer keeps context and verifies its own findings were addressed —
  but `--last` means "newest recorded session for this working directory",
  so a second session reviewing the same repo silently takes over the
  first one's reviewer thread, and both get a coherent-looking transcript
  of the wrong review. The thread id is the first line of the JSONL
  stream (`{"type":"thread.started","thread_id":"..."}`), which is why
  step 3 redirects that stream to a file.
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
- **Verify must execute, not hit the cache**: a developer reported a green
  suite three times; `go test` had served cached results for the very
  package the change touched, and `-count=1` surfaced two more failures at
  once, one of them a port the change had broken. The reviewer's read-only
  sandbox could not run the tests either, so nobody had tested the change
  while both parties believed somebody had. The handover now states the
  exact Verify command and that its tests ran rather than being replayed;
  the reviewer treats a Verify claim without that statement as unverified.
  The rule is about test results, not compilation: a build cache that
  rebuilds what changed still runs the code, a test cache that replays a
  result does not.
- **The task list is whatever the repo names; `TODO.md` is only the
  default**: a repo migrated its findings into a JIRA epic and deleted
  `TODO.md`; the protocol's hardcoded filename would have recreated it on
  the next round. The post-approval "one tree change" exemption exists for
  a list that is a file — filing a ticket touches no tree.
- **A check that cannot run is said so, never implied**: `go test -race`
  needs cgo, the machine had no C toolchain, and the human declined
  installing one. Neither "execute it" nor "defer it" fit. The resolution
  was to make the property statically checkable (remove the racing write,
  not synchronise it), argue it structurally, and state plainly that the
  detector never ran. The temptation under a [blocking] finding is to be
  vague about what actually executed — the protocol names that clause as
  the load-bearing one.
- **Deadlock brake vs. convergence**: a four-round change accepted every
  finding, each round a new, real defect (the last a cash-loss path), and
  the 3-round brake fired on a loop that was visibly working — "summarize
  both positions" with no positions to summarize. Disagreement still stops
  at three; only a review whose every finding was accepted and whose every
  fix held continues, with a report, and it stops at five regardless — a
  limit that is easy to talk past is worth less than a blunt one. Both
  stops are the "review deadlock" touchpoint: a review that will not close.
- **Local conventions are an import, not a feature**: "all our repos file
  findings in JIRA" is real, and it must not leak into a repo published
  for everyone. CLAUDE.md imports already solve it — a private file next
  to the protocol import, supplying Loop parameters lines. What was
  missing was a named parameter to answer: `Task list` exists so a
  convention file can set it, instead of each agent re-deriving the task
  list from the repo's prose.
- **Unfilled Loop parameters have defined behaviour**: an agent met the
  template placeholders verbatim in a wired repo, resolved them from the
  rest of CLAUDE.md and copied both into the handover. It worked, but it
  was an invented convention; the protocol now prescribes exactly that, so
  every agent follows the same procedure and the reviewer sees what was
  assumed. A line the docs do not settle goes to the human.
