# Firety Compatibility

Firety compatibility mode summarizes where a skill looks broadly portable, intentionally targeted, mixed, risky, or under-evidenced across profiles and measured backends.

## Purpose

Use `firety skill compatibility` when you need to answer:

- is this skill best described as generic or tool-specific?
- which profiles or backends look healthy?
- where is the evidence weak, mixed, or risky?
- what support claims can we credibly make?

Compatibility mode is a synthesis layer on top of existing Firety evidence. It does not invent a new rule system.

## Usage

Run compatibility analysis from fresh evidence:

```bash
firety skill compatibility ./path/to/skill
firety skill compatibility ./path/to/skill --profile generic --profile codex
firety skill compatibility ./path/to/skill --backend codex=./codex-runner --backend cursor=./cursor-runner
firety skill compatibility ./path/to/skill --format json --artifact ./compatibility.json
```

Run compatibility analysis from existing artifacts:

```bash
firety skill compatibility --input-artifact ./lint-artifact.json
firety skill compatibility --input-artifact ./analysis-artifact.json
firety skill compatibility --input-artifact ./lint-artifact.json --input-artifact ./eval-multi-artifact.json
```

## Support posture

Current support-posture values are:

- `generic-portable`
- `intentionally-tool-specific`
- `mixed-ambiguous`
- `accidentally-tool-locked`
- `weak-evidence`

These are intentionally conservative.

- `generic-portable` means Firety sees low-noise generic portability evidence and no strong backend-specific risk
- `intentionally-tool-specific` means Firety sees one clear target ecosystem and that posture looks more honest than a generic claim
- `mixed-ambiguous` means the evidence points in different directions and Firety does not yet see one clean support posture
- `accidentally-tool-locked` means Firety sees a stronger ecosystem lock-in than the current wording or portability posture claims justify
- `weak-evidence` means Firety does not have enough profile or backend evidence to make a strong claim

## Evidence model

Compatibility mode reuses existing Firety signals such as:

- profile-aware lint findings
- routing-risk summaries
- targeting-posture heuristics from portability analysis
- optional measured backend pass/fail results
- optional backend disagreement patterns

It does not claim semantic runtime compatibility beyond the measured suite and static lint evidence currently available.

## Output

Text output includes:

- overall support posture
- evidence level
- per-profile summaries
- optional per-backend summaries
- top blockers
- notable strengths
- recommended maintainer positioning

JSON and artifact output include the same information in a structured form.

## Limitations

This first version intentionally does not include:

- repository-wide compatibility analysis across many skills
- hosted support matrices or cloud history
- MCP compatibility
- hidden weighting or a generic scoring engine

If the evidence is incomplete or conflicting, Firety prefers `mixed-ambiguous` or `weak-evidence` over a stronger claim.
