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

func TestSkillAnalyzeCommandFalsePositiveCorrelationText(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = routingEvalBroadGenericSkillMarkdown()
	testutil.WriteFiles(t, root, files)

	stdout, stderr, code, err := executeSkillAnalyze(t, root, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Likely contributors to false positives") {
		t.Fatalf("expected false-positive correlation section, got %q", stdout)
	}
	if !strings.Contains(stdout, "Generic skill name") {
		t.Fatalf("expected contributor titles, got %q", stdout)
	}
}

func TestSkillAnalyzeCommandFalseNegativeCorrelationText(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = routingEvalWeakTriggerSkillMarkdown()
	testutil.WriteFiles(t, root, files)

	stdout, stderr, code, err := executeSkillAnalyze(t, root, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Likely contributors to false negatives") {
		t.Fatalf("expected false-negative correlation section, got %q", stdout)
	}
	if !strings.Contains(stdout, "Weak trigger pattern") {
		t.Fatalf("expected false-negative contributor, got %q", stdout)
	}
}

func TestSkillAnalyzeCommandProfileSpecificCorrelationJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = routingEvalProfileMismatchSkillMarkdown()
	testutil.WriteFiles(t, root, files)

	stdout, stderr, code, err := executeSkillAnalyze(t, root, "--runner", runner, "--profile", "codex", "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSkillAnalyzeJSONOutput(t, stdout)
	if payload.Correlation == nil {
		t.Fatalf("expected correlation output, got %#v", payload)
	}
	if payload.Correlation.MissGroups[0].Key != "false-negatives" || payload.Correlation.MissGroups[1].Key != "profile-specific-misses" {
		t.Fatalf("expected deterministic correlation group order, got %#v", payload.Correlation.MissGroups)
	}
	if payload.Eval.Summary.Failed == 0 {
		t.Fatalf("expected eval failures, got %#v", payload.Eval.Summary)
	}
}

func TestSkillAnalyzeCommandLowNoiseForGoodSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeSkillAnalyze(t, root, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "Lint/eval correlation:") {
		t.Fatalf("expected no correlation section for strong skill, got %q", stdout)
	}
	if !strings.Contains(stdout, "Routing risk: LOW") {
		t.Fatalf("expected routing risk summary, got %q", stdout)
	}
}

func TestSkillAnalyzeCommandArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	artifactPath := filepath.Join(t.TempDir(), "analysis-artifact.json")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = routingEvalBroadGenericSkillMarkdown()
	testutil.WriteFiles(t, root, files)

	_, stderr, code, err := executeSkillAnalyze(t, root, "--runner", runner, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read analyze artifact: %v", err)
	}

	var payload skillAnalyzeArtifactOutput
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("expected valid analyze artifact, got %v; output=%q", err, string(content))
	}
	if payload.ArtifactType != "firety.skill-analysis" {
		t.Fatalf("unexpected artifact type, got %#v", payload)
	}
	if payload.Correlation == nil || len(payload.Correlation.MissGroups) == 0 {
		t.Fatalf("expected artifact correlation data, got %#v", payload)
	}
}

func TestSkillAnalyzeCommandInvalidFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeSkillAnalyze(t, root, "--runner", runner, "--format", "sarif")
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(err.Error(), `invalid format "sarif"`) {
		t.Fatalf("expected invalid format error, got %v", err)
	}
}

func executeSkillAnalyze(t *testing.T, root string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "analyze", root}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

func decodeSkillAnalyzeJSONOutput(t *testing.T, output string) skillAnalyzeJSONOutput {
	t.Helper()

	var payload skillAnalyzeJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid analyze json, got %v; output=%q", err, output)
	}
	return payload
}

func routingEvalBroadGenericSkillMarkdown() string {
	return strings.Join([]string{
		"---",
		"name: Helper",
		"description: Helpful skill for many tasks and general assistance across workflows.",
		"---",
		"# Helper",
		"",
		"## When To Use",
		"",
		"Use this skill whenever you need help with validation, workflow questions, or general support across many tasks.",
		"",
		"## Usage",
		"",
		"Ask for help and let the skill figure out the right workflow.",
		"",
		"## Limitations",
		"",
		"Use judgment.",
		"",
		"## Examples",
		"",
		"Example request: Help with something important.",
		"Example result: A useful answer.",
		"",
		"eval-mode: broad",
	}, "\n")
}

func routingEvalWeakTriggerSkillMarkdown() string {
	return strings.Join([]string{
		"---",
		"name: Checker",
		"description: Reviews local skill bundles before publishing.",
		"---",
		"# Checker",
		"",
		"## When To Use",
		"",
		"Use this skill when you need help reviewing a local skill bundle before publishing.",
		"",
		"## Usage",
		"",
		"Ask for a review of the local bundle.",
		"",
		"## Limitations",
		"",
		"Do not use this skill for remote repositories.",
		"",
		"## Examples",
		"",
		"Request: Review this.",
		"Result: Review output.",
		"",
		"eval-mode: narrow",
	}, "\n")
}

func routingEvalProfileMismatchSkillMarkdown() string {
	return strings.Join([]string{
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
		"",
		"eval-mode: narrow",
	}, "\n")
}

type skillAnalyzeJSONOutput struct {
	SchemaVersion string                      `json:"schema_version"`
	Lint          skillAnalyzeJSONLintOutput  `json:"lint"`
	Eval          skillAnalyzeJSONEvalOutput  `json:"eval"`
	Correlation   *skillAnalyzeCorrelationOut `json:"correlation,omitempty"`
}

type skillAnalyzeJSONLintOutput struct {
	ErrorCount int `json:"error_count"`
}

type skillAnalyzeJSONEvalOutput struct {
	Summary skillAnalyzeEvalSummaryOutput `json:"summary"`
}

type skillAnalyzeEvalSummaryOutput struct {
	Failed int `json:"failed"`
}

type skillAnalyzeCorrelationOut struct {
	MissGroups []skillAnalyzeCorrelationGroupOut `json:"miss_groups"`
}

type skillAnalyzeCorrelationGroupOut struct {
	Key string `json:"key"`
}

type skillAnalyzeArtifactOutput struct {
	ArtifactType string                      `json:"artifact_type"`
	Correlation  *skillAnalyzeCorrelationOut `json:"correlation,omitempty"`
}
