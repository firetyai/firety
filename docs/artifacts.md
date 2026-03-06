# Firety Artifact Workflows

This workflow is currently experimental and hidden from default help.

Firety artifacts are first-class saved outputs. They are meant to support offline review, CI summaries, PR comments, debugging, and later hosted/reporting flows without rerunning analysis.

For a higher-level shareable bundle built from those artifacts, see [docs/evidence-packs.md](evidence-packs.md).
For reproducibility and comparability checks across artifacts, packs, and trust reports, see [docs/provenance.md](provenance.md).
For evidence age and recertification checks on saved outputs, see [docs/freshness.md](freshness.md).

## Commands

Inspect an artifact:

```bash
firety artifact inspect ./lint-artifact.json
firety artifact inspect ./eval-artifact.json --format json
firety artifact inspect ./attestation.json
firety artifact inspect ./readiness.json
firety artifact inspect ./workspace-scope.json
firety artifact inspect ./workspace-report.json
```

Render an artifact into an existing report style:

```bash
firety artifact render ./analysis-artifact.json --render pr-comment
firety artifact render ./gate-artifact.json --render ci-summary
firety artifact render ./attestation.json --render full-report
firety artifact render ./readiness.json --render full-report
firety artifact render ./workspace-scope.json --render ci-summary
firety artifact render ./workspace-report.json --render ci-summary
firety artifact render ./benchmark-artifact.json --render full-report
```

Compare two compatible artifacts:

```bash
firety artifact compare ./baseline-lint.json ./candidate-lint.json
firety artifact compare ./base-eval.json ./candidate-eval.json --format json
firety artifact compare ./base-multi.json ./candidate-multi.json
```

Inspect provenance or comparability without rerunning analysis:

```bash
firety provenance inspect ./lint-artifact.json
firety provenance inspect ./evidence-pack
firety provenance inspect ./trust-report
firety provenance compare ./before-eval.json ./after-eval.json
```

## What inspection shows

Artifact inspection is intended to answer:

- what kind of artifact this is
- which schema version it uses
- which Firety workflow produced it
- what target, profile, strictness, backend, or suite context it carries
- which render and compare operations Firety supports for it

## Supported render flows

The first version supports artifact-driven rendering for Firety artifact types that already have reviewer-facing reports, including:

- lint
- lint compare
- eval
- eval compare
- multi-backend eval
- multi-backend eval compare
- analysis
- improvement plan
- quality gate
- readiness
- workspace change scope
- workspace report
- baseline snapshot
- baseline compare
- compatibility
- attestation
- benchmark report

Artifact rendering reuses Firety's existing presentation layer. It does not rerun lint, eval, compare, or gate logic.

## Supported compare flows

The first version intentionally supports a small explicit comparison matrix:

- lint artifact vs lint artifact
- single-backend eval artifact vs single-backend eval artifact
- multi-backend eval artifact vs multi-backend eval artifact

If two artifacts are not compatible, Firety returns a clear runtime error instead of silently guessing.

## Validation behavior

Artifact commands validate:

- recognized `artifact_type`
- supported `schema_version`
- compatibility for the selected render or compare operation

Firety keeps this layer explicit and conservative. It does not invent missing analysis if the artifact does not contain the required data.

## Limitations

This first version intentionally does not include:

- an artifact database or history store
- cross-type compare for every possible artifact family
- automatic migration of old artifact schema versions
- cloud or hosted artifact management

Those can be added later on top of the artifact contracts that already exist.
