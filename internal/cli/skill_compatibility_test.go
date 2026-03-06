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

func TestSkillCompatibilityCommandGenericPortable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkillCompatibility(t, root)
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
}

func TestSkillCompatibilityCommandJSONWithBackends(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goodRunner := writeRoutingEvalRunner(t, "good")
	badRunner := writeRoutingEvalRunner(t, "always-trigger")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeSkillCompatibility(t,
		root,
		"--format", "json",
		"--backend", "codex="+goodRunner,
		"--backend", "cursor="+badRunner,
	)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload skillCompatibilityJSONOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid json, got %v; output=%q", err, stdout)
	}
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if len(payload.Report.Backends) != 2 {
		t.Fatalf("expected two backend summaries, got %#v", payload.Report.Backends)
	}
	if payload.Report.Backends[0].BackendID != "cursor" {
		t.Fatalf("expected risky backend to sort first, got %#v", payload.Report.Backends)
	}
}

func TestSkillCompatibilityCommandArtifactInputWeakEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "eval-artifact.json")
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	_, _, code, err := executeSkillEval(t, root, "--runner", runner, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeSkillCompatibility(t, "--input-artifact", artifactPath)
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
}

func TestSkillCompatibilityCommandArtifactAndRender(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "compatibility.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executeSkillCompatibility(t, root, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read compatibility artifact: %v", err)
	}
	var payload skillCompatibilityArtifactOutput
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("expected valid artifact json, got %v; output=%q", err, string(content))
	}
	if payload.ArtifactType != "firety.skill-compatibility" {
		t.Fatalf("unexpected artifact payload %#v", payload)
	}

	stdout, stderr, code, err := executeSkillRender(t, artifactPath, "--render", "ci-summary")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Firety Compatibility") {
		t.Fatalf("expected compatibility render, got %q", stdout)
	}
}

func executeSkillCompatibility(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "compatibility"}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)
	return stdout.String(), stderr.String(), code, err
}

type skillCompatibilityJSONOutput struct {
	SchemaVersion string `json:"schema_version"`
	Report        struct {
		SupportPosture string `json:"support_posture"`
		Backends       []struct {
			BackendID string `json:"backend_id"`
		} `json:"backends"`
	} `json:"report"`
}

type skillCompatibilityArtifactOutput struct {
	ArtifactType string `json:"artifact_type"`
	Report       struct {
		SupportPosture string `json:"support_posture"`
	} `json:"report"`
}
