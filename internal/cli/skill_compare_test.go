package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/testutil"
)

func TestSkillCompareCommandImproved(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"---",
			validSkillBody(),
		}, "\n"),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates reusable skill directories before publishing changes or sharing them with other developers.",
			"---",
			validSkillBody(),
		}, "\n"),
	})

	stdout, stderr, code, err := executeSkillCompare(t, base, candidate)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: IMPROVED") {
		t.Fatalf("expected improved summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Resolved findings") || !strings.Contains(stdout, "skill.missing-front-matter-description") {
		t.Fatalf("expected resolved metadata finding, got %q", stdout)
	}
}

func TestSkillCompareCommandRegressedWithRoutingRisk(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	testutil.WriteFiles(t, base, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidate, map[string]string{
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
	})

	stdout, stderr, code, err := executeSkillCompare(t, base, candidate, "--routing-risk")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: REGRESSED") {
		t.Fatalf("expected regressed summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Routing risk: REGRESSED (low -> high)") {
		t.Fatalf("expected routing-risk delta, got %q", stdout)
	}
	if !strings.Contains(stdout, "skill.generic-name") {
		t.Fatalf("expected added trigger finding, got %q", stdout)
	}
}

func TestSkillCompareCommandMixed(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"---",
			validSkillBody(),
		}, "\n"),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Helps maintain a reusable skill with portable instructions.",
			"---",
			"# Example Skill",
			"",
			"## When To Use",
			"",
			"Use this skill when you need to validate a local skill directory before publishing changes.",
			"",
			"## Usage",
			"",
			"Use Claude Code slash commands for every invocation in this workflow.",
			"Install this skill under the .claude/commands directory so Claude Code can discover it.",
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
	})

	stdout, stderr, code, err := executeSkillCompare(t, base, candidate)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: MIXED") {
		t.Fatalf("expected mixed summary, got %q", stdout)
	}
}

func TestSkillCompareCommandUnchanged(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	files := testutil.ValidSkillFiles()
	testutil.WriteFiles(t, base, files)
	testutil.WriteFiles(t, candidate, files)

	stdout, stderr, code, err := executeSkillCompare(t, base, candidate)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: UNCHANGED") {
		t.Fatalf("expected unchanged summary, got %q", stdout)
	}
}

func TestSkillCompareCommandJSONExplainOutput(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	testutil.WriteFiles(t, base, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidate, map[string]string{
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
	})

	stdout, stderr, code, err := executeSkillCompare(t, base, candidate, "--format", "json", "--routing-risk", "--explain")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSkillCompareJSONOutput(t, stdout)
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if payload.Summary.Overall != "regressed" {
		t.Fatalf("expected regressed summary, got %#v", payload.Summary)
	}
	if payload.RoutingRiskDelta == nil || payload.RoutingRiskDelta.Status != "regressed" {
		t.Fatalf("expected routing-risk delta, got %#v", payload)
	}
	if len(payload.AddedFindings) == 0 {
		t.Fatalf("expected added findings, got %#v", payload)
	}
	if payload.AddedFindings[0].WhyItMatters == "" {
		t.Fatalf("expected explain metadata on added findings, got %#v", payload.AddedFindings[0])
	}
}

func TestSkillCompareCommandArtifact(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "compare-artifact.json")
	testutil.WriteFiles(t, base, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": "# Tiny\n",
	})

	_, stderr, code, err := executeSkillCompare(t, base, candidate, "--artifact", artifactPath, "--format", "json", "--routing-risk")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read compare artifact: %v", err)
	}

	var payload skillCompareArtifactOutput
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("expected valid compare artifact, got %v; output=%q", err, string(content))
	}
	if payload.SchemaVersion != "1" || payload.ArtifactType != "firety.skill-lint-compare" {
		t.Fatalf("unexpected compare artifact metadata, got %#v", payload)
	}
	if payload.Comparison.Overall != "regressed" {
		t.Fatalf("expected regressed artifact summary, got %#v", payload)
	}
}

func TestSkillCompareCommandCandidateExitCodeUsesFailPolicy(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	testutil.WriteFiles(t, base, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": "## Missing Title\n\n[Broken](docs/missing.md)\n",
	})

	_, stderr, code, err := executeSkillCompare(t, base, candidate)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSkillCompareCommandInvalidFormat(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	testutil.WriteFiles(t, base, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidate, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillCompare(t, base, candidate, "--format", "sarif")
	if err == nil {
		t.Fatalf("expected runtime error for invalid format")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(err.Error(), `invalid format "sarif"`) {
		t.Fatalf("expected invalid format error, got %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func executeSkillCompare(t *testing.T, base, candidate string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "compare", base, candidate}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

func decodeSkillCompareJSONOutput(t *testing.T, output string) skillCompareJSONOutput {
	t.Helper()

	var payload skillCompareJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid compare json, got %v; output=%q", err, output)
	}

	return payload
}

type skillCompareJSONOutput struct {
	SchemaVersion    string                        `json:"schema_version"`
	Profile          string                        `json:"profile"`
	Strictness       string                        `json:"strictness"`
	Summary          skillCompareSummaryOutput     `json:"summary"`
	RoutingRiskDelta *skillCompareRoutingRiskDelta `json:"routing_risk_delta"`
	AddedFindings    []skillCompareFindingOutput   `json:"added_findings"`
}

type skillCompareSummaryOutput struct {
	Overall string `json:"overall"`
}

type skillCompareRoutingRiskDelta struct {
	Status string `json:"status"`
}

type skillCompareFindingOutput struct {
	RuleID       string `json:"rule_id"`
	WhyItMatters string `json:"why_it_matters"`
}

type skillCompareArtifactOutput struct {
	SchemaVersion string                    `json:"schema_version"`
	ArtifactType  string                    `json:"artifact_type"`
	Comparison    skillCompareSummaryOutput `json:"comparison"`
}
