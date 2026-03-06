# Firety Benchmark Reporting

Firety includes a built-in benchmark reporting surface for its own curated benchmark corpus.

## Purpose

The benchmark commands help Firety demonstrate that:

- the built-in benchmark corpus is runnable and stable
- benchmark invariants still hold for the current Firety build
- low-noise behavior on strong fixtures is preserved over time
- benchmark results can be reviewed locally or published later from saved artifacts

This first version is intentionally focused on Firety's own built-in lint benchmark corpus. It does not benchmark arbitrary user projects and it does not yet model broader hosted/public benchmark infrastructure.

## Commands

Run the built-in benchmark corpus:

```bash
firety benchmark run
firety benchmark run --format json
firety benchmark run --artifact ./benchmark-artifact.json
```

Render a saved benchmark artifact:

```bash
firety benchmark render ./benchmark-artifact.json --render pr-comment
firety benchmark render ./benchmark-artifact.json --render ci-summary
firety benchmark render ./benchmark-artifact.json --render full-report
```

## What the benchmark covers

The built-in benchmark corpus includes representative fixtures such as:

- strong portable skills
- structurally broken skills
- vague or generic skills
- diffuse or overbroad skills
- portability-conflicted skills
- bundle/resource problem skills
- cost/bloat-heavy skills
- weak example-quality skills
- well-signaled intentional tool-specific skills
- accidentally tool-locked skills

Each fixture carries explicit intent and stable regression expectations so maintainers can review whether Firety's behavior is improving or becoming noisier.

## Benchmark result model

Benchmark runs produce a structured result with:

- suite identity and version
- fixture-level pass/fail results
- category summaries
- deterministic/stability signals
- notable regressions
- notable noisy findings
- summary fields suitable for later rendering and history tracking

## Benchmark artifact

When `--artifact <path>` is used, Firety writes a versioned benchmark artifact with:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `suite`
- `summary`
- `categories`
- `fixtures`
- `fingerprint`

Current benchmark artifact details:

- `artifact_type`: `firety.benchmark-report`
- `schema_version`: `1`
- the artifact is deterministic for a given benchmark result
- the artifact intentionally excludes machine-specific or secret-bearing environment details

## Rendering philosophy

Benchmark renderers are presentation-only:

- they consume saved benchmark artifacts
- they do not rerun lint logic
- they keep output concise and reviewer-friendly
- they are intended to support PR comments, CI summaries, and fuller local/public reports later

## Limitations

The first version intentionally does not include:

- MCP benchmarking
- arbitrary user-project benchmark suites
- cloud/SaaS publication
- historical storage or dashboards
- website or HTML frameworks
- measured routing-eval benchmark corpora

Those can build later on top of the benchmark result and artifact contracts introduced here.
