package analysis_test

import (
	"testing"

	"github.com/firety/firety/internal/domain/analysis"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

func TestBuildImprovementPlanLintOnly(t *testing.T) {
	t.Parallel()

	plan := analysis.BuildImprovementPlan(analysis.ImprovementPlanEvidence{
		Findings: []lint.Finding{
			{RuleID: lint.RuleBrokenLocalLink.ID, Severity: lint.SeverityError},
			{RuleID: lint.RuleMissingTitle.ID, Severity: lint.SeverityError},
			{RuleID: lint.RuleLargeSkillMD.ID, Severity: lint.SeverityWarning},
		},
		RoutingRisk: lint.SummarizeRoutingRisk(nil),
	})

	if !plan.HasPriorities() {
		t.Fatalf("expected priorities, got %#v", plan)
	}
	if plan.Priorities[0].Key != "fix-structural-bundle-issues" {
		t.Fatalf("expected structural issues first, got %#v", plan.Priorities)
	}
}

func TestBuildImprovementPlanCorrelationInformed(t *testing.T) {
	t.Parallel()

	plan := analysis.BuildImprovementPlan(analysis.ImprovementPlanEvidence{
		Findings: []lint.Finding{
			{RuleID: lint.RuleGenericName.ID, Severity: lint.SeverityWarning},
			{RuleID: lint.RuleOverbroadWhenToUse.ID, Severity: lint.SeverityWarning},
			{RuleID: lint.RuleWeakNegativeGuidance.ID, Severity: lint.SeverityWarning},
		},
		RoutingRisk: lint.RoutingRiskSummary{
			RiskAreas: []lint.RoutingRiskArea{{Key: "trigger-guidance"}},
		},
		Correlation: analysis.LintEvalCorrelation{
			MissGroups: []analysis.LintEvalMissGroup{
				{
					Key:                   "false-positives",
					SupportingRuleIDs:     []string{lint.RuleOverbroadWhenToUse.ID, lint.RuleWeakNegativeGuidance.ID},
					SupportingEvalCaseIDs: []string{"negative-unrelated-task"},
				},
			},
		},
		EvalReport: &domaineval.RoutingEvalReport{
			Summary: domaineval.RoutingEvalSummary{Failed: 1},
			Results: []domaineval.RoutingEvalCaseResult{
				{ID: "negative-unrelated-task", FailureKind: domaineval.RoutingFalsePositive},
			},
		},
	})

	if !plan.HasPriorities() {
		t.Fatalf("expected priorities, got %#v", plan)
	}
	if plan.Priorities[0].Key != "tighten-trigger-boundaries" {
		t.Fatalf("expected trigger boundaries first, got %#v", plan.Priorities)
	}
}

func TestBuildImprovementPlanMultiBackendSupport(t *testing.T) {
	t.Parallel()

	plan := analysis.BuildImprovementPlan(analysis.ImprovementPlanEvidence{
		Findings: []lint.Finding{
			{RuleID: lint.RuleOverbroadWhenToUse.ID, Severity: lint.SeverityWarning},
		},
		MultiBackendEval: &domaineval.MultiBackendEvalReport{
			Summary: domaineval.MultiBackendEvalSummary{
				DifferingCaseCount: 1,
			},
			DifferingCases: []domaineval.MultiBackendDifferingCase{
				{
					ID:          "negative-unrelated-task",
					Expectation: domaineval.RoutingShouldNotTrigger,
					Tags:        []string{"false-positive-trap"},
					Outcomes: []domaineval.MultiBackendCaseOutcome{
						{BackendName: "Codex"},
						{BackendName: "Claude Code"},
					},
				},
			},
		},
	})

	if !plan.HasPriorities() {
		t.Fatalf("expected priorities, got %#v", plan)
	}
	found := false
	for _, item := range plan.Priorities {
		if item.Key == "tighten-trigger-boundaries" {
			found = true
			if len(item.SupportingBackendDifferences) == 0 {
				t.Fatalf("expected backend differences evidence, got %#v", item)
			}
		}
	}
	if !found {
		t.Fatalf("expected trigger boundaries priority, got %#v", plan.Priorities)
	}
}
