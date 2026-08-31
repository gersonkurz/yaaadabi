# Review protocol (mandatory for every coding task)

Work per @C:/Projects/yaaadabi/developer.md. A task is not done until the
reviewer (Codex, driven as a subprocess) has approved it. The loop is
autonomous — do not stop for human approval between steps; consult the human
only for a material design fork (before implementing), something only they
can supply, or a review deadlock.

The repository's CLAUDE.md declares a **Loop parameters** block: the Verify
commands, the yardstick docs, and the review focus. Those parameters
instantiate this protocol for the repo.

1. Implement, then verify: run the Verify commands from Loop parameters —
   all green before requesting review.
2. Write a numbered handover to a scratch file OUTSIDE the repo
   (`$TEMP/handover.md` — never in the working tree; the review covers
   untracked files): problem statement, motivation, chosen approach and
   rejected alternatives, what changed (files + why), what was deliberately
   NOT changed, the review scope stated explicitly (e.g. "review the
   uncommitted diff plus untracked files" — plain `codex exec` is given no
   diff automatically), and the repo's Loop parameters block copied verbatim
   so the reviewer judges against the right yardstick.
3. Submit as a BACKGROUND task — reviews routinely exceed 10 minutes; never
   wait in the foreground:
   `cat C:/Projects/yaaadabi/reviewer.md $TEMP/handover.md | codex exec -s read-only --json -o $TEMP/verdict.md -`
   The JSONL event stream is the heartbeat: a running review with no new
   output for ~15 minutes is hung — kill it and resubmit via
   `codex exec resume --last`.
4. Read the verdict; report it in the conversation, then continue
   immediately.
5. On NEEDS-WORK: address every [blocking] finding (suggestions at your
   judgment — state what you did with each), or push back with reasons
   grounded in the project docs. Re-review in the same session:
   `codex exec resume --last --json -o $TEMP/verdict.md - < $TEMP/response.md`
   (background + heartbeat rules as in step 3)
   After 3 rounds without approval: stop, summarize both positions, hand
   to the human.
6. On APPROVED: re-run the Verify commands on the final state, then commit
   (per the repo's commit conventions) and report: task, rounds, verdict,
   commit hash.
