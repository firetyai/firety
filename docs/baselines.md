# Firety Baseline Snapshots

Firety baseline snapshots let teams save an explicit accepted quality snapshot and compare later runs against that saved reference.

## Purpose

Use baseline snapshots when you need to answer:

- how does the current skill compare to our last accepted version?
- what regressed since we last approved a snapshot?
- what improved?
- should we update the baseline now?

Baseline workflows are intentionally explicit. Firety does not invent history or keep a hidden timeline in this version.

## Usage

Save a new baseline snapshot from a fresh run:

```bash
firety skill baseline save ./path/to/skill --output ./baseline.json
firety skill baseline save ./path/to/skill --runner ./routing-runner --suite ./path/to/skill/evals/routing.json --output ./baseline.json
firety skill baseline save ./path/to/skill --backend codex=./codex-runner --backend cursor=./cursor-runner --output ./baseline.json
```

Save a baseline from existing Firety artifacts:

```bash
firety skill baseline save --output ./baseline.json --input-artifact ./lint-artifact.json
firety skill baseline save --output ./baseline.json --input-artifact ./analysis-artifact.json
```

Compare the current skill against a saved baseline:

```bash
firety skill baseline compare ./path/to/skill --baseline ./baseline.json
firety skill baseline compare ./path/to/skill --baseline ./baseline.json --format json
firety skill baseline compare ./path/to/skill --baseline ./baseline.json --artifact ./baseline-compare.json
```

Intentionally update a saved baseline in place:

```bash
firety skill baseline update ./path/to/skill --baseline ./baseline.json
```

## What a baseline stores

Baseline snapshots store a saved Firety result context, not raw file contents.

The snapshot can include:

- lint summary and findings
- routing-risk summary
- optional single-backend eval results
- optional multi-backend eval results
- saved profile, strictness, suite, runner, and backend context when those were part of the snapshot

## Baseline compare behavior

Baseline compare is different from ad hoc `skill compare`:

- ad hoc compare uses two live directories
- baseline compare uses one live directory plus one explicit saved snapshot
- the saved baseline context is reused where possible so later comparisons stay aligned with the accepted reference

The comparison summary includes:

- overall improved, regressed, mixed, or unchanged
- lint deltas
- optional eval deltas
- optional multi-backend deltas
- top regressions to review
- notable improvements

## Baseline-aware gating

`firety skill gate` can also use a saved baseline:

```bash
firety skill gate ./path/to/skill --baseline ./baseline.json
firety skill gate ./path/to/skill --baseline ./baseline.json --fail-on-new-errors
firety skill gate ./path/to/skill --baseline ./baseline.json --fail-on-routing-risk-regression
```

This is useful when CI should fail on regressions from a previously accepted snapshot without requiring a second live checkout path.

## Limitations

This first version intentionally does not include:

- a built-in history database
- automatic baseline rotation
- cloud or SaaS baseline storage
- project-wide baseline config files
- hidden compatibility fallbacks when a baseline context is incomplete

If Firety cannot cleanly compare against a saved baseline context, it returns a clear runtime error instead of guessing.
