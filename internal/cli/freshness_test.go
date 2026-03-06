package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/testutil"
)

func TestFreshnessInspectFreshLintArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeFreshness(t, "inspect", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	report := decodeFreshnessJSON(t, stdout)
	if report.Freshness.FreshnessStatus != "fresh" {
		t.Fatalf("expected fresh status, got %#v", report.Freshness)
	}
	assertUseStatus(t, report.Freshness, "compare", "fresh")
	assertUseStatus(t, report.Freshness, "baseline", "fresh")
}

func TestFreshnessInspectStaleLintArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	old := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(artifactPath, old, old); err != nil {
		t.Fatalf("set stale modtime: %v", err)
	}

	stdout, stderr, code, err := executeFreshness(t, "inspect", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	report := decodeFreshnessJSON(t, stdout)
	if report.Freshness.FreshnessStatus != "stale" {
		t.Fatalf("expected stale status, got %#v", report.Freshness)
	}
	assertActionContains(t, report.Freshness.RecertificationActions, "rerun lint")
	assertUseStatus(t, report.Freshness, "compare", "stale")
}

func TestFreshnessInspectEvidencePackUsesSourceArtifactAge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint.json")
	packDir := filepath.Join(t.TempDir(), "pack")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	old := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(artifactPath, old, old); err != nil {
		t.Fatalf("set stale modtime: %v", err)
	}

	_, _, code, err = executeEvidencePack(t, "--input-artifact", artifactPath, "--output", packDir)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeFreshness(t, "inspect", packDir, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	report := decodeFreshnessJSON(t, stdout)
	if report.Freshness.FreshnessStatus != "stale" {
		t.Fatalf("expected stale pack status, got %#v", report.Freshness)
	}
	if len(report.Freshness.StaleComponents) == 0 {
		t.Fatalf("expected stale components, got %#v", report.Freshness)
	}
	assertActionContains(t, report.Freshness.RecertificationActions, "rebuild the evidence pack from fresh artifacts")
	assertActionContains(t, report.Freshness.RecertificationActions, "rerun lint")
}

func TestFreshnessInspectAttestationWithMissingSupportIsInsufficient(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint.json")
	attestationPath := filepath.Join(t.TempDir(), "attestation.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	_, _, code, err = executeSkillAttest(t, "--input-artifact", artifactPath, "--artifact", attestationPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if err := os.Remove(artifactPath); err != nil {
		t.Fatalf("remove supporting artifact: %v", err)
	}

	stdout, stderr, code, err := executeFreshness(t, "inspect", attestationPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	report := decodeFreshnessJSON(t, stdout)
	if report.Freshness.FreshnessStatus != "insufficient-evidence" {
		t.Fatalf("expected insufficient-evidence status, got %#v", report.Freshness)
	}
	assertActionContains(t, report.Freshness.RecertificationActions, "regenerate the attestation from fresh evidence")
	assertUseStatus(t, report.Freshness, "release-claim", "insufficient-evidence")
}

func executeFreshness(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"freshness"}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)

	return stdout.String(), stderr.String(), code, err
}

func decodeFreshnessJSON(t *testing.T, output string) freshnessJSONOutput {
	t.Helper()

	var payload freshnessJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid freshness json, got %v; output=%q", err, output)
	}
	return payload
}

func assertActionContains(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("expected action %q in %#v", expected, values)
}

func assertUseStatus(t *testing.T, report freshnessJSONReport, use string, status string) {
	t.Helper()
	for _, item := range report.IntendedUseSuitability {
		if item.Use == use {
			if item.Status != status {
				t.Fatalf("expected use %s to have status %s, got %#v", use, status, item)
			}
			return
		}
	}
	t.Fatalf("expected use %s in %#v", use, report.IntendedUseSuitability)
}

type freshnessJSONOutput struct {
	SchemaVersion string              `json:"schema_version"`
	Freshness     freshnessJSONReport `json:"freshness"`
}

type freshnessJSONReport struct {
	FreshnessStatus string `json:"freshness_status"`
	StaleComponents []struct {
		Type string `json:"type"`
	} `json:"stale_components"`
	RecertificationActions []string `json:"recertification_actions"`
	IntendedUseSuitability []struct {
		Use    string `json:"use"`
		Status string `json:"status"`
	} `json:"intended_use_suitability"`
}
