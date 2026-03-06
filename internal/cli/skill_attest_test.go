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

func TestSkillAttestCommandFreshWithGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeSkillAttest(t, root, "--profile", "generic", "--runner", runner, "--include-gate")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Support posture: GENERIC-PORTABLE") {
		t.Fatalf("expected generic-portable posture, got %q", stdout)
	}
	if !strings.Contains(stdout, "Tested profiles:") {
		t.Fatalf("expected tested profile output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Quality gate: PASS") {
		t.Fatalf("expected passing gate summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Claims:") {
		t.Fatalf("expected claims section, got %q", stdout)
	}
}

func TestSkillAttestCommandJSONAndArtifactRender(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	artifactPath := filepath.Join(t.TempDir(), "attestation.json")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeSkillAttest(t, root, "--profile", "generic", "--runner", runner, "--include-gate", "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload skillAttestJSONOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid attestation json, got %v; output=%q", err, stdout)
	}
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if payload.Report.SupportPosture != "generic-portable" {
		t.Fatalf("expected generic-portable posture, got %#v", payload.Report)
	}
	if payload.Report.QualityGate == nil || payload.Report.QualityGate.Decision != "pass" {
		t.Fatalf("expected gate summary, got %#v", payload.Report.QualityGate)
	}
	if len(payload.Report.TestedBackends) != 1 {
		t.Fatalf("expected one tested backend, got %#v", payload.Report.TestedBackends)
	}

	rendered, renderErr, renderCode, err := executeArtifact(t, "render", artifactPath, "--render", "ci-summary")
	if err != nil {
		t.Fatalf("expected no runtime error rendering artifact, got %v", err)
	}
	if renderCode != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, renderCode)
	}
	if renderErr != "" {
		t.Fatalf("expected empty stderr, got %q", renderErr)
	}
	if !strings.Contains(rendered, "Firety Skill Attestation") {
		t.Fatalf("expected attestation render output, got %q", rendered)
	}
}

func TestSkillAttestCommandFromEvidencePackShowsWeakEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	evalArtifactPath := filepath.Join(t.TempDir(), "eval-artifact.json")
	packDir := filepath.Join(t.TempDir(), "evidence-pack")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	_, _, code, err := executeSkillEval(t, root, "--runner", runner, "--artifact", evalArtifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	_, _, code, err = executeEvidencePack(t, "--input-artifact", evalArtifactPath, "--output", packDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeSkillAttest(t, "--input-pack", packDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Support posture: WEAK-EVIDENCE") {
		t.Fatalf("expected weak-evidence posture, got %q", stdout)
	}
	if !strings.Contains(stdout, "No measured routing eval evidence was provided.") && !strings.Contains(stdout, "Support claims should be read with caution") {
		t.Fatalf("expected weak-evidence limitations, got %q", stdout)
	}
}

func TestSkillAttestCommandRejectsMixedFreshAndArtifactInputs(t *testing.T) {
	t.Parallel()

	stdout, stderr, code, err := executeSkillAttest(t, "/tmp/skill", "--input-artifact", "/tmp/artifact.json")
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty stdio, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(err.Error(), "artifact-based attestation cannot be combined with a target path") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func executeSkillAttest(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "attest"}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)
	return stdout.String(), stderr.String(), code, err
}

type skillAttestJSONOutput struct {
	SchemaVersion string `json:"schema_version"`
	Report        struct {
		SupportPosture string `json:"support_posture"`
		QualityGate    *struct {
			Decision string `json:"decision"`
		} `json:"quality_gate,omitempty"`
		TestedBackends []struct {
			BackendID string `json:"backend_id"`
		} `json:"tested_backends"`
	} `json:"report"`
}
