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

func TestEvidencePackCommandFreshLintPack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	packDir := filepath.Join(t.TempDir(), "evidence-pack")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeEvidencePack(t, root, "--output", packDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Evidence pack:") || !strings.Contains(stdout, "Artifacts: 1") {
		t.Fatalf("expected pack summary output, got %q", stdout)
	}

	assertFileExists(t, filepath.Join(packDir, "manifest.json"))
	assertFileExists(t, filepath.Join(packDir, "SUMMARY.md"))
	assertFileExists(t, filepath.Join(packDir, "artifacts", "skill-lint.json"))
	assertFileExists(t, filepath.Join(packDir, "reports", "skill-lint-ci-summary.md"))
	assertFileExists(t, filepath.Join(packDir, "reports", "skill-lint-full-report.md"))

	manifest := decodeEvidenceManifest(t, filepath.Join(packDir, "manifest.json"))
	if manifest.PackType != "firety.evidence-pack" {
		t.Fatalf("expected evidence pack type, got %#v", manifest)
	}
	if len(manifest.Artifacts) != 1 {
		t.Fatalf("expected one artifact, got %#v", manifest.Artifacts)
	}
	if len(manifest.Reports) != 2 {
		t.Fatalf("expected two rendered reports, got %#v", manifest.Reports)
	}

	firstBytes, err := os.ReadFile(filepath.Join(packDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read first manifest: %v", err)
	}

	secondPackDir := filepath.Join(t.TempDir(), "evidence-pack")
	_, _, secondCode, secondErr := executeEvidencePack(t, root, "--output", secondPackDir)
	if secondErr != nil {
		t.Fatalf("expected no runtime error on second pack, got %v", secondErr)
	}
	if secondCode != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d on second pack, got %d", cli.ExitCodeOK, secondCode)
	}
	secondBytes, err := os.ReadFile(filepath.Join(secondPackDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read second manifest: %v", err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatalf("expected deterministic manifest output\nfirst:\n%s\nsecond:\n%s", string(firstBytes), string(secondBytes))
	}
}

func TestEvidencePackCommandFreshEvalDerivedPack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	packDir := filepath.Join(t.TempDir(), "evidence-pack")
	runner := writeRoutingEvalRunner(t, "good")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = testutil.RoutingEvalPortableSkillMarkdown()
	testutil.WriteFiles(t, root, files)

	stdout, stderr, code, err := executeEvidencePack(
		t,
		root,
		"--output", packDir,
		"--runner", runner,
		"--include-plan",
		"--include-compatibility",
		"--include-gate",
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
	if !strings.Contains(stdout, "Artifacts: 5") {
		t.Fatalf("expected richer pack summary, got %q", stdout)
	}

	manifest := decodeEvidenceManifest(t, filepath.Join(packDir, "manifest.json"))
	assertManifestArtifactType(t, manifest, "firety.skill-lint")
	assertManifestArtifactType(t, manifest, "firety.skill-routing-eval")
	assertManifestArtifactType(t, manifest, "firety.skill-improvement-plan")
	assertManifestArtifactType(t, manifest, "firety.skill-compatibility")
	assertManifestArtifactType(t, manifest, "firety.skill-quality-gate")

	summaryBytes, err := os.ReadFile(filepath.Join(packDir, "SUMMARY.md"))
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	summary := string(summaryBytes)
	if !strings.Contains(summary, "## Review First") || !strings.Contains(summary, "## Included Artifacts") {
		t.Fatalf("expected navigable summary, got %q", summary)
	}
}

func TestEvidencePackCommandFromExistingArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint-artifact.json")
	packDir := filepath.Join(t.TempDir(), "evidence-pack")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, _, _ = executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeEvidencePack(t, "--input-artifact", artifactPath, "--output", packDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "existing-artifacts") {
		t.Fatalf("expected artifact-based summary, got %q", stdout)
	}

	assertFileExists(t, filepath.Join(packDir, "artifacts", "skill-lint.json"))
	assertFileExists(t, filepath.Join(packDir, "reports", "skill-lint-ci-summary.md"))
}

func TestEvidencePackCommandRejectsIncompatibleArtifacts(t *testing.T) {
	t.Parallel()

	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	firstArtifact := filepath.Join(t.TempDir(), "first-lint.json")
	secondArtifact := filepath.Join(t.TempDir(), "second-lint.json")
	packDir := filepath.Join(t.TempDir(), "evidence-pack")

	testutil.WriteFiles(t, firstRoot, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, secondRoot, testutil.ValidSkillFiles())

	_, _, _, _ = executeSkillLint(t, firstRoot, "--artifact", firstArtifact, "--format", "json")
	_, _, _, _ = executeSkillLint(t, secondRoot, "--artifact", secondArtifact, "--format", "json")

	stdout, stderr, code, err := executeEvidencePack(t,
		"--input-artifact", firstArtifact,
		"--input-artifact", secondArtifact,
		"--output", packDir,
	)
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty stdio, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(err.Error(), "artifact targets are incompatible") {
		t.Fatalf("expected incompatible target error, got %v", err)
	}
}

func executeEvidencePack(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"evidence", "pack"}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)

	return stdout.String(), stderr.String(), code, err
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}

func decodeEvidenceManifest(t *testing.T, path string) evidenceManifestOutput {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest evidenceManifestOutput
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return manifest
}

func assertManifestArtifactType(t *testing.T, manifest evidenceManifestOutput, artifactType string) {
	t.Helper()

	for _, item := range manifest.Artifacts {
		if item.ArtifactType == artifactType {
			return
		}
	}
	t.Fatalf("expected artifact type %s in %#v", artifactType, manifest.Artifacts)
}

type evidenceManifestOutput struct {
	SchemaVersion string                         `json:"schema_version"`
	PackType      string                         `json:"pack_type"`
	Artifacts     []evidenceManifestArtifactItem `json:"artifacts"`
	Reports       []evidenceManifestReportItem   `json:"reports"`
}

type evidenceManifestArtifactItem struct {
	Path         string `json:"path"`
	ArtifactType string `json:"artifact_type"`
}

type evidenceManifestReportItem struct {
	Path string `json:"path"`
}
