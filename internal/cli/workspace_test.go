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

func TestWorkspaceLintCommandSummarizesSkills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceFixture(t, root)

	stdout, stderr, code, err := executeWorkspace(t, "lint", root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Skills: 2") || !strings.Contains(stdout, "Per skill:") {
		t.Fatalf("expected workspace lint summary, got %q", stdout)
	}
}

func TestWorkspaceReadinessCommandShowsPerSkillDecisions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceFixture(t, root)

	stdout, stderr, code, err := executeWorkspace(t, "readiness", root, "--context", "merge")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected lint exit code for not-ready workspace, got %d", code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Not ready:") || !strings.Contains(stdout, "Per skill:") {
		t.Fatalf("expected readiness summary, got %q", stdout)
	}
}

func TestWorkspaceGateCommandFailsOnDefaultThresholds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceFixture(t, root)

	stdout, stderr, code, err := executeWorkspace(t, "gate", root, "--context", "merge")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Decision: FAIL") {
		t.Fatalf("expected failing gate summary, got %q", stdout)
	}
}

func TestWorkspaceReportCommandWritesArtifactAndRenders(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "workspace-report.json")
	writeWorkspaceFixture(t, root)

	stdout, stderr, code, err := executeWorkspace(t, "report", root, "--artifact", artifactPath, "--format", "json")
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
		Report        struct {
			WorkspaceRoot string `json:"workspace_root"`
			Summary       struct {
				SkillCount int `json:"skill_count"`
			} `json:"summary"`
		} `json:"report"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid json, got %v; output=%q", err, stdout)
	}
	if payload.SchemaVersion != "1" || payload.Report.Summary.SkillCount != 2 {
		t.Fatalf("unexpected workspace json payload: %#v", payload)
	}

	rendered, stderr, code, err := executeArtifact(t, "render", artifactPath, "--render", "ci-summary")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK || stderr != "" {
		t.Fatalf("expected successful render, got code=%d stderr=%q err=%v", code, stderr, err)
	}
	if !strings.Contains(rendered, "Firety Workspace Report") {
		t.Fatalf("expected workspace render output, got %q", rendered)
	}
}

func TestWorkspaceReportCommandReportsDiscoveryWarnings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceFixture(t, root)
	if err := os.Mkdir(filepath.Join(root, "broken"), 0o000); err != nil {
		t.Fatalf("expected to create broken dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "broken"), 0o755) })

	stdout, stderr, code, err := executeWorkspace(t, "report", root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK || stderr != "" {
		t.Fatalf("expected successful report, got code=%d stderr=%q err=%v", code, stderr, err)
	}
	if !strings.Contains(stdout, "Top priorities:") {
		t.Fatalf("expected top priorities in full report, got %q", stdout)
	}
}

func executeWorkspace(t *testing.T, subcommand string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"workspace", subcommand}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)
	return stdout.String(), stderr.String(), code, err
}

func writeWorkspaceFixture(t *testing.T, root string) {
	t.Helper()
	testutil.WriteFiles(t, root, map[string]string{
		"skills/alpha/SKILL.md":          testutil.ValidSkillFiles()["SKILL.md"],
		"skills/alpha/docs/reference.md": testutil.ValidSkillFiles()["docs/reference.md"],
		"skills/bravo/SKILL.md":          "tiny skill without a markdown title\n",
	})
}
