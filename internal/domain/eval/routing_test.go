package eval_test

import (
	"testing"

	domaineval "github.com/firety/firety/internal/domain/eval"
)

func TestSummarizeRoutingEval(t *testing.T) {
	t.Parallel()

	results := []domaineval.RoutingEvalCaseResult{
		{
			ID:          "positive",
			Profile:     "generic",
			Tags:        []string{"positive"},
			Expectation: domaineval.RoutingShouldTrigger,
			Passed:      true,
		},
		{
			ID:            "false-positive",
			Profile:       "generic",
			Tags:          []string{"negative"},
			Expectation:   domaineval.RoutingShouldNotTrigger,
			ActualTrigger: true,
			FailureKind:   domaineval.RoutingFalsePositive,
		},
		{
			ID:          "false-negative",
			Profile:     "codex",
			Tags:        []string{"positive"},
			Expectation: domaineval.RoutingShouldTrigger,
			FailureKind: domaineval.RoutingFalseNegative,
		},
	}

	summary := domaineval.SummarizeRoutingEval(results)
	if summary.Total != 3 || summary.Passed != 1 || summary.Failed != 2 {
		t.Fatalf("unexpected summary counts: %#v", summary)
	}
	if summary.FalsePositives != 1 || summary.FalseNegatives != 1 {
		t.Fatalf("unexpected false positive/negative counts: %#v", summary)
	}
	if summary.PassRate <= 0.33 || summary.PassRate >= 0.34 {
		t.Fatalf("expected pass rate near 0.333, got %#v", summary)
	}
	if len(summary.ByProfile) != 2 {
		t.Fatalf("expected profile breakdown, got %#v", summary.ByProfile)
	}
	if len(summary.ByTag) != 2 {
		t.Fatalf("expected tag breakdown, got %#v", summary.ByTag)
	}
	if len(summary.NotableMisses) != 2 {
		t.Fatalf("expected notable misses, got %#v", summary.NotableMisses)
	}
}

func TestCompareReports(t *testing.T) {
	t.Parallel()

	base := domaineval.RoutingEvalReport{
		Target: "base",
		Suite: domaineval.RoutingEvalSuiteInfo{
			SchemaVersion: "1",
			Name:          "suite",
			CaseCount:     2,
		},
		Backend: domaineval.RoutingEvalBackendInfo{Name: "command:test"},
		Summary: domaineval.RoutingEvalSummary{
			Total:          2,
			Passed:         1,
			Failed:         1,
			FalsePositives: 0,
			FalseNegatives: 1,
			PassRate:       0.5,
			ByProfile: []domaineval.RoutingEvalBreakdown{
				{Key: "generic", Total: 2, Passed: 1, Failed: 1, FalseNegatives: 1},
			},
		},
		Results: []domaineval.RoutingEvalCaseResult{
			{ID: "positive", Prompt: "positive", Expectation: domaineval.RoutingShouldTrigger, Passed: false, FailureKind: domaineval.RoutingFalseNegative},
			{ID: "negative", Prompt: "negative", Expectation: domaineval.RoutingShouldNotTrigger, Passed: true},
		},
	}
	candidate := domaineval.RoutingEvalReport{
		Target: "candidate",
		Suite: domaineval.RoutingEvalSuiteInfo{
			SchemaVersion: "1",
			Name:          "suite",
			CaseCount:     2,
		},
		Backend: domaineval.RoutingEvalBackendInfo{Name: "command:test"},
		Summary: domaineval.RoutingEvalSummary{
			Total:          2,
			Passed:         1,
			Failed:         1,
			FalsePositives: 1,
			FalseNegatives: 0,
			PassRate:       0.5,
			ByProfile: []domaineval.RoutingEvalBreakdown{
				{Key: "generic", Total: 2, Passed: 1, Failed: 1, FalsePositives: 1},
			},
		},
		Results: []domaineval.RoutingEvalCaseResult{
			{ID: "positive", Prompt: "positive", Expectation: domaineval.RoutingShouldTrigger, Passed: true, ActualTrigger: true},
			{ID: "negative", Prompt: "negative", Expectation: domaineval.RoutingShouldNotTrigger, Passed: false, ActualTrigger: true, FailureKind: domaineval.RoutingFalsePositive},
		},
	}

	comparison, err := domaineval.CompareReports(base, candidate)
	if err != nil {
		t.Fatalf("expected no compare error, got %v", err)
	}
	if comparison.Summary.Overall != domaineval.ComparisonMixed {
		t.Fatalf("expected mixed outcome, got %#v", comparison.Summary)
	}
	if comparison.Summary.FlippedToFailCount != 1 || comparison.Summary.FlippedToPassCount != 1 {
		t.Fatalf("expected one flip each way, got %#v", comparison.Summary)
	}
	if comparison.Summary.MetricsDelta.FalsePositives != 1 || comparison.Summary.MetricsDelta.FalseNegatives != -1 {
		t.Fatalf("unexpected metrics delta, got %#v", comparison.Summary.MetricsDelta)
	}
	if len(comparison.FlippedToFail) != 1 || comparison.FlippedToFail[0].ID != "negative" {
		t.Fatalf("expected negative case to flip to fail, got %#v", comparison.FlippedToFail)
	}
	if len(comparison.FlippedToPass) != 1 || comparison.FlippedToPass[0].ID != "positive" {
		t.Fatalf("expected positive case to flip to pass, got %#v", comparison.FlippedToPass)
	}
	if len(comparison.ByProfileDeltas) != 1 || comparison.ByProfileDeltas[0].Key != "generic" {
		t.Fatalf("expected generic profile delta, got %#v", comparison.ByProfileDeltas)
	}
}
