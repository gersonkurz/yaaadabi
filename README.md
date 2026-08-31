# yaaadabi

Yet Another Attempt At Dialogue Between AIs.

An autonomous develop→review loop between two coding agents: **Claude Code
develops, Codex reviews**, and the human is no longer the clipboard between
them. Claude drives Codex as a subprocess (`codex exec`); the verdict lands
directly in Claude's context; the loop iterates until the reviewer approves,
then verifies and commits. The human appears at exactly three points: a
material design fork before implementation, something only the human can
supply, or a review deadlock (3 rounds without agreement).

## What's in here

| File | Role |
|---|---|
| `protocol.md` | The review protocol, repo-agnostic. Imported into each repo's CLAUDE.md. |
| `developer.md` | Shared developer role: best-not-quickest, forced alternatives comparison, reporting discipline. |
| `reviewer.md` | Shared reviewer role: judges approach AND correctness, [blocking]/[suggestion] tags, `VERDICT:` contract. |
| `commands/task.md` | The `/task` kickstart command (installed user-level, works in every wired repo). |

Repo-specific knowledge lives in each repo, not here: a short **Loop
parameters** block in its CLAUDE.md (verify commands, yardstick docs, review
focus), which the protocol reads and the handover copies verbatim to the
reviewer. Codex additionally reads the repo's own AGENTS.md as always.

## Install (once per machine)

1. Clone to `C:\Projects\yaaadabi` — the protocol references
   `C:/Projects/yaaadabi/reviewer.md` by absolute path.
2. Copy `commands/task.md` to `~/.claude/commands/task.md`.
3. Requirements: Claude Code, codex-cli ≥ 0.151.0 on PATH; a Go toolchain
   to build the wiring tool (the built `yaaadabi.exe` is self-contained).

## Wire a repo (per repo)

The short way — run BY THE HUMAN from a terminal (the permission patch is
deliberately a human act):

```
go build -o yaaadabi.exe . && ./yaaadabi.exe C:/path/to/repo
```

No flags. The permission rules are a fixed set (codex + git — everything
else runs under the session's normal permission mode); the Loop parameters
land as placeholders for you to fill in CLAUDE.md — that part needs thought,
not arguments.

The tool requires an elevated shell (or `sudo yaaadabi`): an agent must not
be able to grant itself permissions by running the wirer, and the UAC prompt
is out-of-process human consent no agent can click. (A CLAUDECODE env check
refuses agent sessions early with a clearer message, but it is a courtesy,
not the boundary — an agent can scrub its own environment. Corollary: if you
run your agent sessions from an elevated terminal, you have dissolved this
boundary yourself.)

It merges the loop's permission rules into `.claude/settings.local.json`,
appends the Review loop block to CLAUDE.md (fill in the remaining
placeholders), creates an AGENTS.md stub if the repo has none, and installs
the user-level `/task` command. Idempotent — safe to re-run.

The manual way — the same wiring by hand (plus, which the tool also does:
ensure the repo has an AGENTS.md for the reviewer, and that
`commands/task.md` is installed per machine-install step 2):

1. Add to the repo's CLAUDE.md:

   ```markdown
   ## Review loop

   @C:/Projects/yaaadabi/protocol.md

   Loop parameters:
   - Verify: <build command> && <test command>
   - Yardstick docs: <the docs that define "best" for this repo>
   - Review focus: <what this codebase is most at risk of>
   ```

2. Add to the repo's `.claude/settings.local.json` — a HUMAN step; the
   auto-mode classifier rightly blocks agents from editing permission files:

   ```json
   { "permissions": { "allow": [
     "Bash(codex exec:*)",
     "Bash(git add:*)", "Bash(git commit:*)",
     "Bash(git status:*)", "Bash(git diff:*)"
   ] } }
   ```

   (Build/test commands need no rules here in auto mode; add
   `"Bash(<build command>:*)"` only if your permission mode prompts.)

3. C++ repos whose build needs the MSVC environment: add a `build.cmd`
   that `call`s `VsDevCmd.bat` first, and name it in Verify. (CMake presets
   pinning a Visual Studio generator don't need this — CMake locates the
   toolchain itself.)

Then start a FRESH session (not `/resume` — the protocol loads at session
start) and kick off with `/task <description>`.

## Field notes (why it is the way it is)

- `codex exec review` (the built-in subcommand) is unusable for this loop:
  it rejects `--uncommitted` combined with a prompt, and ignores the output
  contract in favor of its own fixed report format. Plain `codex exec` with
  the scope stated in the prompt honors the contract exactly.
- Reviews routinely exceed 10 minutes → always submit as a background task;
  the `--json` JSONL stream doubles as a heartbeat (no output for ~15 min =
  hung → kill, `codex exec resume --last`).
- Scratch files (handover, verdict) live in `$TEMP`, never the working
  tree — the review scope includes untracked files.
- Round 2+ uses `codex exec resume --last`: the reviewer keeps context and
  verifies its own findings were addressed; a fresh session would re-review
  from scratch.
- `-s read-only` on the reviewer: a user-level codex config default of
  `workspace-write` would otherwise let the reviewer modify the tree
  mid-review.
