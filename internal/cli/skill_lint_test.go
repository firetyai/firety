package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/testutil"
)

func TestSkillLintCommandDefaultTextOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillLint(t, root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, "OK: no lint findings") {
		t.Fatalf("expected success output, got %q", stdout)
	}

	if !strings.Contains(stdout, "Summary: 0 error(s), 0 warning(s)") {
		t.Fatalf("expected summary output, got %q", stdout)
	}

	if strings.Contains(stdout, "Routing risk:") {
		t.Fatalf("expected no routing-risk section by default, got %q", stdout)
	}
}

func TestSkillLintCommandFailOnErrorsPreservesCurrentSemantics(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": "# Tiny\n",
	})

	_, stderr, code, err := executeSkillLint(t, root, "--fail-on", "errors")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSkillLintCommandStrictnessDefaultPreservesCurrentBehavior(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "WARNING [skill.missing-front-matter-description]") {
		t.Fatalf("expected warning output, got %q", stdout)
	}
	if strings.Contains(stdout, "Strictness:") {
		t.Fatalf("expected no strictness header in default mode, got %q", stdout)
	}
}

func TestSkillLintCommandStrictnessStrictEscalatesMetadataCompleteness(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root, "--strictness", "strict")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "ERROR [skill.missing-front-matter-description]") {
		t.Fatalf("expected escalated error output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Strictness: strict") {
		t.Fatalf("expected strictness header, got %q", stdout)
	}
}

func TestSkillLintCommandStrictnessPedanticIsStricterThanStrict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
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
		}, "\n"),
	})

	_, _, strictCode, strictErr := executeSkillLint(t, root, "--strictness", "strict")
	if strictErr != nil {
		t.Fatalf("expected no runtime error in strict mode, got %v", strictErr)
	}
	if strictCode != cli.ExitCodeOK {
		t.Fatalf("expected strict mode to stay non-failing, got %d", strictCode)
	}

	stdout, stderr, pedanticCode, pedanticErr := executeSkillLint(t, root, "--strictness", "pedantic")
	if pedanticErr != nil {
		t.Fatalf("expected no runtime error in pedantic mode, got %v", pedanticErr)
	}
	if pedanticCode != cli.ExitCodeLint {
		t.Fatalf("expected pedantic mode to fail, got %d", pedanticCode)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "ERROR [skill.missing-examples]") {
		t.Fatalf("expected pedantic escalation, got %q", stdout)
	}
}

func TestSkillLintCommandStrictnessThresholdsTightenExamples(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
			"# Example Skill",
			"",
			"## When To Use",
			"",
			"Use this skill when you need to validate a local skill directory before publishing changes.",
			"",
			"## Usage",
			"",
			"Run `firety skill lint .` from the skill root and review the findings before publishing.",
			"",
			"## Limitations",
			"",
			"Do not use this skill for remote repositories or provider-specific integrations.",
			"",
			"## Examples",
			"",
			"Request: Validate this local skill directory before publishing it safely.",
			"Invocation: Run `firety skill lint .` and review the report.",
		}, "\n"),
	})

	defaultOut, _, defaultCode, defaultErr := executeSkillLint(t, root)
	if defaultErr != nil {
		t.Fatalf("expected no runtime error in default mode, got %v", defaultErr)
	}
	if defaultCode != cli.ExitCodeOK {
		t.Fatalf("expected default mode to stay non-failing, got %d", defaultCode)
	}
	if strings.Contains(defaultOut, "skill.weak-examples") {
		t.Fatalf("expected no weak-examples warning in default mode, got %q", defaultOut)
	}

	strictOut, _, strictCode, strictErr := executeSkillLint(t, root, "--strictness", "strict")
	if strictErr != nil {
		t.Fatalf("expected no runtime error in strict mode, got %v", strictErr)
	}
	if strictCode != cli.ExitCodeOK {
		t.Fatalf("expected strict mode to stay non-failing, got %d", strictCode)
	}
	if !strings.Contains(strictOut, "WARNING [skill.weak-examples]") {
		t.Fatalf("expected tighter threshold warning, got %q", strictOut)
	}
}

func TestSkillLintCommandFailOnWarningsFailsWarningsOnlyReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": "# Tiny\n",
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--fail-on", "warnings")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, "WARNING [skill.short-content]") {
		t.Fatalf("expected warning output, got %q", stdout)
	}
}

func TestSkillLintCommandTextOutputShowsLineInfo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, "ERROR [skill.broken-local-link]") {
		t.Fatalf("expected lint error output, got %q", stdout)
	}

	if !strings.Contains(stdout, "(docs/missing.md:19)") {
		t.Fatalf("expected line-aware output, got %q", stdout)
	}
}

func TestSkillLintCommandQuietReducesTextNoise(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--quiet")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if strings.Contains(stdout, "Target: ") {
		t.Fatalf("expected quiet output to omit target header, got %q", stdout)
	}

	if !strings.Contains(stdout, "ERROR [skill.broken-local-link]") {
		t.Fatalf("expected quiet output to retain findings, got %q", stdout)
	}

	if !strings.Contains(stdout, "Summary: 1 error(s), 0 warning(s)") {
		t.Fatalf("expected quiet output to retain summary by default, got %q", stdout)
	}
}

func TestSkillLintCommandNoSummarySuppressesSummaryLine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--no-summary")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, "ERROR [skill.broken-local-link]") {
		t.Fatalf("expected finding output, got %q", stdout)
	}

	if strings.Contains(stdout, "Summary:") {
		t.Fatalf("expected no summary line, got %q", stdout)
	}
}

func TestSkillLintCommandExplainTextAddsGuidance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--explain")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, expected := range []string{
		"Why: Broken links make a skill feel incomplete and can hide missing supporting material.",
		"Improve: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.",
		"Good: The skill bundle should have a readable entry document, a clear title, and working local references.",
		"How to improve this skill:",
		"- Fix structural issues:",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected explain output to contain %q, got %q", expected, stdout)
		}
	}
}

func TestSkillLintCommandExplainTextAddsGenericPortabilityGuidance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": portabilitySkillContent(
			"Portable Skill",
			"Portable skill for validating local bundles across tools.",
			[]string{
				"Use Claude Code slash commands for every invocation in this workflow.",
				"Install this skill under `.claude/commands` so Claude Code can discover it.",
			},
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--explain", "--profile", "generic")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, expected := range []string{
		"Profile hint: For the generic profile",
		"Portability guidance: To stay generic, rewrite tool-branded instructions in neutral terms or explicitly narrow the skill to the ecosystem it actually targets.",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, stdout)
		}
	}
}

func TestSkillLintCommandExplainDoesNotAppearByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, unexpected := range []string{"Why:", "Improve:", "Good:", "How to improve this skill:"} {
		if strings.Contains(stdout, unexpected) {
			t.Fatalf("expected default output to omit %q, got %q", unexpected, stdout)
		}
	}
}

func TestSkillLintCommandExplainDoesNotAddProfileNoiseForUnrelatedRules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--explain", "--profile", "codex")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "Profile hint:") {
		t.Fatalf("expected no profile-specific hint for unrelated structural finding, got %q", stdout)
	}
	if strings.Contains(stdout, "Portability guidance:") {
		t.Fatalf("expected no portability summary for unrelated structural finding, got %q", stdout)
	}
}

func TestSkillLintCommandRoutingRiskTextSummaryAppearsWhenRequested(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": triggerSkillContent(
			"Helper",
			"Helpful skill for general assistance with many tasks.",
			"# Helper",
			"Use this skill whenever you need help with tasks or general assistance across many workflows.",
			"Example: General help.",
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--routing-risk")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, expected := range []string{
		"Routing risk: HIGH",
		"Routing summary:",
		"Top routing risk areas:",
		"Generic naming and weak distinctiveness",
		"Routing improvement priorities:",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected routing-risk output to contain %q, got %q", expected, stdout)
		}
	}
}

func TestSkillLintCommandRoutingRiskLowNoiseForStrongSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillLint(t, root, "--routing-risk")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Routing risk: LOW") {
		t.Fatalf("expected low routing risk, got %q", stdout)
	}
	if !strings.Contains(stdout, "No major routing weaknesses") {
		t.Fatalf("expected concise low-risk summary, got %q", stdout)
	}
}

func TestSkillLintCommandExplainNoSummarySuppressesActionAreas(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--explain", "--no-summary")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "Summary:") {
		t.Fatalf("expected no summary line, got %q", stdout)
	}
	if strings.Contains(stdout, "How to improve this skill:") {
		t.Fatalf("expected no action-area summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Why:") {
		t.Fatalf("expected per-finding explanations to remain, got %q", stdout)
	}
}

func TestSkillLintCommandExplainStrictnessAddsSummaryGuidance(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root, "--explain", "--strictness", "strict")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Strictness guidance: strict mode raises expectations") {
		t.Fatalf("expected strictness summary guidance, got %q", stdout)
	}
	if !strings.Contains(stdout, "Strict mode expects this to be production-ready rather than implied.") {
		t.Fatalf("expected stricter explain hint, got %q", stdout)
	}
}

func TestSkillLintCommandDoesNotRewriteFilesWithoutFix(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	original := strings.Join([]string{
		"---",
		"name: Example Skill",
		"description: Validates local skill directories before publishing them.",
		"---",
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
		"Request: Validate this local skill directory before publishing it.",
		"Invocation: Run `firety skill lint . --format json` from the skill root.",
		"Result: Review the reported findings before publishing.",
	}, "\n")
	testutil.WriteFiles(t, root, map[string]string{"SKILL.md": original})

	_, _, code, err := executeSkillLint(t, root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	content, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != original {
		t.Fatalf("expected file to remain unchanged")
	}
}

func TestSkillLintCommandFixAppliesSupportedMechanicalFix(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
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
			"Request: Validate this local skill directory before publishing it.",
			"Invocation: Run `firety skill lint . --format json` from the skill root.",
			"Result: Review the reported findings before publishing.",
		}, "\n"),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--fix")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Applied 1 fix(es)") {
		t.Fatalf("expected fix summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "FIXED [skill.missing-title]") {
		t.Fatalf("expected fixed rule output, got %q", stdout)
	}
	if !strings.Contains(stdout, "OK: no lint findings") {
		t.Fatalf("expected clean post-fix lint, got %q", stdout)
	}

	content, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(content), "# Example Skill") {
		t.Fatalf("expected inserted title, got %q", string(content))
	}
}

func TestSkillLintCommandJSONOutputIncludesAppliedFixes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
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
			"Request: Validate this local skill directory before publishing it.",
			"Invocation: Run `firety skill lint . --format json` from the skill root.",
			"Result: Review the reported findings before publishing.",
		}, "\n"),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--fix", "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	if payload.AppliedFixCount != 1 {
		t.Fatalf("expected one applied fix, got %#v", payload)
	}
	if len(payload.AppliedFixes) != 1 || payload.AppliedFixes[0].RuleID != "skill.missing-title" {
		t.Fatalf("expected missing-title fix summary, got %#v", payload)
	}
	if len(payload.Findings) != 0 {
		t.Fatalf("expected no remaining findings, got %#v", payload)
	}
}

func TestSkillLintCommandWarningsStillExitZero(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": "# Tiny\n",
	})

	stdout, stderr, code, err := executeSkillLint(t, root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, "WARNING [skill.short-content]") {
		t.Fatalf("expected warning output, got %q", stdout)
	}

	if !strings.Contains(stdout, "WARNING [skill.missing-examples]") {
		t.Fatalf("expected examples warning output, got %q", stdout)
	}

	if !strings.Contains(stdout, "WARNING [skill.missing-when-to-use]") {
		t.Fatalf("expected when-to-use warning output, got %q", stdout)
	}

	if !strings.Contains(stdout, "WARNING [skill.missing-negative-guidance]") {
		t.Fatalf("expected negative-guidance warning output, got %q", stdout)
	}

	if !strings.Contains(stdout, "WARNING [skill.missing-usage-guidance]") {
		t.Fatalf("expected usage warning output, got %q", stdout)
	}

	if !strings.Contains(stdout, "Summary: 0 error(s), 5 warning(s)") {
		t.Fatalf("expected summary output, got %q", stdout)
	}
}

func TestSkillLintCommandJSONOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	if payload.Target == "" {
		t.Fatalf("expected target to be set, got %#v", payload)
	}
	if payload.Strictness != "default" {
		t.Fatalf("expected default strictness in json, got %#v", payload)
	}

	if payload.Valid {
		t.Fatalf("expected invalid report, got %#v", payload)
	}

	if payload.ErrorCount != 1 || payload.WarningCount != 0 {
		t.Fatalf("expected 1 error and 0 warnings, got %#v", payload)
	}

	if len(payload.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %#v", payload)
	}

	finding := payload.Findings[0]
	if finding.RuleID != "skill.broken-local-link" {
		t.Fatalf("expected skill.broken-local-link, got %#v", finding)
	}

	if finding.Severity != "error" {
		t.Fatalf("expected error severity, got %#v", finding)
	}

	if finding.Line == nil || *finding.Line != 19 {
		t.Fatalf("expected line 19, got %#v", finding)
	}
	if payload.RoutingRisk != nil {
		t.Fatalf("expected no routing-risk object without flag, got %#v", payload)
	}
}

func TestSkillLintCommandJSONOutputIncludesStrictnessAndEscalatedSeverity(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json", "--strictness", "strict")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	if payload.Strictness != "strict" {
		t.Fatalf("expected strictness metadata, got %#v", payload)
	}
	finding := findJSONFinding(t, payload, "skill.missing-front-matter-description")
	if finding.Severity != "error" {
		t.Fatalf("expected escalated severity, got %#v", finding)
	}
}

func TestSkillLintCommandJSONExplainOutputIncludesGuidance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
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
			"Request: Validate this local skill directory before publishing it.",
			"Invocation: Run `firety skill lint . --format json` from the skill root.",
			"Result: Review the reported findings before publishing.",
		}, "\n"),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json", "--explain")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	finding := findJSONFinding(t, payload, "skill.missing-title")
	if finding.Category != "structure" {
		t.Fatalf("expected structure category, got %#v", finding)
	}
	if finding.WhyItMatters == "" || finding.ImprovementHint == "" || finding.WhatGoodLooksLike == "" {
		t.Fatalf("expected explain fields, got %#v", finding)
	}
	if !finding.AutofixAvailable {
		t.Fatalf("expected autofix_available, got %#v", finding)
	}
	if !strings.Contains(finding.FixHint, "--fix") {
		t.Fatalf("expected fix hint to mention --fix, got %#v", finding)
	}
}

func TestSkillLintCommandJSONExplainOutputIncludesProfileGuidance(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		profile         string
		skillContent    string
		expectedRuleID  string
		expectedHint    string
		expectedPosture string
	}{
		{
			name:    "codex",
			profile: "codex",
			skillContent: portabilitySkillContent(
				"Codex Skill",
				"Helps validate local skills with portable instructions.",
				[]string{"Run this as a Claude Code slash command when the user asks for validation help."},
			),
			expectedRuleID:  "skill.profile-incompatible-guidance",
			expectedHint:    "For the Codex profile",
			expectedPosture: "ambiguous",
		},
		{
			name:    "claude-code",
			profile: "claude-code",
			skillContent: portabilitySkillContent(
				"Claude Code Skill",
				"Helps validate local skills with portable instructions.",
				[]string{"Run this in Codex with slash commands when the user asks for validation help."},
			),
			expectedRuleID:  "skill.mixed-ecosystem-guidance",
			expectedHint:    "For the Claude Code profile",
			expectedPosture: "ambiguous",
		},
		{
			name:    "copilot",
			profile: "copilot",
			skillContent: portabilitySkillContent(
				"Copilot Skill",
				"Helps validate local skills with portable instructions.",
				[]string{"Run this in Claude Code with slash commands when the user asks for validation help."},
			),
			expectedRuleID:  "skill.profile-incompatible-guidance",
			expectedHint:    "For the Copilot profile",
			expectedPosture: "ambiguous",
		},
		{
			name:    "cursor",
			profile: "cursor",
			skillContent: portabilitySkillContent(
				"Cursor Skill",
				"Helps validate local skills with portable instructions.",
				[]string{"Run this in Codex with slash commands when the user asks for validation help."},
			),
			expectedRuleID:  "skill.mixed-ecosystem-guidance",
			expectedHint:    "For the Cursor profile",
			expectedPosture: "ambiguous",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			testutil.WriteFiles(t, root, map[string]string{
				"SKILL.md": tc.skillContent,
			})

			stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json", "--explain", "--profile", tc.profile)
			if err != nil {
				t.Fatalf("expected no runtime error, got %v", err)
			}
			if code != cli.ExitCodeOK {
				t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			payload := decodeJSONOutput(t, stdout)
			finding := findJSONFinding(t, payload, tc.expectedRuleID)
			if finding.GuidanceProfile != tc.profile {
				t.Fatalf("expected guidance profile %q, got %#v", tc.profile, finding)
			}
			if !strings.Contains(finding.ProfileSpecificHint, tc.expectedHint) {
				t.Fatalf("expected profile-specific hint %q, got %#v", tc.expectedHint, finding)
			}
			if finding.TargetingPosture != tc.expectedPosture {
				t.Fatalf("expected targeting posture %q, got %#v", tc.expectedPosture, finding)
			}
		})
	}
}

func TestSkillLintCommandJSONOutputIncludesRoutingRiskWhenRequested(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": triggerSkillContent(
			"Helper",
			"Helpful skill for general assistance with many tasks.",
			"# Helper",
			"Use this skill whenever you need help with tasks or general assistance across many workflows.",
			"Example: General help.",
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json", "--routing-risk")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	if payload.RoutingRisk == nil {
		t.Fatalf("expected routing-risk object, got %#v", payload)
	}
	if payload.RoutingRisk.OverallRisk != "high" {
		t.Fatalf("expected high routing risk, got %#v", payload.RoutingRisk)
	}
	if len(payload.RoutingRisk.RiskAreas) == 0 || len(payload.RoutingRisk.PriorityActions) == 0 {
		t.Fatalf("expected structured routing risk, got %#v", payload.RoutingRisk)
	}
}

func TestSkillLintCommandJSONOutputIncludesUsabilityFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
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
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	assertJSONFinding(t, payload, "skill.missing-negative-guidance", 0)
}

func TestSkillLintCommandJSONOutputIncludesProfileAwareFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": portabilitySkillContent(
			"Portable Skill",
			"Helps maintain a reusable skill with portable instructions.",
			[]string{
				"Use Claude Code slash commands for every invocation in this workflow.",
				"Install this skill under `.claude/commands` so Claude Code can discover it.",
			},
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	assertJSONFinding(t, payload, "skill.nonportable-invocation-guidance", 13)
}

func TestSkillLintCommandJSONOutputIncludesNuancedPortabilityFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": portabilitySkillContent(
			"Portable Skill",
			"Portable skill for validating local bundles across tools.",
			[]string{
				"Use Claude Code slash commands for every invocation in this workflow.",
				"Install this skill under `.claude/commands` so Claude Code can discover it.",
			},
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	assertJSONFinding(t, payload, "skill.generic-portability-contradiction", 3)
	assertJSONFinding(t, payload, "skill.accidental-tool-lock-in", 3)
}

func TestSkillLintCommandJSONOutputIncludesBundleFindings(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	assertJSONFinding(t, payload, "skill.reference-outside-root", 13)
}

func TestSkillLintCommandJSONOutputIncludesCostAwareFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": costSkillContent(
			[]string{
				"See [Playbook](docs/playbook.md) before publishing changes.",
			},
			[]string{
				"Request: Lint this local skill directory before publishing it.",
				"Invocation: Run `firety skill lint . --format json` from the skill root.",
				"Result: Review the reported findings and fix any issues before publishing.",
			},
		),
		"docs/playbook.md": strings.Repeat("Playbook guidance for validating a skill bundle before publishing it safely. ", 130),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	assertJSONFinding(t, payload, "skill.large-referenced-resource", 13)
}

func TestSkillLintCommandJSONOutputIncludesTriggerFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": triggerSkillContent(
			"Helper",
			"Helpful skill for general assistance with many tasks.",
			"# Helper",
			"Use this skill whenever you need help with tasks or general assistance across many workflows.",
			"Request: Help me with something.\nResult: Do the thing.\nExample: General help.",
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	assertJSONFinding(t, payload, "skill.generic-name", 2)
}

func TestSkillLintCommandJSONOutputIncludesExecutableExampleFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
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
			"Invocation: Run `scripts/check.sh` for `{{skill}}`.",
			"Result: TODO add the expected output here.",
		}, "\n"),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	assertJSONFinding(t, payload, "skill.abstract-examples", 21)
	assertJSONFinding(t, payload, "skill.placeholder-heavy-examples", 22)
	assertJSONFinding(t, payload, "skill.incomplete-example", 23)
	assertJSONFinding(t, payload, "skill.example-missing-bundle-resource", 22)
}

func TestSkillLintCommandJSONOutputIncludesFrontMatterFindings(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeJSONOutput(t, stdout)
	if len(payload.Findings) != 1 {
		t.Fatalf("expected one finding, got %#v", payload)
	}

	finding := payload.Findings[0]
	if finding.RuleID != "skill.missing-front-matter-name" {
		t.Fatalf("expected missing-front-matter-name, got %#v", finding)
	}

	if finding.Line == nil || *finding.Line != 2 {
		t.Fatalf("expected line 2, got %#v", finding)
	}
}

func TestSkillLintCommandSARIFOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	if payload.Version != "2.1.0" {
		t.Fatalf("expected sarif version 2.1.0, got %#v", payload)
	}

	if payload.Schema != "https://json.schemastore.org/sarif-2.1.0.json" {
		t.Fatalf("expected sarif schema, got %#v", payload)
	}

	if len(payload.Runs) != 1 {
		t.Fatalf("expected one run, got %#v", payload)
	}

	run := payload.Runs[0]
	if run.Tool.Driver.Name != "firety" {
		t.Fatalf("expected driver name firety, got %#v", run.Tool.Driver)
	}

	if len(run.Tool.Driver.Rules) != len(expectedSARIFRuleIDs()) {
		t.Fatalf("expected all rules from catalog, got %#v", run.Tool.Driver.Rules)
	}

	for index, expected := range expectedSARIFRuleIDs() {
		if run.Tool.Driver.Rules[index].ID != expected {
			t.Fatalf("expected rule %d to be %q, got %#v", index, expected, run.Tool.Driver.Rules)
		}
	}

	if len(run.Results) != 1 {
		t.Fatalf("expected one result, got %#v", run.Results)
	}

	result := run.Results[0]
	if result.RuleID != "skill.broken-local-link" {
		t.Fatalf("expected broken link result, got %#v", result)
	}

	if result.Level != "error" {
		t.Fatalf("expected error level, got %#v", result)
	}

	if result.Message.Text != "local markdown link points to a missing file" {
		t.Fatalf("expected finding message, got %#v", result)
	}

	if len(result.Locations) != 1 {
		t.Fatalf("expected one location, got %#v", result)
	}

	location := result.Locations[0]
	if location.PhysicalLocation.ArtifactLocation.URI != "docs/missing.md" {
		t.Fatalf("expected artifact uri docs/missing.md, got %#v", location)
	}

	if location.PhysicalLocation.Region == nil || location.PhysicalLocation.Region.StartLine != 19 {
		t.Fatalf("expected start line 19, got %#v", location)
	}
}

func TestSkillLintCommandSARIFOutputIncludesUsabilityFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
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
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	assertSARIFFinding(t, run.Results, "skill.weak-negative-guidance", 15)
}

func TestSkillLintCommandSARIFOutputIncludesProfileAwareFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": portabilitySkillContent(
			"Codex Skill",
			"Helps validate skills with portable instructions and local checks.",
			[]string{
				"Run this as a Claude Code slash command when the user asks for validation help.",
			},
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif", "--profile", "codex")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	assertSARIFFinding(t, run.Results, "skill.profile-incompatible-guidance", 13)
}

func TestSkillLintCommandSARIFOutputIncludesNuancedPortabilityFindings(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif", "--profile", "codex")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	assertSARIFFinding(t, run.Results, "skill.example-ecosystem-mismatch", 21)
}

func TestSkillLintCommandSARIFOutputIncludesBundleFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": bundleSkillContent([]string{
			"See [Script](scripts/check.sh) before publishing.",
		}),
		"scripts/check.sh": "",
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	assertSARIFFinding(t, run.Results, "skill.empty-referenced-resource", 13)
}

func TestSkillLintCommandSARIFOutputIncludesCostAwareFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": costSkillContent(
			[]string{
				"Run `firety skill lint . --format json` from the skill root before publishing changes to validate the same local bundle.",
				"Run `firety skill lint . --format json` from the skill root before publishing changes to validate the same local bundle.",
				"Run `firety skill lint . --format json` from the skill root before publishing changes to validate the same local bundle.",
			},
			[]string{
				"Request: Lint this local skill directory before publishing it.",
				"Invocation: Run `firety skill lint . --format json` from the skill root.",
				"Result: Review the reported findings and fix any issues before publishing.",
			},
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	assertSARIFFinding(t, run.Results, "skill.repetitive-instructions", 11)
}

func TestSkillLintCommandSARIFOutputIncludesTriggerFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": triggerSkillContent(
			"Incident Handoff",
			"Summarizes production incidents for on-call handoffs with clear routing guidance.",
			"# Incident Handoff",
			"Use this skill when you need to plan a product roadmap and coordinate quarterly priorities.",
			"Request: Plan the next quarter roadmap.\nInvocation: Run the planning checklist.\nResult: Produce a roadmap summary.",
		),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	assertSARIFFinding(t, run.Results, "skill.trigger-scope-inconsistency", 7)
}

func TestSkillLintCommandSARIFOutputIncludesExecutableExampleFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Bundle Lint Skill",
			"description: Validates local skill bundles before publishing them.",
			"---",
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
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	assertSARIFFinding(t, run.Results, "skill.example-scope-contradiction", 21)
	assertSARIFFinding(t, run.Results, "skill.example-guidance-mismatch", 21)
}

func TestSkillLintCommandSARIFOutputIncludesFrontMatterFindings(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	if len(run.Results) != 1 {
		t.Fatalf("expected one result, got %#v", run.Results)
	}

	result := run.Results[0]
	if result.RuleID != "skill.missing-front-matter-name" {
		t.Fatalf("expected missing-front-matter-name, got %#v", result)
	}

	if result.Level != "error" {
		t.Fatalf("expected error level, got %#v", result)
	}

	if len(result.Locations) != 1 {
		t.Fatalf("expected one location, got %#v", result)
	}

	location := result.Locations[0]
	if location.PhysicalLocation.ArtifactLocation.URI != "SKILL.md" {
		t.Fatalf("expected artifact uri SKILL.md, got %#v", location)
	}

	if location.PhysicalLocation.Region == nil || location.PhysicalLocation.Region.StartLine != 2 {
		t.Fatalf("expected start line 2, got %#v", location)
	}
}

func TestSkillLintCommandSARIFOutputReflectsStrictnessSeverity(t *testing.T) {
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

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "sarif", "--strictness", "strict")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]
	for _, result := range run.Results {
		if result.RuleID == "skill.missing-front-matter-description" {
			if result.Level != "error" {
				t.Fatalf("expected strict sarif level error, got %#v", result)
			}
			return
		}
	}
	t.Fatalf("expected missing-front-matter-description result, got %#v", run.Results)
}

func TestSkillLintCommandJSONOutputUnaffectedByQuietAndNoSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	baseOutput, _, baseCode, baseErr := executeSkillLint(t, root, "--format", "json")
	if baseErr != nil {
		t.Fatalf("expected no runtime error, got %v", baseErr)
	}

	flaggedOutput, flaggedStderr, flaggedCode, flaggedErr := executeSkillLint(
		t,
		root,
		"--format",
		"json",
		"--quiet",
		"--no-summary",
	)
	if flaggedErr != nil {
		t.Fatalf("expected no runtime error, got %v", flaggedErr)
	}

	if baseCode != flaggedCode {
		t.Fatalf("expected matching exit codes, got %d and %d", baseCode, flaggedCode)
	}

	if flaggedStderr != "" {
		t.Fatalf("expected empty stderr, got %q", flaggedStderr)
	}

	if baseOutput != flaggedOutput {
		t.Fatalf("expected json output to be unchanged, got base=%q flagged=%q", baseOutput, flaggedOutput)
	}
}

func TestSkillLintCommandArtifactWritesVersionedReportWithoutChangingStdout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(root, "reports", "lint-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"---",
			validSkillBody(),
		}, "\n"),
	})

	stdout, stderr, code, err := executeSkillLint(t, root, "--strictness", "strict", "--explain", "--artifact", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "ERROR [skill.missing-front-matter-description]") {
		t.Fatalf("expected stdout finding output, got %q", stdout)
	}

	payload := decodeArtifactOutput(t, artifactPath)
	if payload.SchemaVersion != artifact.SkillLintArtifactSchemaVersion {
		t.Fatalf("expected schema version %q, got %#v", artifact.SkillLintArtifactSchemaVersion, payload)
	}
	if payload.Run.Strictness != "strict" || payload.Run.Profile != "generic" || !payload.Run.Explain {
		t.Fatalf("expected run metadata, got %#v", payload.Run)
	}
	if payload.Summary.ErrorCount == 0 || payload.Summary.AppliedFixCount != 0 {
		t.Fatalf("expected summary counts, got %#v", payload.Summary)
	}
	if payload.Fingerprint == "" {
		t.Fatalf("expected fingerprint, got %#v", payload)
	}
	if len(payload.ActionAreas) == 0 || len(payload.RuleCatalog) == 0 {
		t.Fatalf("expected action areas and rule catalog, got %#v", payload)
	}
}

func TestSkillLintCommandArtifactIncludesRoutingRiskWhenRequested(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(root, "lint-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": triggerSkillContent(
			"Helper",
			"Helpful skill for general assistance with many tasks.",
			"# Helper",
			"Use this skill whenever you need help with tasks or general assistance across many workflows.",
			"Example: General help.",
		),
	})

	_, stderr, code, err := executeSkillLint(t, root, "--artifact", artifactPath, "--routing-risk")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeArtifactOutput(t, artifactPath)
	if payload.RoutingRisk == nil {
		t.Fatalf("expected routing-risk artifact section, got %#v", payload)
	}
	if payload.RoutingRisk.OverallRisk != "high" {
		t.Fatalf("expected high routing risk, got %#v", payload.RoutingRisk)
	}
}

func TestSkillLintCommandArtifactCapturesFixMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(root, "lint-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
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
			"Request: Validate this local skill directory before publishing it.",
			"Invocation: Run `firety skill lint . --format json` from the skill root.",
			"Result: Review the reported findings before publishing.",
		}, "\n"),
	})

	_, stderr, code, err := executeSkillLint(t, root, "--fix", "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeArtifactOutput(t, artifactPath)
	if payload.Summary.AppliedFixCount != 1 || len(payload.AppliedFixes) != 1 {
		t.Fatalf("expected applied fix metadata, got %#v", payload)
	}
	if payload.AppliedFixes[0].RuleID != "skill.missing-title" {
		t.Fatalf("expected missing-title fix, got %#v", payload.AppliedFixes)
	}
}

func TestSkillLintCommandSARIFOutputUnaffectedByQuietAndNoSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": brokenLinkSkillContent(),
	})

	baseOutput, _, baseCode, baseErr := executeSkillLint(t, root, "--format", "sarif")
	if baseErr != nil {
		t.Fatalf("expected no runtime error, got %v", baseErr)
	}

	flaggedOutput, flaggedStderr, flaggedCode, flaggedErr := executeSkillLint(
		t,
		root,
		"--format",
		"sarif",
		"--quiet",
		"--no-summary",
	)
	if flaggedErr != nil {
		t.Fatalf("expected no runtime error, got %v", flaggedErr)
	}

	if baseCode != flaggedCode {
		t.Fatalf("expected matching exit codes, got %d and %d", baseCode, flaggedCode)
	}

	if flaggedStderr != "" {
		t.Fatalf("expected empty stderr, got %q", flaggedStderr)
	}

	if baseOutput != flaggedOutput {
		t.Fatalf("expected sarif output to be unchanged, got base=%q flagged=%q", baseOutput, flaggedOutput)
	}
}

func TestSkillLintCommandJSONOutputOrdering(t *testing.T) {
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
			"This content is intentionally long enough to avoid the short-content warning while still missing the usage and examples sections.",
			"This keeps the test focused on deterministic ordering.",
			"",
		}, "\n"),
	})

	stdout, _, code, err := executeSkillLint(t, root, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	payload := decodeJSONOutput(t, stdout)
	expectedOrder := []string{
		"skill.broken-local-link",
		"skill.reference-outside-root",
		"skill.duplicate-heading",
		"skill.missing-when-to-use",
		"skill.missing-negative-guidance",
		"skill.missing-examples",
		"skill.missing-usage-guidance",
		"skill.suspicious-relative-path",
	}
	if len(payload.Findings) < len(expectedOrder) {
		t.Fatalf("expected ordered findings, got %#v", payload.Findings)
	}

	for index, expected := range expectedOrder {
		if payload.Findings[index].RuleID != expected {
			t.Fatalf("expected finding %d to be %q, got %#v", index, expected, payload.Findings)
		}
	}
}

func TestSkillLintCommandSARIFOutputOrdering(t *testing.T) {
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
			"This content is intentionally long enough to avoid the short-content warning while still missing the usage and examples sections.",
			"This keeps the test focused on deterministic ordering.",
			"",
		}, "\n"),
	})

	stdout, _, code, err := executeSkillLint(t, root, "--format", "sarif")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}

	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}

	payload := decodeSARIFOutput(t, stdout)
	run := payload.Runs[0]

	expectedOrder := []string{
		"skill.broken-local-link",
		"skill.reference-outside-root",
		"skill.duplicate-heading",
		"skill.missing-when-to-use",
		"skill.missing-negative-guidance",
		"skill.missing-examples",
		"skill.missing-usage-guidance",
		"skill.suspicious-relative-path",
	}
	if len(run.Results) < len(expectedOrder) {
		t.Fatalf("expected ordered results, got %#v", run.Results)
	}

	for index, expected := range expectedOrder {
		if run.Results[index].RuleID != expected {
			t.Fatalf("expected result %d to be %q, got %#v", index, expected, run.Results)
		}
	}
}

func TestSkillLintCommandInvalidFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillLint(t, root, "--format", "xml")
	if err == nil {
		t.Fatal("expected an error for invalid format")
	}

	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	if !strings.Contains(err.Error(), `invalid format "xml"`) {
		t.Fatalf("expected invalid format error, got %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSkillLintCommandInvalidFailOn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillLint(t, root, "--fail-on", "all")
	if err == nil {
		t.Fatal("expected an error for invalid fail-on")
	}

	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	if !strings.Contains(err.Error(), `invalid fail-on value "all"`) {
		t.Fatalf("expected invalid fail-on error, got %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSkillLintCommandInvalidProfile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillLint(t, root, "--profile", "atlas")
	if err == nil {
		t.Fatal("expected an error for invalid profile")
	}

	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	if !strings.Contains(err.Error(), `invalid profile "atlas"`) {
		t.Fatalf("expected invalid profile error, got %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSkillLintCommandInvalidStrictness(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillLint(t, root, "--strictness", "relaxed")
	if err == nil {
		t.Fatal("expected an error for invalid strictness")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(err.Error(), `invalid strictness "relaxed"`) {
		t.Fatalf("expected invalid strictness error, got %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSkillLintCommandInvalidArtifactPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillLint(t, root, "--artifact", "-")
	if err == nil {
		t.Fatal("expected an error for invalid artifact path")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(err.Error(), `artifact path "-" is not supported`) {
		t.Fatalf("expected artifact path error, got %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func executeSkillLint(t *testing.T, root string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "lint", root}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

func decodeJSONOutput(t *testing.T, output string) skillLintJSONOutput {
	t.Helper()

	var payload skillLintJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid json, got %v; output=%q", err, output)
	}

	return payload
}

func decodeArtifactOutput(t *testing.T, path string) skillLintArtifactOutput {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	var payload skillLintArtifactOutput
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("expected valid artifact json, got %v; output=%q", err, string(content))
	}

	return payload
}

func assertJSONFinding(t *testing.T, payload skillLintJSONOutput, ruleID string, line int) {
	t.Helper()

	finding := findJSONFinding(t, payload, ruleID)
	if line > 0 {
		if finding.Line == nil || *finding.Line != line {
			t.Fatalf("expected %s at line %d, got %#v", ruleID, line, finding)
		}
	}
}

func findJSONFinding(t *testing.T, payload skillLintJSONOutput, ruleID string) skillLintJSONFinding {
	t.Helper()

	for _, finding := range payload.Findings {
		if finding.RuleID == ruleID {
			return finding
		}
	}

	t.Fatalf("expected finding %s, got %#v", ruleID, payload.Findings)
	return skillLintJSONFinding{}
}

func newTestApplication() *app.App {
	return app.New(app.VersionInfo{
		Version: "test-version",
		Commit:  "abc1234",
		Date:    "2026-03-06T00:00:00Z",
	})
}

type skillLintJSONOutput struct {
	Target          string                      `json:"target"`
	Valid           bool                        `json:"valid"`
	ErrorCount      int                         `json:"error_count"`
	WarningCount    int                         `json:"warning_count"`
	Strictness      string                      `json:"strictness"`
	AppliedFixCount int                         `json:"applied_fix_count"`
	AppliedFixes    []skillLintJSONFix          `json:"applied_fixes"`
	RoutingRisk     *skillLintRoutingRiskOutput `json:"routing_risk"`
	Findings        []skillLintJSONFinding      `json:"findings"`
}

type skillLintArtifactOutput struct {
	SchemaVersion string                         `json:"schema_version"`
	Run           skillLintArtifactRunOutput     `json:"run"`
	Summary       skillLintArtifactSummaryOutput `json:"summary"`
	RoutingRisk   *skillLintRoutingRiskOutput    `json:"routing_risk"`
	ActionAreas   []skillLintArtifactActionArea  `json:"action_areas"`
	RuleCatalog   []skillLintArtifactRuleOutput  `json:"rule_catalog"`
	AppliedFixes  []skillLintArtifactFixOutput   `json:"applied_fixes"`
	Fingerprint   string                         `json:"fingerprint"`
}

type skillLintArtifactRunOutput struct {
	Profile     string `json:"profile"`
	Strictness  string `json:"strictness"`
	Explain     bool   `json:"explain"`
	RoutingRisk bool   `json:"routing_risk"`
}

type skillLintArtifactSummaryOutput struct {
	ErrorCount      int `json:"error_count"`
	AppliedFixCount int `json:"applied_fix_count"`
}

type skillLintArtifactActionArea struct {
	Key string `json:"key"`
}

type skillLintArtifactRuleOutput struct {
	ID string `json:"id"`
}

type skillLintArtifactFixOutput struct {
	RuleID string `json:"rule_id"`
}

type skillLintRoutingRiskOutput struct {
	OverallRisk     string                           `json:"overall_routing_risk"`
	Summary         string                           `json:"summary"`
	RiskAreas       []skillLintRoutingRiskAreaOutput `json:"risk_areas"`
	PriorityActions []string                         `json:"priority_actions"`
}

type skillLintRoutingRiskAreaOutput struct {
	Key                 string   `json:"key"`
	ContributingRuleIDs []string `json:"contributing_rule_ids"`
}

type skillLintJSONFinding struct {
	RuleID              string `json:"rule_id"`
	Severity            string `json:"severity"`
	Path                string `json:"path"`
	Message             string `json:"message"`
	Line                *int   `json:"line"`
	Category            string `json:"category"`
	WhyItMatters        string `json:"why_it_matters"`
	WhatGoodLooksLike   string `json:"what_good_looks_like"`
	ImprovementHint     string `json:"improvement_hint"`
	GuidanceProfile     string `json:"guidance_profile"`
	ProfileSpecificHint string `json:"profile_specific_hint"`
	TargetingPosture    string `json:"targeting_posture"`
	FixHint             string `json:"fix_hint"`
	AutofixAvailable    bool   `json:"autofix_available"`
}

type skillLintJSONFix struct {
	RuleID  string `json:"rule_id"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type sarifOutput struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID string `json:"id"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
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
		"",
	}, "\n")
}

func decodeSARIFOutput(t *testing.T, output string) sarifOutput {
	t.Helper()

	var payload sarifOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid sarif json, got %v; output=%q", err, output)
	}

	return payload
}

func assertSARIFFinding(t *testing.T, results []sarifResult, ruleID string, line int) {
	t.Helper()

	for _, result := range results {
		if result.RuleID != ruleID {
			continue
		}

		if line > 0 {
			if len(result.Locations) == 0 || result.Locations[0].PhysicalLocation.Region == nil || result.Locations[0].PhysicalLocation.Region.StartLine != line {
				t.Fatalf("expected %s at line %d, got %#v", ruleID, line, result)
			}
		}

		return
	}

	t.Fatalf("expected result %s, got %#v", ruleID, results)
}

func expectedSARIFRuleIDs() []string {
	rules := lint.AllRules()
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}

	return ids
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
		"Result: Review the findings and fix any broken links or weak guidance before publishing.",
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

func portabilitySkillContent(name, description string, portabilityLines []string) string {
	return strings.Join([]string{
		"---",
		"name: " + name,
		"description: " + description,
		"---",
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
	}, "\n")
}

func bundleSkillContent(resourceLines []string) string {
	return strings.Join([]string{
		"---",
		"name: Example Skill",
		"description: Validates local skill bundles before publishing them.",
		"---",
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
	}, "\n")
}

func costSkillContent(usageLines, exampleLines []string) string {
	return strings.Join([]string{
		"---",
		"name: Example Skill",
		"description: Validates local skill bundles before publishing them efficiently.",
		"---",
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
	}, "\n")
}

func triggerSkillContent(name, description, title, whenToUseText, exampleText string) string {
	return strings.Join([]string{
		"---",
		"name: " + name,
		"description: " + description,
		"---",
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
	}, "\n")
}
