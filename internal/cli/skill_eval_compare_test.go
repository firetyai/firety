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

func TestSkillEvalCompareCommandImproved(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("narrow"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("good"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: IMPROVED") {
		t.Fatalf("expected improved summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Flipped to pass") {
		t.Fatalf("expected flipped-to-pass section, got %q", stdout)
	}
}

func TestSkillEvalCompareCommandRegressed(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("good"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("broad"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: REGRESSED") {
		t.Fatalf("expected regressed summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Flipped to fail") {
		t.Fatalf("expected flipped-to-fail section, got %q", stdout)
	}
}

func TestSkillEvalCompareCommandMixed(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("narrow"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("mixed"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: MIXED") {
		t.Fatalf("expected mixed summary, got %q", stdout)
	}
}

func TestSkillEvalCompareCommandUnchanged(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("good"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("good"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate, "--runner", runner)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Overall: UNCHANGED") {
		t.Fatalf("expected unchanged summary, got %q", stdout)
	}
}

func TestSkillEvalCompareCommandJSONOutput(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("good"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("broad"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate, "--runner", runner, "--format", "json")
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeLint {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	payload := decodeSkillEvalCompareJSONOutput(t, stdout)
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if payload.Summary.Overall != "regressed" {
		t.Fatalf("expected regressed summary, got %#v", payload.Summary)
	}
	if payload.Summary.MetricsDelta.FalsePositives <= 0 {
		t.Fatalf("expected false-positive regression, got %#v", payload.Summary)
	}
	if len(payload.FlippedToFail) == 0 {
		t.Fatalf("expected flipped-to-fail cases, got %#v", payload)
	}
}

func TestSkillEvalCompareCommandArtifact(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	artifactPath := filepath.Join(t.TempDir(), "eval-compare-artifact.json")
	runner := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("good"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("broad"),
	})

	_, stderr, code, err := executeSkillEvalCompare(t, base, candidate, "--runner", runner, "--artifact", artifactPath, "--format", "json")
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
		t.Fatalf("read compare artifact: %v", err)
	}

	var payload skillEvalCompareArtifactOutput
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("expected valid compare artifact, got %v; output=%q", err, string(content))
	}
	if payload.SchemaVersion != "1" || payload.ArtifactType != "firety.skill-routing-eval-compare" {
		t.Fatalf("unexpected compare artifact metadata, got %#v", payload)
	}
	if payload.Comparison.Overall != "regressed" {
		t.Fatalf("expected regressed compare summary, got %#v", payload)
	}
}

func TestSkillEvalCompareCommandMultiBackendImproved(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runnerA := writeRoutingEvalRunner(t, "compare")
	runnerB := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("narrow"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("good"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate,
		"--backend", "codex="+runnerA,
		"--backend", "cursor="+runnerB,
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
	if !strings.Contains(stdout, "Overall: IMPROVED") {
		t.Fatalf("expected improved multi-backend summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Per-backend deltas:") {
		t.Fatalf("expected per-backend section, got %q", stdout)
	}
}

func TestSkillEvalCompareCommandMultiBackendMixed(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	improveRunner := writeRoutingEvalRunner(t, "compare")
	regressRunner := writeRoutingEvalRunner(t, "regress-good")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("broad"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("good"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate,
		"--backend", "codex="+improveRunner,
		"--backend", "cursor="+regressRunner,
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
	if !strings.Contains(stdout, "Overall: MIXED") {
		t.Fatalf("expected mixed multi-backend summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "Codex: IMPROVED") || !strings.Contains(stdout, "Cursor: REGRESSED") {
		t.Fatalf("expected backend-specific improvement and regression, got %q", stdout)
	}
}

func TestSkillEvalCompareCommandMultiBackendWidenedAndNarrowedDisagreement(t *testing.T) {
	t.Parallel()

	t.Run("widened", func(t *testing.T) {
		base := t.TempDir()
		candidate := t.TempDir()
		compareRunner := writeRoutingEvalRunner(t, "compare")
		goodRunner := writeRoutingEvalRunner(t, "good")
		testutil.WriteFiles(t, base, map[string]string{
			"SKILL.md":           compareEvalSkillMarkdown("good"),
			"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
		})
		testutil.WriteFiles(t, candidate, map[string]string{
			"SKILL.md": compareEvalSkillMarkdown("broad"),
		})

		stdout, _, code, err := executeSkillEvalCompare(t, base, candidate,
			"--backend", "codex="+compareRunner,
			"--backend", "cursor="+goodRunner,
		)
		if err != nil {
			t.Fatalf("expected no runtime error, got %v", err)
		}
		if code != cli.ExitCodeLint {
			t.Fatalf("expected exit code %d, got %d", cli.ExitCodeLint, code)
		}
		if !strings.Contains(stdout, "Widened disagreements") {
			t.Fatalf("expected widened disagreement section, got %q", stdout)
		}
	})

	t.Run("narrowed-json-artifact", func(t *testing.T) {
		base := t.TempDir()
		candidate := t.TempDir()
		compareRunner := writeRoutingEvalRunner(t, "compare")
		goodRunner := writeRoutingEvalRunner(t, "good")
		artifactPath := filepath.Join(t.TempDir(), "eval-compare-multi-artifact.json")
		testutil.WriteFiles(t, base, map[string]string{
			"SKILL.md":           compareEvalSkillMarkdown("narrow"),
			"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
		})
		testutil.WriteFiles(t, candidate, map[string]string{
			"SKILL.md": compareEvalSkillMarkdown("good"),
		})

		stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate,
			"--backend", "codex="+compareRunner,
			"--backend", "cursor="+goodRunner,
			"--format", "json",
			"--artifact", artifactPath,
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

		payload := decodeSkillEvalMultiCompareJSONOutput(t, stdout)
		if payload.AggregateSummary.Overall != "improved" {
			t.Fatalf("expected improved aggregate summary, got %#v", payload.AggregateSummary)
		}
		if len(payload.NarrowedDisagreements) == 0 {
			t.Fatalf("expected narrowed disagreements, got %#v", payload)
		}
		if len(payload.PerBackendDeltas) != 2 {
			t.Fatalf("expected per-backend deltas, got %#v", payload)
		}

		content, err := os.ReadFile(artifactPath)
		if err != nil {
			t.Fatalf("read compare artifact: %v", err)
		}
		var artifactPayload skillEvalMultiCompareArtifactOutput
		if err := json.Unmarshal(content, &artifactPayload); err != nil {
			t.Fatalf("expected valid compare artifact, got %v; output=%q", err, string(content))
		}
		if artifactPayload.ArtifactType != "firety.skill-routing-eval-compare-multi" {
			t.Fatalf("unexpected compare artifact type, got %#v", artifactPayload)
		}
		if artifactPayload.AggregateSummary.NarrowedDisagreementCount == 0 {
			t.Fatalf("expected narrowed disagreement count in artifact, got %#v", artifactPayload)
		}
	})
}

func TestSkillEvalCompareCommandInvalidFormat(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runner := writeRoutingEvalRunner(t, "compare")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("good"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("good"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate, "--runner", runner, "--format", "sarif")
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(err.Error(), `invalid format "sarif"`) {
		t.Fatalf("expected invalid format error, got %v", err)
	}
}

func TestSkillEvalCompareCommandRejectsRunnerWithBackend(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("good"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("good"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate,
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
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected runner/backend validation error, got %v", err)
	}
}

func TestSkillEvalCompareCommandRejectsUnsupportedBackend(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	candidate := t.TempDir()
	runner := writeRoutingEvalRunner(t, "good")
	testutil.WriteFiles(t, base, map[string]string{
		"SKILL.md":           compareEvalSkillMarkdown("good"),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})
	testutil.WriteFiles(t, candidate, map[string]string{
		"SKILL.md": compareEvalSkillMarkdown("good"),
	})

	stdout, stderr, code, err := executeSkillEvalCompare(t, base, candidate,
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

func executeSkillEvalCompare(t *testing.T, base, candidate string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "eval-compare", base, candidate}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

func decodeSkillEvalCompareJSONOutput(t *testing.T, output string) skillEvalCompareJSONOutput {
	t.Helper()

	var payload skillEvalCompareJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid eval compare json, got %v; output=%q", err, output)
	}
	return payload
}

func compareEvalSkillMarkdown(mode string) string {
	return strings.Join([]string{
		testutil.RoutingEvalPortableSkillMarkdown(),
		"",
		"eval-mode: " + mode,
	}, "\n")
}

type skillEvalCompareJSONOutput struct {
	SchemaVersion string                        `json:"schema_version"`
	Summary       skillEvalCompareSummaryOutput `json:"summary"`
	FlippedToFail []skillEvalCompareCaseOutput  `json:"flipped_to_fail"`
}

type skillEvalCompareSummaryOutput struct {
	Overall      string                        `json:"overall"`
	MetricsDelta skillEvalCompareMetricsOutput `json:"metrics_delta"`
}

type skillEvalCompareMetricsOutput struct {
	FalsePositives int `json:"false_positives"`
}

type skillEvalCompareCaseOutput struct {
	ID string `json:"id"`
}

type skillEvalCompareArtifactOutput struct {
	SchemaVersion string                        `json:"schema_version"`
	ArtifactType  string                        `json:"artifact_type"`
	Comparison    skillEvalCompareSummaryOutput `json:"comparison"`
}

type skillEvalMultiCompareJSONOutput struct {
	SchemaVersion         string                               `json:"schema_version"`
	AggregateSummary      skillEvalMultiCompareSummaryOutput   `json:"aggregate_summary"`
	PerBackendDeltas      []skillEvalMultiCompareBackendOutput `json:"per_backend_deltas"`
	NarrowedDisagreements []skillEvalCompareCaseOutput         `json:"narrowed_disagreements"`
}

type skillEvalMultiCompareSummaryOutput struct {
	Overall string `json:"overall"`
}

type skillEvalMultiCompareBackendOutput struct {
	Backend skillEvalBackendInfoOutput `json:"backend"`
}

type skillEvalMultiCompareArtifactOutput struct {
	ArtifactType     string                               `json:"artifact_type"`
	AggregateSummary skillEvalMultiCompareArtifactSummary `json:"aggregate_summary"`
}

type skillEvalMultiCompareArtifactSummary struct {
	NarrowedDisagreementCount int `json:"narrowed_disagreement_count"`
}

func decodeSkillEvalMultiCompareJSONOutput(t *testing.T, output string) skillEvalMultiCompareJSONOutput {
	t.Helper()

	var payload skillEvalMultiCompareJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid multi compare json, got %v; output=%q", err, output)
	}
	return payload
}
