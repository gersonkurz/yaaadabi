# TODO

Findings deferred from review rounds, recorded verbatim (reviewer's wording).

- 2026-09-03 **[task]** [protocol.md](C:/Projects/yaaadabi/protocol.md:96)
  pre-existingly omits `-s read-only` from every resumed review. The executed
  probe’s second turn consequently ran with `workspace-write`, confirming that
  resume uses the new invocation’s sandbox configuration rather than preserving
  the initial one. A round-two reviewer can therefore modify the working tree,
  contradicting [README.md](C:/Projects/yaaadabi/README.md:164). This is
  deferrable because the omission already existed in the old `resume --last`
  command and fixing sandbox propagation was not the task that opened this loop.

  Developer note (not the reviewer's words, 2026-09-03): the remedy is not
  `-s read-only` — `codex exec resume` has no `-s`/`--sandbox` option and
  rejects it with `error: unexpected argument '-s' found`. Executed on
  codex-cli 0.153.0: `-c sandbox_mode="read-only"` is accepted on resume and
  is enforced (the resumed turn's attempt to create a file in the repo failed
  with `Access to the path ... is denied` and no file appeared).
