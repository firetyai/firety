package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/domain/readiness"
	"github.com/firety/firety/internal/testutil"
)

func TestReadinessCheckFreshMergeReady(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeReadinessCheck(t, root, "--context", "merge", "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Decision: READY") {
		t.Fatalf("expected ready decision, got %q", stdout)
	}
	if !strings.Contains(stdout, "quality gate: PASS") {
		t.Fatalf("expected gate evidence, got %q", stdout)
	}
}

func TestReadinessCheckPublicAttestationWithLintOnlyIsCaveated(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, stderr, code, err := executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK || stderr != "" {
		t.Fatalf("expected clean lint artifact generation, got code=%d stderr=%q err=%v", code, stderr, err)
	}

	stdout, stderr, code, err := executeReadinessCheck(t, "--context", "public-attestation", "--input-artifact", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Decision: READY-WITH-CAVEATS") {
		t.Fatalf("expected caveated decision, got %q", stdout)
	}
	if !strings.Contains(stdout, "Attestation can be published only with clear tested-vs-supported caveats") {
		t.Fatalf("expected attestation caveat, got %q", stdout)
	}
}

func TestReadinessCheckPublicTrustReportFailsOnStaleEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, stderr, code, err := executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK || stderr != "" {
		t.Fatalf("expected clean lint artifact generation, got code=%d stderr=%q err=%v", code, stderr, err)
	}

	staleTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(artifactPath, staleTime, staleTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	stdout, stderr, code, err := executeReadinessCheck(t, "--context", "public-trust-report", "--input-artifact", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Decision: NOT-READY") {
		t.Fatalf("expected not-ready decision, got %q", stdout)
	}
	if !strings.Contains(stdout, "stale") {
		t.Fatalf("expected stale evidence summary, got %q", stdout)
	}
}

func TestReadinessCheckJSONAndArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	artifactPath := filepath.Join(t.TempDir(), "readiness.json")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeReadinessCheck(t, root, "--context", "merge", "--runner", runner, "--format", "json", "--artifact", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeReadinessJSONOutput(t, stdout)
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if payload.Readiness.Decision != readiness.DecisionReady {
		t.Fatalf("expected ready decision, got %#v", payload)
	}

	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read readiness artifact: %v", err)
	}
	var saved artifact.SkillReadinessArtifact
	if err := json.Unmarshal(content, &saved); err != nil {
		t.Fatalf("expected valid readiness artifact, got %v", err)
	}
	if saved.ArtifactType != "firety.skill-readiness" || saved.Readiness.Decision != readiness.DecisionReady {
		t.Fatalf("unexpected readiness artifact %#v", saved)
	}
}

func executeReadinessCheck(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	commandArgs := append([]string{"readiness", "check"}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)
	return stdout.String(), stderr.String(), code, err
}

func decodeReadinessJSONOutput(t *testing.T, output string) readinessJSONOutput {
	t.Helper()

	var payload readinessJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid readiness json, got %v; output=%q", err, output)
	}
	return payload
}

type readinessJSONOutput struct {
	SchemaVersion string           `json:"schema_version"`
	Readiness     readiness.Result `json:"readiness"`
}
