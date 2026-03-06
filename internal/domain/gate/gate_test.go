package gate_test

import (
	"testing"

	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/lint"
)

func TestEvaluatePassesWithHealthyLintEvidence(t *testing.T) {
	t.Parallel()

	result, err := gate.Evaluate(gate.Criteria{
		MaxLintErrors: intPointer(0),
	}, gate.Evidence{
		LintCurrent: &gate.LintCurrentEvidence{
			Target:       ".",
			ErrorCount:   0,
			WarningCount: 2,
			RuleIDs:      []string{"skill.weak-examples", "skill.short-content"},
			RoutingRisk: &lint.RoutingRiskSummary{
				OverallRisk: lint.RoutingRiskLow,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Decision != gate.DecisionPass {
		t.Fatalf("expected pass decision, got %#v", result)
	}
	if len(result.BlockingReasons) != 0 {
		t.Fatalf("expected no blocking reasons, got %#v", result.BlockingReasons)
	}
}

func TestEvaluateFailsOnCompareRegressions(t *testing.T) {
	t.Parallel()

	result, err := gate.Evaluate(gate.Criteria{
		MaxPassRateRegression:       floatPointer(0),
		MaxFalsePositiveIncrease:    intPointer(0),
		FailOnNewErrors:             true,
		FailOnNewPortabilityRegress: true,
	}, gate.Evidence{
		LintCurrent: &gate.LintCurrentEvidence{
			Target:       "candidate",
			ErrorCount:   1,
			WarningCount: 2,
		},
		LintCompare: &gate.LintCompareEvidence{
			BaseTarget:      "base",
			CandidateTarget: "candidate",
			Summary: lint.ReportComparisonSummary{
				Overall: lint.ComparisonRegressed,
			},
			AddedFindings: []gate.LintFindingRef{
				{RuleID: lint.RuleMissingTitle.ID, Severity: lint.SeverityError},
				{RuleID: lint.RuleMixedEcosystemGuidance.ID, Category: lint.CategoryPortability, Severity: lint.SeverityWarning},
			},
		},
		EvalCurrent: &gate.EvalCurrentEvidence{
			Target: "candidate",
			Summary: domaineval.RoutingEvalSummary{
				Total:          4,
				Passed:         2,
				Failed:         2,
				FalsePositives: 1,
				PassRate:       0.5,
			},
		},
		EvalCompare: &gate.EvalCompareEvidence{
			BaseTarget:      "base",
			CandidateTarget: "candidate",
			Comparison: domaineval.RoutingEvalComparison{
				Summary: domaineval.RoutingEvalComparisonSummary{
					Overall: domaineval.ComparisonRegressed,
					MetricsDelta: domaineval.RoutingEvalMetricsDelta{
						FalsePositives: 1,
						PassRate:       -0.25,
					},
				},
				FlippedToFail: []domaineval.RoutingEvalCaseChange{
					{ID: "neg-1", CandidateFailureKind: domaineval.RoutingFalsePositive},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Decision != gate.DecisionFail {
		t.Fatalf("expected fail decision, got %#v", result)
	}
	if len(result.BlockingReasons) < 4 {
		t.Fatalf("expected multiple blocking reasons, got %#v", result.BlockingReasons)
	}
}

func TestEvaluateFailsOnBackendThresholds(t *testing.T) {
	t.Parallel()

	rate := 0.5
	result, err := gate.Evaluate(gate.Criteria{
		MinPerBackendPassRate:      floatPointer(1.0),
		MaxBackendDisagreementRate: floatPointer(0.25),
	}, gate.Evidence{
		MultiBackendCurrent: &gate.MultiBackendCurrentEvidence{
			Target: "candidate",
			Summary: domaineval.MultiBackendEvalSummary{
				BackendCount: 2,
				TotalCases:   4,
			},
			DisagreementRate: &rate,
			DifferingCaseIDs: []string{"case-a", "case-b"},
			Backends: []domaineval.BackendEvalReport{
				{
					Backend: domaineval.RoutingEvalBackendInfo{ID: "codex", Name: "Codex"},
					Summary: domaineval.RoutingEvalSummary{PassRate: 1.0},
				},
				{
					Backend: domaineval.RoutingEvalBackendInfo{ID: "cursor", Name: "Cursor"},
					Summary: domaineval.RoutingEvalSummary{PassRate: 0.5},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Decision != gate.DecisionFail {
		t.Fatalf("expected fail decision, got %#v", result)
	}
	if len(result.PerBackendResults) != 2 {
		t.Fatalf("expected per-backend results, got %#v", result.PerBackendResults)
	}
}

func intPointer(value int) *int {
	return &value
}

func floatPointer(value float64) *float64 {
	return &value
}
