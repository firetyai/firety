package analysis

import (
	"fmt"
	"slices"

	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type ImprovementPlan struct {
	Summary    string                `json:"summary"`
	Priorities []ImprovementPlanItem `json:"priorities,omitempty"`
}

type ImprovementPlanItem struct {
	Key                          string   `json:"key"`
	Title                        string   `json:"title"`
	WhyItMatters                 string   `json:"why_it_matters"`
	WhatToImprove                string   `json:"what_to_improve"`
	ImpactAreas                  []string `json:"impact_areas"`
	SupportingRuleIDs            []string `json:"supporting_rule_ids,omitempty"`
	SupportingEvalCaseIDs        []string `json:"supporting_eval_case_ids,omitempty"`
	SupportingBackendDifferences []string `json:"supporting_backend_differences,omitempty"`
}

type ImprovementPlanEvidence struct {
	Findings         []lint.Finding
	RoutingRisk      lint.RoutingRiskSummary
	ActionAreas      []lint.ActionArea
	Correlation      LintEvalCorrelation
	EvalReport       *domaineval.RoutingEvalReport
	MultiBackendEval *domaineval.MultiBackendEvalReport
}

type planDefinition struct {
	Key             string
	Title           string
	Why             string
	What            string
	ImpactAreas     []string
	RuleIDs         []string
	CorrelationKeys []string
	RiskAreaKeys    []string
	BackendTags     []string
}

type planCandidate struct {
	item  ImprovementPlanItem
	score int
}

var planDefinitions = []planDefinition{
	{
		Key:         "tighten-trigger-boundaries",
		Title:       "Tighten trigger wording and boundaries",
		Why:         "Overbroad triggers and weak boundaries are likely contributors to false positives and confused routing.",
		What:        "Narrow the when-to-use language, add stronger limitations, and make it clearer which requests should not trigger the skill.",
		ImpactAreas: []string{"false-positives", "trigger-quality", "routing-risk"},
		RuleIDs: []string{
			lint.RuleOverbroadWhenToUse.ID,
			lint.RuleMissingNegativeGuidance.ID,
			lint.RuleWeakNegativeGuidance.ID,
			lint.RuleDiffuseScope.ID,
		},
		CorrelationKeys: []string{"false-positives"},
		RiskAreaKeys:    []string{"trigger-guidance", "scope"},
		BackendTags:     []string{"false-positive-trap", "ambiguous"},
	},
	{
		Key:         "improve-distinctiveness",
		Title:       "Improve distinctiveness and trigger clarity",
		Why:         "Weak or generic trigger signals make intended requests harder to recognize and are consistent with false negatives.",
		What:        "Sharpen the skill name, description, and trigger framing so the skill stands out from generic assistant behavior.",
		ImpactAreas: []string{"false-negatives", "trigger-quality", "routing-risk"},
		RuleIDs: []string{
			lint.RuleGenericName.ID,
			lint.RuleGenericTriggerDescription.ID,
			lint.RuleLowDistinctiveness.ID,
			lint.RuleWeakTriggerPattern.ID,
			lint.RuleTriggerScopeInconsistency.ID,
		},
		CorrelationKeys: []string{"false-negatives"},
		RiskAreaKeys:    []string{"distinctiveness", "signal-consistency"},
	},
	{
		Key:         "strengthen-examples",
		Title:       "Strengthen examples to reinforce the intended routing",
		Why:         "Weak or misaligned examples reduce trust in the trigger pattern and can contribute to both false negatives and backend disagreement.",
		What:        "Replace generic examples with concrete requests, clear invocation patterns, and expected outcomes that match the documented scope.",
		ImpactAreas: []string{"examples", "false-negatives", "backend-consistency"},
		RuleIDs: []string{
			lint.RuleMissingExamples.ID,
			lint.RuleWeakExamples.ID,
			lint.RuleGenericExamples.ID,
			lint.RuleExamplesMissingInvocationPattern.ID,
			lint.RuleExampleTriggerMismatch.ID,
			lint.RuleExampleGuidanceMismatch.ID,
			lint.RuleLowVarietyExamples.ID,
		},
		CorrelationKeys: []string{"false-negatives"},
		RiskAreaKeys:    []string{"example-alignment"},
	},
	{
		Key:         "clarify-portability-targeting",
		Title:       "Clarify portability posture and tool targeting",
		Why:         "Mixed ecosystem guidance and accidental tool lock-in can hurt portability and create profile-specific routing misses.",
		What:        "Either keep the skill tool-neutral or state the intended tool target and audience boundaries consistently across the document and examples.",
		ImpactAreas: []string{"portability", "profile-fit", "backend-consistency"},
		RuleIDs: []string{
			lint.RuleProfileTargetMismatch.ID,
			lint.RuleProfileIncompatibleGuidance.ID,
			lint.RuleMixedEcosystemGuidance.ID,
			lint.RuleExampleEcosystemMismatch.ID,
			lint.RuleAccidentalToolLockIn.ID,
			lint.RuleGenericPortabilityContradiction.ID,
		},
		CorrelationKeys: []string{"profile-specific-misses"},
		RiskAreaKeys:    []string{"profile-fit"},
		BackendTags:     []string{"profile-sensitive"},
	},
	{
		Key:         "fix-structural-bundle-issues",
		Title:       "Fix structural and bundle issues first",
		Why:         "Broken files, missing links, and stale resources undermine the rest of the quality signal and create avoidable authoring risk.",
		What:        "Repair entry-document issues, broken local references, and stale or misleading bundle resources before refining higher-level guidance.",
		ImpactAreas: []string{"structure", "bundle", "maintainability"},
		RuleIDs: []string{
			lint.RuleMissingTitle.ID,
			lint.RuleBrokenLocalLink.ID,
			lint.RuleMissingMentionedResource.ID,
			lint.RuleReferenceOutsideRoot.ID,
			lint.RuleReferencedDirectoryInsteadOfFile.ID,
			lint.RulePossiblyStaleResource.ID,
		},
		RiskAreaKeys: []string{},
	},
	{
		Key:         "trim-costly-content",
		Title:       "Trim bloated instructions and examples",
		Why:         "Oversized or repetitive skill content increases context cost and makes the skill harder to maintain consistently.",
		What:        "Cut repeated instructions, oversized examples, and large referenced text resources so the core guidance stays focused.",
		ImpactAreas: []string{"cost", "maintainability"},
		RuleIDs: []string{
			lint.RuleLargeSkillMD.ID,
			lint.RuleExcessiveExampleVolume.ID,
			lint.RuleDuplicateExamples.ID,
			lint.RuleLargeReferencedResource.ID,
			lint.RuleExcessiveBundleSize.ID,
			lint.RuleRepetitiveInstructions.ID,
			lint.RuleUnbalancedSkillContent.ID,
		},
	},
}

func BuildImprovementPlan(evidence ImprovementPlanEvidence) ImprovementPlan {
	if len(evidence.Findings) == 0 && evidence.EvalReport == nil && evidence.MultiBackendEval == nil {
		return ImprovementPlan{
			Summary: "Firety did not find strong improvement priorities from the current evidence.",
		}
	}

	candidates := make([]planCandidate, 0, len(planDefinitions))
	for _, definition := range planDefinitions {
		candidate := buildPlanCandidate(definition, evidence)
		if candidate.score == 0 {
			continue
		}
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return ImprovementPlan{
			Summary: "Firety did not find strong improvement priorities from the current evidence.",
		}
	}

	slices.SortStableFunc(candidates, func(left, right planCandidate) int {
		if left.score != right.score {
			return right.score - left.score
		}
		return compareStrings(left.item.Key, right.item.Key)
	})

	priorities := make([]ImprovementPlanItem, 0, minInt(5, len(candidates)))
	for _, candidate := range candidates {
		priorities = append(priorities, candidate.item)
		if len(priorities) == 5 {
			break
		}
	}

	summary := "Firety found a small set of high-signal improvements that are likely to strengthen skill quality."
	if evidence.EvalReport != nil && evidence.EvalReport.Summary.Failed > 0 {
		summary = "Firety found a small set of high-signal improvements that are likely to strengthen measured routing behavior and overall skill quality."
	}
	if evidence.MultiBackendEval != nil && evidence.MultiBackendEval.Summary.DifferingCaseCount > 0 {
		summary = "Firety found a small set of high-signal improvements that are likely to strengthen measured routing quality and reduce backend inconsistency."
	}

	return ImprovementPlan{
		Summary:    summary,
		Priorities: priorities,
	}
}

func buildPlanCandidate(definition planDefinition, evidence ImprovementPlanEvidence) planCandidate {
	ruleIDs := matchingRuleIDs(evidence.Findings, definition.RuleIDs)
	evalCaseIDs := make([]string, 0)
	backendDiffs := make([]string, 0)
	score := 0

	for _, ruleID := range ruleIDs {
		score++
		rule, ok := lint.FindRule(ruleID)
		if ok && rule.Severity == lint.SeverityError {
			score += 2
		}
	}

	for _, key := range definition.CorrelationKeys {
		group, ok := findCorrelationGroup(evidence.Correlation, key)
		if !ok {
			continue
		}
		score += 3 + len(group.SupportingEvalCaseIDs)
		evalCaseIDs = appendUniqueStrings(evalCaseIDs, group.SupportingEvalCaseIDs...)
		ruleIDs = appendUniqueStrings(ruleIDs, group.SupportingRuleIDs...)
	}

	for _, key := range definition.RiskAreaKeys {
		if matchesRoutingRiskArea(evidence.RoutingRisk, key) {
			score += 2
		}
	}

	if evidence.EvalReport != nil {
		for _, result := range evidence.EvalReport.Results {
			if result.Passed {
				continue
			}
			if matchesPlanEvalCase(definition, result) {
				score++
				evalCaseIDs = appendUniqueStrings(evalCaseIDs, result.ID)
			}
		}
	}

	if evidence.MultiBackendEval != nil {
		for _, differingCase := range evidence.MultiBackendEval.DifferingCases {
			if matchesPlanDifferingCase(definition, differingCase) {
				score += 2
				evalCaseIDs = appendUniqueStrings(evalCaseIDs, differingCase.ID)
				backendDiffs = appendUniqueStrings(backendDiffs, differingBackendNames(differingCase)...)
			}
		}
	}

	if score == 0 {
		return planCandidate{}
	}

	return planCandidate{
		score: score,
		item: ImprovementPlanItem{
			Key:                          definition.Key,
			Title:                        definition.Title,
			WhyItMatters:                 definition.Why,
			WhatToImprove:                definition.What,
			ImpactAreas:                  append([]string(nil), definition.ImpactAreas...),
			SupportingRuleIDs:            ruleIDs,
			SupportingEvalCaseIDs:        evalCaseIDs,
			SupportingBackendDifferences: backendDiffs,
		},
	}
}

func matchingRuleIDs(findings []lint.Finding, allowed []string) []string {
	matches := make([]string, 0, len(allowed))
	for _, ruleID := range allowed {
		for _, finding := range findings {
			if finding.RuleID == ruleID {
				matches = append(matches, ruleID)
				break
			}
		}
	}
	return matches
}

func findCorrelationGroup(correlation LintEvalCorrelation, key string) (LintEvalMissGroup, bool) {
	for _, group := range correlation.MissGroups {
		if group.Key == key {
			return group, true
		}
	}
	return LintEvalMissGroup{}, false
}

func matchesRoutingRiskArea(summary lint.RoutingRiskSummary, key string) bool {
	for _, area := range summary.RiskAreas {
		if area.Key == key {
			return true
		}
	}
	return false
}

func matchesPlanEvalCase(definition planDefinition, result domaineval.RoutingEvalCaseResult) bool {
	switch definition.Key {
	case "tighten-trigger-boundaries":
		return result.FailureKind == domaineval.RoutingFalsePositive
	case "improve-distinctiveness", "strengthen-examples":
		return result.FailureKind == domaineval.RoutingFalseNegative
	case "clarify-portability-targeting":
		return result.Profile != "" && result.Profile != "generic"
	default:
		return false
	}
}

func matchesPlanDifferingCase(definition planDefinition, differingCase domaineval.MultiBackendDifferingCase) bool {
	if len(definition.BackendTags) == 0 {
		return false
	}
	for _, tag := range differingCase.Tags {
		if slices.Contains(definition.BackendTags, tag) {
			return true
		}
	}

	if definition.Key == "tighten-trigger-boundaries" && differingCase.Expectation == domaineval.RoutingShouldNotTrigger {
		return true
	}
	if (definition.Key == "improve-distinctiveness" || definition.Key == "strengthen-examples") && differingCase.Expectation == domaineval.RoutingShouldTrigger {
		return true
	}

	return false
}

func differingBackendNames(differingCase domaineval.MultiBackendDifferingCase) []string {
	names := make([]string, 0, len(differingCase.Outcomes))
	for _, outcome := range differingCase.Outcomes {
		names = appendUniqueStrings(names, outcome.BackendName)
	}
	return names
}

func appendUniqueStrings(values []string, items ...string) []string {
	for _, item := range items {
		if item == "" || slices.Contains(values, item) {
			continue
		}
		values = append(values, item)
	}
	return values
}

func compareStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func (p ImprovementPlan) HasPriorities() bool {
	return len(p.Priorities) > 0
}

func (p ImprovementPlanItem) EvidenceSummary() string {
	parts := make([]string, 0, 3)
	if len(p.SupportingRuleIDs) > 0 {
		parts = append(parts, fmt.Sprintf("%d supporting rule(s)", len(p.SupportingRuleIDs)))
	}
	if len(p.SupportingEvalCaseIDs) > 0 {
		parts = append(parts, fmt.Sprintf("%d supporting eval case(s)", len(p.SupportingEvalCaseIDs)))
	}
	if len(p.SupportingBackendDifferences) > 0 {
		parts = append(parts, fmt.Sprintf("%d backend difference(s)", len(p.SupportingBackendDifferences)))
	}
	return joinPlanParts(parts)
}

func joinPlanParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + ", " + parts[1] + func() string {
		if len(parts) == 2 {
			return ""
		}
		return ", " + parts[2]
	}()
}
