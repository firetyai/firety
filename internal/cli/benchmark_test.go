package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/cli"
)

func TestBenchmarkRunCommandTextOutput(t *testing.T) {
	t.Parallel()

	stdout, stderr, code, err := executeBenchmarkRun(t)
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
		"Firety benchmark health",
		"Category overview:",
		"Confidence signals:",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, stdout)
		}
	}
}

func TestBenchmarkRunCommandJSONAndArtifact(t *testing.T) {
	t.Parallel()

	artifactPath := filepath.Join(t.TempDir(), "benchmark-artifact.json")

	stdout, stderr, code, err := executeBenchmarkRun(t, "--format", "json", "--artifact", artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload benchmarkJSONOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid benchmark json, got %v; output=%q", err, stdout)
	}
	if payload.SchemaVersion != "1" || payload.Summary.TotalFixtures == 0 {
		t.Fatalf("expected structured benchmark json, got %#v", payload)
	}

	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read benchmark artifact: %v", err)
	}
	var artifactPayload benchmarkArtifactOutput
	if err := json.Unmarshal(content, &artifactPayload); err != nil {
		t.Fatalf("expected valid benchmark artifact, got %v; output=%q", err, string(content))
	}
	if artifactPayload.ArtifactType != "firety.benchmark-report" {
		t.Fatalf("unexpected benchmark artifact type, got %#v", artifactPayload)
	}
}

func TestBenchmarkRenderCommand(t *testing.T) {
	t.Parallel()

	artifactPath := filepath.Join(t.TempDir(), "benchmark-artifact.json")
	_, _, _, _ = executeBenchmarkRun(t, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeBenchmarkRender(t, artifactPath, "--render", "ci-summary")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "### Firety Benchmark Health") {
		t.Fatalf("expected rendered benchmark title, got %q", stdout)
	}
	if !strings.Contains(stdout, "Category overview") {
		t.Fatalf("expected category overview in rendered output, got %q", stdout)
	}
}

func TestBenchmarkRenderCommandInvalidMode(t *testing.T) {
	t.Parallel()

	stdout, stderr, code, err := executeBenchmarkRender(t, "/tmp/missing.json", "--render", "html")
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty stdio, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(err.Error(), `invalid render mode "html"`) {
		t.Fatalf("expected invalid render mode error, got %v", err)
	}
}

func executeBenchmarkRun(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"benchmark", "run"}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

func executeBenchmarkRender(t *testing.T, artifactPath string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"benchmark", "render", artifactPath}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

type benchmarkJSONOutput struct {
	SchemaVersion string                 `json:"schema_version"`
	Summary       benchmarkSummaryOutput `json:"summary"`
}

type benchmarkSummaryOutput struct {
	TotalFixtures int `json:"total_fixtures"`
}

type benchmarkArtifactOutput struct {
	ArtifactType string `json:"artifact_type"`
}
