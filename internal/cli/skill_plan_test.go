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

func TestSkillPlanCommandLintOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": routingEvalBroadGenericSkillMarkdown(),
	})

	stdout, stderr, code, err := executeSkillPlan(t, root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Top priorities:") {
		t.Fatalf("expected prioritized plan, got %q", stdout)
	}
	if !strings.Contains(stdout, "Tighten trigger wording and boundaries") {
		t.Fatalf("expected trigger-boundaries priority, got %q", stdout)
	}
}

func TestSkillPlanCommandWithEvalCorrelation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = routingEvalBroadGenericSkillMarkdown()
	testutil.WriteFiles(t, root, files)

	stdout, stderr, code, err := executeSkillPlan(t, root, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Evidence:") {
		t.Fatalf("expected evidence summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "supporting eval case") {
		t.Fatalf("expected eval evidence in plan, got %q", stdout)
	}
}

func TestSkillPlanCommandWithMultiBackendEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goodRunner := writeRoutingEvalRunner(t, "good")
	badRunner := writeRoutingEvalRunner(t, "always-trigger")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = routingEvalBroadGenericSkillMarkdown()
	testutil.WriteFiles(t, root, files)

	stdout, stderr, code, err := executeSkillPlan(t, root,
		"--backend", "codex="+goodRunner,
		"--backend", "claude-code="+badRunner,
	)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "backend difference") {
		t.Fatalf("expected backend-difference evidence, got %q", stdout)
	}
}

func TestSkillPlanCommandJSONAndArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "plan-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": routingEvalBroadGenericSkillMarkdown(),
	})

	stdout, stderr, code, err := executeSkillPlan(t, root, "--format", "json", "--artifact", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSkillPlanJSONOutput(t, stdout)
	if payload.SchemaVersion != "1" || len(payload.Plan.Priorities) == 0 {
		t.Fatalf("expected structured plan json, got %#v", payload)
	}

	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read plan artifact: %v", err)
	}
	var artifactPayload skillPlanArtifactOutput
	if err := json.Unmarshal(content, &artifactPayload); err != nil {
		t.Fatalf("expected valid plan artifact, got %v; output=%q", err, string(content))
	}
	if artifactPayload.ArtifactType != "firety.skill-improvement-plan" {
		t.Fatalf("unexpected artifact type, got %#v", artifactPayload)
	}
}

func TestSkillPlanCommandLowNoiseForGoodSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillPlan(t, root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "Top priorities:") {
		t.Fatalf("expected no priority list for strong skill, got %q", stdout)
	}
	if !strings.Contains(stdout, "did not find strong improvement priorities") {
		t.Fatalf("expected low-noise summary, got %q", stdout)
	}
}

func executeSkillPlan(t *testing.T, root string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "plan", root}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

func decodeSkillPlanJSONOutput(t *testing.T, output string) skillPlanJSONOutput {
	t.Helper()

	var payload skillPlanJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid plan json, got %v; output=%q", err, output)
	}
	return payload
}

type skillPlanJSONOutput struct {
	SchemaVersion string                  `json:"schema_version"`
	Plan          skillPlanJSONPlanOutput `json:"plan"`
}

type skillPlanJSONPlanOutput struct {
	Priorities []skillPlanJSONPriorityOutput `json:"priorities"`
}

type skillPlanJSONPriorityOutput struct {
	Key string `json:"key"`
}

type skillPlanArtifactOutput struct {
	ArtifactType string `json:"artifact_type"`
}
