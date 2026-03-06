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

func TestPublishReportCommandFreshBuild(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "trust-report")
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executePublishReport(t, root, "--output", outputDir, "--runner", runner, "--include-gate", "--include-plan")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Trust report:") || !strings.Contains(stdout, "Entrypoint: index.html") {
		t.Fatalf("expected publish summary output, got %q", stdout)
	}

	assertFileExists(t, filepath.Join(outputDir, "index.html"))
	assertFileExists(t, filepath.Join(outputDir, "manifest.json"))
	assertFileExists(t, filepath.Join(outputDir, "pages", "attestation.html"))
	assertFileExists(t, filepath.Join(outputDir, "evidence-pack", "manifest.json"))

	indexBytes, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	index := string(indexBytes)
	for _, expected := range []string{"Support posture", "Quality gate", "Support claims", "Evidence files"} {
		if !strings.Contains(index, expected) {
			t.Fatalf("expected index to contain %q, got %q", expected, index)
		}
	}
}

func TestPublishReportCommandFromExistingPack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	packDir := filepath.Join(t.TempDir(), "evidence-pack")
	outputDir := filepath.Join(t.TempDir(), "trust-report")
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	_, _, code, err := executeEvidencePack(t, root, "--output", packDir, "--runner", runner, "--include-compatibility", "--include-gate")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executePublishReport(t, "--input-pack", packDir, "--output", outputDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Pages:") || !strings.Contains(stdout, "Artifacts:") {
		t.Fatalf("expected report counts, got %q", stdout)
	}

	assertFileExists(t, filepath.Join(outputDir, "evidence-packs", "pack-01", "manifest.json"))
	assertFileExists(t, filepath.Join(outputDir, "pages", "compatibility.html"))
}

func TestPublishReportCommandWeakEvidenceFromEvalArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "trust-report")
	evalArtifact := filepath.Join(t.TempDir(), "eval-artifact.json")
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	_, _, code, err := executeSkillEval(t, root, "--runner", runner, "--artifact", evalArtifact, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	_, stderr, code, err := executePublishReport(t, "--input-artifact", evalArtifact, "--output", outputDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	indexBytes, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	index := strings.ToLower(string(indexBytes))
	if !strings.Contains(index, "weak-evidence") {
		t.Fatalf("expected weak-evidence posture in index, got %q", index)
	}
}

func TestPublishReportCommandDeterministicManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	firstDir := filepath.Join(t.TempDir(), "trust-report-a")
	secondDir := filepath.Join(t.TempDir(), "trust-report-b")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executePublishReport(t, root, "--output", firstDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	_, _, code, err = executePublishReport(t, root, "--output", secondDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	firstBytes, err := os.ReadFile(filepath.Join(firstDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read first manifest: %v", err)
	}
	secondBytes, err := os.ReadFile(filepath.Join(secondDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read second manifest: %v", err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatalf("expected deterministic manifest output\nfirst:\n%s\nsecond:\n%s", string(firstBytes), string(secondBytes))
	}
}

func TestPublishReportCommandFromBenchmarkArtifact(t *testing.T) {
	t.Parallel()

	artifactPath := filepath.Join(t.TempDir(), "benchmark.json")
	outputDir := filepath.Join(t.TempDir(), "trust-report")

	_, _, code, err := executeBenchmarkRun(t, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executePublishReport(t, "--input-artifact", artifactPath, "--output", outputDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Trust report:") {
		t.Fatalf("expected publish summary, got %q", stdout)
	}
	assertFileExists(t, filepath.Join(outputDir, "pages", "benchmark.html"))
}

func TestPublishReportCommandManifestJSON(t *testing.T) {
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

	var manifest publishManifestOutput
	data, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("expected valid manifest json, got %v", err)
	}
	if manifest.SchemaVersion != "1" || manifest.ReportType != "firety.trust-report" {
		t.Fatalf("unexpected manifest %#v", manifest)
	}
	if len(manifest.Pages) == 0 {
		t.Fatalf("expected rendered pages, got %#v", manifest)
	}
}

func executePublishReport(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"publish", "report"}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)

	return stdout.String(), stderr.String(), code, err
}

type publishManifestOutput struct {
	SchemaVersion string `json:"schema_version"`
	ReportType    string `json:"report_type"`
	Pages         []struct {
		Path string `json:"path"`
	} `json:"pages"`
}
