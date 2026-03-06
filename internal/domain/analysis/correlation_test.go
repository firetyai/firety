package analysis_test

import (
	"testing"

	"github.com/firety/firety/internal/domain/analysis"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

func TestCorrelateLintAndEvalFalsePositives(t *testing.T) {
	t.Parallel()

	report := domaineval.RoutingEvalReport{
		Summary: domaineval.RoutingEvalSummary{
			Failed: 2,
		},
		Results: []domaineval.RoutingEvalCaseResult{
			{
				ID:          "negative-unrelated-task",
				Passed:      false,
				FailureKind: domaineval.RoutingFalsePositive,
			},
			{
				ID:          "ambiguous-help-request",
				Passed:      false,
				FailureKind: domaineval.RoutingFalsePositive,
			},
		},
	}
	findings := []lint.Finding{
		{RuleID: lint.RuleGenericName.ID},
		{RuleID: lint.RuleOverbroadWhenToUse.ID},
		{RuleID: lint.RuleWeakNegativeGuidance.ID},
	}

	correlation := analysis.CorrelateLintAndEval(findings, report)

	if !correlation.HasEvidence() {
		t.Fatalf("expected correlation evidence")
	}
	if len(correlation.MissGroups) != 1 {
		t.Fatalf("expected one miss group, got %#v", correlation.MissGroups)
	}

	group := correlation.MissGroups[0]
	if group.Key != "false-positives" {
		t.Fatalf("expected false-positives group, got %#v", group)
	}
	expectedRuleIDs := []string{
		lint.RuleGenericName.ID,
		lint.RuleOverbroadWhenToUse.ID,
		lint.RuleWeakNegativeGuidance.ID,
	}
	assertStringSliceEqual(t, group.SupportingRuleIDs, expectedRuleIDs)
	assertStringSliceEqual(t, group.SupportingEvalCaseIDs, []string{"ambiguous-help-request", "negative-unrelated-task"})
	if len(correlation.PriorityActions) == 0 {
		t.Fatalf("expected priority action, got %#v", correlation)
	}
}

func TestCorrelateLintAndEvalFalseNegativesAndProfileMisses(t *testing.T) {
	t.Parallel()

	report := domaineval.RoutingEvalReport{
		Summary: domaineval.RoutingEvalSummary{
			Failed: 2,
		},
		Results: []domaineval.RoutingEvalCaseResult{
			{
				ID:          "positive-validate-local-skill",
				Passed:      false,
				FailureKind: domaineval.RoutingFalseNegative,
			},
			{
				ID:          "profile-codex-positive",
				Passed:      false,
				Profile:     "codex",
				Tags:        []string{"positive", "profile-sensitive"},
				FailureKind: domaineval.RoutingFalseNegative,
			},
		},
	}
	findings := []lint.Finding{
		{RuleID: lint.RuleWeakTriggerPattern.ID},
		{RuleID: lint.RuleExamplesMissingInvocationPattern.ID},
		{RuleID: lint.RuleMixedEcosystemGuidance.ID},
		{RuleID: lint.RuleProfileIncompatibleGuidance.ID},
	}

	correlation := analysis.CorrelateLintAndEval(findings, report)

	if len(correlation.MissGroups) != 2 {
		t.Fatalf("expected two miss groups, got %#v", correlation.MissGroups)
	}
	if correlation.MissGroups[0].Key != "false-negatives" {
		t.Fatalf("expected false-negatives first, got %#v", correlation.MissGroups)
	}
	if correlation.MissGroups[1].Key != "profile-specific-misses" {
		t.Fatalf("expected profile-specific misses second, got %#v", correlation.MissGroups)
	}
	assertStringSliceEqual(t, correlation.MissGroups[1].SupportingEvalCaseIDs, []string{"profile-codex-positive"})
}

func TestCorrelateLintAndEvalLowNoiseWithoutSupportingFindings(t *testing.T) {
	t.Parallel()

	report := domaineval.RoutingEvalReport{
		Summary: domaineval.RoutingEvalSummary{
			Failed: 1,
		},
		Results: []domaineval.RoutingEvalCaseResult{
			{
				ID:          "positive-validate-local-skill",
				Passed:      false,
				FailureKind: domaineval.RoutingFalseNegative,
			},
		},
	}
	findings := []lint.Finding{
		{RuleID: lint.RuleBrokenLocalLink.ID},
	}

	correlation := analysis.CorrelateLintAndEval(findings, report)

	if correlation.HasEvidence() {
		t.Fatalf("expected no correlation evidence, got %#v", correlation)
	}
	if correlation.Summary == "" {
		t.Fatalf("expected low-noise summary, got %#v", correlation)
	}
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected slice length: got=%#v want=%#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected slice value at %d: got=%#v want=%#v", i, got, want)
		}
	}
}
