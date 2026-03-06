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

func TestSkillEvalCommandTextOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	stdout, stderr, code, err := executeSkillEval(t, root, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Summary: 4 passed, 0 failed") {
		t.Fatalf("expected eval summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Notable misses: none") {
		t.Fatalf("expected no misses, got %q", stdout)
	}
}

func TestSkillEvalCommandFailingCasesProduceLintExitCode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "always-trigger")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	stdout, stderr, code, err := executeSkillEval(t, root, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "false positive(s)") {
		t.Fatalf("expected false-positive summary, got %q", stdout)
	}
}

func TestSkillEvalCommandJSONOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	stdout, stderr, code, err := executeSkillEval(t, root, "--runner", runner, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSkillEvalJSONOutput(t, stdout)
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if payload.Summary.Total != 4 || payload.Summary.Passed != 4 {
		t.Fatalf("expected full pass summary, got %#v", payload.Summary)
	}
	if len(payload.Results) != 4 {
		t.Fatalf("expected four case results, got %#v", payload.Results)
	}
}

func TestSkillEvalCommandArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	artifactPath := filepath.Join(t.TempDir(), "eval-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	_, stderr, code, err := executeSkillEval(t, root, "--runner", runner, "--artifact", artifactPath, "--format", "json")
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
		t.Fatalf("read eval artifact: %v", err)
	}

	var payload skillEvalArtifactOutput
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("expected valid eval artifact, got %v; output=%q", err, string(content))
	}
	if payload.SchemaVersion != "1" || payload.ArtifactType != "firety.skill-routing-eval" {
		t.Fatalf("unexpected eval artifact metadata, got %#v", payload)
	}
	if payload.Summary.Total != 4 || payload.Summary.Passed != 4 {
		t.Fatalf("unexpected eval artifact summary, got %#v", payload)
	}
}

func TestSkillEvalCommandMissingRunnerIsRuntimeError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	stdout, stderr, code, err := executeSkillEval(t, root)
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
	if !strings.Contains(err.Error(), "routing eval runner is not configured") {
		t.Fatalf("expected missing-runner error, got %v", err)
	}
}

func TestSkillEvalCommandInvalidFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	stdout, stderr, code, err := executeSkillEval(t, root, "--runner", runner, "--format", "sarif")
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

func executeSkillEval(t *testing.T, root string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "eval", root}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

func writeRoutingEvalRunner(t *testing.T, mode string) string {
	t.Helper()

	scriptPath := filepath.Join(t.TempDir(), "runner.sh")
	script := strings.Join([]string{
		"#!/bin/sh",
		"payload=$(cat)",
		"case \"" + mode + "\" in",
		"  always-trigger)",
		"    echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"forced trigger\"}'",
		"    ;;",
		"  regress-good)",
		"    if printf '%s' \"$payload\" | grep -q 'eval-mode: good'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"regress-good mode\"}';",
		"    elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"positive-validate-local-skill\"'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"positive trigger\"}';",
		"    elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"profile-codex-positive\"'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"profile-sensitive positive\"}';",
		"    elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"negative-unrelated-task\"'; then echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"negative case\"}';",
		"    elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"ambiguous-help-request\"'; then echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"ambiguous should stay off\"}';",
		"    else echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"default negative\"}'; fi",
		"    ;;",
		"  compare)",
		"    if printf '%s' \"$payload\" | grep -q 'eval-mode: broad'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"broad mode\"}';",
		"    elif printf '%s' \"$payload\" | grep -q 'eval-mode: mixed'; then",
		"      if printf '%s' \"$payload\" | grep -q '\"case_id\":\"negative-unrelated-task\"'; then echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"mixed mode negative stays off\"}';",
		"      else echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"mixed mode\"}'; fi;",
		"    elif printf '%s' \"$payload\" | grep -q 'eval-mode: narrow'; then",
		"      if printf '%s' \"$payload\" | grep -q '\"case_id\":\"positive-validate-local-skill\"'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"narrow local positive\"}';",
		"      else echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"narrow mode\"}'; fi;",
		"    else",
		"      if printf '%s' \"$payload\" | grep -q '\"case_id\":\"positive-validate-local-skill\"'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"positive trigger\"}';",
		"      elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"profile-codex-positive\"'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"profile-sensitive positive\"}';",
		"      elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"negative-unrelated-task\"'; then echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"negative case\"}';",
		"      elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"ambiguous-help-request\"'; then echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"ambiguous should stay off\"}';",
		"      else echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"default negative\"}'; fi;",
		"    fi",
		"    ;;",
		"  *)",
		"    if printf '%s' \"$payload\" | grep -q '\"case_id\":\"positive-validate-local-skill\"'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"positive trigger\"}';",
		"    elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"profile-codex-positive\"'; then echo '{\"schema_version\":\"1\",\"trigger\":true,\"reason\":\"profile-sensitive positive\"}';",
		"    elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"negative-unrelated-task\"'; then echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"negative case\"}';",
		"    elif printf '%s' \"$payload\" | grep -q '\"case_id\":\"ambiguous-help-request\"'; then echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"ambiguous should stay off\"}';",
		"    else echo '{\"schema_version\":\"1\",\"trigger\":false,\"reason\":\"default negative\"}'; fi",
		"    ;;",
		"esac",
	}, "\n")

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write runner script: %v", err)
	}

	return scriptPath
}

func decodeSkillEvalJSONOutput(t *testing.T, output string) skillEvalJSONOutput {
	t.Helper()

	var payload skillEvalJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid eval json, got %v; output=%q", err, output)
	}

	return payload
}

type skillEvalJSONOutput struct {
	SchemaVersion string                  `json:"schema_version"`
	Summary       skillEvalSummaryOutput  `json:"summary"`
	Results       []skillEvalResultOutput `json:"results"`
}

type skillEvalSummaryOutput struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
}

type skillEvalResultOutput struct {
	ID     string `json:"id"`
	Passed bool   `json:"passed"`
}

type skillEvalArtifactOutput struct {
	SchemaVersion string                 `json:"schema_version"`
	ArtifactType  string                 `json:"artifact_type"`
	Summary       skillEvalSummaryOutput `json:"summary"`
}
