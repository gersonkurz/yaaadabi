# Review protocol (mandatory for every coding task)

Work per @C:/Projects/yaaadabi/developer.md. A task is not done until the
reviewer (Codex, driven as a subprocess) has approved it. The loop is
autonomous — do not stop for human approval between steps; consult the human
only for a material design fork (before implementing), something only they
can supply, or a review deadlock (a review that will not close — defined in
step 5).

The repository's CLAUDE.md declares a **Loop parameters** block: the Verify
commands, the yardstick docs, the review focus, and optionally the task list
(step 4). Those parameters instantiate this protocol for the repo. A line may
reach CLAUDE.md through a file it imports rather than written inline — the
same thing, except that a value stated in the repo's own block wins over an
imported default. If the block is still the unfilled template, resolve each
line from the repo's own docs and state that resolution verbatim in the
handover, so the reviewer judges against a stated yardstick, not an assumed
one. A line the repo's docs do not settle is something only the human can
supply — ask; `Task list` is the exception, because step 4 defines its
default.

Every scratch file this loop writes — handover, verdict, response, the
reviewer's event log — goes in a scratch directory OUTSIDE the repo that is
unique to THIS session, `$SCRATCH` below. Claude Code names a per-session
scratchpad directory in the session's environment; use that one. If the
environment names none, create one once (`mktemp -d`) and reuse it for the
whole task. Never a fixed path such as `$TEMP/handover.md`: concurrent
sessions share `$TEMP`, and two of them would overwrite each other's handover
and verdict. Keep the paths absolute — `codex exec` runs with the repo as its
working directory (it refuses to run outside a trusted directory), so a path
relative to the scratch directory does not resolve.

1. Implement, then verify: run the Verify commands from Loop parameters —
   all green before requesting review. The tests and checks must actually
   EXECUTE: a test result replayed from a cache is not evidence. Some test
   runners and build tools replay or skip results by default (Go's test
   cache, Gradle's up-to-date checks) — where the repo's does, defeat it
   (`go test -count=1`, `gradle --rerun-tasks`, or the repo's equivalent)
   and say in the handover which form ran. A compilation cache
   that rebuilds what changed (Go's build cache, ccache) is fine — the code
   still runs; only replayed or skipped test/check results are excluded.
2. Write a numbered handover to a scratch file OUTSIDE the repo
   (`$SCRATCH/handover.md` — never in the working tree; the review covers
   untracked files): problem statement, motivation, chosen approach and
   rejected alternatives, what changed (files + why), what was deliberately
   NOT changed, an evidence statement (which changed paths have actually
   EXECUTED — test, selftest, probe — and which have only compiled and been
   read, plus the exact Verify command that ran and that its tests executed
   rather than being replayed or skipped),
   the review scope stated explicitly (e.g. "review the
   uncommitted diff plus untracked files" — plain `codex exec` is given no
   diff automatically), and the repo's Loop parameters block copied verbatim
   so the reviewer judges against the right yardstick.
3. Submit as a BACKGROUND task — reviews routinely exceed 10 minutes; never
   wait in the foreground:
   `cat C:/Projects/yaaadabi/reviewer.md $SCRATCH/handover.md | codex exec -s read-only -c approval_policy="never" --json -o $SCRATCH/verdict.md - > $SCRATCH/review.jsonl`
   Both settings are load-bearing, and neither substitutes for the other:
   the sandbox blocks a write, the approval policy decides whether the
   reviewer may escalate past that block. With only the sandbox pinned, a
   user config that permits escalation lets the reviewer retry the denied
   write with elevated permissions and succeed. Pin them on every review
   command, so the loop does not depend on the reviewer's own config.
   The redirect matters: that JSONL event stream is both the heartbeat and
   the only place the reviewer's thread id appears — its first line is
   `{"type":"thread.started","thread_id":"<uuid>"}`. Keep that id; step 5
   needs it. A running review whose `review.jsonl` has not grown for ~15
   minutes is hung — kill it and re-run the exact command that hung, this
   one verbatim (it starts a fresh reviewer thread on the same handover) or
   step 5's verbatim (it re-sends the same response to the same thread).
   What does not work is inventing a recovery command: a `codex exec
   resume` with no prompt is rejected outright, and before the first
   verdict there is no response to resume with.
4. Read the verdict; report it in the conversation, then continue
   immediately. Whatever the verdict, first record any [task] findings
   verbatim in the repo's task list: the `Task list` line of Loop
   parameters where it names one (a tracker project or epic, a file,
   whatever), otherwise whatever the repo's own agent instructions name,
   otherwise `TODO.md` at the repo root — created if absent. An APPROVED
   review can carry tasks too, and a finding that lives only in a review
   transcript or completion report is lost. When the task list is a file in
   the repo, this verbatim transcription of the reviewer's own findings is
   the one tree change permitted between approval and commit — its content
   was authored by the reviewer, so re-reviewing it adds a round and no
   information (human-blessed exemption, 2026-08-31). Filing into a tracker
   outside the repo is not a tree change; the exemption is simply not
   needed there.
5. On NEEDS-WORK: address every [blocking] finding (suggestions at your
   judgment — state what you did with each), or push back with reasons
   grounded in the project docs; [task] findings are already recorded per
   step 4 and are NOT acted on in this change. A [blocking]
   "needs an executed check" finding resolves one of three ways: execute
   the check; or — with the human's explicit acceptance, obtained via the
   only-the-human touchpoint — file a follow-up task in the repo's task
   list (created per step 4 if the repo has none) and state both the
   acceptance and the filed task in the response, whereupon the reviewer
   treats it as a deferred blocker; or, where the check cannot run in the
   available environment and the human declines the prerequisite, say so
   explicitly and argue the property structurally instead — never imply
   the check ran. If addressing
   findings makes the change materially more invasive (a file format, a
   migration, a new subsystem), stop the loop and hand the split decision
   to the human — rounds must narrow the change's scope, not expand it;
   added tests, probes, and executed checks are always in scope, whatever
   they do to the diff's line count. Re-review in the same session:
   `codex exec resume <thread_id> -c sandbox_mode="read-only" -c approval_policy="never" --json -o $SCRATCH/verdict.md - < $SCRATCH/response.md >> $SCRATCH/review.jsonl`
   — both of step 3's settings must be re-stated on every resumed round,
   and the sandbox in this form: a resumed turn takes the sandbox and
   approval policy of the new invocation, not the ones the thread started
   with, and `codex exec resume` has no `-s` (it fails with
   `error: unexpected argument '-s' found`). The id is the one kept from
   step 3, never `--last`: `--last` resolves to the newest recorded session
   for the working directory, so a second session reviewing the same repo
   silently hijacks this one's reviewer thread.
   (background + heartbeat rules as in step 3)
   After 3 rounds without approval, continue only on convergence: every
   [blocking] finding so far was accepted and none has come back — each
   round found a new defect and the fix for the previous one held. Report
   the round history in the conversation and go on, but stop at round 5
   regardless. Anything else is deadlock — a finding still disputed, a
   finding that returned because its fix was incomplete, or round 5
   reached: stop, summarize the positions (or the finding and the fixes
   that did not hold), hand to the human. A review that will not close is
   the deadlock touchpoint named at the top of this protocol.
6. On APPROVED: re-run the Verify commands (tests executing, as in step 1)
   on the final state, then commit (per the repo's commit conventions) and
   report: task, rounds, verdict, commit hash.
