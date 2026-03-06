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

func TestSkillBaselineSaveAndCompare(t *testing.T) {
	t.Parallel()

	baselineRoot := t.TempDir()
	candidateRoot := t.TempDir()
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")

	testutil.WriteFiles(t, baselineRoot, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidateRoot, map[string]string{
		"SKILL.md": routingEvalBroadGenericSkillMarkdown(),
	})

	stdout, stderr, code, err := executeSkillBaseline(t, "save", baselineRoot, "--output", baselinePath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Saved baseline snapshot") {
		t.Fatalf("expected baseline save output, got %q", stdout)
	}

	content, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline snapshot artifact: %v", err)
	}

	var snapshot skillBaselineSnapshotArtifactOutput
	if err := json.Unmarshal(content, &snapshot); err != nil {
		t.Fatalf("expected valid baseline snapshot artifact, got %v; output=%q", err, string(content))
	}
	if snapshot.ArtifactType != "firety.skill-baseline" {
		t.Fatalf("unexpected artifact type %#v", snapshot)
	}
	if snapshot.Snapshot.Summary.Scope != "lint" {
		t.Fatalf("expected lint-only baseline scope, got %#v", snapshot.Snapshot.Summary)
	}

	stdout, stderr, code, err = executeSkillBaseline(t, "compare", candidateRoot, "--baseline", baselinePath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: REGRESSED") {
		t.Fatalf("expected baseline regression summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Top regressions:") {
		t.Fatalf("expected regressions section, got %q", stdout)
	}
}

func TestSkillBaselineSaveFromExistingArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	lintArtifactPath := filepath.Join(t.TempDir(), "lint.json")
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, code, err := executeSkillLint(t, root, "--artifact", lintArtifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeSkillBaseline(t, "save", "--output", baselinePath, "--input-artifact", lintArtifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Saved baseline snapshot") {
		t.Fatalf("expected baseline save output, got %q", stdout)
	}

	content, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline snapshot artifact: %v", err)
	}

	var snapshot skillBaselineSnapshotArtifactOutput
	if err := json.Unmarshal(content, &snapshot); err != nil {
		t.Fatalf("expected valid baseline snapshot artifact, got %v; output=%q", err, string(content))
	}
	if snapshot.Snapshot.Context.Target != root {
		t.Fatalf("expected snapshot target %q, got %#v", root, snapshot.Snapshot.Context)
	}
}

func TestSkillGateCommandSupportsBaselineAwareRegressionChecks(t *testing.T) {
	t.Parallel()

	baselineRoot := t.TempDir()
	candidateRoot := t.TempDir()
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")

	testutil.WriteFiles(t, baselineRoot, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidateRoot, map[string]string{
		"SKILL.md": routingEvalBroadGenericSkillMarkdown(),
	})

	_, _, code, err := executeSkillBaseline(t, "save", baselineRoot, "--output", baselinePath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeSkillGate(t,
		candidateRoot,
		"--baseline", baselinePath,
		"--fail-on-routing-risk-regression",
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
	if !strings.Contains(stdout, "Routing risk regressed versus the baseline") {
		t.Fatalf("expected routing-risk regression failure, got %q", stdout)
	}
}

func TestSkillRenderCommandSupportsBaselineCompareArtifact(t *testing.T) {
	t.Parallel()

	baselineRoot := t.TempDir()
	candidateRoot := t.TempDir()
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	compareArtifactPath := filepath.Join(t.TempDir(), "baseline-compare.json")

	testutil.WriteFiles(t, baselineRoot, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidateRoot, map[string]string{
		"SKILL.md": routingEvalBroadGenericSkillMarkdown(),
	})

	_, _, code, err := executeSkillBaseline(t, "save", baselineRoot, "--output", baselinePath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	_, _, code, err = executeSkillBaseline(t, "compare", candidateRoot, "--baseline", baselinePath, "--artifact", compareArtifactPath, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	stdout, stderr, code, err := executeSkillRender(t, compareArtifactPath, "--render", "ci-summary")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Firety Baseline Compare") {
		t.Fatalf("expected baseline compare render title, got %q", stdout)
	}
	if !strings.Contains(stdout, "Status: regressed") {
		t.Fatalf("expected baseline compare render summary, got %q", stdout)
	}
}

func executeSkillBaseline(t *testing.T, subcommand string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "baseline", subcommand}, args...)
	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

type skillBaselineSnapshotArtifactOutput struct {
	ArtifactType string `json:"artifact_type"`
	Snapshot     struct {
		Context struct {
			Target string `json:"target"`
		} `json:"context"`
		Summary struct {
			Scope string `json:"scope"`
		} `json:"summary"`
	} `json:"snapshot"`
}
