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
- **Adopt a repo**: run `yaaadabi /path/to/repo` as a human (sudo on
  macOS/Linux, an elevated shell on Windows), fill in the Loop parameters it
  leaves in CLAUDE.md, start a fresh session, type `/task <what you want>`.
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

1. Clone this repo wherever you keep things. No fixed location: the clone's
   path is written into each wired repo's CLAUDE.md import line, and
   everything else is derived from it — which is also why different repos
   can use different instruction sets, see [Customizing](#customizing).
2. Build the wiring tool once, inside the clone: `just build`, or plain
   `go build` if you would rather not install [just](https://just.systems).
   Either way you get `yaaadabi` on macOS and Linux and `yaaadabi.exe` on
   Windows — `go build` names it per platform, so no `-o` is needed.
   Self-contained binary; the Go toolchain is only needed for this step.
   Building it *inside the clone* is what makes the zero-argument default
   work: the binary looks for the prose files in its own directory, which is
   also why the justfile deliberately does not build into an `out/` dir.
3. Requirements to USE it: Claude Code, and codex-cli on PATH — ≥ 0.151.0,
   last exercised with 0.153.2 (macOS arm64, Homebrew). To BUILD it: Go.
   To work on this repo itself: `just`, which its `Verify` parameter calls.

The tool installs the user-level `/task` command itself; you only need to
copy `commands/task.md` to `~/.claude/commands/task.md` by hand if you skip
the tool entirely.

**Once per repo**, and you run it yourself — an agent cannot:

| Platform | Command |
|---|---|
| macOS, Linux | `sudo ~/dev/yaaadabi/yaaadabi /path/to/repo` |
| Windows, from an elevated (administrator) shell | `…\yaaadabi\yaaadabi.exe C:/path/to/repo` |

Granting an agent permissions is a human act, and the tool enforces that out
of process: it refuses to run unless the process is elevated, and fast-fails
inside an agent session, precisely so no agent can wire up its own
permissions. `-h` is exempt — printing usage never needs elevation.

### What differs between Windows and macOS

|  | macOS and Linux | Windows |
|---|---|---|
| **Consent** | `sudo` — the tool requires uid 0 | an elevated (administrator) token, i.e. the UAC prompt you already answered |
| **Afterwards** | root is given straight back, before the tool looks at any path: groups, gid and uid drop to the account behind `SUDO_UID`, and it refuses to continue if the drop did not take | nothing to drop — an elevated process is the same user with a fuller token |
| **The weak spot** | sudo caches its authentication for a few minutes, so an agent starting right after you sudo'd can ride that timestamp without knowing your password; `Defaults timestamp_timeout=0` in sudoers closes it | run your agent sessions non-elevated, or you dissolve the boundary yourself |
| **Environment** | `sudo` usually strips it, so `$YAAADABI_DIR` needs `sudo -E` — or just pass `-loop-dir` | inherited as usual |
| **Path in the import** | `/Users/you/dev/yaaadabi` | `C:/Projects/yaaadabi` — forward slashes, written for you |

Two platform-specific traps worth knowing. On Windows, `sudo` is not a
substitute for an elevated shell in practice: without inline mode it opens a
new window whose output vanishes with it. On macOS and Linux, `~` is
resolved by looking up `SUDO_USER` rather than `$HOME`, because whether
`sudo` resets `HOME` is a per-machine sudoers policy — otherwise `/task`
would land in `/var/root` and be invisible to every session while looking
like a success.

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

| Parameter | What it says | If you omit it |
|---|---|---|
| `Verify` | The commands that must be green before and after review, in the form whose tests actually **execute** — name the variant that defeats the runner's result cache (`go test -count=1 ./...`, not `go test ./...`), because a replayed green is not evidence. Compilation caches are fine. If the repo is worked on from more than one OS, put the checks behind a task-runner recipe (`just verify`, a make target) so this line is one portable command rather than one shell's string — see the field note below. | The agent resolves it from the repo's own docs and states that resolution in the handover; if the docs do not settle it, it asks you. |
| `Yardstick docs` | What defines "best" in this repo — the docs the reviewer judges against. | Same as above. |
| `Review focus` | What this codebase is most at risk of. Violations there are `[blocking]`. | Same as above. |
| `Task list` | Where deferred `[task]` findings go — a tracker project, an epic, a file. | `TODO.md` at the repo root, created if absent. |
| `Codex command` | The reviewer binary, when it is not plain `codex`. The one parameter paired with a permission rule, so it is changed with the tool rather than by hand alone — see [Customizing](#one-repo-the-loop-parameters-block). | `codex`. The wiring tool only writes this line when you passed `-codex`. |

C++ repos whose build needs the MSVC environment: add a `build.cmd` that
`call`s `VsDevCmd.bat` first and name it in Verify (CMake presets pinning
a Visual Studio generator don't need this — CMake locates the toolchain
itself).

Start a **fresh** session (not `/resume` — the protocol loads at session
start) and kick off with `/task <description>`.

**The first session in a newly wired repo asks you to approve the import.**
The protocol lives outside the repo, so Claude Code treats it as an external
import and shows a one-time approval dialog listing the files. Accept it. If
you decline, imports stay disabled for that project and the dialog never
comes back — the session then loads no protocol at all, silently, and the
loop simply does not happen. The flag is
`projects.<repo>.hasClaudeMdExternalIncludesApproved` in `~/.claude.json` if
you need to undo a mistaken decline. Non-interactive runs (`claude -p`)
cannot show the dialog, so they never load an unapproved external import.

## Customizing

Three dials, from the most local to the most global. None of them is a
config file: each is either a line in a repo's own CLAUDE.md, or a flag you
pass once when wiring that repo.

### One repo: the Loop parameters block

The block the tool leaves in CLAUDE.md — the table above — is the per-repo
dial, and it is plain markdown you edit yourself. `Verify`, `Yardstick
docs`, `Review focus` and `Task list` are pure prose read by the protocol:
change them whenever the repo changes, nothing caches them, no tool run is
needed, and the edit takes effect in the next **fresh** session, because the
protocol loads at session start.

`Codex command` is the exception, because it is the one line paired with
something outside CLAUDE.md — the `Bash(<cmd> exec:*)` rule in
`.claude/settings.local.json`. Editing it alone leaves the protocol driving
one binary while the permission rule names another, and the loop then stalls
on a permission prompt in a session that is supposed to be autonomous. So
after changing that line, re-run the wiring tool with a matching
`-codex <cmd>`; it accepts the hand edit and installs the rule. The
superseded rule stays behind — the tool only ever adds rules — so remove it
by hand if you would rather not leave a stale allow for a binary you no
longer drive.

### Several of your repos: one extra import

Conventions shared by a group of your repos but nobody else's — a tracker
all of them file into, a house commit style — do not belong in this repo,
and repeating them in every CLAUDE.md rots. Put them in one file outside
every project and import it next to the protocol:

```markdown
## Review loop

@~/dev/yaaadabi/protocol.md
@~/acme-conventions.md

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

### A different protocol entirely: `-loop-dir`

Which instruction set a repo uses is per-repo DATA, not a machine-wide
setting: the wiring tool writes that directory into the repo's own import
line, and nothing else anywhere records it. So you can keep more than one
set of the shared prose and point repos at different ones — a fork with a
harsher `reviewer.md` for the code that must not break, the stock one for
everything else:

```
sudo ~/dev/yaaadabi/yaaadabi         /path/to/ordinary-repo
sudo ~/dev/yaaadabi-strict/yaaadabi  /path/to/high-stakes-repo
```

Each repo's CLAUDE.md then names its own, and `$LOOP` — how the protocol
finds `reviewer.md` to feed Codex — resolves per repo from that same line,
so the two never mix. To fork the prose, clone this repo again, edit
`protocol.md` / `developer.md` / `reviewer.md`, build the tool inside the
fork, and wire with that binary.

**How the loop directory is chosen**, in order: the `-loop-dir` flag, else
`$YAAADABI_DIR`, else the directory of the running binary — which is the
clone itself if you built it there, so the common case needs neither. On
macOS and Linux remember that `sudo` usually strips the environment, so
`$YAAADABI_DIR` needs `sudo -E`; the flag is simpler.

Whatever it resolves to is validated before anything is written:

- A directory that does not contain `protocol.md`, `developer.md` and
  `reviewer.md` is refused, rather than written into a CLAUDE.md that would
  then import nothing.
- A path containing whitespace or shell metacharacters is refused. The
  protocol interpolates it unquoted into both a shell command and an
  `@`-import, so the safety has to be in the value.
- A `~/...` path is kept verbatim in the import line instead of being
  expanded. That is what lets ONE committed CLAUDE.md work on two machines
  whose home directories differ — measured, not assumed: a `@~/…` import
  expands, and a protocol imported that way still reaches its role file by
  relative import.

**Changing it later** is a hand edit, deliberately. Re-running the tool
against a repo already wired to a different directory does not rewrite it:
it reports the path that is in the file and the path this run would have
written, and stops. Same for `-codex` disagreeing with the repo's
`Codex command` line. The tool never guesses which of two values you meant —
so after moving a clone, or moving machines, fix the import line in each
repo's CLAUDE.md (or delete the `## Review loop` block and re-run).

**If the codex binary is not called plain `codex`** here, pass
`-codex <cmd>` when wiring: it goes into the permission rule AND into a
`Codex command` line of Loop parameters, so the rule and the protocol always
name the same binary.

## What's in the box

| File | Role |
|---|---|
| `protocol.md` | The review protocol, repo-agnostic. Imported into each repo's CLAUDE.md. |
| `developer.md` | Developer role: best-not-quickest, forced alternatives comparison, execution over reading, reporting discipline. |
| `reviewer.md` | Reviewer role: judges approach AND correctness, `[blocking]`/`[suggestion]`/`[task]` tags, evidence weighting, `VERDICT:` contract. |
| `commands/task.md` | The `/task` kickstart command (installed user-level, works in every wired repo). |
| `justfile` | `build` / `clean` / `rebuild`, and `verify` — the checks this repo's own `Verify` parameter runs, in one place instead of restated per shell. |
| `main.go` | The wiring tool. Two flags, elevation-gated, idempotent. |
| `elevate_windows.go`, `elevate_unix.go` | The consent gate per platform: an elevated token on Windows; on unix, uid 0 plus an immediate, verified drop back to the account behind `SUDO_UID`. |
| `AGENTS.md` | Codex's entry point here, pointing at CLAUDE.md — the same stub the wiring tool writes into a repo that has none. |

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
- **Read-only reviewer = TWO settings, re-stated every round**: a sandbox
  alone is not a boundary. Pinning `read-only` and leaving the approval
  policy to the user's config was probed on identical prompts: with
  `approval_policy="on-request"` the reviewer's write was denied, it
  escalated, retried, and the file appeared in the working tree; with
  `approval_policy="never"` the same write was denied and stayed denied.
  The sandbox decides what is blocked, the approval policy decides whether
  the agent may climb over the block — so every review command pins both,
  and the loop stops depending on whatever `~/.codex/config.toml` says.
  Spelling differs by command: a fresh `codex exec` takes `-s read-only`,
  while `codex exec resume` has no `-s` at all (`error: unexpected argument
  '-s' found`) and needs `-c sandbox_mode="read-only"`. Both must be
  repeated on each resumed round, because a resumed turn runs under the new
  invocation's settings, not the ones its thread started with. Found by a
  reviewer noticing the resume command had never carried either: rounds 2+
  of every review had been running writable, contradicting this very note.
- **Elevation, not environment checks, as the consent boundary**: a
  `CLAUDECODE` env guard and a `net session` subprocess check were both
  bypassed by the reviewer itself (scrubbed variable; fake `net.exe` on
  PATH). The tool queries the process token in-process via the Windows API.
  Corollary: run your agent sessions non-elevated, or you dissolve the
  boundary yourself.
- **A trailing period silently swallows an `@import`**: the protocol's very
  first line used to read ``Work per @<clone>/developer.md.`` — and that
  period is parsed as part of the filename, so the developer role file was
  never imported, on any platform, for as long as the line existed. Measured
  with canary strings in a throwaway repo: the same import on a line of its
  own resolves, the same import ending a sentence does not, absolute or
  relative, inside the working directory or not. The import now sits on its
  own line with a maintainer comment saying why.
- **A nested relative import DOES resolve against its own file** — which is
  what lets the clone move: `protocol.md` reaches `developer.md` with a bare
  `@developer.md` and no path at all. This was documented but not believed
  until it was measured, and rightly so: the first probe of it failed (the
  trailing period above), and a design resting on an unverified platform
  behaviour would have shipped broken. Probe first, then depend on it. The
  `@~/…` form was measured the same way, and needed a trick to measure at
  all: a tilde path that resolves OUTSIDE the working directory is an
  external import, so the approval gate below hides the answer. Putting the
  probe repo under `$HOME` separates the two questions, and both then pass.
- **The external-import approval is part of the setup, not a detail**: a
  wired repo imports the protocol from outside the working directory, so the
  first session shows a one-time approval dialog — and a declined or
  unanswered one means the session loads no protocol at all, with no error
  anywhere. Verified by running a wired repo non-interactively, where the
  dialog cannot be shown: every canary came back NONE. The quick start says
  to accept it, and names the `~/.claude.json` flag that undoes a decline.
- **Dropping root was necessary but not sufficient — don't write through a
  symlink either**: the check that proved the privilege drop works also
  showed what it does NOT fix. An agent that can create files in the repo can
  pre-place `CLAUDE.md` (or `.claude/`, or `settings.local.json`) as a
  symlink; before the drop that redirected a ROOT write anywhere on the
  machine, and after the drop it still redirects a write to anything the
  HUMAN can write — their `~/.zshrc` will do. None of the four paths this
  tool writes has any business being a link, so each is `Lstat`-ed and
  refused at PLAN time, before anything is written — which is the whole
  point of where the check sits: the run that first exposed this had already
  written `settings.local.json` before the `CLAUDE.md` write was denied by
  the filesystem, and moving the check into the plan phase means the same
  scenario now writes nothing at all. Remaining honesty: the plan/apply
  split gives all-or-nothing for POLICY refusals, not for I/O errors. A disk
  filling up between two applies can still leave a half-wired repo. The tool
  is idempotent, so re-running finishes the job; making four small writes
  atomic would need a staging directory, which is a lot of machinery for
  "run it again".
- **A Verify line is itself a platform assumption**: the macOS port added
  `GOOS=windows go vet ./...` to this repo's own Verify so the tagged-out
  half of the tool could not break silently — and in doing so made the line
  unusable on the very machine it was protecting. `VAR=x cmd` is POSIX shell
  syntax that neither cmd.exe nor PowerShell parses, and on a Windows host
  `GOOS=windows` re-vets the build that was just vetted while the unix half
  goes unchecked. Loop parameters are copied verbatim into every handover and
  run by whichever machine picks the task up, so a parameter that only works
  in one shell is the same defect class as a hardcoded path — and this repo's
  review focus names exactly that. A Verify line for a repo built on more
  than one OS should therefore name the CHECKS rather than one string that
  happens to run where its author was sitting. Spelling each shell's syntax
  out in the parameter was the first fix and it worked, but it restated in
  prose what a task runner already solves; the checks now live in a
  `just verify` recipe — `just`'s own `os()` conditional picks
  `GOOS=x cmd` or `set GOOS=x&& cmd` — and the parameter is one portable
  command. Which is the same principle one level up: the parameter names
  what must pass, not how one machine spells it.
- **macOS has no UAC, so the gate is `sudo` — with two consequences**:
  `geteuid() == 0` is the only in-process, unspoofable equivalent, and it is
  weaker than UAC in one specific way, because sudo's timestamp cache lets a
  process that starts minutes after you authenticated skip the password
  (`Defaults timestamp_timeout=0` if that matters to you). The second
  consequence is that root must be given straight back. The first attempt
  chowned every created file to `SUDO_UID` afterwards, and the reviewer
  killed it: repairing ownership does not undo a root write. Every path this
  tool touches is one an agent could have replaced with a symlink first
  (`CLAUDE.md`, `.claude/`, `settings.local.json`), a root write follows that
  symlink anywhere on the machine, and a root `chown` would then hand the
  target to the user — that is a local privilege escalation built out of a
  convenience. So the tool drops groups, gid and uid before it looks at a
  path, restores the invoking user's FULL group list rather than just the
  primary one (a repo writable through a supplementary group is writable to
  the human, so it must stay writable to the tool), verifies the drop, and
  refuses to continue if it did not take — including the case of a root
  login with no `SUDO_UID` to return to, which is refused outright rather
  than run as root —
  measured end to end on 2026-09-04 with a script that wires a throwaway
  repo under sudo and checks the owner of every file it creates, and that a
  `CLAUDE.md` symlinked into a root-only directory is refused rather than
  followed: 9 of 9 checks, `dropped root, now uid 501/gid 20`. The
  privileged region is three statements long and touches no path at all, so
  `-h` needs no password. The
  home directory still comes from looking up `SUDO_USER`, never `$HOME` —
  whether sudo resets `HOME` is per-machine sudoers policy, and a `/task`
  installed into `/var/root/.claude/commands` would be invisible to every
  session while looking like a success.
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
- **The clone's location is data, not a constant**: `C:/Projects/yaaadabi`
  was hardcoded in the wiring tool and written into the protocol three more
  times — the developer role import, the reviewer `cat`, and the README's
  install step. Moving to a second machine broke all four at once, and the
  duplication is what made it a rewrite rather than an edit. The path is now
  recorded in exactly ONE place per wired repo, the CLAUDE.md import line,
  and everything else derives from it: the protocol reaches its role file
  through a relative `@developer.md` (Claude Code resolves a relative import
  against the importing file's own directory, so it lands next to the
  protocol wherever the clone is), and `$LOOP` for the reviewer `cat` is
  that import line's own directory. Hence also no `Loop directory`
  parameter: a second copy of the path is a second thing to get out of sync.
  A repo still wired to an old clone is not silently rewritten — the tool
  reports which path is in the file and which one this run would write, and
  leaves the decision to the human.
- **Two flags, and only because they are machine facts**: the tool used to
  advertise zero flags, and that was the right instinct for Loop parameters
  (they need thought, so they belong in CLAUDE.md). `-loop-dir` and `-codex`
  are a different category — the tool cannot guess either one, and neither
  is a judgement call. Their defaults still cover the normal case: the
  binary's own directory, which is the clone if you built it there, and
  plain `codex`. A `Codex command` line is written into a repo only when it
  is NOT the default, so the common repo stays free of noise.
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
