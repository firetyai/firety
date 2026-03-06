# Firety Evidence Packs

Firety evidence packs package saved quality evidence into a deterministic directory for offline review, CI artifacts, release attachments, and future hosted/reporting workflows.

## Command

Build a pack from fresh analysis:

```bash
firety evidence pack ./path/to/skill --output ./evidence-pack
firety evidence pack ./path/to/skill --output ./evidence-pack --runner ./routing-runner --include-plan --include-compatibility --include-gate
```

Build a pack from existing artifacts:

```bash
firety evidence pack --input-artifact ./lint-artifact.json --output ./evidence-pack
firety evidence pack --input-artifact ./analysis-artifact.json --input-artifact ./gate-artifact.json --output ./evidence-pack
```

## Directory layout

The first version writes a deterministic directory with:

- `manifest.json`
- `SUMMARY.md`
- `artifacts/`
- `reports/`

`manifest.json` records what the pack contains, what Firety context produced it, and which files a reviewer should open first.

`SUMMARY.md` is the human entrypoint for the pack. It points reviewers to the highest-value reports first and lists the included artifacts and rendered summaries.

## Pack contents

The first version intentionally keeps the content set modest:

- selected versioned Firety artifacts
- rendered CI summaries for renderable artifact types
- rendered full reports for renderable artifact types
- one top-level pack summary and one manifest

Fresh packs always include a lint artifact. They can also include:

- routing eval evidence when `--runner` is used
- multi-backend eval evidence when `--backend` is used
- improvement plan when `--include-plan` is selected
- compatibility summary when `--include-compatibility` is selected
- quality gate summary when `--include-gate` is selected

## Validation and compatibility

Firety validates supplied artifacts before packaging them.

The first version also checks for obviously incompatible artifact inputs, such as artifacts that point at different current targets in the same pack.

Firety keeps this conservative:

- no hidden reruns when artifact inputs are sufficient
- no guessed compatibility when artifact context conflicts
- no automatic schema migration

## Limitations

This first version intentionally does not include:

- archive formats beyond deterministic directory output
- a pack database or history store
- package signing
- cloud upload or hosted report publishing
- every possible intermediate artifact by default

It is meant to be a durable, reviewable packaging layer built on Firety's existing artifact and rendering contracts.
