# firety

`firety` is a lightweight open-source CLI for linting local `SKILL.md` packages.

It checks a skill directory as a package, not just a single markdown file:

- `SKILL.md` structure and metadata
- local links and referenced resources
- trigger clarity and portability heuristics
- example quality and bundle hygiene

## Quickstart

Lint the current directory:

```bash
firety skill lint
```

Lint a specific skill:

```bash
firety skill lint ./path/to/skill
```

Machine-readable output:

```bash
firety skill lint ./path/to/skill --format json
firety skill lint ./path/to/skill --format sarif > firety.sarif
```

Helpful lint flags:

```bash
firety skill lint ./path/to/skill --explain
firety skill lint ./path/to/skill --fix
firety skill lint ./path/to/skill --strictness strict
firety skill lint ./path/to/skill --artifact ./lint-artifact.json
```

## What `skill lint` does

`firety skill lint` is the only default public workflow right now.

It validates:

- markdown structure
- front matter and metadata
- trigger and routing clarity
- examples and negative guidance
- portability wording
- bundle/resources referenced from the skill
- token/cost heuristics

The output formats are:

- `text` for local use
- `json` for automation
- `sarif` for code scanning workflows

## Examples

If you want a tiny local sample, lint [examples/minimal-skill/SKILL.md](/Users/marian2js/workspace/firety/firety/examples/minimal-skill/SKILL.md):

```bash
firety skill lint ./examples/minimal-skill
```

## Experimental / hidden

The repo still contains other command paths and reporting experiments, but they are not part of the default UX and are hidden from normal help.

Examples include:

- eval and compare flows
- workspace and change-scope flows
- artifact/render/report helpers
- gate, readiness, attestation, and publishing experiments
- benchmark and provenance tooling

Those paths remain in the codebase for ongoing iteration, but they are not the product center of `v0.1`.

## Docs

The main lint artifact format is documented in [docs/lint-artifact.md](/Users/marian2js/workspace/firety/firety/docs/lint-artifact.md).

Everything else in `docs/` should be read as experimental or internal-facing unless it directly supports `firety skill lint`.
