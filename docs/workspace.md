# Firety Workspace Workflows

Firety workspace mode extends the existing single-skill engines to repositories that contain multiple local skills.

The first version is intentionally conservative:

- Firety discovers skill directories by recursively finding directories that contain `SKILL.md`
- it runs the existing single-skill lint and readiness workflows per discovered skill
- it then produces a concise aggregate summary plus per-skill drilldown

Workspace mode does not introduce a project config file or a separate orchestration framework.

## Commands

```bash
firety workspace lint ./path/to/repo
firety workspace readiness ./path/to/repo --context merge
firety workspace gate ./path/to/repo --context public-release
firety workspace report ./path/to/repo --artifact ./workspace-report.json
```

## Discovery behavior

Firety currently uses one strong discovery rule:

- recursively detect directories that contain `SKILL.md`

It intentionally skips only a small set of noisy directories:

- `.git`
- `node_modules`
- `vendor`

If Firety cannot inspect part of the tree cleanly, it records a discovery warning instead of silently hiding that problem.

## What workspace mode summarizes

The first version focuses on high-value aggregate questions:

- how many skills are clean, warning-only, or failing lint
- how many skills are ready, ready-with-caveats, not-ready, or insufficient-evidence
- which skills are the top blockers
- which skills should maintainers review first
- whether a modest workspace gate passed or failed

Per-skill results are still kept visible. Workspace mode is not meant to flatten important regressions into one vague score.

## Workspace gate

`firety workspace gate` uses a small explicit aggregate policy surface:

- maximum allowed not-ready skills
- maximum allowed insufficient-evidence skills
- maximum allowed skills with lint errors
- maximum allowed discovery warnings

The defaults are strict:

- `--max-not-ready-skills 0`
- `--max-insufficient-evidence-skills 0`
- `--max-skills-with-lint-errors 0`
- `--max-discovery-warnings 0`

That makes the first version immediately useful for CI without introducing a larger policy language.

## Workspace report artifacts

`firety workspace report --artifact <path>` writes a versioned `firety.workspace-report` artifact.

That artifact can be:

- inspected with `firety artifact inspect`
- rendered with `firety artifact render --render pr-comment|ci-summary|full-report`
- attached to CI or review workflows without rerunning the workspace analysis

## Limitations

This first version intentionally does not include:

- workspace-wide measured eval orchestration
- workspace compare or workspace baseline flows
- workspace evidence packs or trust-report publishing
- repository-level policy configuration
- MCP-aware workspace analysis

It is a focused first layer for multi-skill repositories, built directly on top of Firety's existing single-skill engines.
