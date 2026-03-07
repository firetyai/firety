package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/firety/firety/internal/testutil"
)

func TestSkillRulesHaveStableIDs(t *testing.T) {
	t.Parallel()

	expectedIDs := []string{
		"skill.target-not-found",
		"skill.target-not-directory",
		"skill.missing-skill-md",
		"skill.unreadable-skill-md",
		"skill.empty-skill-md",
		"skill.invalid-front-matter",
		"skill.missing-front-matter-name",
		"skill.empty-front-matter-name",
		"skill.missing-front-matter-description",
		"skill.empty-front-matter-description",
		"skill.long-front-matter-name",
		"skill.short-front-matter-description",
		"skill.long-front-matter-description",
		"skill.vague-description",
		"skill.name-title-mismatch",
		"skill.description-body-mismatch",
		"skill.scope-mismatch",
		"skill.generic-name",
		"skill.generic-trigger-description",
		"skill.diffuse-scope",
		"skill.overbroad-when-to-use",
		"skill.weak-trigger-pattern",
		"skill.low-distinctiveness",
		"skill.example-trigger-mismatch",
		"skill.trigger-scope-inconsistency",
		"skill.missing-title",
		"skill.broken-local-link",
		"skill.reference-outside-root",
		"skill.referenced-directory-instead-of-file",
		"skill.empty-referenced-resource",
		"skill.suspicious-referenced-resource",
		"skill.duplicate-resource-reference",
		"skill.missing-mentioned-resource",
		"skill.inconsistent-bundle-structure",
		"skill.possibly-stale-resource",
		"skill.unhelpful-referenced-resource",
		"skill.large-skill-md",
		"skill.excessive-example-volume",
		"skill.duplicate-examples",
		"skill.large-referenced-resource",
		"skill.excessive-bundle-size",
		"skill.repetitive-instructions",
		"skill.unbalanced-skill-content",
		"skill.large-content",
		"skill.duplicate-heading",
		"skill.missing-when-to-use",
		"skill.missing-negative-guidance",
		"skill.weak-negative-guidance",
		"skill.missing-examples",
		"skill.weak-examples",
		"skill.generic-examples",
		"skill.examples-missing-invocation-pattern",
		"skill.abstract-examples",
		"skill.placeholder-heavy-examples",
		"skill.examples-missing-expected-outcome",
		"skill.examples-missing-trigger-input",
		"skill.example-scope-contradiction",
		"skill.example-guidance-mismatch",
		"skill.incomplete-example",
		"skill.example-missing-bundle-resource",
		"skill.low-variety-examples",
		"skill.missing-usage-guidance",
		"skill.tool-specific-branding",
		"skill.profile-incompatible-guidance",
		"skill.tool-specific-install-assumption",
		"skill.nonportable-invocation-guidance",
		"skill.generic-profile-tool-locking",
		"skill.unclear-tool-targeting",
		"skill.accidental-tool-lock-in",
		"skill.generic-portability-contradiction",
		"skill.mixed-ecosystem-guidance",
		"skill.missing-tool-target-boundary",
		"skill.profile-target-mismatch",
		"skill.example-ecosystem-mismatch",
		"skill.suspicious-relative-path",
		"skill.short-content",
	}

	rules := lint.AllRules()
	if len(rules) != len(expectedIDs) {
		t.Fatalf("expected %d rules, got %#v", len(expectedIDs), rules)
	}

	for index, expected := range expectedIDs {
		if rules[index].ID != expected {
			t.Fatalf("expected rule %d id %q, got %#v", index, expected, rules[index])
		}
	}
}

func TestSkillLinterValidSkillDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	report, err := service.NewSkillLinter().Lint(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
}

func TestSkillLinterTargetErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing target path", func(t *testing.T) {
		t.Parallel()

		report, err := service.NewSkillLinter().Lint(filepath.Join(t.TempDir(), "missing"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleTargetNotFound.ID, 0)
	})

	t.Run("target is not a directory", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		filePath := filepath.Join(root, "plain-file")
		if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		report, err := service.NewSkillLinter().Lint(filePath)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleTargetNotDirectory.ID, 0)
	})

	t.Run("missing skill file", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingSkillMD.ID, 0)
	})

	t.Run("empty skill file", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": "   \n\t",
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleEmptySkillMD.ID, 0)
	})
}

func TestSkillLinterFrontMatterValidation(t *testing.T) {
	t.Parallel()

	t.Run("malformed front matter", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"name: [broken",
				"description: Validates reusable skill directories before sharing them with a team.",
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleInvalidFrontMatter.ID, 2)
	})

	t.Run("missing name", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"description: Validates reusable skill directories before sharing them with a team.",
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingFrontMatterName.ID, 2)
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				`name: ""`,
				"description: Validates reusable skill directories before sharing them with a team.",
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleEmptyFrontMatterName.ID, 2)
	})

	t.Run("missing description", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"name: Example Skill",
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingFrontMatterDescription.ID, 2)
	})

	t.Run("empty description", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"name: Example Skill",
				`description: ""`,
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleEmptyFrontMatterDescription.ID, 3)
	})

	t.Run("long name", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"name: Example Skill For Very Long Skill Titles That Keep Going Past Reasonable Review Boundaries",
				"description: Validates reusable skill directories before sharing them with a team.",
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleLongFrontMatterName.ID, 2)
	})

	t.Run("short and vague description", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"name: Example Skill",
				"description: Useful skill.",
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleShortFrontMatterDescription.ID, 3)
		assertFinding(t, report, lint.RuleVagueDescription.ID, 3)
	})

	t.Run("long description", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		longDescription := strings.Repeat("This description keeps adding context without sharpening the focus of the skill definition. ", 4)
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"name: Example Skill",
				"description: " + longDescription,
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleLongFrontMatterDescription.ID)
	})

	t.Run("long description in strict mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		longDescription := strings.Repeat("This description keeps adding context without sharpening the focus of the skill definition. ", 4)
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"name: Example Skill",
				"description: " + longDescription,
				"---",
				validSkillBody(),
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleLongFrontMatterDescription.ID, 3)
	})
}

func TestSkillLinterLineAwareStructureFindings(t *testing.T) {
	t.Parallel()

	t.Run("missing title uses body start line", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"name: Example Skill",
				"description: Validates reusable skill directories before sharing them with a team.",
				"---",
				"## Usage",
				"",
				"Run `firety skill lint .` before publishing changes.",
				"",
				"## Examples",
				"",
				"Request: Lint this skill before publishing it.",
				"Invocation: Run `firety skill lint .` from the skill root.",
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleMissingTitle.ID)
	})

	t.Run("missing title without front matter name uses body start line", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": strings.Join([]string{
				"---",
				"description: Validates reusable skill directories before sharing them with a team.",
				"---",
				"## Usage",
				"",
				"Run `firety skill lint .` before publishing changes.",
				"",
				"## Examples",
				"",
				"Request: Lint this skill before publishing it.",
				"Invocation: Run `firety skill lint .` from the skill root.",
			}, "\n"),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingTitle.ID, 4)
	})

	t.Run("broken local links include line", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": brokenLinkSkillContent(),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleBrokenLocalLink.ID, 19)
	})
}

func TestSkillLinterSkillQualityWarnings(t *testing.T) {
	t.Parallel()

	t.Run("missing invocation guidance", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to review a local skill directory before publishing it.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or external tool runtimes.",
					"",
					"## Examples",
					"",
					"Request: Lint this skill before publishing it.",
					"Result: Review the reported findings before merging the change.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingUsageGuidance.ID, 0)
	})

	t.Run("narrative usage guidance counts in default mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Use this skill when the user asks to validate a local skill directory before publishing changes.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"The user provides a local skill directory and may include publication context or quality concerns.",
					"",
					"## Examples",
					"",
					"Request: Lint this skill before publishing it.",
					"Result: Review the findings and fix any broken references before publishing.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleMissingUsageGuidance.ID)
	})

	t.Run("missing when to use guidance", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` before publishing the skill.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or provider-specific integrations.",
					"",
					"## Examples",
					"",
					"Request: Lint this skill before publishing it.",
					"Invocation: Run `firety skill lint .` from the skill root.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingWhenToUse.ID, 0)
	})

	t.Run("front matter trigger guidance counts in default mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Use this skill when the user asks to validate a local skill directory before publishing changes.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` before publishing the skill.",
					"",
					"## Examples",
					"",
					"Request: Lint this skill before publishing it.",
					"Invocation: Run `firety skill lint .` from the skill root.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleMissingWhenToUse.ID)
	})

	t.Run("missing negative guidance", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to validate a local skill directory before publishing changes.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` from the skill root.",
					"",
					"## Examples",
					"",
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleMissingNegativeGuidance.ID)
	})

	t.Run("missing negative guidance in strict mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to validate a local skill directory before publishing changes.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` from the skill root.",
					"",
					"## Examples",
					"",
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingNegativeGuidance.ID, 0)
	})

	t.Run("weak negative guidance", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
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
					"Use judgment.",
					"",
					"## Examples",
					"",
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleWeakNegativeGuidance.ID, 15)
	})

	t.Run("missing examples", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to review a skill directory before publishing changes.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` before publishing the skill.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or tool-runtime validation.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleMissingExamples.ID)
	})

	t.Run("missing examples in strict mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to review a skill directory before publishing changes.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` before publishing the skill.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or tool-runtime validation.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingExamples.ID, 0)
	})

	t.Run("weak and generic examples", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
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
					"Do not use this skill for remote repositories or provider-specific workflows.",
					"",
					"## Examples",
					"",
					"Example scenario: use this skill when needed.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleWeakExamples.ID, 19)
		assertFinding(t, report, lint.RuleGenericExamples.ID, 19)
		assertFinding(t, report, lint.RuleExamplesMissingInvocationPattern.ID, 19)
	})

	t.Run("incidental for example phrase does not create examples section", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Use this skill when the user asks to validate a local skill directory before publishing changes.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` from the skill root before publishing changes.",
					"",
					"## Notes",
					"",
					"Use a distinctive output format, for example a compact markdown checklist.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleWeakExamples.ID)
		assertNoFinding(t, report, lint.RuleGenericExamples.ID)
		assertNoFinding(t, report, lint.RuleExamplesMissingInvocationPattern.ID)
	})

	t.Run("examples missing invocation pattern", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates reusable skill directories before sharing them with a team.",
				strings.Join([]string{
					"# Example Skill",
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
					"Do not use this skill for remote repositories or provider-specific workflows.",
					"",
					"## Examples",
					"",
					"Request: Lint this local skill directory before publishing it.",
					"Result: Fix any broken links or weak guidance before merging.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleExamplesMissingInvocationPattern.ID, 19)
	})
}

func TestSkillLinterExecutableExampleRealismWarnings(t *testing.T) {
	t.Parallel()

	t.Run("abstract and placeholder-heavy examples", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates local skill directories before publishing them.",
				strings.Join([]string{
					"# Example Skill",
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
					"Invocation: Run `firety skill lint <path>` for `{{skill}}`.",
					"Result: It works.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleAbstractExamples.ID, 21)
		assertFinding(t, report, lint.RulePlaceholderHeavyExamples.ID, 22)
	})

	t.Run("missing expected outcome", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates local skill directories before publishing them.",
				strings.Join([]string{
					"# Example Skill",
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
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
					"Request: Check whether docs/reference.md is linked correctly before publishing.",
					"Invocation: Review the lint findings before publishing the skill.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleExamplesMissingExpectedOutcome.ID, 21)
	})

	t.Run("missing trigger input", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates local skill directories before publishing them.",
				strings.Join([]string{
					"# Example Skill",
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
					"Result: The lint report shows two broken links and one weak examples warning.",
					"Output: Summary: 0 error(s), 3 warning(s).",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleExamplesMissingTriggerInput.ID, 21)
	})

	t.Run("scope contradiction and guidance mismatch", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Bundle Lint Skill",
				"Validates local skill bundles before publishing them.",
				strings.Join([]string{
					"# Bundle Lint Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to validate local skill bundles, bundle links, and publication readiness before merging.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` from the skill root.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories, release planning, or production rollout work.",
					"",
					"## Examples",
					"",
					"Request: Prepare a remote production rollout plan for the GitHub release and deployment window.",
					"Invocation: Run the release checklist and publish the rollout notes.",
					"Result: Share the release plan with the on-call rotation.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleExampleScopeContradiction.ID, 21)
		assertFinding(t, report, lint.RuleExampleGuidanceMismatch.ID, 21)
	})

	t.Run("incomplete example and missing bundle resource", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates local skill directories before publishing them.",
				strings.Join([]string{
					"# Example Skill",
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
					"Request: Validate the current skill bundle before publishing it.",
					"Invocation: Run `scripts/check.sh` before `firety skill lint .`.",
					"Result: TODO add the expected output here.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleExampleMissingBundleResource.ID, 22)
		assertFinding(t, report, lint.RuleIncompleteExample.ID, 23)
	})

	t.Run("low-variety examples", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates local skill directories before publishing them.",
				strings.Join([]string{
					"# Example Skill",
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
					"Request: Lint the local skill bundle before publishing it to the team registry.",
					"Request: Lint the local skill package before publishing it to the internal registry.",
					"Request: Lint the local skill directory before publishing it to the shared registry.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
					"Result: Review the lint findings before publishing.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleLowVarietyExamples.ID, 22)
	})
}

func TestSkillLinterFrontMatterBodyConsistencyWarnings(t *testing.T) {
	t.Parallel()

	t.Run("name and title mismatch", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Skill Linter",
				"Deploys documentation sites with clear release guidance and rollout steps.",
				strings.Join([]string{
					"# Release Manager",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to plan and communicate a documentation deployment.",
					"",
					"## Usage",
					"",
					"Run the release checklist and publish the site update.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for local linting or repository-wide code scanning.",
					"",
					"## Examples",
					"",
					"Request: Prepare the documentation deployment for today's release.",
					"Invocation: Run the release checklist and publish the site.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleNameTitleMismatch.ID, 5)
	})

	t.Run("description and body mismatch", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Release Manager",
				"Summarizes database incidents for on-call handoffs.",
				strings.Join([]string{
					"# Release Manager",
					"",
					"## When To Use",
					"",
					"Use this skill when you need to prepare a documentation deployment and coordinate release steps.",
					"",
					"## Usage",
					"",
					"Run the release checklist, verify the rollout window, and publish the site.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for local linting or markdown validation.",
					"",
					"## Examples",
					"",
					"Request: Prepare the documentation deployment for today's release.",
					"Invocation: Run the release checklist and publish the site.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleDescriptionBodyMismatch.ID, 3)
	})

	t.Run("scope mismatch", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Skill Linter",
				"Validates local skill directories before publishing them.",
				strings.Join([]string{
					"# Skill Linter",
					"",
					"## When To Use",
					"",
					"Use this skill when you need a general purpose helper for any task across many workflows.",
					"",
					"## Usage",
					"",
					"Run `firety skill lint .` when you want a quick local lint pass.",
					"",
					"## Limitations",
					"",
					"Do not use this skill for remote repositories or provider-specific integrations.",
					"",
					"## Examples",
					"",
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleScopeMismatch.ID, 9)
	})
}

func TestSkillLinterDeterministicOrdering(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"# Example Skill",
			"",
			"## Overview",
			"",
			"Read [Outside](../shared/guide.md) for more context.",
			"",
			"## Overview",
			"",
			"This content is intentionally long enough to avoid the short-content warning while still missing the guidance sections.",
			"This keeps the test focused on deterministic ordering.",
			"",
		}, "\n"),
	})

	report, err := service.NewSkillLinter().Lint(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedOrder := []string{
		lint.RuleBrokenLocalLink.ID,
		lint.RuleReferenceOutsideRoot.ID,
		lint.RuleDuplicateHeading.ID,
		lint.RuleMissingWhenToUse.ID,
		lint.RuleMissingUsageGuidance.ID,
		lint.RuleSuspiciousRelativePath.ID,
	}
	if len(report.Findings) < len(expectedOrder) {
		t.Fatalf("expected ordered findings, got %#v", report.Findings)
	}

	for index, expected := range expectedOrder {
		if report.Findings[index].RuleID != expected {
			t.Fatalf("expected finding %d to be %q, got %#v", index, expected, report.Findings)
		}
	}
}

func TestSkillLinterProfileAwarePortabilityWarnings(t *testing.T) {
	t.Parallel()

	t.Run("generic profile stays quiet for portable skill", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileGeneric)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		for _, finding := range report.Findings {
			switch finding.RuleID {
			case lint.RuleToolSpecificBranding.ID,
				lint.RuleProfileIncompatibleGuidance.ID,
				lint.RuleToolSpecificInstallAssumption.ID,
				lint.RuleNonportableInvocationGuidance.ID,
				lint.RuleGenericProfileToolLocking.ID,
				lint.RuleUnclearToolTargeting.ID,
				lint.RuleAccidentalToolLockIn.ID,
				lint.RuleGenericPortabilityContradiction.ID,
				lint.RuleMixedEcosystemGuidance.ID,
				lint.RuleMissingToolTargetBoundary.ID,
				lint.RuleProfileTargetMismatch.ID,
				lint.RuleExampleEcosystemMismatch.ID:
				t.Fatalf("expected no portability finding, got %#v", report.Findings)
			}
		}
	})

	t.Run("generic profile warns on accidental tool lock-in", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": portabilitySkillContent(
				"Portable Skill",
				"Helps maintain a reusable skill with portable instructions.",
				[]string{
					"Use Claude Code slash commands for every invocation in this workflow.",
					"Install this skill under `.claude/commands` so Claude Code can discover it.",
					"Open Claude Code and keep the skill in your `.claude/commands` directory.",
				},
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileGeneric)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleGenericPortabilityContradiction.ID, 3)
		assertFinding(t, report, lint.RuleAccidentalToolLockIn.ID, 3)
		assertFinding(t, report, lint.RuleToolSpecificBranding.ID, 13)
		assertFinding(t, report, lint.RuleNonportableInvocationGuidance.ID, 13)
		assertFinding(t, report, lint.RuleToolSpecificInstallAssumption.ID, 14)
	})

	t.Run("word forms do not trigger tool branding", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Use this skill when the user asks for a polished frontend treatment.",
				strings.Join([]string{
					"# Example Skill",
					"",
					"## Usage",
					"",
					"Apply decorative borders, custom cursors, and layered gradients when they fit the design.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileGeneric)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleToolSpecificBranding.ID)
	})

	t.Run("honest tool specific skill reduces generic-profile noise", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Codex Review Skill",
				"Codex skill for validating local skill bundles in Codex workflows.",
				strings.Join([]string{
					"# Codex Review Skill",
					"",
					"## When To Use",
					"",
					"Use this skill in Codex when you need to validate a local skill bundle before publishing changes.",
					"",
					"## Usage",
					"",
					"Install this skill in `$CODEX_HOME/skills` and run it from Codex chat.",
					"",
					"## Limitations",
					"",
					"Do not use this skill outside Codex workflows. Use another tool for Claude Code, Cursor, or Copilot-specific workflows.",
					"",
					"## Examples",
					"",
					"Request: Validate this local skill bundle in Codex before publishing it.",
					"Invocation: Run `firety skill lint . --profile codex` from Codex chat.",
					"Result: Review the Codex-focused lint findings before publishing.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileGeneric)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		for _, finding := range report.Findings {
			switch finding.RuleID {
			case lint.RuleToolSpecificBranding.ID,
				lint.RuleToolSpecificInstallAssumption.ID,
				lint.RuleNonportableInvocationGuidance.ID,
				lint.RuleGenericProfileToolLocking.ID,
				lint.RuleAccidentalToolLockIn.ID,
				lint.RuleGenericPortabilityContradiction.ID,
				lint.RuleUnclearToolTargeting.ID,
				lint.RuleMissingToolTargetBoundary.ID:
				t.Fatalf("expected no portability-noise finding, got %#v", report.Findings)
			}
		}
	})

	t.Run("codex profile warns on claude-specific guidance", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": portabilitySkillContent(
				"Codex Lint Skill",
				"Lints reusable skills while staying portable across local environments.",
				[]string{
					"Run this as a Claude Code slash command when the user asks for validation help.",
				},
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileCodex)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleProfileIncompatibleGuidance.ID, 13)
	})

	t.Run("claude code profile warns on codex install assumption", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": portabilitySkillContent(
				"Claude Code Skill",
				"Guides contributors through portable local skill validation.",
				[]string{
					"Install this skill in `$CODEX_HOME/skills` before running it locally.",
				},
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileClaudeCode)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleToolSpecificInstallAssumption.ID, 13)
	})

	t.Run("selected profile mismatch is reported for clearly targeted skill", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Codex Skill",
				"Codex skill for validating local bundles before publishing them.",
				strings.Join([]string{
					"# Codex Skill",
					"",
					"## When To Use",
					"",
					"Use this skill in Codex when you need to validate a local skill bundle before publishing changes.",
					"",
					"## Usage",
					"",
					"Install this skill in `$CODEX_HOME/skills` before running it in Codex.",
					"",
					"## Limitations",
					"",
					"Do not use this skill outside Codex workflows.",
					"",
					"## Examples",
					"",
					"Request: Validate this local skill bundle in Codex before publishing it.",
					"Invocation: Run `firety skill lint . --profile codex` from Codex.",
					"Result: Review the Codex-focused findings before publishing.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileClaudeCode)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleProfileTargetMismatch.ID, 2)
	})

	t.Run("mixed ecosystem guidance is called out explicitly", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Portable Skill",
				"Portable skill for validating local bundles across tools.",
				strings.Join([]string{
					"# Portable Skill",
					"",
					"## When To Use",
					"",
					"Use this skill when you need a portable lint pass across tools.",
					"",
					"## Usage",
					"",
					"Open Claude Code slash commands and GitHub Copilot Chat for the same workflow.",
					"",
					"## Limitations",
					"",
					"Use judgment when mixing tools.",
					"",
					"## Examples",
					"",
					"Request: Validate this skill bundle before publishing it.",
					"Invocation: Open Claude Code slash commands and GitHub Copilot Chat for the same review.",
					"Result: Compare the mixed workflow outputs.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileGeneric)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMixedEcosystemGuidance.ID, 13)
	})

	t.Run("missing target boundary for tool specific skill", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Cursor Skill",
				"Cursor skill for validating local skill bundles before publishing them.",
				strings.Join([]string{
					"# Cursor Skill",
					"",
					"## When To Use",
					"",
					"Use this skill in Cursor when you need to validate a local skill bundle before publishing changes.",
					"",
					"## Usage",
					"",
					"Install this skill under `.cursor/rules` and invoke it from Cursor.",
					"",
					"## Examples",
					"",
					"Request: Validate this local skill bundle in Cursor before publishing it.",
					"Invocation: Run `firety skill lint . --profile cursor` from Cursor.",
					"Result: Review the Cursor-focused findings before publishing.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileGeneric)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingToolTargetBoundary.ID, 2)
	})

	t.Run("example ecosystem mismatch is detected", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Codex Skill",
				"Codex skill for validating local bundles before publishing them.",
				strings.Join([]string{
					"# Codex Skill",
					"",
					"## When To Use",
					"",
					"Use this skill in Codex when you need to validate a local skill bundle before publishing changes.",
					"",
					"## Usage",
					"",
					"Install this skill in `$CODEX_HOME/skills` before running it in Codex.",
					"",
					"## Limitations",
					"",
					"Do not use this skill outside Codex workflows.",
					"",
					"## Examples",
					"",
					"Request: Validate this local skill bundle from Claude Code before publishing it.",
					"Invocation: Run this as a Claude Code slash command.",
					"Result: Review the Claude Code findings.",
				}, "\n"),
			),
		})

		report, err := service.NewSkillLinter().LintWithProfile(root, service.SkillLintProfileCodex)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleExampleEcosystemMismatch.ID, 21)
	})
}

func TestSkillLinterBundleResourceWarnings(t *testing.T) {
	t.Parallel()

	t.Run("reference outside root", func(t *testing.T) {
		t.Parallel()

		workspace := t.TempDir()
		root := filepath.Join(workspace, "skill")
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"See [Shared](../shared.md) for more context.",
			}),
		})
		if err := os.WriteFile(filepath.Join(workspace, "shared.md"), []byte("shared context"), 0o644); err != nil {
			t.Fatalf("write shared resource: %v", err)
		}

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleReferenceOutsideRoot.ID, 13)
	})

	t.Run("directory referenced where file is expected", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"See [Docs](docs/) before publishing.",
			}),
			"docs/.keep": "",
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleReferencedDirectoryInsteadOfFile.ID, 13)
	})

	t.Run("empty referenced script", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"Run [Script](scripts/check.sh) before publishing.",
			}),
			"scripts/check.sh": "",
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleEmptyReferencedResource.ID, 13)
	})

	t.Run("suspicious referenced resource type", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"Download [Bundle](assets/tool.bin) before using this skill.",
			}),
			"assets/tool.bin": "binary payload",
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleSuspiciousReferencedResource.ID, 13)
	})

	t.Run("unhelpful referenced markdown resource", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"See [Reference](docs/reference.md) before publishing.",
			}),
			"docs/reference.md": "tiny",
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleUnhelpfulReferencedResource.ID, 13)
	})

	t.Run("missing mentioned resource", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"Run `scripts/lint.sh` before publishing this skill.",
			}),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleMissingMentionedResource.ID)
	})

	t.Run("missing mentioned resource in strict mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"Run `scripts/lint.sh` before publishing this skill.",
			}),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleMissingMentionedResource.ID, 13)
	})

	t.Run("project file globs and API methods are not treated as local resources", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"Look for `*.py`, `package.json`, and `JSON.parse()` in the user's project before making changes.",
			}),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleMissingMentionedResource.ID)
	})

	t.Run("duplicate resource references", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": bundleSkillContent([]string{
				"See [Reference](docs/reference.md) before publishing.",
				"Review [Reference](docs/reference.md) again after editing the skill.",
				"Keep [Reference](docs/reference.md) open during the final review.",
			}),
			"docs/reference.md": "This reference explains the expected lint workflow in enough detail to avoid the short-resource warning.",
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleDuplicateResourceReference.ID, 15)
	})

	t.Run("possibly stale helper resources", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": skillWithFrontMatter(
				"Example Skill",
				"Validates local skill bundles before publishing them.",
				validSkillBody(),
			),
			"docs/old-a.md": "Old helper resource A.",
			"docs/old-b.md": "Old helper resource B.",
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RulePossiblyStaleResource.ID, 0)
	})
}

func TestSkillLinterCostAwareWarnings(t *testing.T) {
	t.Parallel()

	t.Run("large skill markdown", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": costSkillContent(
				[]string{
					strings.Repeat("Use this skill to validate local bundles before publishing changes with careful step-by-step review guidance. ", 90),
				},
				[]string{
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
					"Result: Review the reported findings and fix any issues before publishing.",
				},
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleLargeSkillMD.ID)
	})

	t.Run("large skill markdown in strict mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": costSkillContent(
				[]string{
					strings.Repeat("Use this skill to validate local bundles before publishing changes with careful step-by-step review guidance. ", 90),
				},
				[]string{
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
					"Result: Review the reported findings and fix any issues before publishing.",
				},
			),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleLargeSkillMD.ID, 0)
	})

	t.Run("excessive and duplicated examples", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		duplicateExample := "Request: Lint a large reusable skill bundle before publishing it to the team registry with every helper resource included."
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": costSkillContent(
				[]string{
					"Run `firety skill lint .` from the skill root before publishing changes.",
				},
				[]string{
					duplicateExample,
					duplicateExample,
					duplicateExample,
					"Invocation: Run `firety skill lint . --format json` from the skill root before publishing changes.",
					strings.Repeat("Result: Review the reported findings, compare repeated scenarios, and keep only the minimal example set that still demonstrates realistic usage. ", 12),
				},
			),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleExcessiveExampleVolume.ID, 19)
		assertFinding(t, report, lint.RuleDuplicateExamples.ID, 19)
	})

	t.Run("unbalanced skill content", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": costSkillContent(
				[]string{
					"Run `firety skill lint .` from the skill root before publishing changes.",
				},
				[]string{
					"Request: Lint a large reusable skill bundle before publishing it to the team registry with every helper resource included.",
					"Invocation: Run `firety skill lint . --format json` from the skill root before publishing changes.",
					strings.Repeat("Result: Review the reported findings, compare repeated scenarios, and keep only the minimal example set that still demonstrates realistic usage. ", 20),
				},
			),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleUnbalancedSkillContent.ID, 19)
	})

	t.Run("large referenced resource and bundle size", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": costSkillContent(
				[]string{
					"See [Reference](docs/reference.md) before publishing changes.",
					"See [Playbook](docs/playbook.md) before publishing changes.",
				},
				[]string{
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
				},
			),
			"docs/reference.md": strings.Repeat("Reference guidance for validating a skill bundle before publishing it safely. ", 70),
			"docs/playbook.md":  strings.Repeat("Playbook guidance for validating a skill bundle before publishing it safely. ", 130),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertNoFinding(t, report, lint.RuleLargeReferencedResource.ID)
		assertNoFinding(t, report, lint.RuleExcessiveBundleSize.ID)
	})

	t.Run("large referenced resource and bundle size in strict mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": costSkillContent(
				[]string{
					"See [Reference](docs/reference.md) before publishing changes.",
					"See [Playbook](docs/playbook.md) before publishing changes.",
				},
				[]string{
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
				},
			),
			"docs/reference.md": strings.Repeat("Reference guidance for validating a skill bundle before publishing it safely. ", 70),
			"docs/playbook.md":  strings.Repeat("Playbook guidance for validating a skill bundle before publishing it safely. ", 130),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessStrict)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleLargeReferencedResource.ID, 13)
		assertFinding(t, report, lint.RuleExcessiveBundleSize.ID, 0)
	})

	t.Run("repetitive instructions", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		repeatedLine := "Run `firety skill lint . --format json` from the skill root before publishing changes to validate the same local bundle."
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": costSkillContent(
				[]string{
					repeatedLine,
					repeatedLine,
					repeatedLine,
				},
				[]string{
					"Request: Lint this local skill directory before publishing it.",
					"Invocation: Run `firety skill lint . --format json` from the skill root.",
					"Result: Review the reported findings and fix any issues before publishing.",
				},
			),
		})

		report, err := service.NewSkillLinter().LintWithProfileAndStrictness(root, service.SkillLintProfileGeneric, lint.StrictnessPedantic)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleRepetitiveInstructions.ID, 11)
	})
}

func TestSkillLinterTriggerQualityWarnings(t *testing.T) {
	t.Parallel()

	t.Run("generic skill name", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": triggerSkillContent(
				"Helper",
				"Summarizes production incidents for on-call handoffs with clear routing guidance.",
				"# Incident Handoff\n",
				"Use this skill when you need to summarize a production incident for the next on-call engineer.",
				"Request: Summarize the latest production incident for handoff.\nInvocation: Run the incident handoff checklist.",
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleGenericName.ID, 2)
	})

	t.Run("generic description and low distinctiveness", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": triggerSkillContent(
				"Task Router",
				"Helpful skill for general assistance with many tasks.",
				"# Task Router\n",
				"Use this skill whenever you need help with tasks or general assistance across many workflows.",
				"Request: Help me with something.\nInvocation: Use the skill when you need help.\nResult: Be helpful.",
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleGenericTriggerDescription.ID, 3)
		assertFinding(t, report, lint.RuleLowDistinctiveness.ID, 3)
	})

	t.Run("diffuse scope and overbroad when to use", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": triggerSkillContent(
				"Workflow Assistant",
				"Handles debugging, writing, planning, research, testing, and documentation requests.",
				"# Workflow Assistant\n",
				"Use this skill for any task across debugging, writing, planning, research, testing, and documentation whenever you need help.",
				"Request: Help with a workflow task.\nInvocation: Use the workflow assistant.\nResult: Continue the task.",
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleDiffuseScope.ID, 3)
		assertFinding(t, report, lint.RuleOverbroadWhenToUse.ID, 8)
	})

	t.Run("weak example trigger alignment", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": triggerSkillContent(
				"Incident Handoff",
				"Summarizes production incidents for on-call handoffs with clear routing guidance.",
				"# Incident Handoff\n",
				"Use this skill when you need to summarize a production incident for the next on-call engineer.",
				"Request: Help me with something.\nResult: Do the thing.\nExample: General help.",
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleWeakTriggerPattern.ID, 20)
		assertFinding(t, report, lint.RuleExampleTriggerMismatch.ID, 20)
	})

	t.Run("trigger scope inconsistency", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.WriteFiles(t, root, map[string]string{
			"SKILL.md": triggerSkillContent(
				"Incident Handoff",
				"Summarizes production incidents for on-call handoffs with clear routing guidance.",
				"# Incident Handoff\n",
				"Use this skill when you need to plan a product roadmap and coordinate quarterly priorities.",
				"Request: Plan the next quarter roadmap.\nInvocation: Run the planning checklist.\nResult: Produce a roadmap summary.",
			),
		})

		report, err := service.NewSkillLinter().Lint(root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		assertFinding(t, report, lint.RuleTriggerScopeInconsistency.ID, 8)
	})
}

func assertFinding(t *testing.T, report lint.Report, ruleID string, line int) {
	t.Helper()

	for _, finding := range report.Findings {
		if finding.RuleID != ruleID {
			continue
		}

		if line > 0 && finding.Line != line {
			t.Fatalf("expected rule %q at line %d, got %#v", ruleID, line, finding)
		}

		return
	}

	t.Fatalf("expected finding %q, got %#v", ruleID, report.Findings)
}

func assertNoFinding(t *testing.T, report lint.Report, ruleID string) {
	t.Helper()

	for _, finding := range report.Findings {
		if finding.RuleID == ruleID {
			t.Fatalf("expected no finding %q, got %#v", ruleID, finding)
		}
	}
}

func validSkillBody() string {
	return strings.Join([]string{
		"# Example Skill",
		"",
		"## When To Use",
		"",
		"Use this skill when you need to validate a reusable skill directory before publishing changes or sharing it with other developers.",
		"",
		"## Usage",
		"",
		"Run `firety skill lint .` from the skill root before opening a pull request.",
		"",
		"## Limitations",
		"",
		"Do not use this skill for remote repositories, provider-specific integrations, or runtime execution validation.",
		"Use another tool when you need network-aware analysis or execution traces.",
		"",
		"## Examples",
		"",
		"Request: Lint this local skill directory before publishing it.",
		"Invocation: Run `firety skill lint . --format json` from the skill root.",
		"Result: Review the reported findings and fix any broken links or weak guidance before publishing.",
	}, "\n")
}

func skillWithFrontMatter(name, description, body string) string {
	return strings.Join([]string{
		"---",
		"name: " + name,
		"description: " + description,
		"---",
		body,
	}, "\n")
}

func brokenLinkSkillContent() string {
	return strings.Join([]string{
		"# Example Skill",
		"",
		"## When To Use",
		"",
		"Use this skill when you need to validate a local skill directory before publishing changes.",
		"",
		"## Usage",
		"",
		"Run `firety skill lint .` to validate this skill before publishing changes.",
		"",
		"## Limitations",
		"",
		"Do not use this skill for remote repositories or provider-specific integrations.",
		"",
		"## Examples",
		"",
		"Request: Lint this local skill directory before publishing it.",
		"Invocation: Run `firety skill lint . --format json` from the skill root.",
		"See [Missing](docs/missing.md).",
	}, "\n")
}

func portabilitySkillContent(name, description string, portabilityLines []string) string {
	return skillWithFrontMatter(
		name,
		description,
		strings.Join([]string{
			"# " + name,
			"",
			"## When To Use",
			"",
			"Use this skill when you need to validate a local skill directory before publishing changes.",
			"",
			"## Usage",
			"",
			strings.Join(portabilityLines, "\n"),
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
	)
}

func bundleSkillContent(resourceLines []string) string {
	return skillWithFrontMatter(
		"Example Skill",
		"Validates local skill bundles before publishing them.",
		strings.Join([]string{
			"# Example Skill",
			"",
			"## When To Use",
			"",
			"Use this skill when you need to validate a local skill bundle before publishing changes.",
			"",
			"## Usage",
			"",
			strings.Join(resourceLines, "\n"),
			"",
			"## Limitations",
			"",
			"Do not use this skill for remote repositories or provider-specific integrations.",
			"",
			"## Examples",
			"",
			"Request: Lint this local skill directory before publishing it.",
			"Invocation: Run `firety skill lint . --format json` from the skill root.",
			"Result: Review the reported findings and fix any bundle issues before publishing.",
		}, "\n"),
	)
}

func costSkillContent(usageLines, exampleLines []string) string {
	return skillWithFrontMatter(
		"Example Skill",
		"Validates local skill bundles before publishing them efficiently.",
		strings.Join([]string{
			"# Example Skill",
			"",
			"## When To Use",
			"",
			"Use this skill when you need to validate a local skill bundle before publishing changes.",
			"",
			"## Usage",
			"",
			strings.Join(usageLines, "\n"),
			"",
			"## Limitations",
			"",
			"Do not use this skill for remote repositories or provider-specific integrations.",
			"",
			"## Examples",
			"",
			strings.Join(exampleLines, "\n"),
		}, "\n"),
	)
}

func triggerSkillContent(name, description, title, whenToUseText, exampleText string) string {
	return skillWithFrontMatter(
		name,
		description,
		strings.Join([]string{
			title,
			"",
			"## When To Use",
			"",
			whenToUseText,
			"",
			"## Usage",
			"",
			"Run `firety skill lint . --format json` from the skill root before publishing changes.",
			"",
			"## Limitations",
			"",
			"Do not use this skill for remote repositories or provider-specific integrations.",
			"",
			"## Examples",
			"",
			exampleText,
		}, "\n"),
	)
}
