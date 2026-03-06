# Firety Evidence Freshness and Recertification

Firety freshness is a lifecycle check for saved evidence. It helps answer:

- is this saved evidence still current enough to trust
- what parts of a pack, report, or attestation have gone stale
- what can still be reused for compare, baseline, publish, or release-claim workflows
- what should be rerun or regenerated now

Firety keeps this conservative. Freshness is not a claim that the evidence is correct, and stale does not mean the evidence is definitely wrong. It is a trust signal about whether the saved result is still current enough for its intended use.

## Command

Inspect freshness for a saved Firety output:

```bash
firety freshness inspect ./lint-artifact.json
firety freshness inspect ./evidence-pack
firety freshness inspect ./trust-report --max-report-age 48h
firety freshness inspect ./attestation.json --format json
```

## Statuses

Firety reports one of four statuses:

- `fresh`
- `usable-with-caveats`
- `stale`
- `insufficient-evidence`

Typical examples:

- `fresh`: the saved output is within the selected freshness threshold and no stale supporting evidence was found
- `usable-with-caveats`: the saved output is still usable, but missing provenance or partial dependency context limits confidence
- `stale`: the saved output or one of its key supporting inputs is older than the selected threshold
- `insufficient-evidence`: Firety cannot validate freshness well enough because supporting evidence is missing or incomplete

## Thresholds

The first version supports a small explicit threshold surface:

- `--max-age`
- `--max-eval-age`
- `--max-multi-eval-age`
- `--max-benchmark-age`
- `--max-attestation-age`
- `--max-report-age`

These are intentionally explicit duration-based controls instead of a general policy language.

## Intended-use suitability

Firety also reports whether saved evidence is still suitable for:

- compare reuse
- baseline reuse
- publish workflows
- release claims
- local debugging

This helps distinguish cases like:

- stale for release or publishing
- still usable for local debugging
- not suitable as a baseline even if still readable

## How freshness works

The first version relies on:

- saved file modification times
- Firety provenance
- explicit supporting artifact or pack references
- simple dependency-aware checks for packs, trust reports, and attestations

Firety does not claim exact environment replay or signed timestamp trust. It only uses the evidence it actually has.

## Recertification advice

When saved evidence is stale or caveated, Firety gives short operational advice such as:

- rerun lint
- rerun the routing eval suite
- rerun selected backend evals
- rebuild the evidence pack
- rebuild the trust report
- regenerate the attestation

## Limitations

This first version intentionally does not include:

- signed timestamps
- cloud or hosted freshness tracking
- project-wide recertification policies
- exact environment equivalence checks

It is a focused trust layer on top of Firety's saved artifacts, packs, reports, and attestations.
