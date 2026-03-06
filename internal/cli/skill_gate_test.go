package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

func TestSkillGateCommandFailsOnLintErrorsByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	stdout, stderr, code, err := executeSkillGate(t, root)
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
		t.Fatalf("expected fail decision, got %q", stdout)
	}
	if !strings.Contains(stdout, "Lint errors exceed policy") && !strings.Contains(stdout, "blocking") {
		t.Fatalf("expected lint gate failure details, got %q", stdout)
	}
}

func TestSkillGateCommandJSONAndArtifactFromCompareArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lintComparePath := filepath.Join(dir, "lint-compare.json")
	evalComparePath := filepath.Join(dir, "eval-compare.json")
	gateArtifactPath := filepath.Join(dir, "gate.json")

	writeJSONFile(t, lintComparePath, artifact.SkillLintCompareArtifact{
		SchemaVersion: "1",
		ArtifactType:  "firety.skill-lint-compare",
		Run: artifact.SkillLintCompareArtifactRun{
			BaseTarget:      "base-skill",
			CandidateTarget: "candidate-skill",
		},
		Candidate: artifact.SkillLintArtifactSummary{
			ErrorCount:   0,
			WarningCount: 2,
		},
		Comparison: lint.ReportComparisonSummary{
			Overall: lint.ComparisonRegressed,
			Summary: "The candidate introduces new lint regressions.",
		},
		AddedFindings: []artifact.SkillLintArtifactFinding{
			{
				RuleID:   lint.RuleMixedEcosystemGuidance.ID,
				Category: string(lint.CategoryPortability),
				Severity: string(lint.SeverityWarning),
			},
		},
	})
	writeJSONFile(t, evalComparePath, artifact.SkillEvalCompareArtifact{
		SchemaVersion: "1",
		ArtifactType:  "firety.skill-routing-eval-compare",
		Run: artifact.SkillEvalCompareArtifactRun{
			BaseTarget:      "base-skill",
			CandidateTarget: "candidate-skill",
		},
		Suite:   eval.RoutingEvalSuiteInfo{Name: "suite", CaseCount: 4},
		Backend: eval.RoutingEvalBackendInfo{Name: "Generic"},
		Base: eval.RoutingEvalSideSummary{
			Target:  "base-skill",
			Summary: eval.RoutingEvalSummary{Total: 4, Passed: 4, PassRate: 1},
		},
		Candidate: eval.RoutingEvalSideSummary{
			Target:  "candidate-skill",
			Summary: eval.RoutingEvalSummary{Total: 4, Passed: 3, Failed: 1, FalsePositives: 1, PassRate: 0.75},
		},
		Comparison: eval.RoutingEvalComparisonSummary{
			Overall: eval.ComparisonRegressed,
			Summary: "The candidate regresses measured routing performance.",
			MetricsDelta: eval.RoutingEvalMetricsDelta{
				Failed:         1,
				FalsePositives: 1,
				PassRate:       -0.25,
			},
		},
		FlippedToFail: []eval.RoutingEvalCaseChange{
			{ID: "neg-1", CandidateFailureKind: eval.RoutingFalsePositive},
		},
	})

	stdout, stderr, code, err := executeSkillGate(t,
		"--input-artifact", lintComparePath,
		"--input-artifact", evalComparePath,
		"--format", "json",
		"--max-pass-rate-regression", "0",
		"--fail-on-new-portability-regressions",
		"--artifact", gateArtifactPath,
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

	payload := decodeSkillGateJSONOutput(t, stdout)
	if payload.SchemaVersion != "1" {
		t.Fatalf("expected schema version 1, got %#v", payload)
	}
	if payload.Result.Decision != "fail" {
		t.Fatalf("expected fail decision, got %#v", payload)
	}
	if len(payload.Result.BlockingReasons) < 2 {
		t.Fatalf("expected multiple blocking reasons, got %#v", payload.Result.BlockingReasons)
	}

	content, err := os.ReadFile(gateArtifactPath)
	if err != nil {
		t.Fatalf("read gate artifact: %v", err)
	}
	var gateArtifact struct {
		ArtifactType string `json:"artifact_type"`
		Result       struct {
			Decision string `json:"decision"`
		} `json:"result"`
	}
	if err := json.Unmarshal(content, &gateArtifact); err != nil {
		t.Fatalf("expected valid gate artifact, got %v; output=%q", err, string(content))
	}
	if gateArtifact.ArtifactType != "firety.skill-quality-gate" || gateArtifact.Result.Decision != "fail" {
		t.Fatalf("unexpected gate artifact payload %#v", gateArtifact)
	}
}

func TestSkillGateCommandErrorsWhenCriterionNeedsMissingEvidence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lintArtifactPath := filepath.Join(dir, "lint.json")
	writeJSONFile(t, lintArtifactPath, artifact.SkillLintArtifact{
		SchemaVersion: "1",
		ArtifactType:  "firety.skill-lint",
		Run: artifact.SkillLintArtifactRun{
			Target: "candidate-skill",
		},
		Summary: artifact.SkillLintArtifactSummary{
			ErrorCount:   0,
			WarningCount: 1,
		},
	})

	stdout, stderr, code, err := executeSkillGate(t,
		"--input-artifact", lintArtifactPath,
		"--min-eval-pass-rate", "90",
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
	if !strings.Contains(err.Error(), "requires eval evidence") {
		t.Fatalf("expected missing eval evidence error, got %v", err)
	}
}

func TestSkillGateCommandFailsOnPerBackendThreshold(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	multiArtifactPath := filepath.Join(dir, "multi.json")
	writeJSONFile(t, multiArtifactPath, artifact.SkillEvalMultiArtifact{
		SchemaVersion: "1",
		ArtifactType:  "firety.skill-routing-eval-multi",
		Run: artifact.SkillEvalMultiArtifactRun{
			Target: "candidate-skill",
		},
		Suite: eval.RoutingEvalSuiteInfo{Name: "suite", CaseCount: 4},
		Results: []eval.BackendEvalReport{
			{
				Backend: eval.RoutingEvalBackendInfo{ID: "codex", Name: "Codex"},
				Summary: eval.RoutingEvalSummary{Total: 4, Passed: 4, PassRate: 1},
			},
			{
				Backend: eval.RoutingEvalBackendInfo{ID: "cursor", Name: "Cursor"},
				Summary: eval.RoutingEvalSummary{Total: 4, Passed: 2, Failed: 2, FalseNegatives: 2, PassRate: 0.5},
			},
		},
		Summary: eval.MultiBackendEvalSummary{
			BackendCount:       2,
			TotalCases:         4,
			DifferingCaseCount: 2,
		},
		DifferingCases: []eval.MultiBackendDifferingCase{
			{ID: "case-a"},
			{ID: "case-b"},
		},
	})

	stdout, stderr, code, err := executeSkillGate(t,
		"--input-artifact", multiArtifactPath,
		"--min-per-backend-pass-rate", "100",
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
	if !strings.Contains(stdout, "Per backend:") || !strings.Contains(stdout, "Cursor: FAIL") {
		t.Fatalf("expected per-backend failure output, got %q", stdout)
	}
}

func executeSkillGate(t *testing.T, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", "gate"}, args...)

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		commandArgs...,
	)

	return stdout.String(), stderr.String(), code, err
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func decodeSkillGateJSONOutput(t *testing.T, output string) skillGateJSONOutput {
	t.Helper()

	var payload skillGateJSONOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected valid gate json, got %v; output=%q", err, output)
	}
	return payload
}

type skillGateJSONOutput struct {
	SchemaVersion string              `json:"schema_version"`
	Result        skillGateJSONResult `json:"result"`
}

type skillGateJSONResult struct {
	Decision        string                `json:"decision"`
	BlockingReasons []skillGateJSONReason `json:"blocking_reasons"`
}

type skillGateJSONReason struct {
	Code string `json:"code"`
}
