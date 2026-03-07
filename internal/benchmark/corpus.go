package benchmark

import (
	"strings"

	"github.com/firety/firety/internal/domain/lint"
)

const SkillLintBenchmarkSuiteVersion = "1"

type FixtureCategory string

const (
	CategoryPortableQuality FixtureCategory = "portable-quality"
	CategoryStructure       FixtureCategory = "structure"
	CategoryTriggerQuality  FixtureCategory = "trigger-quality"
	CategoryScope           FixtureCategory = "scope"
	CategoryPortability     FixtureCategory = "portability"
	CategoryBundle          FixtureCategory = "bundle"
	CategoryCost            FixtureCategory = "cost"
	CategoryExamples        FixtureCategory = "examples"
	CategoryTargeting       FixtureCategory = "targeting"
)

type BenchmarkSkillFixture struct {
	Name       string
	Intent     string
	Category   FixtureCategory
	Profile    string
	Strictness string
	Files      map[string]string
	Expect     BenchmarkExpectations
}

type BenchmarkExpectations struct {
	RequiredRuleIDs  []string
	ForbiddenRuleIDs []string
	MinErrorCount    int
	MaxErrorCount    int
	MinWarningCount  int
	MaxWarningCount  int
	RoutingRiskLevel lint.RoutingRiskLevel
	RoutingRiskAreas []string
	VerifyArtifact   bool
}

func SkillLintBenchmarkCorpus() []BenchmarkSkillFixture {
	return []BenchmarkSkillFixture{
		{
			Name:     "good-portable-skill",
			Intent:   "Well-authored portable skill with clear routing, examples, and supporting resources.",
			Category: CategoryPortableQuality,
			Files:    validSkillFiles(),
			Expect: BenchmarkExpectations{
				ForbiddenRuleIDs: []string{
					"skill.generic-name",
					"skill.missing-when-to-use",
					"skill.missing-examples",
					"skill.accidental-tool-lock-in",
				},
				MaxErrorCount:    0,
				MaxWarningCount:  0,
				RoutingRiskLevel: lint.RoutingRiskLow,
			},
		},
		{
			Name:     "structurally-broken-skill",
			Intent:   "Broken entry document with no title and a missing local reference.",
			Category: CategoryStructure,
			Files: map[string]string{
				"SKILL.md": strings.Join([]string{
					"---",
					"description: Validates a broken local skill bundle with enough context to isolate structural failures.",
					"---",
					"## When To Use",
					"",
					"Use this skill when you need to validate a local skill bundle before publishing it.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` from the skill root.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or runtime-specific validation.",
					"",
					"## Examples",
					"",
					"Request: Validate this local skill bundle before publishing it.",
					"Invocation: Review [Missing](docs/missing.md) before running the lint command.",
					"Result: Fix any reported issues before publishing.",
				}, "\n"),
			},
			Expect: BenchmarkExpectations{
				RequiredRuleIDs: []string{
					"skill.missing-title",
					"skill.broken-local-link",
				},
				MinErrorCount:    2,
				MaxWarningCount:  0,
				VerifyArtifact:   true,
				RoutingRiskLevel: lint.RoutingRiskLow,
			},
		},
		{
			Name:       "vague-generic-skill",
			Intent:     "Generic helper-style skill with weak routing signals and little distinctiveness.",
			Category:   CategoryTriggerQuality,
			Strictness: "pedantic",
			Files: map[string]string{
				"SKILL.md": strings.Join([]string{
					"---",
					"name: Helper",
					"description: Helpful skill for many tasks and general assistance when you need support.",
					"---",
					"# Helper",
					"",
					"## When To Use",
					"",
					"Use this skill whenever you need help with tasks, assistance, or general support across many workflows.",
					"",
					"## Usage",
					"",
					"Ask for help and let the skill figure it out.",
					"",
					"## Limitations",
					"",
					"Use judgment.",
					"",
					"## Examples",
					"",
					"Example request: Help with something important.",
					"Example result: A useful answer.",
				}, "\n"),
			},
			Expect: BenchmarkExpectations{
				RequiredRuleIDs: []string{
					"skill.generic-name",
					"skill.generic-trigger-description",
					"skill.overbroad-when-to-use",
					"skill.low-distinctiveness",
					"skill.weak-negative-guidance",
					"skill.weak-examples",
				},
				MinWarningCount:  5,
				MaxErrorCount:    0,
				RoutingRiskLevel: lint.RoutingRiskHigh,
				RoutingRiskAreas: []string{"distinctiveness", "trigger-guidance", "example-alignment"},
				VerifyArtifact:   true,
			},
		},
		{
			Name:     "diffuse-overbroad-skill",
			Intent:   "Skill that claims to cover too many unrelated tasks and broad trigger conditions.",
			Category: CategoryScope,
			Files: map[string]string{
				"SKILL.md": strings.Join([]string{
					"---",
					"name: Workflow Utility",
					"description: Helps debug, write, plan, research, analyze, review, build, and document many kinds of work across broad workflows.",
					"---",
					"# Workflow Utility",
					"",
					"## When To Use",
					"",
					"Use this skill whenever you need help with debugging, writing, planning, research, analysis, reviews, building, or documentation in any workflow.",
					"",
					"## Usage",
					"",
					"Give it the task and let it work across the full scope.",
					"",
					"## Limitations",
					"",
					"Do not use this skill when a narrow specialist skill already exists for the task.",
					"",
					"## Examples",
					"",
					"Request: Help me debug, rewrite, and document this release plan.",
					"Invocation: Use this skill to handle the whole workflow.",
					"Result: Produce a combined plan, review, and documentation update.",
				}, "\n"),
			},
			Expect: BenchmarkExpectations{
				RequiredRuleIDs: []string{
					"skill.diffuse-scope",
					"skill.overbroad-when-to-use",
				},
				MinWarningCount:  2,
				MaxErrorCount:    0,
				RoutingRiskLevel: lint.RoutingRiskMedium,
				RoutingRiskAreas: []string{"scope", "trigger-guidance"},
			},
		},
		{
			Name:     "mixed-ecosystem-portability",
			Intent:   "Skill with mixed tool guidance that should look incompatible for a Codex-oriented review.",
			Category: CategoryPortability,
			Profile:  "codex",
			Files: map[string]string{
				"SKILL.md": strings.Join([]string{
					"---",
					"name: Portable Validation Skill",
					"description: Validates local skill bundles across tools with reusable instructions.",
					"---",
					"# Portable Validation Skill",
					"",
					"## When To Use",
					"",
					"Use this skill in Codex when you need to validate a local skill bundle before publishing changes.",
					"",
					"## Usage",
					"",
					"Open Copilot Chat for one step, then run this as a Claude Code slash command for the rest of the workflow.",
					"",
					"## Limitations",
					"",
					"Do not use this skill outside local bundle validation workflows.",
					"",
					"## Examples",
					"",
					"Request: Validate this local skill bundle before publishing it from Claude Code.",
					"Invocation: Run this as a Claude Code slash command after checking Copilot Chat.",
					"Result: Review the validation findings before publishing.",
				}, "\n"),
			},
			Expect: BenchmarkExpectations{
				RequiredRuleIDs: []string{
					"skill.mixed-ecosystem-guidance",
					"skill.profile-incompatible-guidance",
				},
				MinWarningCount:  2,
				MaxErrorCount:    0,
				RoutingRiskLevel: lint.RoutingRiskMedium,
				RoutingRiskAreas: []string{"profile-fit"},
			},
		},
		{
			Name:     "bundle-resource-problem",
			Intent:   "Skill bundle with referenced empty resource and stale helper directory clutter.",
			Category: CategoryBundle,
			Files: map[string]string{
				"SKILL.md": strings.Join([]string{
					"---",
					"name: Bundle Validation Skill",
					"description: Validates local skill bundle structure and resource hygiene before publishing.",
					"---",
					"# Bundle Validation Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to inspect local bundle resources before publishing.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` and review [Script](scripts/check.sh) before publishing.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or runtime execution checks.",
					"",
					"## Examples",
					"",
					"Request: Validate this local bundle before publishing it.",
					"Invocation: Run `firety skill lint .` from the skill root.",
					"Result: Review the findings and update the bundle.",
				}, "\n"),
				"scripts/check.sh": "",
				"docs/old.md":      "legacy notes",
				"docs/unused.md":   "older reference",
			},
			Expect: BenchmarkExpectations{
				RequiredRuleIDs: []string{
					"skill.empty-referenced-resource",
					"skill.possibly-stale-resource",
				},
				MinWarningCount: 2,
				MaxErrorCount:   0,
			},
		},
		{
			Name:       "cost-bloat-problem",
			Intent:     "Skill with repetitive instructions, oversized examples, and a large referenced playbook.",
			Category:   CategoryCost,
			Strictness: "pedantic",
			Files: map[string]string{
				"SKILL.md": costBloatBenchmarkSkill(),
				"docs/playbook.md": strings.Repeat(
					"Playbook guidance for validating a large skill bundle before publishing it safely and consistently. ",
					140,
				),
			},
			Expect: BenchmarkExpectations{
				RequiredRuleIDs: []string{
					"skill.repetitive-instructions",
					"skill.large-referenced-resource",
					"skill.excessive-bundle-size",
				},
				MinWarningCount: 3,
				MaxErrorCount:   0,
			},
		},
		{
			Name:     "example-quality-problem",
			Intent:   "Skill with abstract, placeholder-heavy, incomplete examples that reference missing local resources.",
			Category: CategoryExamples,
			Files: map[string]string{
				"SKILL.md": strings.Join([]string{
					"---",
					"name: Example Quality Skill",
					"description: Validates local skill directories before publishing them safely.",
					"---",
					"# Example Quality Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to validate a local skill directory before publishing changes.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` from the skill root.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or provider-specific integrations.",
					"",
					"## Examples",
					"",
					"Request: Help me with something relevant.",
					"Invocation: Run `scripts/check.sh` for `{{skill}}`.",
					"Result: TODO add the expected output here.",
				}, "\n"),
			},
			Expect: BenchmarkExpectations{
				RequiredRuleIDs: []string{
					"skill.abstract-examples",
					"skill.placeholder-heavy-examples",
					"skill.incomplete-example",
					"skill.example-missing-bundle-resource",
				},
				MinWarningCount: 4,
				MaxErrorCount:   0,
			},
		},
		{
			Name:     "good-intentional-codex-skill",
			Intent:   "Well-signaled Codex-specific skill that should not be over-penalized when reviewed as Codex-targeted.",
			Category: CategoryTargeting,
			Profile:  "codex",
			Files: map[string]string{
				"SKILL.md": strings.Join([]string{
					"---",
					"name: Codex Bundle Validation",
					"description: Helps Codex users validate local skill bundles before publishing them.",
					"---",
					"# Codex Bundle Validation",
					"",
					"## When To Use",
					"",
					"Use this skill in Codex when you need to validate a local skill bundle before publishing changes.",
					"",
					"## Usage",
					"",
					"Install this skill in `$CODEX_HOME/skills` and run `firety skill lint .` from the skill root.",
					"",
					"## Limitations",
					"",
					"Do not use this skill outside Codex-oriented local validation workflows.",
					"Use another skill when you need cross-tool portability guidance.",
					"",
					"## Examples",
					"",
					"Request: Validate this local skill bundle in Codex before publishing it.",
					"Invocation: Run `firety skill lint . --profile codex` from the skill root in Codex.",
					"Result: Review the validation findings before publishing.",
				}, "\n"),
			},
			Expect: BenchmarkExpectations{
				ForbiddenRuleIDs: []string{
					"skill.profile-target-mismatch",
					"skill.accidental-tool-lock-in",
					"skill.generic-portability-contradiction",
					"skill.mixed-ecosystem-guidance",
				},
				MaxErrorCount:    0,
				MaxWarningCount:  1,
				RoutingRiskLevel: lint.RoutingRiskLow,
			},
		},
		{
			Name:     "accidentally-tool-locked-skill",
			Intent:   "Skill that claims portability but is actually locked to Claude Code conventions.",
			Category: CategoryTargeting,
			Files: map[string]string{
				"SKILL.md": strings.Join([]string{
					"---",
					"name: Portable Validation Skill",
					"description: Helps maintain a reusable skill with portable instructions.",
					"---",
					"# Portable Validation Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to validate a local skill directory before publishing changes.",
					"",
					"## Usage",
					"",
					"Use Claude Code slash commands for every invocation in this workflow.",
					"Install this skill under the .claude/commands directory so Claude Code can discover it.",
					"Open Claude Code and keep the skill in your .claude/commands directory.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or provider-specific integrations.",
					"",
					"## Examples",
					"",
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
					"Result: Review the reported findings and fix any portability issues before publishing.",
				}, "\n"),
			},
			Expect: BenchmarkExpectations{
				RequiredRuleIDs: []string{
					"skill.accidental-tool-lock-in",
					"skill.generic-portability-contradiction",
					"skill.tool-specific-install-assumption",
					"skill.nonportable-invocation-guidance",
				},
				MinWarningCount:  4,
				MaxErrorCount:    0,
				RoutingRiskLevel: lint.RoutingRiskHigh,
				RoutingRiskAreas: []string{"profile-fit"},
				VerifyArtifact:   true,
			},
		},
	}
}

func CategoryLabel(category FixtureCategory) string {
	switch category {
	case CategoryPortableQuality:
		return "Portable Quality"
	case CategoryStructure:
		return "Structure"
	case CategoryTriggerQuality:
		return "Trigger Quality"
	case CategoryScope:
		return "Scope"
	case CategoryPortability:
		return "Portability"
	case CategoryBundle:
		return "Bundle"
	case CategoryCost:
		return "Cost"
	case CategoryExamples:
		return "Examples"
	case CategoryTargeting:
		return "Targeting"
	default:
		return string(category)
	}
}

func validSkillFiles() map[string]string {
	return map[string]string{
		"SKILL.md": `---
name: Example Skill
description: Validates local skill directories and demonstrates clear invocation guidance with practical examples.
---

# Example Skill

## When To Use

Use this skill when you need to validate a reusable skill directory before sharing it or adding it to a larger skill collection.

## Usage

Run ` + "`firety skill lint .`" + ` to validate the skill directory.
Use this skill when you want a compact example of a documented capability with clear invocation guidance and a working local reference.

## Limitations

Do not use this skill to evaluate remote repositories, external tool integrations, or runtime behavior.
Use another tool when you need execution traces, network-aware validation, or provider-specific checks.

## Examples

- Request: "Lint this local skill directory before I publish it."
- Invocation: Run ` + "`firety skill lint . --format json`" + ` from the skill root.
- Result: Review the reported findings and fix any missing links or weak guidance before publishing.
- Reference: see [Reference](docs/reference.md)
`,
		"docs/reference.md": "# Reference\n\nThis reference explains the expected lint workflow and the supporting files that belong in the bundle.\n",
	}
}

func costBloatBenchmarkSkill() string {
	return strings.Join([]string{
		"---",
		"name: Cost Heavy Skill",
		"description: Validates large local skill bundles before publishing and demonstrates context-heavy guidance.",
		"---",
		"# Cost Heavy Skill",
		"",
		"## When To Use",
		"",
		"Use this skill when you need to validate a large local skill bundle before publishing changes.",
		"",
		"## Usage",
		"",
		"Run `firety skill lint .` from the skill root before publishing changes to validate the same local bundle with the same local workflow.",
		"Run `firety skill lint .` from the skill root before publishing changes to validate the same local bundle with the same local workflow.",
		"Run `firety skill lint .` from the skill root before publishing changes to validate the same local bundle with the same local workflow.",
		"",
		"## Limitations",
		"",
		"Do not use this skill for remote repositories or runtime execution checks.",
		"",
		"## Examples",
		"",
		"Request: Validate this local skill directory before publishing it with a full pre-publish review.",
		"Invocation: Run `firety skill lint . --format json` from the skill root and review [Playbook](docs/playbook.md) first to follow the same large validation checklist in the same bundle.",
		"Result: Review the reported findings and fix any issues before publishing the bundle.",
		"",
		"Request: Validate this local skill directory before publishing it with a full pre-publish review.",
		"Invocation: Run `firety skill lint . --format json` from the skill root and review [Playbook](docs/playbook.md) first to follow the same large validation checklist in the same bundle.",
		"Result: Review the reported findings and fix any issues before publishing the bundle.",
		"",
		"Request: Validate this local skill directory before publishing it with a full pre-publish review.",
		"Invocation: Run `firety skill lint . --format json` from the skill root and review [Playbook](docs/playbook.md) first to follow the same large validation checklist in the same bundle.",
		"Result: Review the reported findings and fix any issues before publishing the bundle.",
	}, "\n")
}
