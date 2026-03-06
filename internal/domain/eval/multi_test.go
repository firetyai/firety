package eval_test

import (
	"testing"

	"github.com/firety/firety/internal/domain/eval"
)

func TestBuildMultiBackendEvalReport(t *testing.T) {
	t.Parallel()

	report, err := eval.BuildMultiBackendEvalReport("/tmp/skill", []eval.RoutingEvalReport{
		{
			Target: "/tmp/skill",
			Suite:  eval.RoutingEvalSuiteInfo{Name: "suite", SchemaVersion: "1", CaseCount: 2},
			Backend: eval.RoutingEvalBackendInfo{
				ID:   "codex",
				Name: "Codex",
			},
			Summary: eval.RoutingEvalSummary{
				Total:          2,
				Passed:         2,
				Failed:         0,
				FalsePositives: 0,
				FalseNegatives: 0,
				PassRate:       1,
			},
			Results: []eval.RoutingEvalCaseResult{
				{ID: "negative", Prompt: "negative", Expectation: eval.RoutingShouldNotTrigger, Passed: true, ActualTrigger: false},
				{ID: "positive", Prompt: "positive", Expectation: eval.RoutingShouldTrigger, Passed: true, ActualTrigger: true},
			},
		},
		{
			Target: "/tmp/skill",
			Suite:  eval.RoutingEvalSuiteInfo{Name: "suite", SchemaVersion: "1", CaseCount: 2},
			Backend: eval.RoutingEvalBackendInfo{
				ID:   "claude-code",
				Name: "Claude Code",
			},
			Summary: eval.RoutingEvalSummary{
				Total:          2,
				Passed:         1,
				Failed:         1,
				FalsePositives: 1,
				FalseNegatives: 0,
				PassRate:       0.5,
			},
			Results: []eval.RoutingEvalCaseResult{
				{ID: "negative", Prompt: "negative", Expectation: eval.RoutingShouldNotTrigger, Passed: false, ActualTrigger: true, FailureKind: eval.RoutingFalsePositive},
				{ID: "positive", Prompt: "positive", Expectation: eval.RoutingShouldTrigger, Passed: true, ActualTrigger: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if report.Summary.StrongestBackend != "Codex" {
		t.Fatalf("expected strongest backend Codex, got %#v", report.Summary)
	}
	if report.Summary.WeakestBackend != "Claude Code" {
		t.Fatalf("expected weakest backend Claude Code, got %#v", report.Summary)
	}
	if report.Summary.DifferingCaseCount != 1 {
		t.Fatalf("expected one differing case, got %#v", report.Summary)
	}
	if len(report.DifferingCases) != 1 || report.DifferingCases[0].ID != "negative" {
		t.Fatalf("expected differing negative case, got %#v", report.DifferingCases)
	}
	if len(report.Summary.BackendSpecificStrengths) != 1 || len(report.Summary.BackendSpecificMisses) != 1 {
		t.Fatalf("expected backend-specific summaries, got %#v", report.Summary)
	}
}

func TestCompareMultiBackendReports(t *testing.T) {
	t.Parallel()

	base, err := eval.BuildMultiBackendEvalReport("/tmp/base", []eval.RoutingEvalReport{
		{
			Target:  "/tmp/base",
			Suite:   eval.RoutingEvalSuiteInfo{Name: "suite", SchemaVersion: "1", CaseCount: 2},
			Backend: eval.RoutingEvalBackendInfo{ID: "codex", Name: "Codex"},
			Summary: eval.RoutingEvalSummary{Total: 2, Passed: 1, Failed: 1, FalseNegatives: 1, PassRate: 0.5},
			Results: []eval.RoutingEvalCaseResult{
				{ID: "negative", Prompt: "negative", Expectation: eval.RoutingShouldNotTrigger, Passed: true, ActualTrigger: false},
				{ID: "positive", Prompt: "positive", Expectation: eval.RoutingShouldTrigger, Passed: false, ActualTrigger: false, FailureKind: eval.RoutingFalseNegative},
			},
		},
		{
			Target:  "/tmp/base",
			Suite:   eval.RoutingEvalSuiteInfo{Name: "suite", SchemaVersion: "1", CaseCount: 2},
			Backend: eval.RoutingEvalBackendInfo{ID: "cursor", Name: "Cursor"},
			Summary: eval.RoutingEvalSummary{Total: 2, Passed: 2, Failed: 0, PassRate: 1},
			Results: []eval.RoutingEvalCaseResult{
				{ID: "negative", Prompt: "negative", Expectation: eval.RoutingShouldNotTrigger, Passed: true, ActualTrigger: false},
				{ID: "positive", Prompt: "positive", Expectation: eval.RoutingShouldTrigger, Passed: true, ActualTrigger: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("build base report: %v", err)
	}

	candidate, err := eval.BuildMultiBackendEvalReport("/tmp/candidate", []eval.RoutingEvalReport{
		{
			Target:  "/tmp/candidate",
			Suite:   eval.RoutingEvalSuiteInfo{Name: "suite", SchemaVersion: "1", CaseCount: 2},
			Backend: eval.RoutingEvalBackendInfo{ID: "codex", Name: "Codex"},
			Summary: eval.RoutingEvalSummary{Total: 2, Passed: 2, Failed: 0, PassRate: 1},
			Results: []eval.RoutingEvalCaseResult{
				{ID: "negative", Prompt: "negative", Expectation: eval.RoutingShouldNotTrigger, Passed: true, ActualTrigger: false},
				{ID: "positive", Prompt: "positive", Expectation: eval.RoutingShouldTrigger, Passed: true, ActualTrigger: true},
			},
		},
		{
			Target:  "/tmp/candidate",
			Suite:   eval.RoutingEvalSuiteInfo{Name: "suite", SchemaVersion: "1", CaseCount: 2},
			Backend: eval.RoutingEvalBackendInfo{ID: "cursor", Name: "Cursor"},
			Summary: eval.RoutingEvalSummary{Total: 2, Passed: 1, Failed: 1, FalsePositives: 1, PassRate: 0.5},
			Results: []eval.RoutingEvalCaseResult{
				{ID: "negative", Prompt: "negative", Expectation: eval.RoutingShouldNotTrigger, Passed: false, ActualTrigger: true, FailureKind: eval.RoutingFalsePositive},
				{ID: "positive", Prompt: "positive", Expectation: eval.RoutingShouldTrigger, Passed: true, ActualTrigger: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("build candidate report: %v", err)
	}

	comparison, err := eval.CompareMultiBackendReports(base, candidate)
	if err != nil {
		t.Fatalf("compare multi-backend reports: %v", err)
	}

	if comparison.AggregateSummary.Overall != eval.ComparisonMixed {
		t.Fatalf("expected mixed aggregate summary, got %#v", comparison.AggregateSummary)
	}
	if len(comparison.PerBackend) != 2 {
		t.Fatalf("expected two backend deltas, got %#v", comparison.PerBackend)
	}
	if comparison.PerBackend[0].Comparison.Summary.Overall != eval.ComparisonImproved {
		t.Fatalf("expected first backend improved, got %#v", comparison.PerBackend[0].Comparison.Summary)
	}
	if comparison.PerBackend[1].Comparison.Summary.Overall != eval.ComparisonRegressed {
		t.Fatalf("expected second backend regressed, got %#v", comparison.PerBackend[1].Comparison.Summary)
	}
	if len(comparison.WidenedDisagreements) != 1 {
		t.Fatalf("expected widened disagreement, got %#v", comparison.WidenedDisagreements)
	}
	if comparison.WidenedDisagreements[0].ID != "negative" {
		t.Fatalf("expected negative case to widen disagreement, got %#v", comparison.WidenedDisagreements)
	}
}
