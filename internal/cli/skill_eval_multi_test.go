package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/testutil"
)

func TestSkillEvalCommandMultiBackendTextOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goodRunner := writeRoutingEvalRunner(t, "good")
	badRunner := writeRoutingEvalRunner(t, "always-trigger")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	stdout, stderr, code, err := executeSkillEval(t, root,
		"--backend", "codex="+goodRunner,
		"--backend", "claude-code="+badRunner,
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
	if !strings.Contains(stdout, "Per-backend results:") {
		t.Fatalf("expected per-backend summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Differing cases:") {
		t.Fatalf("expected differing cases section, got %q", stdout)
	}
	if !strings.Contains(stdout, "Strongest backend: Codex") {
		t.Fatalf("expected strongest backend summary, got %q", stdout)
	}
}

func TestSkillEvalCommandMultiBackendJSONOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goodRunner := writeRoutingEvalRunner(t, "good")
	narrowRunner := writeRoutingEvalRunner(t, "compare")
	files := testutil.RoutingEvalSkillFiles()
	files["SKILL.md"] = testutil.RoutingEvalPortableSkillMarkdown() + "\n\neval-mode: narrow\n"
	testutil.WriteFiles(t, root, files)

	stdout, stderr, code, err := executeSkillEval(t, root,
		"--backend", "codex="+goodRunner,
		"--backend", "cursor="+narrowRunner,
		"--format", "json",
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

	payload := decodeSkillEvalMultiJSONOutput(t, stdout)
	if payload.Summary.BackendCount != 2 {
		t.Fatalf("expected two backends, got %#v", payload.Summary)
	}
	if len(payload.Backends) != 2 {
		t.Fatalf("expected per-backend results, got %#v", payload.Backends)
	}
	if len(payload.DifferingCases) == 0 {
		t.Fatalf("expected differing cases, got %#v", payload)
	}
}

func TestSkillEvalCommandMultiBackendArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goodRunner := writeRoutingEvalRunner(t, "good")
	badRunner := writeRoutingEvalRunner(t, "always-trigger")
	artifactPath := filepath.Join(t.TempDir(), "eval-multi-artifact.json")
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	_, stderr, code, err := executeSkillEval(t, root,
		"--backend", "codex="+goodRunner,
		"--backend", "copilot="+badRunner,
		"--artifact", artifactPath,
		"--format", "json",
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

	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read multi-backend artifact: %v", err)
	}

	var payload skillEvalMultiArtifactOutput
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("expected valid multi-backend eval artifact, got %v; output=%q", err, string(content))
	}
	if payload.ArtifactType != "firety.skill-routing-eval-multi" {
		t.Fatalf("unexpected artifact type, got %#v", payload)
	}
	if payload.Summary.BackendCount != 2 {
		t.Fatalf("expected backend count in artifact, got %#v", payload)
	}
}

func TestSkillEvalCommandRejectsDuplicateBackends(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeSkillEval(t, root,
		"--backend", "codex="+runner,
		"--backend", "codex="+runner,
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
	if !strings.Contains(err.Error(), `selected more than once`) {
		t.Fatalf("expected duplicate backend error, got %v", err)
	}
}

func TestSkillEvalCommandRejectsUnsupportedBackend(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeSkillEval(t, root,
		"--backend", "codex="+runner,
		"--backend", "atlas="+runner,
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
	if !strings.Contains(err.Error(), `unsupported backend "atlas"`) {
		t.Fatalf("expected unsupported backend error, got %v", err)
	}
}

func TestSkillEvalCommandRejectsRunnerWithBackend(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, root, testutil.RoutingEvalSkillFiles())

	stdout, stderr, code, err := executeSkillEval(t, root,
		"--runner", runner,
		"--backend", "codex="+runner,
		"--backend", "cursor="+runner,
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
	if !strings.Contains(err.Error(), `cannot be combined`) {
		t.Fatalf("expected runner/backend validation error, got %v", err)
	}
}

func decodeSkillEvalMultiJSONOutput(t *testing.T, output string) skillEvalMultiJSONOutput {
	t.Helper()

	var payload skillEvalMultiJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid multi-backend eval json, got %v; output=%q", err, output)
	}

	return payload
}

type skillEvalMultiJSONOutput struct {
	SchemaVersion  string                           `json:"schema_version"`
	Backends       []skillEvalMultiBackendOutput    `json:"backends"`
	Summary        skillEvalMultiSummaryOutput      `json:"summary"`
	DifferingCases []skillEvalMultiDifferingCaseOut `json:"differing_cases"`
}

type skillEvalMultiBackendOutput struct {
	Backend skillEvalBackendInfoOutput `json:"backend"`
	Summary skillEvalSummaryOutput     `json:"summary"`
}

type skillEvalBackendInfoOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type skillEvalMultiSummaryOutput struct {
	BackendCount int `json:"backend_count"`
}

type skillEvalMultiDifferingCaseOut struct {
	ID string `json:"id"`
}

type skillEvalMultiArtifactOutput struct {
	ArtifactType string                      `json:"artifact_type"`
	Summary      skillEvalMultiSummaryOutput `json:"summary"`
}
