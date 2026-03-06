# Firety Static Trust Reports

Firety trust reports turn existing Firety evidence into a small static report bundle that can be reviewed offline, attached to releases, or hosted on any static host such as GitHub Pages.

This workflow is presentation-focused. It does not invent new analysis. It packages and renders existing Firety evidence into a more public-facing, reviewer-friendly form.

## Command

Build a report from fresh analysis:

```bash
firety publish report ./path/to/skill --output ./trust-report
firety publish report ./path/to/skill --output ./trust-report --runner ./routing-runner --include-gate
firety publish report ./path/to/skill --output ./trust-report --backend codex=./codex-runner --backend cursor=./cursor-runner --include-plan
```

Build a report from existing Firety evidence:

```bash
firety publish report --input-artifact ./analysis-artifact.json --output ./trust-report
firety publish report --input-pack ./evidence-pack --output ./trust-report
firety publish report --input-artifact ./benchmark-artifact.json --output ./trust-report
```

## Output layout

The first version writes a deterministic directory with:

- `index.html`
- `manifest.json`
- `pages/`
- `evidence-pack/` for fresh or artifact-driven pack generation
- `evidence-packs/` when existing packs are copied in directly
- `artifacts/` for any derived report-local artifacts, such as a synthesized attestation

`index.html` is the primary entrypoint. It summarizes:

- support posture
- tested profiles and backends
- quality gate status when available
- support claims
- strengths
- limitations
- caution areas
- where to read next

## What pages are generated

The first version generates full-report pages for the renderable Firety artifacts present in the bundle, such as:

- attestation
- compatibility
- quality gate
- lint
- eval
- multi-backend eval
- improvement plan
- benchmark summary

Firety only renders what the bundle actually contains. It does not fabricate missing sections.

## How trust reports relate to other Firety workflows

- evidence packs remain the underlying review bundle
- attestation remains the support-claims source of truth when available
- artifact inspection and rendering remain the low-level offline tooling
- trust reports provide a cleaner public-facing entrypoint on top of those existing contracts

## Honesty and limits

Firety keeps trust reports conservative:

- it does not invent support claims not backed by evidence
- it does not hide mixed or weak evidence
- it does not add hosted-service behavior or client-side analysis
- it does not replace deeper artifact inspection for audit/debug workflows

The first version intentionally does not include:

- a frontend framework
- client-side JavaScript application behavior
- theming systems
- archive formats
- hosted publishing or syncing
- repo-wide multi-skill report sites
