# Contributing

## Principles

Keep the codebase explicit, small, and easy to change.

- prefer straightforward code over clever abstractions
- keep CLI commands thin
- add interfaces only when they improve testability or support a real adapter boundary
- avoid vendor-specific behavior in shared layers
- write tests with each change

## Local workflow

```bash
make tidy
make fmt
make lint
make test
make test-race
make build
```

Use `make precommit` before opening a pull request.

## Coding standards

- standard library first; add dependencies only with clear justification
- package names should be short and descriptive
- comments should explain intent, not restate code
- exported APIs should stay stable and predictable once introduced
- prefer constructor functions for top-level wiring

## Testing expectations

Every new behavior should include the right level of tests:

- unit tests for domain and service logic
- command-level tests for CLI behavior
- focused helpers in `internal/testutil` when patterns repeat
- regression coverage for the curated lint benchmark corpus when rule behavior changes

## Lint benchmark corpus

Firety maintains a small curated skill corpus to protect linter quality, not just correctness.

- keep fixture intent explicit so reviewers can see what each benchmark represents
- prefer durable invariants such as required rule IDs, severity ranges, routing-risk areas, and forbidden noisy findings
- avoid giant brittle snapshots when a smaller invariant expresses the real quality expectation

When changing lint rules or heuristics:

1. Run the benchmark regression tests first.
2. Review whether the changed findings improve or degrade signal on the curated fixtures.
3. Update fixture expectations only when the new behavior is intentionally better.
4. Keep good fixtures low-noise and intentionally targeted fixtures honest rather than artificially warning-free.

For public-facing benchmark review:

1. Run `firety benchmark run` to confirm the built-in corpus still passes and the summary stays clean.
2. If you need a reviewable saved result, write `--artifact` and render it with `firety benchmark render`.
3. Treat benchmark artifact or summary changes as product-surface changes, not just test churn.

When adding new commands:

- validate args in the command
- delegate behavior to an application service
- test the command through the public command tree

When adding new adapters:

- keep them in `internal/platform`
- keep adapter concerns out of `internal/domain`
- use interfaces only at the seam that needs replacement in tests or alternate implementations

## Pull requests

Before submitting a PR:

1. Run `make precommit`.
2. Add or update tests for the behavior you changed.
3. Update documentation when the architecture or developer workflow changes.

Small, focused PRs are strongly preferred.
