package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/testutil"
)

func TestArtifactInspectCommandText(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint-artifact.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, _, _ = executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeArtifact(t, "inspect", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Type: firety.skill-lint") {
		t.Fatalf("expected lint artifact type, got %q", stdout)
	}
	if !strings.Contains(stdout, "Can render: pr-comment, ci-summary, full-report") {
		t.Fatalf("expected supported render modes, got %q", stdout)
	}
	if !strings.Contains(stdout, "Can compare with: firety.skill-lint") {
		t.Fatalf("expected compare compatibility, got %q", stdout)
	}
}

func TestArtifactInspectCommandJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "eval-artifact.json")
	runner := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	_, _, _, _ = executeSkillEval(t, root, "--runner", runner, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeArtifact(t, "inspect", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Inspection    struct {
			ArtifactType string   `json:"artifact_type"`
			Origin       string   `json:"origin"`
			Context      []string `json:"context"`
		} `json:"inspection"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid inspect json, got %v; output=%q", err, stdout)
	}
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if payload.Inspection.ArtifactType != "firety.skill-routing-eval" {
		t.Fatalf("expected eval artifact type, got %#v", payload.Inspection)
	}
	if payload.Inspection.Origin != "firety skill eval" {
		t.Fatalf("expected eval origin, got %#v", payload.Inspection)
	}
	if len(payload.Inspection.Context) == 0 {
		t.Fatalf("expected inspect context, got %#v", payload.Inspection)
	}
}

func TestArtifactInspectCommandFailsOnUnsupportedSchema(t *testing.T) {
	t.Parallel()

	artifactPath := filepath.Join(t.TempDir(), "bad-artifact.json")
	writeJSONFile(t, artifactPath, map[string]any{
		"artifact_type":   "firety.skill-lint",
		"schema_version":  "2",
		"target":          "ignored",
		"supported_modes": []string{"full-report"},
	})

	stdout, stderr, code, err := executeArtifact(t, "inspect", artifactPath)
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty stdio, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(err.Error(), `unsupported schema version "2"`) {
		t.Fatalf("expected schema validation error, got %v", err)
	}
}

func TestArtifactRenderCommandFromArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "plan-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": routingEvalBroadGenericSkillMarkdown(),
	})

	_, _, _, _ = executeSkillPlan(t, root, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeArtifact(t, "render", artifactPath, "--render", "pr-comment")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "## Firety Skill Quality") {
		t.Fatalf("expected rendered PR comment, got %q", stdout)
	}
}

func TestArtifactInspectCommandForAttestationArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	artifactPath := filepath.Join(t.TempDir(), "attestation.json")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	_, _, _, _ = executeSkillAttest(t, root, "--runner", runner, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeArtifact(t, "inspect", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Type: firety.skill-attestation") {
		t.Fatalf("expected attestation artifact type, got %q", stdout)
	}
	if !strings.Contains(stdout, "Origin: firety skill attest") {
		t.Fatalf("expected attestation origin, got %q", stdout)
	}
}

func TestArtifactInspectAndRenderForReadinessArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	artifactPath := filepath.Join(t.TempDir(), "readiness.json")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	_, _, _, _ = executeReadinessCheck(t, root, "--context", "merge", "--runner", runner, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeArtifact(t, "inspect", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Type: firety.skill-readiness") {
		t.Fatalf("expected readiness artifact type, got %q", stdout)
	}

	rendered, stderr, code, err := executeArtifact(t, "render", artifactPath, "--render", "ci-summary")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK || stderr != "" {
		t.Fatalf("expected successful render, got code=%d stderr=%q err=%v", code, stderr, err)
	}
	if !strings.Contains(rendered, "Firety Readiness") {
		t.Fatalf("expected readiness render output, got %q", rendered)
	}
}

func TestArtifactCompareCommandForLintArtifacts(t *testing.T) {
	t.Parallel()

	baseRoot := t.TempDir()
	candidateRoot := t.TempDir()
	baseArtifact := filepath.Join(t.TempDir(), "base-lint.json")
	candidateArtifact := filepath.Join(t.TempDir(), "candidate-lint.json")

	testutil.WriteFiles(t, baseRoot, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidateRoot, map[string]string{
		"SKILL.md": "# Tiny\n",
	})

	_, _, _, _ = executeSkillLint(t, baseRoot, "--artifact", baseArtifact, "--format", "json")
	_, _, _, _ = executeSkillLint(t, candidateRoot, "--artifact", candidateArtifact, "--format", "json")

	stdout, stderr, code, err := executeArtifact(t, "compare", baseArtifact, candidateArtifact, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Comparison    struct {
			ArtifactType string   `json:"artifact_type"`
			Overall      string   `json:"overall"`
			Summary      string   `json:"summary"`
			Regressions  []string `json:"high_priority_regressions"`
		} `json:"comparison"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid compare json, got %v; output=%q", err, stdout)
	}
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if payload.Comparison.ArtifactType != "firety.skill-lint" {
		t.Fatalf("expected lint compare artifact type, got %#v", payload.Comparison)
	}
	if payload.Comparison.Overall != "regressed" {
		t.Fatalf("expected regressed comparison, got %#v", payload.Comparison)
	}
	if len(payload.Comparison.Regressions) == 0 {
		t.Fatalf("expected high-priority regressions, got %#v", payload.Comparison)
	}
}

func TestArtifactCompareCommandRejectsIncompatibleArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	lintArtifact := filepath.Join(t.TempDir(), "lint.json")
	evalArtifact := filepath.Join(t.TempDir(), "eval.json")
	runner := writeRoutingEvalRunner(t, "compare")

	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())
	_, _, _, _ = executeSkillLint(t, root, "--artifact", lintArtifact, "--format", "json")
	_, _, _, _ = executeSkillEval(t, root, "--runner", runner, "--artifact", evalArtifact, "--format", "json")

	stdout, stderr, code, err := executeArtifact(t, "compare", lintArtifact, evalArtifact)
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty stdio, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(err.Error(), "artifact types are incompatible for compare") {
		t.Fatalf("expected incompatible artifact error, got %v", err)
	}
}

func executeArtifact(t *testing.T, subcommand string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"artifact", subcommand}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)

	return stdout.String(), stderr.String(), code, err
}
