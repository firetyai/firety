# Firety Readiness Workflows

Firety readiness mode turns existing quality evidence into a publish decision for a specific context.

It is meant to answer:

- can this skill be published now
- what is blocking publication or release
- which issues are hard blockers versus caveats
- whether saved evidence is still current enough for the intended use
- what should be rerun or fixed next

## Command

Run readiness from a fresh target:

```bash
firety readiness check ./path/to/skill
firety readiness check ./path/to/skill --context merge --runner ./routing-runner
firety readiness check ./path/to/skill --context public-release --backend codex=./codex-runner --backend cursor=./cursor-runner
firety readiness check ./path/to/skill --context public-attestation --format json --artifact ./readiness.json
```

Run readiness from saved Firety outputs:

```bash
firety readiness check --context public-attestation --input-artifact ./attestation.json
firety readiness check --context public-trust-report --input-pack ./evidence-pack
firety readiness check --context release-candidate --input-report ./trust-report
```

## Publish contexts

Firety supports a small explicit set of contexts:

- `internal`
- `merge`
- `release-candidate`
- `public-release`
- `public-attestation`
- `public-trust-report`

These contexts are intentionally opinionated and deterministic. Public contexts care more about freshness, evidence completeness, and publishable support posture than internal or merge contexts.

## Decisions

Firety emits one of four decisions:

- `ready`
- `ready-with-caveats`
- `not-ready`
- `insufficient-evidence`

Interpretation:

- `ready` means the current Firety evidence supports the selected context without major caveats
- `ready-with-caveats` means the skill can move forward only if the caveats are communicated honestly
- `not-ready` means Firety found blocking quality or freshness issues
- `insufficient-evidence` means Firety does not have enough current evidence to make a trustworthy decision

## What readiness uses

Readiness reuses existing Firety evidence instead of inventing new analysis:

- quality gate results
- compatibility and support-posture summaries
- attestation suitability
- freshness and recertification status
- existing saved artifacts, evidence packs, and trust reports when supplied

## Blockers vs caveats

Readiness keeps these separate:

- blockers are issues that stop the selected publish context
- caveats are limitations that should be communicated or reviewed, but do not automatically block
- improvement opportunities are non-blocking ways to strengthen the skill

Examples:

- stale or incomplete evidence can block public publication contexts
- a failing quality gate blocks merge and release-oriented contexts
- mixed support posture may still allow publication, but only with caveated support claims

## Artifact output

`--artifact <path>` writes a versioned `firety.skill-readiness` artifact.

That artifact can later be:

- inspected with `firety artifact inspect`
- rendered with `firety artifact render`
- checked for provenance with `firety provenance inspect`
- checked for freshness with `firety freshness inspect`

## Limitations

The first version intentionally does not include:

- a configurable publish policy language
- project-wide readiness configuration
- hidden business logic about what a team should publish
- guarantees that `ready` means production success

It is a conservative publish-decision layer on top of Firety's existing evidence, not a release-management platform.
