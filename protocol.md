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
   NOT changed, an evidence statement (which changed paths have actually
   EXECUTED — test, selftest, probe — and which have only compiled and been
   read), the review scope stated explicitly (e.g. "review the
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
   immediately. Whatever the verdict, first record any [task] findings
   verbatim in the repo's task list, creating one (`TODO.md` at the repo
   root) if the repo has none — an APPROVED review can carry tasks too, and
   a finding that lives only in a review transcript or completion report is
   lost. This verbatim transcription of the reviewer's own findings is the
   one tree change permitted between approval and commit — its content was
   authored by the reviewer, so re-reviewing it adds a round and no
   information (human-blessed exemption, 2026-08-31).
5. On NEEDS-WORK: address every [blocking] finding (suggestions at your
   judgment — state what you did with each), or push back with reasons
   grounded in the project docs; [task] findings are already recorded per
   step 4 and are NOT acted on in this change. A [blocking]
   "needs an executed check" finding resolves one of two ways: execute the
   check, or — with the human's explicit acceptance, obtained via the
   only-the-human touchpoint — file a follow-up task in the repo's task
   list (created per step 4 if the repo has none) and state both the
   acceptance and the filed task in the response;
   the reviewer then treats it as a deferred blocker. If addressing
   findings makes the change materially more invasive (a file format, a
   migration, a new subsystem), stop the loop and hand the split decision
   to the human — rounds must narrow the change's scope, not expand it;
   added tests, probes, and executed checks are always in scope, whatever
   they do to the diff's line count. Re-review in the same session:
   `codex exec resume --last --json -o $TEMP/verdict.md - < $TEMP/response.md`
   (background + heartbeat rules as in step 3)
   After 3 rounds without approval: stop, summarize both positions, hand
   to the human.
6. On APPROVED: re-run the Verify commands on the final state, then commit
   (per the repo's commit conventions) and report: task, rounds, verdict,
   commit hash.
