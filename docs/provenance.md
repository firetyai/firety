# Firety Provenance and Reproducibility

Firety provenance is a focused reproducibility layer for saved artifacts, evidence packs, and trust reports. It is meant to answer:

- what command or workflow produced this result
- which profile, strictness, suite, and backend context mattered
- whether a saved result is suitable for baselines or later comparison
- whether two saved results are meaningfully comparable
- what should be rerun instead of reused directly

Firety keeps this conservative. It does not claim exact environment equivalence, and it does not compare saved outputs when the required context is missing or materially different.

## Commands

Inspect a saved Firety output:

```bash
firety provenance inspect ./lint-artifact.json
firety provenance inspect ./evidence-pack
firety provenance inspect ./trust-report --format json
```

Compare two saved Firety outputs:

```bash
firety provenance compare ./baseline-lint.json ./candidate-lint.json
firety provenance compare ./baseline-eval.json ./candidate-eval.json --format json
```

## What Firety captures

The first version captures only focused, non-secret context that materially affects reuse or comparison, such as:

- Firety version and commit
- command origin
- target path
- target fingerprint when Firety can compute one cleanly
- selected profile
- selected strictness
- fail policy
- suite identity
- selected backends
- artifact and pack dependencies
- comparability and reproducibility notes

Firety avoids noisy machine-specific environment dumps and does not capture secrets.

## Comparability statuses

Firety reports one of three statuses:

- `comparable`: the saved outputs carry matching required context
- `partially-comparable`: some required context is missing or caveated, so comparison is possible only with caution
- `not-comparable`: the saved outputs differ in ways that make direct comparison misleading, such as different suites or backend sets

Examples of explicit incompatibility:

- different eval suites
- different backend sets for multi-backend eval evidence
- different strictness or profile when those materially affect the output
- different object kinds or artifact families

## Evidence packs and trust reports

Evidence packs and trust reports now carry focused provenance in their `manifest.json` files. This helps reviewers understand:

- whether the bundle came from fresh analysis or preexisting artifacts
- which source artifacts or packs it depends on
- whether it is suitable for reuse in later compare or reporting workflows

## Limitations

This first version intentionally does not include:

- exact machine or dependency capture
- signed provenance
- schema migration for old saved outputs
- cloud or hosted provenance history

It is a conservative trust layer built on top of Firety's existing artifact contracts.
