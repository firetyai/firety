# Firety Skill Attestation

Firety attestation mode turns existing Firety evidence into a conservative support-claims manifest for releases, CI bundles, and maintainer-facing documentation.

It is designed to answer:

- what this skill can credibly claim to support
- what was actually tested
- which quality-gate decision backed the release
- which limitations or caution areas remain
- where a reviewer should go next for deeper evidence

## Commands

Generate an attestation from fresh analysis:

```bash
firety skill attest ./path/to/skill
firety skill attest ./path/to/skill --runner ./routing-runner --include-gate
firety skill attest ./path/to/skill --backend codex=./codex-runner --backend cursor=./cursor-runner --artifact ./attestation.json
```

Generate an attestation from existing Firety evidence:

```bash
firety skill attest --input-artifact ./compatibility.json --input-artifact ./gate.json
firety skill attest --input-pack ./evidence-pack
firety skill attest --input-artifact ./compatibility.json --format json --artifact ./attestation.json
```

## What attestation says

The first version keeps the model intentionally small and explicit. It includes:

- support posture
- evidence level
- supported profiles
- tested profiles
- supported backends
- tested backends
- quality-gate summary when available
- conservative support claims
- known limitations and caution areas
- evidence references
- recommended reading order

## Tested vs supported

Firety keeps these separate on purpose:

- supported means Firety's existing lint, portability, and compatibility evidence suggests the maintainer can credibly position the skill that way
- tested means Firety has measured routing evidence for those profiles or backends

If only one side is available, Firety says so rather than collapsing them into the same statement.

## Evidence sources

Attestation can reuse existing Firety evidence such as:

- compatibility artifacts
- quality-gate artifacts
- routing eval artifacts
- multi-backend routing eval artifacts
- analysis artifacts
- lint artifacts
- evidence packs that contain those artifacts

Firety does not invent missing evidence. If the supplied artifacts are partial, the attestation will reflect that with weaker claims and explicit caution areas.

## Human and machine outputs

- `--format text` prints a concise human-readable support statement
- `--format json` prints a structured manifest-friendly view
- `--artifact <path>` writes a versioned attestation artifact for later inspection and rendering

Attestation artifacts work with the existing artifact-first workflows:

```bash
firety artifact inspect ./attestation.json
firety artifact render ./attestation.json --render ci-summary
firety artifact render ./attestation.json --render full-report
```

## Limitations

This first version intentionally does not include:

- cryptographic signing
- supply-chain or provenance guarantees
- repo-wide support statements across multiple skills
- cloud or hosted attestation management
- support claims that go beyond Firety's actual measured or static evidence

It is a disciplined support statement workflow, not a marketing generator.
