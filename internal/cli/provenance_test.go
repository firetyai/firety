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

func TestProvenanceInspectEvidencePackDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	packDir := filepath.Join(t.TempDir(), "evidence-pack")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executeEvidencePack(t, root, "--output", packDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeProvenance(t, "inspect", packDir)
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
		"Kind: evidence-pack",
		"Type: firety.evidence-pack",
		"Command origin: firety evidence pack",
		"Suitable for comparison: true",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, stdout)
		}
	}
	if !strings.Contains(stdout, "Target fingerprint:") {
		t.Fatalf("expected target fingerprint in provenance output, got %q", stdout)
	}
}

func TestProvenanceInspectTrustReportDirectoryJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "trust-report")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executePublishReport(t, root, "--output", outputDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeProvenance(t, "inspect", outputDir, "--format", "json")
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
		Inspection struct {
			Kind       string `json:"kind"`
			Type       string `json:"type"`
			Provenance struct {
				CommandOrigin string `json:"command_origin"`
			} `json:"provenance"`
		} `json:"inspection"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if payload.Inspection.Kind != "trust-report" || payload.Inspection.Type != "firety.trust-report" {
		t.Fatalf("unexpected inspection payload %#v", payload)
	}
	if payload.Inspection.Provenance.CommandOrigin != "firety publish report" {
		t.Fatalf("unexpected command origin %#v", payload)
	}
}

func TestProvenanceCompareLintArtifactsComparable(t *testing.T) {
	t.Parallel()

	basePath := filepath.Join(t.TempDir(), "base.json")
	candidatePath := filepath.Join(t.TempDir(), "candidate.json")
	writeProvenanceJSONFile(t, basePath, map[string]any{
		"schema_version": "1",
		"artifact_type":  "firety.skill-lint",
		"tool":           map[string]any{"version": "test-version", "commit": "abc1234"},
		"run": map[string]any{
			"target":        "/tmp/skill",
			"profile":       "generic",
			"strictness":    "default",
			"fail_on":       "errors",
			"stdout_format": "json",
		},
		"summary": map[string]any{
			"error_count":   0,
			"warning_count": 0,
			"finding_count": 0,
		},
	})
	writeProvenanceJSONFile(t, candidatePath, map[string]any{
		"schema_version": "1",
		"artifact_type":  "firety.skill-lint",
		"tool":           map[string]any{"version": "test-version", "commit": "abc1234"},
		"run": map[string]any{
			"target":        "/tmp/skill",
			"profile":       "generic",
			"strictness":    "default",
			"fail_on":       "errors",
			"stdout_format": "json",
		},
		"summary": map[string]any{
			"error_count":   0,
			"warning_count": 0,
			"finding_count": 0,
		},
	})

	stdout, stderr, code, err := executeProvenance(t, "compare", basePath, candidatePath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Status: COMPARABLE") {
		t.Fatalf("expected comparable status, got %q", stdout)
	}
}

func TestProvenanceCompareEvalArtifactsDifferentSuite(t *testing.T) {
	t.Parallel()

	basePath := filepath.Join(t.TempDir(), "base.json")
	candidatePath := filepath.Join(t.TempDir(), "candidate.json")
	writeProvenanceJSONFile(t, basePath, map[string]any{
		"schema_version": "1",
		"artifact_type":  "firety.skill-routing-eval",
		"tool":           map[string]any{"version": "test-version", "commit": "abc1234"},
		"run": map[string]any{
			"target":        "/tmp/skill",
			"profile":       "generic",
			"suite_path":    "evals/routing-a.json",
			"stdout_format": "json",
		},
		"backend": map[string]any{"id": "generic", "name": "Generic"},
		"summary": map[string]any{
			"total":     1,
			"passed":    1,
			"failed":    0,
			"pass_rate": 1.0,
		},
	})
	writeProvenanceJSONFile(t, candidatePath, map[string]any{
		"schema_version": "1",
		"artifact_type":  "firety.skill-routing-eval",
		"tool":           map[string]any{"version": "test-version", "commit": "abc1234"},
		"run": map[string]any{
			"target":        "/tmp/skill",
			"profile":       "generic",
			"suite_path":    "evals/routing-b.json",
			"stdout_format": "json",
		},
		"backend": map[string]any{"id": "generic", "name": "Generic"},
		"summary": map[string]any{
			"total":     1,
			"passed":    1,
			"failed":    0,
			"pass_rate": 1.0,
		},
	})

	stdout, stderr, code, err := executeProvenance(t, "compare", basePath, candidatePath, "--format", "json")
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
		Comparison struct {
			Status  string   `json:"status"`
			Reasons []string `json:"reasons"`
		} `json:"comparison"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if payload.Comparison.Status != "not-comparable" {
		t.Fatalf("expected not-comparable status, got %#v", payload)
	}
	if len(payload.Comparison.Reasons) == 0 {
		t.Fatalf("expected comparison reasons, got %#v", payload)
	}
}

func executeProvenance(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"provenance"}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)

	return stdout.String(), stderr.String(), code, err
}

func writeProvenanceJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write json file: %v", err)
	}
}
