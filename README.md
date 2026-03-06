# firety

`firety` is an open-source Go CLI for testing and comparing reusable agent capabilities across tools.

Current scope is intentionally small:

- project scaffolding plus the first real skill linter
- no business logic yet
- no YAML configuration parsing yet
- no tool-specific integrations yet

The goal of this foundation is to make future growth predictable without turning the codebase into a framework.

## Current commands

```text
firety artifact
firety skill lint [path]
firety skill baseline
firety skill compatibility [path]
firety skill plan [path]
firety skill analyze [path]
firety skill eval [path]
firety skill eval-compare <base> <candidate>
firety skill gate [path]
firety skill compare <base> <candidate>
firety skill render <artifact>
firety artifact inspect <artifact>
firety artifact render <artifact>
firety artifact compare <base-artifact> <candidate-artifact>
firety skill rules
firety benchmark run
firety benchmark render <artifact>
firety evidence pack [path]
firety mcp
firety agent
firety version
```

`firety skill lint` is the first implemented feature. Other command areas remain placeholders so application services, adapters, and reporting features can be added incrementally without rewriting the CLI foundation.

## Skill lint

Lint a local skill directory:

```bash
firety skill lint
firety skill lint ./path/to/skill
firety skill lint ./path/to/skill --format json
firety skill lint ./path/to/skill --format sarif > firety.sarif
firety skill lint ./path/to/skill --fail-on warnings --quiet
firety skill lint ./path/to/skill --profile codex
firety skill lint ./path/to/skill --fix
firety skill lint ./path/to/skill --explain
firety skill lint ./path/to/skill --strictness strict
firety skill lint ./path/to/skill --artifact ./lint-artifact.json
firety skill lint ./path/to/skill --routing-risk
firety skill baseline save ./path/to/skill --output ./baseline.json
firety skill baseline compare ./path/to/skill --baseline ./baseline.json
firety skill compatibility ./path/to/skill
firety skill compatibility ./path/to/skill --backend codex=./codex-runner --backend cursor=./cursor-runner
firety skill eval ./path/to/skill --runner ./routing-runner
firety skill eval ./path/to/skill --format json --artifact ./eval-artifact.json
firety skill eval ./path/to/skill --backend codex=./codex-runner --backend claude-code=./claude-runner
firety skill plan ./path/to/skill
firety skill plan ./path/to/skill --runner ./routing-runner
firety skill analyze ./path/to/skill --runner ./routing-runner
firety skill analyze ./path/to/skill --runner ./routing-runner --format json
firety skill eval-compare ./before ./after --runner ./routing-runner
firety skill compare ./before ./after
firety skill compare ./before ./after --format json
firety skill compare ./before ./after --routing-risk --artifact ./compare-artifact.json
```

List the authoritative lint rule catalog:

```bash
firety skill rules
firety skill rules --format json
```

Compare two versions of a skill directory:

```bash
firety skill compare ./before ./after
firety skill compare ./before ./after --format json
firety skill compare ./before ./after --profile codex --strictness strict --routing-risk
```

Measure routing behavior against a local eval suite:

```bash
firety skill eval ./path/to/skill --runner ./routing-runner
firety skill eval ./path/to/skill --runner ./routing-runner --format json
firety skill eval ./path/to/skill --runner ./routing-runner --artifact ./eval-artifact.json
firety skill eval ./path/to/skill --backend codex=./codex-runner --backend claude-code=./claude-runner
firety skill eval ./path/to/skill --backend codex --backend claude-code
```

Compare measured routing behavior for two skill versions:

```bash
firety skill eval-compare ./before ./after --runner ./routing-runner
firety skill eval-compare ./before ./after --runner ./routing-runner --format json
firety skill eval-compare ./before ./after --runner ./routing-runner --artifact ./eval-compare-artifact.json
firety skill eval-compare ./before ./after --backend codex=./codex-runner --backend cursor=./cursor-runner
```

Correlate static lint quality with measured routing misses:

```bash
firety skill analyze ./path/to/skill --runner ./routing-runner
firety skill analyze ./path/to/skill --runner ./routing-runner --profile codex --format json
firety skill analyze ./path/to/skill --runner ./routing-runner --artifact ./analysis-artifact.json
```

Build a short prioritized improvement plan:

```bash
firety skill plan ./path/to/skill
firety skill plan ./path/to/skill --runner ./routing-runner
firety skill plan ./path/to/skill --backend codex=./codex-runner --backend claude-code=./claude-runner
firety skill plan ./path/to/skill --format json --artifact ./plan-artifact.json
firety skill gate ./path/to/skill
firety skill gate ./path/to/skill --runner ./routing-runner
firety skill gate ./path/to/skill --base ./before --runner ./routing-runner --max-pass-rate-regression 0
firety skill gate ./path/to/skill --baseline ./baseline.json --fail-on-routing-risk-regression
firety skill gate --input-artifact ./analysis-artifact.json --max-false-positives 0
firety skill render ./analysis-artifact.json --render pr-comment
firety skill render ./compare-artifact.json --render ci-summary
firety skill render ./gate-artifact.json --render ci-summary
firety skill render ./plan-artifact.json --render full-report
firety artifact inspect ./analysis-artifact.json
firety artifact render ./analysis-artifact.json --render pr-comment
firety artifact compare ./before-lint.json ./after-lint.json
firety evidence pack ./path/to/skill --output ./evidence-pack
firety evidence pack --input-artifact ./analysis-artifact.json --output ./evidence-pack
firety benchmark run
firety benchmark run --format json --artifact ./benchmark-artifact.json
firety benchmark render ./benchmark-artifact.json --render ci-summary
```

If no path is provided, Firety lints the current directory.

Output formats:

- `--format text`: human-readable terminal output (default)
- `--format json`: machine-readable output for CI and automation
- `--format sarif`: SARIF 2.1.0 output for CI/code-scanning workflows

CI-friendly options:

- `--fail-on errors`: fail only when lint errors are present (default)
- `--fail-on warnings`: fail when lint errors or warnings are present
- `--profile generic|codex|claude-code|copilot|cursor`: portability profile for cross-tool linting
- `--strictness default|strict|pedantic`: choose the lint posture; `default` preserves Firety's current conservative baseline
- `--fix`: apply supported safe mechanical fixes before linting the final file state
- `--explain`: add deterministic rule-aware guidance about why findings matter and how to improve them
- `--artifact <path>`: write a versioned machine-readable lint artifact for later reporting, SaaS ingestion, or regression tooling
- `--routing-risk`: add a focused summary of why the skill may not trigger clearly and what to improve first
- `--quiet`: reduce text-mode output noise
- `--no-summary`: suppress the final summary line in text mode

Lint findings use stable rule IDs intended for automation and future integrations. Findings may also include line numbers when the linter can determine them reliably with its lightweight markdown scanning approach.

The rule catalog is also a first-class product surface:

- `firety skill rules` lists the full rule catalog in text form
- `firety skill rules --format json` exports the same catalog in a stable machine-readable form
- `firety skill baseline` manages explicit saved baseline snapshots for long-lived regression workflows
- `firety skill compatibility` summarizes support posture, portability, and backend health from Firety's existing evidence
- `firety skill gate` turns selected Firety evidence into a deterministic PASS/FAIL policy decision
- `firety skill render <artifact> --render pr-comment|ci-summary|full-report` turns existing artifacts into reviewer-friendly summaries without rerunning analysis
- `firety artifact inspect <artifact>` validates a saved artifact and explains what it represents
- `firety artifact render <artifact> --render pr-comment|ci-summary|full-report` renders a saved artifact without using the original live command path
- `firety artifact compare <base-artifact> <candidate-artifact>` compares compatible saved artifacts without rerunning analysis
- `firety evidence pack [path] --output <dir>` packages Firety artifacts and rendered summaries into a deterministic review bundle
- `firety benchmark run` turns Firety's built-in benchmark corpus into a structured maintainer/public quality summary
- `firety benchmark render <artifact> --render pr-comment|ci-summary|full-report` renders saved benchmark artifacts without rerunning the corpus
- artifact-first workflows are documented in [docs/artifacts.md](docs/artifacts.md)
- evidence-pack workflows are documented in [docs/evidence-packs.md](docs/evidence-packs.md)
- dedicated rule documentation lives in [docs/lint-rules.md](docs/lint-rules.md)
- the versioned lint artifact format is documented in [docs/lint-artifact.md](docs/lint-artifact.md)
- routing eval behavior and the local suite/runner format are documented in [docs/skill-eval.md](docs/skill-eval.md)
- baseline snapshot workflows are documented in [docs/baselines.md](docs/baselines.md)
- compatibility workflows are documented in [docs/compatibility.md](docs/compatibility.md)
- quality gate behavior is documented in [docs/quality-gate.md](docs/quality-gate.md)
- benchmark reporting is documented in [docs/benchmark-reporting.md](docs/benchmark-reporting.md)
- Firety also maintains a curated benchmark corpus and regression suite to protect lint quality over time

Autofix philosophy:

- Firety only autofixes deterministic, low-risk mechanical issues
- Firety does not rewrite descriptions, examples, guidance, portability wording, or other semantic content automatically
- in this version, the only supported automatic fix is `skill.missing-title`

Explain mode philosophy:

- Firety keeps explain mode deterministic, concise, and rule-driven
- explain output is derived from the rule catalog rather than generated dynamically
- explain mode can tailor portability guidance for the selected `--profile` when a finding is meaningfully profile-sensitive
- explain mode can also reflect the selected `--strictness` when stricter modes raise expectations for completeness or discipline
- Firety explains why a finding matters and what a better skill usually does, but it does not attempt to rewrite semantic content for the author

Routing-risk view:

- `--routing-risk` summarizes the most important trigger and routing weaknesses derived from existing lint findings
- it is heuristic, conservative, and deterministic
- it is intended to help authors understand why a skill may fail to trigger or route well
- it is not a substitute for real evaluation runs against actual agents or runtimes

Report rendering:

- `firety skill render` is a presentation-layer command built on Firety's versioned artifacts
- `firety artifact render` is the artifact-first companion for offline/reporting workflows where a saved artifact is the primary input
- `--render pr-comment` is optimized for compact PR review summaries
- `--render ci-summary` is optimized for CI job summaries and automation logs
- `--render full-report` is the default and produces a fuller local review report
- renderers reuse existing analysis and artifact data; they do not rerun lint or eval logic
- the first version is artifact-driven and intentionally plain markdown/text rather than HTML or a themed report system

Artifact workflows:

- `firety artifact inspect` explains artifact kind, schema version, origin, summary context, and supported render/compare operations
- `firety artifact render` produces the same reviewer-facing report styles from saved artifacts without rerunning analysis
- `firety artifact compare` currently supports lint artifacts, single-backend eval artifacts, and multi-backend eval artifacts
- artifact workflows fail clearly on unsupported schema versions, unsupported artifact kinds, or incompatible compare pairs

Evidence packs:

- `firety evidence pack [path] --output <dir>` builds a deterministic directory bundle with `manifest.json`, `SUMMARY.md`, packaged artifacts, and rendered reports
- `--input-artifact <path>` lets Firety assemble a pack from existing saved artifacts without rerunning analysis
- fresh packs always include lint evidence and can also include eval, plan, compatibility, and gate evidence when explicitly requested
- the first version is directory-first and intentionally avoids a more complex archive or publishing system

Benchmark reporting:

- `firety benchmark run` executes Firety's built-in benchmark corpus against the current Firety build
- benchmark reporting is intentionally focused on Firety's own curated corpus, not arbitrary user projects
- the first version is lint-benchmark focused; it demonstrates benchmark stability, low-noise behavior, and invariant coverage for the built-in fixtures
- benchmark artifacts are intended for future CI summaries, historical comparisons, and hosted/public benchmark pages without requiring those systems yet
- benchmark output is deterministic and reviewer-friendly rather than a large raw fixture dump

Quality gate:

- `firety skill gate` evaluates explicit selected criteria against Firety's existing lint, eval, compare, and multi-backend evidence
- default behavior is intentionally operational rather than hidden:
  - if lint evidence is present, the default gate blocks on lint errors
  - if eval evidence is present, the default gate requires a 100% pass rate
  - if multi-backend eval evidence is present, the default gate requires a 100% pass rate on each backend
- compare-aware thresholds are opt-in and include pass-rate regression, false-positive increase, false-negative increase, widened backend disagreements, new error findings, and new portability regressions
- `--baseline <path>` evaluates the current skill against a saved baseline snapshot instead of requiring a live `--base` directory
- `--fail-on-routing-risk-regression` blocks when routing risk worsens versus the selected base or saved baseline
- `--input-artifact <path>` allows artifact-based gating without rerunning analysis, as long as the selected criteria are supported by the supplied evidence
- PASS/FAIL is always tied to explicit selected criteria; Firety does not hide extra policy logic behind the command

Baseline snapshots:

- `firety skill baseline save [path] --output <file>` saves an explicit accepted Firety snapshot
- `firety skill baseline compare [path] --baseline <file>` compares the current skill against that saved snapshot
- `firety skill baseline update [path] --baseline <file>` intentionally refreshes the saved snapshot after review
- baseline snapshots capture the saved profile, strictness, suite, and backend context needed for later regression checks
- baseline workflows are meant for long-lived CI and release management, not as a general history store

Compatibility:

- `firety skill compatibility [path]` summarizes whether Firety sees the skill as generic-portable, intentionally tool-specific, mixed-ambiguous, accidentally-tool-locked, or weak-evidence
- the compatibility view is built from existing lint portability signals, routing-risk summaries, and optional measured backend results
- use `--backend` to add measured backend evidence to the compatibility matrix
- use `--input-artifact` to compute compatibility from existing Firety artifacts without rerunning analysis
- Firety keeps support-posture claims conservative; if evidence is incomplete or conflicting, the output says so

Strictness philosophy:

- `default` is the recommended baseline for most users and preserves Firety's current conservative behavior
- `strict` raises expectations for production-quality metadata, invocation guidance, example quality, and portability discipline
- `pedantic` is the most opinionated mode and can escalate additional completeness findings to errors
- strictness behavior is explicit and deterministic; Firety does not expose a generic policy matrix or per-rule configuration in this version

### Skill-spec linting

`firety skill lint` now checks both markdown structure and first-pass skill-spec quality:

- optional YAML front matter at the top of `SKILL.md`
- core front matter metadata quality for `name` and `description`
- structural markdown checks such as titles, links, and duplicate headings
- skill-quality guidance about when to use the skill, when not to use it, how to invoke it, and whether usage examples are actually useful
- conservative executable-example realism checks for concrete inputs, expected outcomes, bundle references, and realistic variation
- simple consistency checks between front matter metadata and the markdown body
- conservative portability checks for tool branding, install paths, tool-specific invocation conventions, and whether tool targeting appears intentional or accidental
- conservative bundle/resource checks for linked local files, mentioned helper resources, and stale bundle contents
- conservative token/cost-aware checks using approximate size estimates rather than exact model tokenizers
- conservative trigger-quality checks for distinctiveness, routing clarity, and cross-section trigger alignment

Front matter behavior:

- front matter is optional in this version
- if present, Firety parses it as YAML
- malformed front matter is a lint error
- when front matter exists, Firety validates `name` and `description`

Portability philosophy:

- intentionally tool-specific skills are acceptable when the target ecosystem is clear and the boundaries are honest
- Firety warns more aggressively when a skill claims to be generic or portable but behaves as though it is locked to one ecosystem
- mixed or contradictory ecosystem guidance is treated as a stronger signal than a single casual tool mention
- profile-specific explain guidance is heuristic and conservative; Firety does not claim to model the full real behavior of each tool ecosystem

### Current checks

Errors:

- target path does not exist
- target path is not a directory
- `SKILL.md` is missing
- `SKILL.md` cannot be read
- `SKILL.md` is empty
- malformed front matter
- missing or empty front matter `name`
- missing top-level markdown title
- broken local markdown links that point to missing files

Warnings:

- missing or empty front matter `description`
- suspiciously long front matter `name`
- front matter `description` that is too short, too long, or too vague
- front matter `name` or `description` that appears inconsistent with the body
- body scope that appears broader than the front matter suggests
- missing guidance about when to use the skill
- missing or weak negative guidance about limitations, boundaries, or when not to use the skill
- `SKILL.md` is very large
- duplicate headings
- no obvious examples section
- examples that exist but appear weak, generic, or missing a clear invocation pattern
- examples that appear abstract, placeholder-heavy, incomplete, low-variety, or disconnected from the documented guidance
- examples that reference missing local bundle resources or omit the trigger/result context needed to look practically usable
- no obvious usage or invocation guidance
- tool-specific branding or conventions that reduce cross-tool portability
- install paths or filesystem assumptions that look tied to one ecosystem
- profile-specific guidance that appears incompatible with the selected portability profile
- tool targeting that is unclear, accidentally locked to one ecosystem, or inconsistent with claims of portability
- mixed ecosystem guidance, mismatched examples, or tool-specific skills that fail to explain their intended audience and boundaries
- local resources that escape the skill root, point at directories, look stale, or appear suspiciously empty/unhelpful
- strongly-mentioned helper resources that are missing from the bundle
- helper/resource directories that appear stale or inconsistent with `SKILL.md`
- skill content that appears unnecessarily large, repetitive, or unbalanced for a single reusable skill
- referenced text resources that look expensive to load alongside `SKILL.md`
- skill names, descriptions, examples, or when-to-use guidance that are too generic or inconsistent to trigger cleanly
- suspicious relative paths
- very short content

Exit codes:

- `0`: no failing findings under the chosen `--fail-on` policy
- `1`: failing findings under the chosen `--fail-on` policy
- `2`: internal/runtime failure

With `--fix`, Firety applies supported fixes first, then lints the updated file state. Exit codes are based on the remaining findings after fixes are applied.

JSON output includes:

- `target`
- `valid`
- `error_count`
- `warning_count`
- `findings`

Each finding includes stable machine-readable fields such as `rule_id`, `severity`, `path`, `message`, and `line` when available.

With `--explain`, text output adds short rule-aware guidance for each finding plus a deterministic action-area summary. For portability-sensitive findings, Firety can also tailor short hints for `generic`, `codex`, `claude-code`, `copilot`, or `cursor` based on the selected `--profile`. `--format json --explain` adds explanation metadata such as `category`, `why_it_matters`, `what_good_looks_like`, `improvement_hint`, `guidance_profile`, and `profile_specific_hint` to each finding.

JSON output also includes the selected `strictness` so CI and automation can distinguish between baseline and opinionated lint runs.

With `--routing-risk`, text output adds a focused routing-risk section, and JSON/artifact output can include a structured routing-risk object with grouped risk areas and improvement priorities.

Artifact output:

- `--artifact <path>` writes a versioned lint artifact to a file without changing stdout behavior
- compare mode can also write a versioned compare artifact for later review, dashboards, or PR workflows
- the artifact is intended for future SaaS/reporting/PR/dashboard workflows
- schema version `1` is designed for additive evolution
- Firety intentionally excludes machine-specific noise and secret-bearing environment data from the artifact

Compare mode:

- `firety skill compare <base> <candidate>` compares Firety's own lint analysis for two skill directories
- it reports added findings, resolved findings, severity changes, category deltas, and routing-risk deltas when requested
- `improved`, `regressed`, `mixed`, and `unchanged` are derived from Firety's lint outputs rather than raw markdown diffs
- compare mode does not claim to measure semantic runtime quality; it compares Firety's lint judgments for reviewer and maintainer workflows

Routing eval:

- `firety skill eval` is separate from lint; it measures routing behavior against curated prompts instead of applying static document rules
- eval uses a small local JSON suite, by default at `evals/routing.json` inside the skill directory
- Firety sends each case to a local runner executable configured with `--runner` or `FIRETY_SKILL_EVAL_RUNNER`
- for cross-tool routing checks, Firety also supports repeated `--backend <id>[=/path/to/runner]`
- supported backend IDs are `generic`, `codex`, `claude-code`, `copilot`, and `cursor`
- multi-backend eval uses each backend's profile affinity instead of a shared `--profile`
- `firety skill analyze` runs both lint and measured routing evals, then adds a deterministic correlation layer that highlights likely contributors to false positives, false negatives, and profile-sensitive misses
- correlation output is heuristic and evidence-based; it is intended to help prioritize fixes, not to prove causality
- `firety skill plan` turns existing Firety evidence into a short prioritized remediation plan
- plan mode is lint-driven by default and adds richer measured evidence only when you pass `--runner` or `--backend`
- plan priorities are heuristic and evidence-based; they are meant to help choose what to fix first, not to prove causality or guarantee outcomes
- the runner returns a measured trigger / do-not-trigger decision for each case
- eval output reports pass rate, false positives, false negatives, breakdowns by profile/tag, and notable misses
- multi-backend eval also reports strongest/weakest backend, backend-specific misses, backend-specific strengths, and cases where backends disagree
- `firety skill eval-compare` runs the same suite against a base and candidate skill directory and compares measured results by case ID
- `firety skill eval-compare --backend ...` compares two versions across multiple selected backends and shows which backends improved, regressed, or widened disagreement
- eval compare reports pass-rate deltas, false positive/negative deltas, flipped cases, and profile/tag-specific regressions or improvements
- multi-backend eval compare adds backend-by-backend deltas plus widened or narrowed disagreement summaries
- exit codes for eval are:
  - `0` when all cases pass
  - `1` when one or more eval cases fail
  - `2` for invalid usage or backend/runtime failures
- `eval-compare` uses the candidate skill's measured result for exit-code purposes, so it stays aligned with the normal `skill eval` contract
- this first version is intentionally narrow: local subprocess runners, one JSON suite format, and no cloud/SaaS upload workflow yet

Benchmark corpus:

- Firety keeps a small curated corpus of representative skill fixtures in tests to protect lint signal quality over time
- the corpus covers strong, weak, vague, portability-problem, bundle-problem, cost-heavy, and example-quality scenarios
- regression tests assert durable invariants such as expected rule IDs, severity counts, routing-risk areas, and the absence of noisy findings on good skills
- maintainers should update corpus expectations intentionally when a rule change improves or degrades signal quality
- `firety benchmark run` uses that same built-in corpus to produce structured benchmark summaries and benchmark artifacts

Token/cost-aware findings use heuristic size estimates. Firety currently approximates likely token cost from content size; it does not yet use provider-specific tokenizers or pricing models.

Recommended CI usage:

```bash
# Fail only on errors, but get machine-readable output.
firety skill lint ./path/to/skill --format json

# Emit SARIF for code-scanning tools.
firety skill lint ./path/to/skill --format sarif > firety.sarif

# Treat warnings as failures in CI.
firety skill lint ./path/to/skill --fail-on warnings --format json

# Check that a skill stays portable for Codex users.
firety skill lint ./path/to/skill --profile codex --format json

# Ask for a stricter production-quality posture.
firety skill lint ./path/to/skill --strictness strict --format json

# Use the most opinionated mode for disciplined authoring teams.
firety skill lint ./path/to/skill --strictness pedantic --fail-on errors --format json

# Write a versioned artifact for later report rendering or ingestion.
firety skill lint ./path/to/skill --artifact ./lint-artifact.json --format text

# Ask Firety why the skill may not trigger cleanly.
firety skill lint ./path/to/skill --routing-risk --explain

# Keep text logs compact while still printing findings.
firety skill lint ./path/to/skill --fail-on warnings --quiet

# Get rule-aware guidance while reviewing findings locally.
firety skill lint ./path/to/skill --explain
```

Conservative local cleanup:

```bash
# Apply supported safe fixes, then lint the final file state.
firety skill lint ./path/to/skill --fix

# Apply safe fixes and consume the final report in JSON.
firety skill lint ./path/to/skill --fix --format json
```

The full catalog is documented in [docs/lint-rules.md](docs/lint-rules.md) and exported directly by `firety skill rules`. Rule IDs are stable and intended for CI, SARIF, and automation integrations.

This version intentionally does not check deeper skill semantics yet, such as:

- whether examples are semantically correct beyond lightweight string heuristics
- whether examples actually execute or produce the documented output
- whether semantic/content-quality issues can be rewritten safely without human judgment
- whether negative guidance is complete enough for every runtime or tool family
- whether the skill’s instructions are internally consistent beyond modest metadata/body checks
- whether a skill is truly executable across all supported tools beyond conservative wording/path checks
- whether the tool-specific terminology is truly correct for the targeted tool rather than only clearly signaled
- whether invocation guidance matches any external tool runtime
- whether referenced examples are semantically correct beyond file existence
- whether referenced scripts/resources actually execute correctly or are semantically correct
- whether every unreferenced bundle file is truly stale rather than intentionally optional
- exact tokenizer counts or provider-specific pricing/cost estimates
- exact model routing behavior or retrieval/ranking outcomes

## Quick start

```bash
make tidy
make build
./bin/firety version
make test
make rules-docs
```

## Architecture

The codebase uses a thin CLI layer over an application wiring layer:

- `cmd/firety`: process entrypoint only
- `internal/cli`: Cobra commands, help text, argument validation, and output formatting
- `internal/app`: top-level dependency assembly
- `internal/domain`: stable domain concepts that should remain independent from delivery details
- `internal/service`: future use cases and application services
- `internal/platform`: concrete runtime/build adapters
- `internal/testutil`: reusable helpers for tests

The main rule is simple: command files should stay thin. As behavior is added, commands should delegate to application services rather than accumulating business logic.

The skill linter follows that rule:

- `internal/cli` parses args and formats terminal output
- `internal/service` runs a small fixed set of lint checks
- `internal/domain/lint` defines structured findings and reports

More detail is in [docs/architecture.md](docs/architecture.md).

## Testing strategy

Testing is part of the project structure from day one:

- unit tests cover domain and service behavior directly
- command-level tests execute the Cobra command tree through its public interface
- shared helpers live in `internal/testutil` so test patterns stay consistent
- race detection and coverage are first-class make targets

Use these commands regularly:

```bash
make test
make coverage
make test-race
```

## Tooling

The project keeps tooling intentionally lightweight:

- `gofmt` for formatting
- `go vet` for linting
- `go test` for tests
- `make rules-docs` to regenerate [docs/lint-rules.md](docs/lint-rules.md) from the code catalog
- `Makefile` targets for local workflows and CI
- GitHub Actions workflow for formatting, vet, tests, race tests, and build verification

## Roadmap direction

This scaffold is designed to support future additions without changing the basic layout:

- config loading
- local runners and adapters
- report generation
- GitHub integration
- cloud upload/reporting
- richer command trees
- multiple target tools

Those features should be added by extending `internal/service` and `internal/platform`, not by pushing more logic into `internal/cli`.
