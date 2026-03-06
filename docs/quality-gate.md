# Firety Quality Gate

Firety's quality gate turns existing lint, eval, compare, and multi-backend evidence into a deterministic PASS or FAIL decision for CI, code review, and release workflows.

## Purpose

Use `firety skill gate` when you need Firety to answer:

- should this skill change pass the selected quality bar?
- which explicit criterion caused failure?
- what evidence supports that decision?

The gate is intentionally explicit. Firety does not expose a policy DSL or hidden scoring engine in this version.

## Usage

Gate the current skill directory with fresh evidence:

```bash
firety skill gate
firety skill gate ./path/to/skill
firety skill gate ./path/to/skill --runner ./routing-runner
firety skill gate ./path/to/skill --backend codex=./codex-runner --backend cursor=./cursor-runner
```

Gate compare-aware evidence against a base version:

```bash
firety skill gate ./after --base ./before
firety skill gate ./after --base ./before --runner ./routing-runner --max-pass-rate-regression 0
firety skill gate ./after --base ./before --backend codex=./codex-runner --backend cursor=./cursor-runner --max-widened-disagreements 0
```

Gate from saved artifacts without rerunning analysis:

```bash
firety skill gate --input-artifact ./lint-artifact.json
firety skill gate --input-artifact ./analysis-artifact.json --max-false-positives 0
firety skill gate --input-artifact ./lint-compare-artifact.json --input-artifact ./eval-compare-artifact.json --fail-on-new-errors --max-pass-rate-regression 0
```

Write a machine-readable gate artifact:

```bash
firety skill gate ./path/to/skill --artifact ./gate-artifact.json
firety skill gate --input-artifact ./analysis-artifact.json --format json --artifact ./gate-artifact.json
```

## Default behavior

The default gate is intentionally modest and operational:

- if lint evidence is present, Firety blocks on lint errors
- if eval evidence is present, Firety requires a 100% pass rate
- if multi-backend eval evidence is present, Firety requires a 100% pass rate on each backend

Compare-aware failures are opt-in. Firety does not fail a gate on regressions versus base unless you select regression-sensitive criteria.

## Supported criteria

Current criteria are explicit command flags, not a general policy language:

- `--max-lint-errors`
- `--max-lint-warnings`
- `--max-routing-risk low|medium|high`
- `--min-eval-pass-rate`
- `--max-false-positives`
- `--max-false-negatives`
- `--min-per-backend-pass-rate`
- `--max-backend-disagreement-rate`
- `--max-pass-rate-regression`
- `--max-false-positive-increase`
- `--max-false-negative-increase`
- `--max-widened-disagreements`
- `--fail-on-new-errors`
- `--fail-on-new-portability-regressions`

Percentage-based CLI flags use `0` to `100`. Firety normalizes those to internal fractional values in machine-readable outputs.

## Output

Text output is decision-oriented:

- overall decision: `PASS` or `FAIL`
- concise summary
- blocking reasons
- optional warnings
- key supporting metrics
- per-backend results when relevant
- a short next action

JSON output adds a structured result with:

- `schema_version`
- run context
- selected `criteria`
- `decision`
- `blocking_reasons`
- `warnings`
- `supporting_metrics`
- optional `per_backend_results`
- optional `compare_context`

When `--artifact <path>` is used, Firety writes a versioned `firety.skill-quality-gate` artifact suitable for future PR rendering, dashboards, and hosted reports.

## Evidence rules

The gate only evaluates criteria that are supported by available evidence.

For example:

- `--min-eval-pass-rate` requires eval evidence
- `--max-pass-rate-regression` requires eval-compare evidence
- `--max-backend-disagreement-rate` requires current multi-backend evidence
- `--fail-on-new-portability-regressions` requires lint-compare evidence

If you select a criterion that Firety cannot evaluate from the supplied fresh runs or artifacts, Firety returns a runtime error instead of silently ignoring the criterion.

## Limitations

This first version intentionally does not include:

- a YAML or DSL policy language
- per-rule suppressions or team-specific config files
- cloud/SaaS policy management
- business-specific risk scoring
- hidden merge heuristics outside the selected criteria

The quality gate operationalizes Firety's existing evidence. It does not claim to know business impact or semantic runtime correctness beyond the underlying lint and eval inputs.
