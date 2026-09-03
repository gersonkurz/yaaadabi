# TODO

Findings deferred from review rounds, recorded verbatim (reviewer's wording).

- 2026-09-03 [task] README.md:74 pre-existingly says the wiring tool leaves
  "two placeholder lines," while main.go:47 emits three parameter lines
  containing four placeholders. A user following the quick start may leave
  parameters unresolved. This predates the current diff and fixing the
  placeholder-count documentation is separable from the six protocol
  proposals, so it is deferrable.
