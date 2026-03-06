# Firety Lint Artifact

Firety can write a versioned machine-readable lint artifact for future consumers such as SaaS ingestion, PR rendering, dashboards, local report generation, and regression tooling.

## Purpose

The artifact is a stable product contract for a lint run. It is separate from Firety's terminal-focused output formats:

- `--format text` stays human-readable
- `--format json` stays a lightweight stdout-oriented automation format
- `--format sarif` stays a SARIF interoperability format
- `--artifact <path>` writes the richer versioned artifact to a file
- compare mode can also write a versioned compare artifact for diff-style review workflows
- `firety skill baseline save --output <path>` writes a sibling saved-baseline snapshot artifact for explicit accepted references
- `firety skill baseline compare --artifact <path>` writes a sibling baseline-compare artifact for regression workflows against saved snapshots
- `firety skill compatibility --artifact <path>` writes a sibling compatibility artifact with support posture, profile summaries, backend summaries, blockers, and strengths
- `firety skill eval-compare --backend ... --artifact <path>` writes a sibling multi-backend compare artifact with per-backend version deltas and disagreement changes
- `firety skill analyze --artifact <path>` writes a sibling combined analysis artifact that includes lint, eval, and correlation data
- `firety skill eval --backend ... --artifact <path>` writes a sibling multi-backend eval artifact with per-backend measured results
- `firety skill plan --artifact <path>` writes a sibling improvement-plan artifact with prioritized remediation items and supporting evidence
- `firety skill gate --artifact <path>` writes a sibling quality-gate artifact with a deterministic PASS or FAIL decision plus the selected policy criteria and blocking reasons
- `firety skill render <artifact> --render pr-comment|ci-summary|full-report` renders those artifacts into reviewer-friendly summaries without re-running analysis

## Versioning

- current `schema_version`: `"1"`
- changes should be additive when possible
- consumers should branch on `schema_version` rather than assuming the format is unversioned

## Top-level shape

The single-run lint artifact currently includes:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `summary`
- `findings`
- `routing_risk`
- `action_areas`
- `rule_catalog`
- `applied_fixes`
- `fingerprint`

For compare mode, Firety writes a sibling artifact type with:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `base`
- `candidate`
- `comparison`
- `added_findings`
- `removed_findings`
- `changed_findings`
- `category_deltas`
- optional `routing_risk_delta`
- `rule_catalog`
- `fingerprint`

For combined lint/eval analysis mode, Firety writes another sibling artifact type with:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `lint`
- `eval`
- `correlation`
- `fingerprint`

For improvement-plan mode, Firety writes another sibling artifact type with:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `lint_summary`
- optional `eval_summary`
- optional `multi_backend_eval`
- optional `correlation`
- `routing_risk`
- `action_areas`
- `plan`
- `fingerprint`

For multi-backend eval compare mode, Firety writes another sibling artifact type with:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `suite`
- `backends`
- `base`
- `candidate`
- `aggregate_summary`
- `per_backend_deltas`
- optional `differing_cases`
- optional `widened_disagreements`
- optional `narrowed_disagreements`
- `fingerprint`

For quality-gate mode, Firety writes another sibling artifact type with:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `result`
- `fingerprint`

For baseline snapshot mode, Firety writes another sibling artifact type with:

- `schema_version`
- `artifact_type`
- `tool`
- `snapshot`
- `fingerprint`

For baseline compare mode, Firety writes another sibling artifact type with:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `comparison`
- `fingerprint`

For compatibility mode, Firety writes another sibling artifact type with:

- `schema_version`
- `artifact_type`
- `tool`
- `run`
- `report`
- `fingerprint`

## Important fields

### `tool`

Minimal metadata about the Firety binary that produced the artifact:

- `name`
- `version`
- `commit`
- `build_date`

### `run`

Captures the lint posture used for the run:

- `target`
- `profile`
- `strictness`
- `fail_on`
- `explain`
- `fix`
- `exit_code`
- `stdout_format`

### `summary`

Captures the overall result relevant to later rendering and policy evaluation:

- `valid`
- `passes_fail_policy`
- `error_count`
- `warning_count`
- `finding_count`
- `applied_fix_count`
- `severity_counts`
- optional summary notes such as strictness or portability guidance

### `findings`

Each finding includes the stable fields needed for later rendering:

- `rule_id`
- `severity`
- `category`
- `path`
- `line`
- `message`
- rule-derived metadata such as `fixability`, `profile_aware`, and `line_aware`
- explain-mode metadata when `--explain` is enabled, such as:
  - `why_it_matters`
  - `what_good_looks_like`
  - `improvement_hint`
  - `guidance_profile`
  - `profile_specific_hint`
  - `targeting_posture`

### `routing_risk`

When `--routing-risk` is requested, the artifact can include a derived routing-risk summary with:

- `overall_routing_risk`
- `summary`
- `risk_areas`
- `priority_actions`

This section is derived from existing trigger-quality, consistency, examples, and portability findings. It is intended to help later renderers answer "why this may not trigger" without reimplementing Firety's grouping logic.

### `rule_catalog`

The artifact includes the referenced subset of Firety's rule catalog so later systems can render findings without re-querying the CLI:

- `id`
- `slug`
- `category`
- `default_severity`
- strictness severity overrides when present
- `title`
- `description`
- `profile_aware`
- `line_aware`
- `fixability`

## Determinism

The artifacts are designed to be deterministic for a given lint result or compare result:

- no timestamps are included in schema version `1`
- ordering follows Firety's stable finding and rule ordering
- `fingerprint` is derived from stable run/result content to help with later comparison or deduplication

## Compare artifacts

Compare artifacts are intended for workflows such as:

- PR review summaries
- historical regressions or improvements
- dashboards that track lint quality over time
- future SaaS/reporting layers that should not need to re-run Firety just to render a diff

Firety's render command is intentionally artifact-first: it treats these artifacts as the stable product surface for PR comments, CI summaries, and fuller local reports.

Analysis artifacts are intended for workflows such as:

- PR summaries that connect measured misses to likely lint contributors
- hosted reporting that wants one combined skill-quality payload
- dashboards that track routing risk, eval misses, and likely improvement priorities together

Quality-gate artifacts are intended for workflows such as:

- CI pass/fail enforcement that wants a stable machine-readable decision record
- release checks that need explicit blocking reasons rather than raw analyzer output
- future PR or hosted report layers that should render gate decisions without re-running Firety

Baseline snapshot artifacts are intended for workflows such as:

- storing an explicit accepted quality reference in a repository or release pipeline
- comparing later skill revisions against a known-good snapshot without supplying a second live path
- baseline-aware quality gating in CI

Compare mode analyzes Firety's own lint outputs for a base and candidate skill directory. It does not attempt to diff raw markdown semantically or predict runtime behavior directly.

## Intentionally excluded for now

Firety intentionally does not include:

- machine-specific environment dumps
- usernames, hostnames, or secret-bearing environment data
- raw file contents
- SaaS-specific IDs
- PR-comment rendering markup
- HTML-ready presentation data
- historical comparison state

Those can be added later in additive ways without changing the core artifact purpose.
