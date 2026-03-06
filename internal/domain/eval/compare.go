package eval

import (
	"fmt"
	"slices"
	"sort"
)

type ComparisonOutcome string

const (
	ComparisonImproved  ComparisonOutcome = "improved"
	ComparisonRegressed ComparisonOutcome = "regressed"
	ComparisonMixed     ComparisonOutcome = "mixed"
	ComparisonUnchanged ComparisonOutcome = "unchanged"
)

type RoutingEvalSideSummary struct {
	Target  string             `json:"target"`
	Summary RoutingEvalSummary `json:"summary"`
}

type RoutingEvalMetricsDelta struct {
	Passed         int     `json:"passed"`
	Failed         int     `json:"failed"`
	FalsePositives int     `json:"false_positives"`
	FalseNegatives int     `json:"false_negatives"`
	PassRate       float64 `json:"pass_rate"`
}

type RoutingEvalCaseChange struct {
	ID                     string             `json:"id"`
	Label                  string             `json:"label,omitempty"`
	Prompt                 string             `json:"prompt"`
	Profile                string             `json:"profile,omitempty"`
	Tags                   []string           `json:"tags,omitempty"`
	Expectation            RoutingExpectation `json:"expectation"`
	BasePassed             bool               `json:"base_passed"`
	CandidatePassed        bool               `json:"candidate_passed"`
	BaseActualTrigger      bool               `json:"base_actual_trigger"`
	CandidateActualTrigger bool               `json:"candidate_actual_trigger"`
	BaseFailureKind        RoutingFailureKind `json:"base_failure_kind,omitempty"`
	CandidateFailureKind   RoutingFailureKind `json:"candidate_failure_kind,omitempty"`
	BaseReason             string             `json:"base_reason,omitempty"`
	CandidateReason        string             `json:"candidate_reason,omitempty"`
}

type RoutingEvalBreakdownDelta struct {
	Key                     string `json:"key"`
	BaseTotal               int    `json:"base_total"`
	CandidateTotal          int    `json:"candidate_total"`
	BasePassed              int    `json:"base_passed"`
	CandidatePassed         int    `json:"candidate_passed"`
	BaseFailed              int    `json:"base_failed"`
	CandidateFailed         int    `json:"candidate_failed"`
	BaseFalsePositives      int    `json:"base_false_positives"`
	CandidateFalsePositives int    `json:"candidate_false_positives"`
	BaseFalseNegatives      int    `json:"base_false_negatives"`
	CandidateFalseNegatives int    `json:"candidate_false_negatives"`
	PassedDelta             int    `json:"passed_delta"`
	FailedDelta             int    `json:"failed_delta"`
	FalsePositiveDelta      int    `json:"false_positive_delta"`
	FalseNegativeDelta      int    `json:"false_negative_delta"`
}

type RoutingEvalComparisonSummary struct {
	Overall                 ComparisonOutcome       `json:"overall"`
	Summary                 string                  `json:"summary"`
	FlippedToFailCount      int                     `json:"flipped_to_fail_count"`
	FlippedToPassCount      int                     `json:"flipped_to_pass_count"`
	ChangedFailureCount     int                     `json:"changed_failure_count"`
	MetricsDelta            RoutingEvalMetricsDelta `json:"metrics_delta"`
	HighPriorityRegressions []string                `json:"high_priority_regressions,omitempty"`
	NotableImprovements     []string                `json:"notable_improvements,omitempty"`
	RegressionAreas         []string                `json:"regression_areas,omitempty"`
	ImprovementAreas        []string                `json:"improvement_areas,omitempty"`
}

type RoutingEvalComparison struct {
	Base            RoutingEvalSideSummary       `json:"base"`
	Candidate       RoutingEvalSideSummary       `json:"candidate"`
	Suite           RoutingEvalSuiteInfo         `json:"suite"`
	Backend         RoutingEvalBackendInfo       `json:"backend"`
	Summary         RoutingEvalComparisonSummary `json:"summary"`
	FlippedToFail   []RoutingEvalCaseChange      `json:"flipped_to_fail,omitempty"`
	FlippedToPass   []RoutingEvalCaseChange      `json:"flipped_to_pass,omitempty"`
	ChangedCases    []RoutingEvalCaseChange      `json:"changed_cases,omitempty"`
	ByProfileDeltas []RoutingEvalBreakdownDelta  `json:"by_profile_deltas,omitempty"`
	ByTagDeltas     []RoutingEvalBreakdownDelta  `json:"by_tag_deltas,omitempty"`
}

func CompareReports(base, candidate RoutingEvalReport) (RoutingEvalComparison, error) {
	if base.Suite.SchemaVersion != candidate.Suite.SchemaVersion || base.Suite.Name != candidate.Suite.Name || base.Suite.CaseCount != candidate.Suite.CaseCount {
		return RoutingEvalComparison{}, fmt.Errorf("routing eval reports do not share the same suite identity")
	}

	baseIndex := make(map[string]RoutingEvalCaseResult, len(base.Results))
	for _, result := range base.Results {
		baseIndex[result.ID] = result
	}
	candidateIndex := make(map[string]RoutingEvalCaseResult, len(candidate.Results))
	for _, result := range candidate.Results {
		candidateIndex[result.ID] = result
	}

	if len(baseIndex) != len(candidateIndex) {
		return RoutingEvalComparison{}, fmt.Errorf("routing eval reports do not contain the same case ids")
	}

	caseIDs := make([]string, 0, len(baseIndex))
	for caseID := range baseIndex {
		if _, ok := candidateIndex[caseID]; !ok {
			return RoutingEvalComparison{}, fmt.Errorf("routing eval case %q is missing from candidate report", caseID)
		}
		caseIDs = append(caseIDs, caseID)
	}
	slices.Sort(caseIDs)

	flippedToFail := make([]RoutingEvalCaseChange, 0)
	flippedToPass := make([]RoutingEvalCaseChange, 0)
	changedCases := make([]RoutingEvalCaseChange, 0)

	for _, caseID := range caseIDs {
		baseResult := baseIndex[caseID]
		candidateResult := candidateIndex[caseID]
		change := RoutingEvalCaseChange{
			ID:                     caseID,
			Label:                  candidateResult.Label,
			Prompt:                 candidateResult.Prompt,
			Profile:                candidateResult.Profile,
			Tags:                   append([]string(nil), candidateResult.Tags...),
			Expectation:            candidateResult.Expectation,
			BasePassed:             baseResult.Passed,
			CandidatePassed:        candidateResult.Passed,
			BaseActualTrigger:      baseResult.ActualTrigger,
			CandidateActualTrigger: candidateResult.ActualTrigger,
			BaseFailureKind:        baseResult.FailureKind,
			CandidateFailureKind:   candidateResult.FailureKind,
			BaseReason:             baseResult.Reason,
			CandidateReason:        candidateResult.Reason,
		}

		switch {
		case baseResult.Passed && !candidateResult.Passed:
			flippedToFail = append(flippedToFail, change)
		case !baseResult.Passed && candidateResult.Passed:
			flippedToPass = append(flippedToPass, change)
		case !baseResult.Passed && !candidateResult.Passed && (baseResult.FailureKind != candidateResult.FailureKind || baseResult.ActualTrigger != candidateResult.ActualTrigger):
			changedCases = append(changedCases, change)
		}
	}

	byProfileDeltas := compareBreakdownSets(base.Summary.ByProfile, candidate.Summary.ByProfile)
	byTagDeltas := compareBreakdownSets(base.Summary.ByTag, candidate.Summary.ByTag)

	summary := RoutingEvalComparisonSummary{
		FlippedToFailCount:  len(flippedToFail),
		FlippedToPassCount:  len(flippedToPass),
		ChangedFailureCount: len(changedCases),
		MetricsDelta: RoutingEvalMetricsDelta{
			Passed:         candidate.Summary.Passed - base.Summary.Passed,
			Failed:         candidate.Summary.Failed - base.Summary.Failed,
			FalsePositives: candidate.Summary.FalsePositives - base.Summary.FalsePositives,
			FalseNegatives: candidate.Summary.FalseNegatives - base.Summary.FalseNegatives,
			PassRate:       candidate.Summary.PassRate - base.Summary.PassRate,
		},
	}

	hasRegression := len(flippedToFail) > 0 || summary.MetricsDelta.FalsePositives > 0 || summary.MetricsDelta.FalseNegatives > 0 || summary.MetricsDelta.PassRate < 0
	hasImprovement := len(flippedToPass) > 0 || summary.MetricsDelta.FalsePositives < 0 || summary.MetricsDelta.FalseNegatives < 0 || summary.MetricsDelta.PassRate > 0

	switch {
	case hasRegression && hasImprovement:
		summary.Overall = ComparisonMixed
		summary.Summary = "The candidate improves some measured routing cases but regresses others."
	case hasRegression:
		summary.Overall = ComparisonRegressed
		summary.Summary = "The candidate regresses measured routing performance."
	case hasImprovement:
		summary.Overall = ComparisonImproved
		summary.Summary = "The candidate improves measured routing performance without introducing new misses."
	default:
		summary.Overall = ComparisonUnchanged
		summary.Summary = "Firety did not detect measurable routing changes between the two versions."
	}

	summary.HighPriorityRegressions = summarizeEvalRegressions(flippedToFail, summary.MetricsDelta)
	summary.NotableImprovements = summarizeEvalImprovements(flippedToPass, summary.MetricsDelta)
	summary.RegressionAreas, summary.ImprovementAreas = summarizeEvalAreas(byProfileDeltas, byTagDeltas, summary.MetricsDelta)

	return RoutingEvalComparison{
		Base: RoutingEvalSideSummary{
			Target:  base.Target,
			Summary: base.Summary,
		},
		Candidate: RoutingEvalSideSummary{
			Target:  candidate.Target,
			Summary: candidate.Summary,
		},
		Suite:           base.Suite,
		Backend:         candidate.Backend,
		Summary:         summary,
		FlippedToFail:   flippedToFail,
		FlippedToPass:   flippedToPass,
		ChangedCases:    changedCases,
		ByProfileDeltas: byProfileDeltas,
		ByTagDeltas:     byTagDeltas,
	}, nil
}

func compareBreakdownSets(base, candidate []RoutingEvalBreakdown) []RoutingEvalBreakdownDelta {
	baseIndex := make(map[string]RoutingEvalBreakdown, len(base))
	for _, breakdown := range base {
		baseIndex[breakdown.Key] = breakdown
	}
	candidateIndex := make(map[string]RoutingEvalBreakdown, len(candidate))
	for _, breakdown := range candidate {
		candidateIndex[breakdown.Key] = breakdown
	}

	keys := make([]string, 0, len(baseIndex)+len(candidateIndex))
	seen := make(map[string]struct{}, len(baseIndex)+len(candidateIndex))
	for key := range baseIndex {
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	for key := range candidateIndex {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)

	deltas := make([]RoutingEvalBreakdownDelta, 0, len(keys))
	for _, key := range keys {
		baseValue := baseIndex[key]
		candidateValue := candidateIndex[key]
		delta := RoutingEvalBreakdownDelta{
			Key:                     key,
			BaseTotal:               baseValue.Total,
			CandidateTotal:          candidateValue.Total,
			BasePassed:              baseValue.Passed,
			CandidatePassed:         candidateValue.Passed,
			BaseFailed:              baseValue.Failed,
			CandidateFailed:         candidateValue.Failed,
			BaseFalsePositives:      baseValue.FalsePositives,
			CandidateFalsePositives: candidateValue.FalsePositives,
			BaseFalseNegatives:      baseValue.FalseNegatives,
			CandidateFalseNegatives: candidateValue.FalseNegatives,
			PassedDelta:             candidateValue.Passed - baseValue.Passed,
			FailedDelta:             candidateValue.Failed - baseValue.Failed,
			FalsePositiveDelta:      candidateValue.FalsePositives - baseValue.FalsePositives,
			FalseNegativeDelta:      candidateValue.FalseNegatives - baseValue.FalseNegatives,
		}
		if delta.PassedDelta == 0 && delta.FailedDelta == 0 && delta.FalsePositiveDelta == 0 && delta.FalseNegativeDelta == 0 {
			continue
		}
		deltas = append(deltas, delta)
	}

	sort.SliceStable(deltas, func(i, j int) bool {
		leftWeight := absInt(deltas[i].PassedDelta) + absInt(deltas[i].FailedDelta) + absInt(deltas[i].FalsePositiveDelta) + absInt(deltas[i].FalseNegativeDelta)
		rightWeight := absInt(deltas[j].PassedDelta) + absInt(deltas[j].FailedDelta) + absInt(deltas[j].FalsePositiveDelta) + absInt(deltas[j].FalseNegativeDelta)
		if leftWeight != rightWeight {
			return leftWeight > rightWeight
		}
		return deltas[i].Key < deltas[j].Key
	})

	return deltas
}

func summarizeEvalRegressions(flippedToFail []RoutingEvalCaseChange, metrics RoutingEvalMetricsDelta) []string {
	items := make([]string, 0, 3)
	if len(flippedToFail) > 0 {
		items = append(items, fmt.Sprintf("%d eval case%s flipped from pass to fail.", len(flippedToFail), pluralSuffix(len(flippedToFail))))
	}
	if metrics.FalsePositives > 0 {
		items = append(items, fmt.Sprintf("False positives increased by %d.", metrics.FalsePositives))
	}
	if metrics.FalseNegatives > 0 {
		items = append(items, fmt.Sprintf("False negatives increased by %d.", metrics.FalseNegatives))
	}
	if metrics.PassRate < 0 {
		items = append(items, fmt.Sprintf("Pass rate dropped by %.0f percentage points.", -metrics.PassRate*100))
	}
	if len(items) > 3 {
		return items[:3]
	}
	return items
}

func summarizeEvalImprovements(flippedToPass []RoutingEvalCaseChange, metrics RoutingEvalMetricsDelta) []string {
	items := make([]string, 0, 3)
	if len(flippedToPass) > 0 {
		items = append(items, fmt.Sprintf("%d eval case%s flipped from fail to pass.", len(flippedToPass), pluralSuffix(len(flippedToPass))))
	}
	if metrics.FalsePositives < 0 {
		items = append(items, fmt.Sprintf("False positives decreased by %d.", -metrics.FalsePositives))
	}
	if metrics.FalseNegatives < 0 {
		items = append(items, fmt.Sprintf("False negatives decreased by %d.", -metrics.FalseNegatives))
	}
	if metrics.PassRate > 0 {
		items = append(items, fmt.Sprintf("Pass rate improved by %.0f percentage points.", metrics.PassRate*100))
	}
	if len(items) > 3 {
		return items[:3]
	}
	return items
}

func summarizeEvalAreas(byProfile, byTag []RoutingEvalBreakdownDelta, metrics RoutingEvalMetricsDelta) ([]string, []string) {
	regressions := make([]string, 0, 3)
	improvements := make([]string, 0, 3)

	appendArea := func(values *[]string, prefix, key string, delta int) {
		*values = append(*values, fmt.Sprintf("%s %s (%+d passed)", prefix, key, delta))
	}

	for _, delta := range byProfile {
		switch {
		case delta.PassedDelta < 0:
			appendArea(&regressions, "Profile", delta.Key, delta.PassedDelta)
		case delta.PassedDelta > 0:
			appendArea(&improvements, "Profile", delta.Key, delta.PassedDelta)
		}
	}
	for _, delta := range byTag {
		switch {
		case delta.PassedDelta < 0:
			appendArea(&regressions, "Tag", delta.Key, delta.PassedDelta)
		case delta.PassedDelta > 0:
			appendArea(&improvements, "Tag", delta.Key, delta.PassedDelta)
		}
	}

	if metrics.FalsePositives > 0 {
		regressions = append(regressions, fmt.Sprintf("False positives (+%d)", metrics.FalsePositives))
	}
	if metrics.FalseNegatives > 0 {
		regressions = append(regressions, fmt.Sprintf("False negatives (+%d)", metrics.FalseNegatives))
	}
	if metrics.FalsePositives < 0 {
		improvements = append(improvements, fmt.Sprintf("False positives (%d)", metrics.FalsePositives))
	}
	if metrics.FalseNegatives < 0 {
		improvements = append(improvements, fmt.Sprintf("False negatives (%d)", metrics.FalseNegatives))
	}

	if len(regressions) > 3 {
		regressions = regressions[:3]
	}
	if len(improvements) > 3 {
		improvements = improvements[:3]
	}

	return regressions, improvements
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
