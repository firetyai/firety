package analysis

import (
	"slices"

	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type LintEvalCorrelation struct {
	Summary         string              `json:"correlation_summary,omitempty"`
	MissGroups      []LintEvalMissGroup `json:"miss_groups,omitempty"`
	PriorityActions []string            `json:"priority_actions,omitempty"`
}

type LintEvalMissGroup struct {
	Key                   string                `json:"key"`
	Title                 string                `json:"title"`
	Summary               string                `json:"summary"`
	LikelyContributors    []LintEvalContributor `json:"likely_contributors"`
	SupportingRuleIDs     []string              `json:"supporting_rule_ids"`
	SupportingEvalCaseIDs []string              `json:"supporting_eval_case_ids"`
	PriorityAction        string                `json:"priority_action"`
}

type LintEvalContributor struct {
	RuleID   string `json:"rule_id"`
	Category string `json:"category,omitempty"`
	Title    string `json:"title"`
}

type correlationGroupDefinition struct {
	Key            string
	Title          string
	Summary        string
	PriorityAction string
	RuleIDs        []string
	MatchesMiss    func(domaineval.RoutingEvalCaseResult) bool
}

var correlationGroupDefinitions = []correlationGroupDefinition{
	{
		Key:            "false-positives",
		Title:          "Likely contributors to false positives",
		Summary:        "The false positives are consistent with trigger language that is too broad, weak boundaries, or portability signals that make unrelated prompts look in scope.",
		PriorityAction: "Tighten the trigger language and negative guidance so unrelated prompts do not read as valid triggers.",
		RuleIDs: []string{
			lint.RuleGenericName.ID,
			lint.RuleGenericTriggerDescription.ID,
			lint.RuleDiffuseScope.ID,
			lint.RuleOverbroadWhenToUse.ID,
			lint.RuleMissingNegativeGuidance.ID,
			lint.RuleWeakNegativeGuidance.ID,
			lint.RuleUnclearToolTargeting.ID,
			lint.RuleAccidentalToolLockIn.ID,
			lint.RuleGenericPortabilityContradiction.ID,
			lint.RuleMixedEcosystemGuidance.ID,
		},
		MatchesMiss: func(result domaineval.RoutingEvalCaseResult) bool {
			return result.FailureKind == domaineval.RoutingFalsePositive
		},
	},
	{
		Key:            "false-negatives",
		Title:          "Likely contributors to false negatives",
		Summary:        "The false negatives are consistent with weak or inconsistent trigger signals that do not make the intended requests stand out clearly enough.",
		PriorityAction: "Sharpen the name, trigger guidance, and examples so the intended requests are easier to recognize.",
		RuleIDs: []string{
			lint.RuleWeakTriggerPattern.ID,
			lint.RuleLowDistinctiveness.ID,
			lint.RuleMissingWhenToUse.ID,
			lint.RuleMissingUsageGuidance.ID,
			lint.RuleWeakExamples.ID,
			lint.RuleGenericExamples.ID,
			lint.RuleExamplesMissingInvocationPattern.ID,
			lint.RuleExampleTriggerMismatch.ID,
			lint.RuleTriggerScopeInconsistency.ID,
			lint.RuleNameTitleMismatch.ID,
			lint.RuleGenericTriggerDescription.ID,
		},
		MatchesMiss: func(result domaineval.RoutingEvalCaseResult) bool {
			return result.FailureKind == domaineval.RoutingFalseNegative
		},
	},
	{
		Key:            "profile-specific-misses",
		Title:          "Likely contributors to profile-specific misses",
		Summary:        "The profile-sensitive misses are consistent with portability or targeting cues that do not line up cleanly with the selected profile.",
		PriorityAction: "Align the wording, examples, and boundary guidance with the selected profile or restate the skill as intentionally generic.",
		RuleIDs: []string{
			lint.RuleProfileTargetMismatch.ID,
			lint.RuleProfileIncompatibleGuidance.ID,
			lint.RuleMixedEcosystemGuidance.ID,
			lint.RuleExampleEcosystemMismatch.ID,
			lint.RuleMissingToolTargetBoundary.ID,
			lint.RuleToolSpecificBranding.ID,
			lint.RuleAccidentalToolLockIn.ID,
			lint.RuleGenericProfileToolLocking.ID,
		},
		MatchesMiss: func(result domaineval.RoutingEvalCaseResult) bool {
			if result.Passed {
				return false
			}
			if result.Profile != "" && result.Profile != "generic" {
				return true
			}
			return containsTag(result.Tags, "profile-sensitive")
		},
	},
}

func CorrelateLintAndEval(findings []lint.Finding, report domaineval.RoutingEvalReport) LintEvalCorrelation {
	if len(findings) == 0 || report.Summary.Failed == 0 {
		return LintEvalCorrelation{}
	}

	findingsByRuleID := indexFindingsByRuleID(findings)
	missGroups := make([]LintEvalMissGroup, 0, len(correlationGroupDefinitions))
	priorityActions := make([]string, 0, 3)

	for _, definition := range correlationGroupDefinitions {
		caseIDs := matchingEvalCaseIDs(report.Results, definition.MatchesMiss)
		if len(caseIDs) == 0 {
			continue
		}

		contributors := make([]LintEvalContributor, 0, len(definition.RuleIDs))
		supportingRuleIDs := make([]string, 0, len(definition.RuleIDs))
		for _, ruleID := range definition.RuleIDs {
			ruleFindings := findingsByRuleID[ruleID]
			if len(ruleFindings) == 0 {
				continue
			}

			rule, ok := lint.FindRule(ruleID)
			if !ok {
				continue
			}

			contributors = append(contributors, LintEvalContributor{
				RuleID:   rule.ID,
				Category: string(rule.Category),
				Title:    rule.Title,
			})
			supportingRuleIDs = append(supportingRuleIDs, rule.ID)
		}

		if len(contributors) == 0 {
			continue
		}

		missGroups = append(missGroups, LintEvalMissGroup{
			Key:                   definition.Key,
			Title:                 definition.Title,
			Summary:               definition.Summary,
			LikelyContributors:    contributors,
			SupportingRuleIDs:     supportingRuleIDs,
			SupportingEvalCaseIDs: caseIDs,
			PriorityAction:        definition.PriorityAction,
		})
		if len(priorityActions) < 3 && !slices.Contains(priorityActions, definition.PriorityAction) {
			priorityActions = append(priorityActions, definition.PriorityAction)
		}
	}

	if len(missGroups) == 0 {
		return LintEvalCorrelation{
			Summary: "Firety did not find strong lint signals that clearly line up with the current measured misses.",
		}
	}

	summary := "The measured misses line up with a small number of likely skill-quality problem areas."
	if len(missGroups) == 1 {
		summary = "The measured misses line up with one clear skill-quality problem area."
	}

	return LintEvalCorrelation{
		Summary:         summary,
		MissGroups:      missGroups,
		PriorityActions: priorityActions,
	}
}

func (c LintEvalCorrelation) HasEvidence() bool {
	return len(c.MissGroups) > 0
}

func indexFindingsByRuleID(findings []lint.Finding) map[string][]lint.Finding {
	index := make(map[string][]lint.Finding, len(findings))
	for _, finding := range findings {
		index[finding.RuleID] = append(index[finding.RuleID], finding)
	}
	return index
}

func matchingEvalCaseIDs(results []domaineval.RoutingEvalCaseResult, match func(domaineval.RoutingEvalCaseResult) bool) []string {
	caseIDs := make([]string, 0, len(results))
	for _, result := range results {
		if match(result) {
			caseIDs = append(caseIDs, result.ID)
		}
	}
	slices.Sort(caseIDs)
	return caseIDs
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
