package workspace

import (
	"fmt"
	"slices"
	"strings"

	"github.com/firety/firety/internal/domain/compatibility"
	"github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/readiness"
)

type DiscoveryWarning struct {
	Path    string `json:"path,omitempty"`
	Summary string `json:"summary"`
}

type SkillRef struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Discovery struct {
	WorkspaceRoot string             `json:"workspace_root"`
	SourcePath    string             `json:"source_path"`
	Skills        []SkillRef         `json:"skills"`
	Warnings      []DiscoveryWarning `json:"warnings,omitempty"`
}

type SkillLintSummary struct {
	Valid        bool   `json:"valid"`
	ErrorCount   int    `json:"error_count"`
	WarningCount int    `json:"warning_count"`
	Summary      string `json:"summary"`
}

type SkillReadinessSummary struct {
	Decision           readiness.Decision           `json:"decision"`
	Summary            string                       `json:"summary"`
	SupportPosture     compatibility.SupportPosture `json:"support_posture,omitempty"`
	FreshnessStatus    readiness.FreshnessStatus    `json:"freshness_status,omitempty"`
	TopBlockers        []readiness.Reason           `json:"top_blockers,omitempty"`
	TopCaveats         []readiness.Reason           `json:"top_caveats,omitempty"`
	RecommendedActions []string                     `json:"recommended_actions,omitempty"`
}

type SkillResult struct {
	Skill     SkillRef               `json:"skill"`
	Lint      SkillLintSummary       `json:"lint"`
	Readiness *SkillReadinessSummary `json:"readiness,omitempty"`
}

type Summary struct {
	SkillCount                 int            `json:"skill_count"`
	CleanSkills                int            `json:"clean_skills"`
	SkillsWithWarnings         int            `json:"skills_with_warnings"`
	SkillsWithLintErrors       int            `json:"skills_with_lint_errors"`
	ReadySkills                int            `json:"ready_skills,omitempty"`
	ReadyWithCaveatsSkills     int            `json:"ready_with_caveats_skills,omitempty"`
	NotReadySkills             int            `json:"not_ready_skills,omitempty"`
	InsufficientEvidenceSkills int            `json:"insufficient_evidence_skills,omitempty"`
	TotalLintErrors            int            `json:"total_lint_errors"`
	TotalLintWarnings          int            `json:"total_lint_warnings"`
	SupportPostureCounts       map[string]int `json:"support_posture_counts,omitempty"`
	WorkspaceBlockers          []string       `json:"workspace_blockers,omitempty"`
	WorkspaceCaveats           []string       `json:"workspace_caveats,omitempty"`
	TopPriorities              []string       `json:"top_priorities,omitempty"`
}

type GateCriteria struct {
	MaxNotReadySkills             int `json:"max_not_ready_skills"`
	MaxInsufficientEvidenceSkills int `json:"max_insufficient_evidence_skills"`
	MaxSkillsWithLintErrors       int `json:"max_skills_with_lint_errors"`
	MaxDiscoveryWarnings          int `json:"max_discovery_warnings"`
}

type GateResult struct {
	Decision        gate.Decision `json:"decision"`
	Summary         string        `json:"summary"`
	BlockingReasons []string      `json:"blocking_reasons,omitempty"`
	Metrics         GateMetrics   `json:"metrics"`
}

type GateMetrics struct {
	NotReadySkills             int `json:"not_ready_skills"`
	InsufficientEvidenceSkills int `json:"insufficient_evidence_skills"`
	SkillsWithLintErrors       int `json:"skills_with_lint_errors"`
	DiscoveryWarnings          int `json:"discovery_warnings"`
}

type Report struct {
	WorkspaceRoot string        `json:"workspace_root"`
	Discovery     Discovery     `json:"discovery"`
	Summary       Summary       `json:"summary"`
	Skills        []SkillResult `json:"skills"`
	Gate          *GateResult   `json:"gate,omitempty"`
}

func BuildSummary(skills []SkillResult, warnings []DiscoveryWarning) Summary {
	summary := Summary{
		SkillCount:           len(skills),
		SupportPostureCounts: make(map[string]int),
	}
	blockers := make([]string, 0, len(warnings)+len(skills))
	caveats := make([]string, 0, len(skills))
	priorities := make([]string, 0, len(skills))

	for _, warning := range warnings {
		blockers = append(blockers, warning.Summary)
		priorities = append(priorities, fmt.Sprintf("Resolve discovery warning: %s", warning.Summary))
	}

	for _, skill := range skills {
		summary.TotalLintErrors += skill.Lint.ErrorCount
		summary.TotalLintWarnings += skill.Lint.WarningCount
		switch {
		case skill.Lint.ErrorCount > 0:
			summary.SkillsWithLintErrors++
		case skill.Lint.WarningCount > 0:
			summary.SkillsWithWarnings++
		default:
			summary.CleanSkills++
		}

		if skill.Readiness == nil {
			continue
		}

		switch skill.Readiness.Decision {
		case readiness.DecisionReady:
			summary.ReadySkills++
		case readiness.DecisionReadyWithCaveats:
			summary.ReadyWithCaveatsSkills++
		case readiness.DecisionNotReady:
			summary.NotReadySkills++
		case readiness.DecisionInsufficient:
			summary.InsufficientEvidenceSkills++
		}

		if skill.Readiness.SupportPosture != "" {
			summary.SupportPostureCounts[string(skill.Readiness.SupportPosture)]++
		}

		if len(skill.Readiness.TopBlockers) > 0 {
			blockers = append(blockers, fmt.Sprintf("%s: %s", skill.Skill.Name, skill.Readiness.TopBlockers[0].Summary))
			priorities = append(priorities, fmt.Sprintf("Fix %s first: %s", skill.Skill.Name, skill.Readiness.TopBlockers[0].Summary))
		} else if skill.Lint.ErrorCount > 0 {
			priorities = append(priorities, fmt.Sprintf("Clear lint errors in %s.", skill.Skill.Name))
		}

		if len(skill.Readiness.TopCaveats) > 0 {
			caveats = append(caveats, fmt.Sprintf("%s: %s", skill.Skill.Name, skill.Readiness.TopCaveats[0].Summary))
		}

		if len(skill.Readiness.RecommendedActions) > 0 {
			priorities = append(priorities, fmt.Sprintf("%s: %s", skill.Skill.Name, skill.Readiness.RecommendedActions[0]))
		}
	}

	if len(summary.SupportPostureCounts) == 0 {
		summary.SupportPostureCounts = nil
	}

	summary.WorkspaceBlockers = uniqueSortedStrings(blockers)
	summary.WorkspaceCaveats = uniqueSortedStrings(caveats)
	summary.TopPriorities = firstN(uniqueSortedStrings(priorities), 5)
	return summary
}

func EvaluateGate(report Report, criteria GateCriteria) GateResult {
	reasons := make([]string, 0, 4)
	metrics := GateMetrics{
		NotReadySkills:             report.Summary.NotReadySkills,
		InsufficientEvidenceSkills: report.Summary.InsufficientEvidenceSkills,
		SkillsWithLintErrors:       report.Summary.SkillsWithLintErrors,
		DiscoveryWarnings:          len(report.Discovery.Warnings),
	}

	if metrics.NotReadySkills > criteria.MaxNotReadySkills {
		reasons = append(reasons, fmt.Sprintf("workspace has %d not-ready skill(s), above the allowed maximum of %d", metrics.NotReadySkills, criteria.MaxNotReadySkills))
	}
	if metrics.InsufficientEvidenceSkills > criteria.MaxInsufficientEvidenceSkills {
		reasons = append(reasons, fmt.Sprintf("workspace has %d skill(s) with insufficient evidence, above the allowed maximum of %d", metrics.InsufficientEvidenceSkills, criteria.MaxInsufficientEvidenceSkills))
	}
	if metrics.SkillsWithLintErrors > criteria.MaxSkillsWithLintErrors {
		reasons = append(reasons, fmt.Sprintf("workspace has %d skill(s) with lint errors, above the allowed maximum of %d", metrics.SkillsWithLintErrors, criteria.MaxSkillsWithLintErrors))
	}
	if metrics.DiscoveryWarnings > criteria.MaxDiscoveryWarnings {
		reasons = append(reasons, fmt.Sprintf("workspace has %d discovery warning(s), above the allowed maximum of %d", metrics.DiscoveryWarnings, criteria.MaxDiscoveryWarnings))
	}

	result := GateResult{
		Decision:        gate.DecisionPass,
		BlockingReasons: uniqueSortedStrings(reasons),
		Metrics:         metrics,
	}
	if len(result.BlockingReasons) > 0 {
		result.Decision = gate.DecisionFail
		result.Summary = "The workspace quality gate failed because one or more aggregate thresholds were exceeded."
		return result
	}
	result.Summary = "The workspace quality gate passed for the selected aggregate thresholds."
	return result
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	slices.Sort(out)
	return out
}

func firstN(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
