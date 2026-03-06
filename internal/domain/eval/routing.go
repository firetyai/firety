package eval

import (
	"slices"
	"sort"
)

const RoutingEvalSchemaVersion = "1"

type RoutingExpectation string
type RoutingFailureKind string

const (
	RoutingShouldTrigger    RoutingExpectation = "trigger"
	RoutingShouldNotTrigger RoutingExpectation = "do-not-trigger"

	RoutingFalsePositive RoutingFailureKind = "false-positive"
	RoutingFalseNegative RoutingFailureKind = "false-negative"
)

type RoutingEvalSuite struct {
	SchemaVersion string            `json:"schema_version"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Cases         []RoutingEvalCase `json:"cases"`
}

type RoutingEvalCase struct {
	ID          string             `json:"id"`
	Label       string             `json:"label,omitempty"`
	Prompt      string             `json:"prompt"`
	Expectation RoutingExpectation `json:"expectation"`
	Profile     string             `json:"profile,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Rationale   string             `json:"rationale,omitempty"`
}

type RoutingEvalRequest struct {
	SchemaVersion string   `json:"schema_version"`
	SkillPath     string   `json:"skill_path"`
	SkillMarkdown string   `json:"skill_markdown"`
	Prompt        string   `json:"prompt"`
	Profile       string   `json:"profile,omitempty"`
	CaseID        string   `json:"case_id"`
	Label         string   `json:"label,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

type RoutingEvalDecision struct {
	SchemaVersion string `json:"schema_version,omitempty"`
	Trigger       bool   `json:"trigger"`
	Reason        string `json:"reason,omitempty"`
}

type RoutingEvalCaseResult struct {
	ID            string             `json:"id"`
	Label         string             `json:"label,omitempty"`
	Prompt        string             `json:"prompt"`
	Profile       string             `json:"profile,omitempty"`
	Tags          []string           `json:"tags,omitempty"`
	Expectation   RoutingExpectation `json:"expectation"`
	ActualTrigger bool               `json:"actual_trigger"`
	Passed        bool               `json:"passed"`
	FailureKind   RoutingFailureKind `json:"failure_kind,omitempty"`
	Reason        string             `json:"reason,omitempty"`
}

type RoutingEvalBreakdown struct {
	Key            string `json:"key"`
	Total          int    `json:"total"`
	Passed         int    `json:"passed"`
	Failed         int    `json:"failed"`
	FalsePositives int    `json:"false_positives"`
	FalseNegatives int    `json:"false_negatives"`
}

type RoutingEvalSummary struct {
	Total          int                     `json:"total"`
	Passed         int                     `json:"passed"`
	Failed         int                     `json:"failed"`
	FalsePositives int                     `json:"false_positives"`
	FalseNegatives int                     `json:"false_negatives"`
	PassRate       float64                 `json:"pass_rate"`
	ByProfile      []RoutingEvalBreakdown  `json:"by_profile,omitempty"`
	ByTag          []RoutingEvalBreakdown  `json:"by_tag,omitempty"`
	NotableMisses  []RoutingEvalCaseResult `json:"notable_misses,omitempty"`
}

type RoutingEvalReport struct {
	Target  string                  `json:"target"`
	Suite   RoutingEvalSuiteInfo    `json:"suite"`
	Backend RoutingEvalBackendInfo  `json:"backend"`
	Profile string                  `json:"profile"`
	Summary RoutingEvalSummary      `json:"summary"`
	Results []RoutingEvalCaseResult `json:"results"`
}

type RoutingEvalSuiteInfo struct {
	Path          string `json:"path"`
	SchemaVersion string `json:"schema_version"`
	Name          string `json:"name"`
	CaseCount     int    `json:"case_count"`
}

type RoutingEvalBackendInfo struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	ProfileAffinity string `json:"profile_affinity,omitempty"`
}

func SummarizeRoutingEval(results []RoutingEvalCaseResult) RoutingEvalSummary {
	summary := RoutingEvalSummary{
		Total: len(results),
	}
	if len(results) == 0 {
		return summary
	}

	profileStats := make(map[string]*RoutingEvalBreakdown)
	tagStats := make(map[string]*RoutingEvalBreakdown)
	notableMisses := make([]RoutingEvalCaseResult, 0, 5)

	for _, result := range results {
		if result.Passed {
			summary.Passed++
		} else {
			summary.Failed++
			if result.FailureKind == RoutingFalsePositive {
				summary.FalsePositives++
			}
			if result.FailureKind == RoutingFalseNegative {
				summary.FalseNegatives++
			}
			if len(notableMisses) < 5 {
				notableMisses = append(notableMisses, result)
			}
		}

		if result.Profile != "" {
			updateBreakdown(profileStats, result.Profile, result)
		}
		for _, tag := range result.Tags {
			updateBreakdown(tagStats, tag, result)
		}
	}

	summary.PassRate = float64(summary.Passed) / float64(summary.Total)
	summary.ByProfile = sortedBreakdowns(profileStats)
	summary.ByTag = sortedBreakdowns(tagStats)
	summary.NotableMisses = notableMisses

	return summary
}

func updateBreakdown(index map[string]*RoutingEvalBreakdown, key string, result RoutingEvalCaseResult) {
	entry, ok := index[key]
	if !ok {
		entry = &RoutingEvalBreakdown{Key: key}
		index[key] = entry
	}

	entry.Total++
	if result.Passed {
		entry.Passed++
		return
	}

	entry.Failed++
	if result.FailureKind == RoutingFalsePositive {
		entry.FalsePositives++
	}
	if result.FailureKind == RoutingFalseNegative {
		entry.FalseNegatives++
	}
}

func sortedBreakdowns(index map[string]*RoutingEvalBreakdown) []RoutingEvalBreakdown {
	if len(index) == 0 {
		return nil
	}

	keys := make([]string, 0, len(index))
	for key := range index {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	result := make([]RoutingEvalBreakdown, 0, len(keys))
	for _, key := range keys {
		result = append(result, *index[key])
	}
	return result
}

func BuildCaseResult(testCase RoutingEvalCase, profile string, decision RoutingEvalDecision) RoutingEvalCaseResult {
	result := RoutingEvalCaseResult{
		ID:            testCase.ID,
		Label:         testCase.Label,
		Prompt:        testCase.Prompt,
		Profile:       profile,
		Tags:          append([]string(nil), testCase.Tags...),
		Expectation:   testCase.Expectation,
		ActualTrigger: decision.Trigger,
		Passed:        expectationMatches(testCase.Expectation, decision.Trigger),
		Reason:        decision.Reason,
	}
	if result.Passed {
		return result
	}

	if decision.Trigger {
		result.FailureKind = RoutingFalsePositive
	} else {
		result.FailureKind = RoutingFalseNegative
	}

	return result
}

func expectationMatches(expectation RoutingExpectation, actualTrigger bool) bool {
	if expectation == RoutingShouldNotTrigger {
		return !actualTrigger
	}
	return actualTrigger
}

func SortCaseResults(results []RoutingEvalCaseResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Passed != results[j].Passed {
			return !results[i].Passed
		}
		if results[i].FailureKind != results[j].FailureKind {
			return results[i].FailureKind < results[j].FailureKind
		}
		return results[i].ID < results[j].ID
	})
}
