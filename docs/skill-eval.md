# Firety Skill Routing Eval

Firety's routing eval system is a measured companion to `firety skill lint`.

- `lint` is static analysis of the skill bundle and authoring quality
- `eval` runs a local prompt suite through a runner backend to measure whether the skill triggers when it should and stays off when it should not

## Command

```bash
firety skill eval ./path/to/skill --runner ./routing-runner
firety skill eval ./path/to/skill --backend codex=./codex-runner --backend claude-code=./claude-runner
firety skill plan ./path/to/skill --runner ./routing-runner
firety skill analyze ./path/to/skill --runner ./routing-runner
firety skill eval-compare ./before ./after --runner ./routing-runner
firety skill eval-compare ./before ./after --backend codex=./codex-runner --backend cursor=./cursor-runner
```

Defaults:

- skill path defaults to `.`
- suite path defaults to `evals/routing.json` inside the skill directory
- runner path can be supplied with `--runner` or `FIRETY_SKILL_EVAL_RUNNER`
- multi-backend runner paths can be supplied with repeated `--backend <id>[=/path/to/runner]`
- when a backend runner path is omitted, Firety looks for a backend-specific env var such as `FIRETY_SKILL_EVAL_RUNNER_CODEX`

## Multi-backend eval

Firety can run the same routing suite across multiple explicit backend identities:

```bash
firety skill eval ./path/to/skill --backend codex=./codex-runner --backend claude-code=./claude-runner
firety skill eval ./path/to/skill --backend codex --backend claude-code
```

Supported backend IDs:

- `generic`
- `codex`
- `claude-code`
- `copilot`
- `cursor`

Multi-backend mode:

- runs the same suite against each selected backend
- reports per-backend pass rate, false positives, and false negatives
- highlights cases where backend outcomes differ
- summarizes backend-specific strengths and backend-specific misses
- exits with `1` if any selected backend still has failing eval cases

This stays intentionally explicit and local-first. A backend is currently just a named local runner plus a small static metadata entry, not a plugin system or hosted integration layer.

## Suite format

Firety uses a small local JSON file:

```json
{
  "schema_version": "1",
  "name": "curated-routing-suite",
  "description": "Small routing eval suite for this skill.",
  "cases": [
    {
      "id": "positive-validate-local-skill",
      "label": "clear positive trigger",
      "prompt": "Validate this local skill bundle before publishing it.",
      "expectation": "trigger",
      "tags": ["positive", "bundle"]
    },
    {
      "id": "negative-unrelated-task",
      "prompt": "Draft a postgres migration rollout plan for production.",
      "expectation": "do-not-trigger",
      "tags": ["negative", "false-positive-trap"]
    }
  ]
}
```

Case fields:

- `id`: stable case identifier
- `label`: short human-readable label
- `prompt`: the input Firety sends to the runner
- `expectation`: `trigger` or `do-not-trigger`
- `profile`: optional profile override for the case
- `tags`: optional grouping tags
- `rationale`: optional maintainer note

## Runner contract

Firety's first measured path is a local subprocess runner. For each case, Firety writes a JSON request to the runner's stdin:

```json
{
  "schema_version": "1",
  "skill_path": "/abs/path/to/skill",
  "skill_markdown": "# Skill contents...",
  "prompt": "Validate this local skill bundle before publishing it.",
  "profile": "generic",
  "case_id": "positive-validate-local-skill",
  "label": "clear positive trigger",
  "tags": ["positive", "bundle"]
}
```

The runner must write a JSON response to stdout:

```json
{
  "schema_version": "1",
  "trigger": true,
  "reason": "The prompt clearly asks for local skill validation."
}
```

Firety treats runner failures or invalid JSON as runtime errors, not eval failures.

## Output

Eval output includes:

- total, passed, failed
- false positives and false negatives
- pass rate
- breakdown by profile and tag when present
- notable misses

Firety can also write a versioned eval artifact with `--artifact <path>` for future reporting and regression workflows.
Multi-backend eval writes a sibling artifact that captures per-backend measured results and differing cases.

## Lint/eval correlation

Firety can also run a combined analysis:

```bash
firety skill analyze ./path/to/skill --runner ./routing-runner
```

`skill analyze`:

- runs `lint` and measured routing `eval` for the same skill
- keeps lint and eval conceptually separate
- adds a deterministic correlation layer that surfaces likely contributors to:
  - false positives
  - false negatives
  - profile-sensitive misses
- uses conservative language such as "likely contributor" or "consistent with"

This output is meant to help authors decide what to fix first. It does not prove root cause and it does not replace real eval-suite design.

## Improvement plans

Firety can also turn lint and optional eval evidence into a short prioritized plan:

```bash
firety skill plan ./path/to/skill
firety skill plan ./path/to/skill --runner ./routing-runner
firety skill plan ./path/to/skill --backend codex=./codex-runner --backend claude-code=./claude-runner
```

`skill plan`:

- uses lint evidence by default
- adds single-backend measured evidence when `--runner` is supplied
- adds cross-backend disagreement evidence when repeated `--backend` flags are supplied
- groups related issues into a small set of practical next steps instead of repeating every finding verbatim

The plan is deterministic and evidence-based. It is meant to help authors decide what to fix first, not to act as a generic recommendation engine.

## Compare mode

Firety can compare measured routing results for two versions of a skill:

```bash
firety skill eval-compare ./before ./after --runner ./routing-runner
firety skill eval-compare ./before ./after --backend codex=./codex-runner --backend cursor=./cursor-runner
```

Compare mode:

- runs the same suite against both the base and candidate skill directory
- compares results by stable case ID
- reports:
  - overall outcome: `improved`, `regressed`, `mixed`, or `unchanged`
  - pass-rate delta
  - false positive delta
  - false negative delta
  - cases that flipped from pass to fail
  - cases that flipped from fail to pass
  - profile and tag breakdown deltas when present

Multi-backend compare mode additionally:

- runs the same selected backends against both versions
- reports backend-by-backend outcomes such as "improved on Codex, regressed on Cursor"
- highlights widened or narrowed backend disagreement
- exposes backend-specific regressions and improvements without repeating unchanged passing cases

This compares measured eval outcomes, not raw markdown diffs and not Firety lint findings.

Exit codes follow the candidate side:

- `0` if the candidate passes all eval cases
- `1` if the candidate still has eval failures
- `2` for invalid usage or runner/runtime failures

## Limitations

This first version is intentionally narrow:

- local subprocess runners only
- one local JSON suite format
- a small static backend catalog
- no cloud/SaaS upload flow
- no eval-backed semantic quality checks beyond trigger / non-trigger routing decisions
- no attempt to replace full end-to-end agent runtime testing

The goal is to add a truthful measured routing signal without turning Firety into a full eval platform too early.
