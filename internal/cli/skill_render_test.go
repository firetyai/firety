package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/testutil"
)

func TestSkillRenderCommandPRComment(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "plan-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": routingEvalBroadGenericSkillMarkdown(),
	})

	_, _, _, _ = executeSkillPlan(t, root, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeSkillRender(t, artifactPath, "--render", "pr-comment")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "## Firety Skill Quality") {
		t.Fatalf("expected PR comment title, got %q", stdout)
	}
	if !strings.Contains(stdout, "Review first") || !strings.Contains(stdout, "Tighten trigger wording and boundaries") {
		t.Fatalf("expected prioritized PR summary, got %q", stdout)
	}
	if strings.Contains(stdout, "Artifact:") {
		t.Fatalf("expected compact PR output without artifact path, got %q", stdout)
	}
}

func TestSkillRenderCommandCISummary(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "compare-artifact.json")
	testutil.WriteFiles(t, base, testutil.ValidSkillFiles())
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Helper",
			"description: Helpful skill for many tasks and general assistance when you need support.",
			"---",
			"# Helper",
			"",
			"## When To Use",
			"",
			"Use this skill whenever you need help with tasks, assistance, or general support across many workflows.",
			"",
			"## Usage",
			"",
			"Ask for help and let the skill figure it out.",
			"",
			"## Limitations",
			"",
			"Use judgment.",
			"",
			"## Examples",
			"",
			"Example request: Help with something important.",
			"Example result: A useful answer.",
		}, "\n"),
	})

	_, _, _, _ = executeSkillCompare(t, base, candidate, "--artifact", artifactPath, "--format", "json")

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
	if !strings.Contains(stdout, "### Firety Skill Compare") {
		t.Fatalf("expected CI summary title, got %q", stdout)
	}
	if !strings.Contains(stdout, "Status: regressed") || !strings.Contains(stdout, "Review first") {
		t.Fatalf("expected CI regression summary, got %q", stdout)
	}
	if strings.Contains(stdout, "Artifact:") {
		t.Fatalf("expected CI summary to stay compact, got %q", stdout)
	}
}

func TestSkillRenderCommandFullReportFromAnalysisArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	artifactPath := filepath.Join(t.TempDir(), "analysis-artifact.json")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = routingEvalBroadGenericSkillMarkdown()
	testutil.WriteFiles(t, root, files)

	_, _, _, _ = executeSkillAnalyze(t, root, "--runner", runner, "--artifact", artifactPath, "--format", "json")

	stdout, stderr, code, err := executeSkillRender(t, artifactPath, "--render", "full-report")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "# Firety Skill Analysis") {
		t.Fatalf("expected full report title, got %q", stdout)
	}
	if !strings.Contains(stdout, "Likely contributors") || !strings.Contains(stdout, "Artifact: "+artifactPath) {
		t.Fatalf("expected full analysis report details, got %q", stdout)
	}
}

func TestSkillRenderCommandLowNoiseForStrongSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "lint-artifact.json")
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	_, _, _, _ = executeSkillLint(t, root, "--artifact", artifactPath, "--format", "json", "--routing-risk")

	stdout, stderr, code, err := executeSkillRender(t, artifactPath)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Lint: 0 error(s), 0 warning(s)") {
		t.Fatalf("expected clean lint summary, got %q", stdout)
	}
	if strings.Contains(stdout, "Review first") {
		t.Fatalf("expected low-noise strong-skill render, got %q", stdout)
	}
}

func TestSkillRenderCommandDeterministic(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "plan-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": routingEvalBroadGenericSkillMarkdown(),
	})

	_, _, _, _ = executeSkillPlan(t, root, "--artifact", artifactPath, "--format", "json")

	first, _, code, err := executeSkillRender(t, artifactPath, "--render", "full-report")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	second, _, code, err := executeSkillRender(t, artifactPath, "--render", "full-report")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if first != second {
		t.Fatalf("expected deterministic render output\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestSkillRenderCommandInvalidMode(t *testing.T) {
	t.Parallel()

	stdout, stderr, code, err := executeSkillRender(t, "/tmp/missing.json", "--render", "html")
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

func TestSkillRenderCommandUnsupportedArtifactType(t *testing.T) {
	t.Parallel()

	artifactPath := filepath.Join(t.TempDir(), "unsupported.json")
	if err := os.WriteFile(artifactPath, []byte("{\"artifact_type\":\"firety.unknown\"}\n"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	stdout, stderr, code, err := executeSkillRender(t, artifactPath)
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty stdio, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(err.Error(), `unsupported artifact type "firety.unknown"`) {
		t.Fatalf("expected unsupported artifact error, got %v", err)
	}
}

func executeSkillRender(t *testing.T, artifactPath string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "render", artifactPath}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}
