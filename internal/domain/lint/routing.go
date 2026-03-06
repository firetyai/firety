package lint

type RoutingRiskLevel string

const (
	RoutingRiskLow    RoutingRiskLevel = "low"
	RoutingRiskMedium RoutingRiskLevel = "medium"
	RoutingRiskHigh   RoutingRiskLevel = "high"
)

type RoutingRiskSummary struct {
	OverallRisk     RoutingRiskLevel  `json:"overall_routing_risk"`
	Summary         string            `json:"summary"`
	RiskAreas       []RoutingRiskArea `json:"risk_areas,omitempty"`
	PriorityActions []string          `json:"priority_actions,omitempty"`
}

type RoutingRiskArea struct {
	Key                 string   `json:"key"`
	Title               string   `json:"title"`
	Summary             string   `json:"summary"`
	ContributingRuleIDs []string `json:"contributing_rule_ids"`
	FindingCount        int      `json:"finding_count"`
}

type routingRiskAreaDefinition struct {
	Key      string
	Title    string
	Summary  string
	Priority string
	Weight   int
	RuleIDs  []string
}

var routingRiskAreaDefinitions = []routingRiskAreaDefinition{
	{
		Key:      "distinctiveness",
		Title:    "Generic naming and weak distinctiveness",
		Summary:  "The skill does not stand out clearly enough from generic assistant behavior or neighboring skills.",
		Priority: "Sharpen the name and description so the skill sounds specific, distinctive, and easy to route.",
		Weight:   2,
		RuleIDs: []string{
			RuleGenericName.ID,
			RuleLowDistinctiveness.ID,
			RuleVagueDescription.ID,
			RuleGenericTriggerDescription.ID,
		},
	},
	{
		Key:      "trigger-guidance",
		Title:    "Weak trigger guidance",
		Summary:  "The skill does not explain clearly enough when it should trigger or what input pattern should activate it.",
		Priority: "Add clearer when-to-use and invocation guidance that names the trigger situations and request shape.",
		Weight:   2,
		RuleIDs: []string{
			RuleMissingWhenToUse.ID,
			RuleOverbroadWhenToUse.ID,
			RuleMissingUsageGuidance.ID,
			RuleMissingNegativeGuidance.ID,
			RuleWeakNegativeGuidance.ID,
		},
	},
	{
		Key:      "scope",
		Title:    "Diffuse or overbroad scope",
		Summary:  "The skill appears to cover too many unrelated tasks or too broad a use space to trigger reliably.",
		Priority: "Narrow the documented scope so the skill handles a more coherent set of requests and handoffs.",
		Weight:   2,
		RuleIDs: []string{
			RuleDiffuseScope.ID,
			RuleScopeMismatch.ID,
			RuleDescriptionBodyMismatch.ID,
		},
	},
	{
		Key:      "example-alignment",
		Title:    "Examples do not reinforce routing clearly",
		Summary:  "The examples are missing, weak, or misaligned enough that they do not teach a stable trigger pattern.",
		Priority: "Replace weak or generic examples with concrete requests that reinforce the documented trigger and outcome.",
		Weight:   2,
		RuleIDs: []string{
			RuleMissingExamples.ID,
			RuleWeakExamples.ID,
			RuleGenericExamples.ID,
			RuleExamplesMissingInvocationPattern.ID,
			RuleWeakTriggerPattern.ID,
			RuleExampleTriggerMismatch.ID,
			RuleExampleGuidanceMismatch.ID,
			RuleLowVarietyExamples.ID,
		},
	},
	{
		Key:      "signal-consistency",
		Title:    "Inconsistent trigger signals",
		Summary:  "The name, title, description, body, and examples point at different routing concepts.",
		Priority: "Align the name, title, description, and examples so they all point to the same trigger concept.",
		Weight:   3,
		RuleIDs: []string{
			RuleNameTitleMismatch.ID,
			RuleTriggerScopeInconsistency.ID,
			RuleExampleTriggerMismatch.ID,
			RuleDescriptionBodyMismatch.ID,
			RuleScopeMismatch.ID,
		},
	},
	{
		Key:      "profile-fit",
		Title:    "Profile fit and portability confusion",
		Summary:  "Tool targeting or portability signals are confused enough that they may hurt routing or profile fit.",
		Priority: "Either keep the skill tool-neutral or state the intended tool target and boundaries consistently across the document.",
		Weight:   3,
		RuleIDs: []string{
			RuleToolSpecificBranding.ID,
			RuleProfileIncompatibleGuidance.ID,
			RuleToolSpecificInstallAssumption.ID,
			RuleNonportableInvocationGuidance.ID,
			RuleGenericProfileToolLocking.ID,
			RuleUnclearToolTargeting.ID,
			RuleAccidentalToolLockIn.ID,
			RuleGenericPortabilityContradiction.ID,
			RuleMixedEcosystemGuidance.ID,
			RuleMissingToolTargetBoundary.ID,
			RuleProfileTargetMismatch.ID,
			RuleExampleEcosystemMismatch.ID,
		},
	},
}

func SummarizeRoutingRisk(findings []Finding) RoutingRiskSummary {
	areas := make([]RoutingRiskArea, 0, len(routingRiskAreaDefinitions))
	priorityActions := make([]string, 0, 3)
	score := 0

	for _, definition := range routingRiskAreaDefinitions {
		ruleIDs := make([]string, 0, len(definition.RuleIDs))
		count := 0
		for _, candidateRuleID := range definition.RuleIDs {
			if containsFindingRule(findings, candidateRuleID) {
				ruleIDs = append(ruleIDs, candidateRuleID)
				count += countFindingRule(findings, candidateRuleID)
			}
		}
		if len(ruleIDs) == 0 {
			continue
		}

		score += definition.Weight + count - 1
		areas = append(areas, RoutingRiskArea{
			Key:                 definition.Key,
			Title:               definition.Title,
			Summary:             definition.Summary,
			ContributingRuleIDs: ruleIDs,
			FindingCount:        count,
		})
		if len(priorityActions) < 3 {
			priorityActions = append(priorityActions, definition.Priority)
		}
	}

	if len(areas) == 0 {
		return RoutingRiskSummary{
			OverallRisk: RoutingRiskLow,
			Summary:     "No major routing weaknesses were detected from the current lint findings.",
		}
	}

	level := RoutingRiskMedium
	switch {
	case score >= 8 || len(areas) >= 3:
		level = RoutingRiskHigh
	case score <= 3:
		level = RoutingRiskMedium
	}

	summary := "The skill has some routing weaknesses that may make it harder to trigger consistently."
	if level == RoutingRiskHigh {
		summary = "The skill has several routing weaknesses that may stop it from triggering clearly or at the right time."
	}

	return RoutingRiskSummary{
		OverallRisk:     level,
		Summary:         summary,
		RiskAreas:       areas,
		PriorityActions: priorityActions,
	}
}

func containsFindingRule(findings []Finding, ruleID string) bool {
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			return true
		}
	}
	return false
}

func countFindingRule(findings []Finding, ruleID string) int {
	count := 0
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			count++
		}
	}
	return count
}
