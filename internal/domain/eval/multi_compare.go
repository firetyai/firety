package eval

import (
	"fmt"
	"slices"
)

type MultiBackendEvalComparisonSummary struct {
	Overall                   ComparisonOutcome `json:"overall"`
	Summary                   string            `json:"summary"`
	BackendCount              int               `json:"backend_count"`
	ImprovedBackendCount      int               `json:"improved_backend_count"`
	RegressedBackendCount     int               `json:"regressed_backend_count"`
	MixedBackendCount         int               `json:"mixed_backend_count"`
	UnchangedBackendCount     int               `json:"unchanged_backend_count"`
	WidenedDisagreementCount  int               `json:"widened_disagreement_count"`
	NarrowedDisagreementCount int               `json:"narrowed_disagreement_count"`
	BackendRegressionAreas    []string          `json:"backend_regression_areas,omitempty"`
	BackendImprovementAreas   []string          `json:"backend_improvement_areas,omitempty"`
	HighPriorityRegressions   []string          `json:"high_priority_regressions,omitempty"`
	NotableImprovements       []string          `json:"notable_improvements,omitempty"`
}

type MultiBackendEvalCaseBackendChange struct {
	BackendID              string             `json:"backend_id"`
	BackendName            string             `json:"backend_name"`
	BasePassed             bool               `json:"base_passed"`
	CandidatePassed        bool               `json:"candidate_passed"`
	BaseFailureKind        RoutingFailureKind `json:"base_failure_kind,omitempty"`
	CandidateFailureKind   RoutingFailureKind `json:"candidate_failure_kind,omitempty"`
	BaseActualTrigger      bool               `json:"base_actual_trigger"`
	CandidateActualTrigger bool               `json:"candidate_actual_trigger"`
}

type MultiBackendEvalCaseDelta struct {
	ID                    string                              `json:"id"`
	Label                 string                              `json:"label,omitempty"`
	Prompt                string                              `json:"prompt"`
	Profile               string                              `json:"profile,omitempty"`
	Tags                  []string                            `json:"tags,omitempty"`
	Expectation           RoutingExpectation                  `json:"expectation"`
	BaseDisagreement      bool                                `json:"base_disagreement"`
	CandidateDisagreement bool                                `json:"candidate_disagreement"`
	ChangedBackends       []MultiBackendEvalCaseBackendChange `json:"changed_backends"`
}

type BackendEvalComparison struct {
	Backend    RoutingEvalBackendInfo `json:"backend"`
	Base       RoutingEvalSideSummary `json:"base"`
	Candidate  RoutingEvalSideSummary `json:"candidate"`
	Comparison RoutingEvalComparison  `json:"comparison"`
}

type MultiBackendEvalComparison struct {
	Base                  RoutingEvalSideSummary            `json:"base"`
	Candidate             RoutingEvalSideSummary            `json:"candidate"`
	Suite                 RoutingEvalSuiteInfo              `json:"suite"`
	Backends              []RoutingEvalBackendInfo          `json:"backends"`
	AggregateSummary      MultiBackendEvalComparisonSummary `json:"aggregate_summary"`
	PerBackend            []BackendEvalComparison           `json:"per_backend_deltas"`
	DifferingCases        []MultiBackendEvalCaseDelta       `json:"differing_cases,omitempty"`
	WidenedDisagreements  []MultiBackendEvalCaseDelta       `json:"widened_disagreements,omitempty"`
	NarrowedDisagreements []MultiBackendEvalCaseDelta       `json:"narrowed_disagreements,omitempty"`
}

func CompareMultiBackendReports(base, candidate MultiBackendEvalReport) (MultiBackendEvalComparison, error) {
	if base.Suite.SchemaVersion != candidate.Suite.SchemaVersion || base.Suite.Name != candidate.Suite.Name || base.Suite.CaseCount != candidate.Suite.CaseCount {
		return MultiBackendEvalComparison{}, fmt.Errorf("multi-backend eval reports do not share the same suite identity")
	}
	if len(base.Backends) != len(candidate.Backends) {
		return MultiBackendEvalComparison{}, fmt.Errorf("multi-backend eval reports do not contain the same backend count")
	}

	baseIndex := make(map[string]BackendEvalReport, len(base.Backends))
	candidateIndex := make(map[string]BackendEvalReport, len(candidate.Backends))
	backendIDs := make([]string, 0, len(base.Backends))

	for _, report := range base.Backends {
		baseIndex[report.Backend.ID] = report
		backendIDs = append(backendIDs, report.Backend.ID)
	}
	for _, report := range candidate.Backends {
		candidateIndex[report.Backend.ID] = report
	}
	slices.Sort(backendIDs)

	perBackend := make([]BackendEvalComparison, 0, len(backendIDs))
	backends := make([]RoutingEvalBackendInfo, 0, len(backendIDs))
	for _, backendID := range backendIDs {
		baseBackend, ok := baseIndex[backendID]
		if !ok {
			return MultiBackendEvalComparison{}, fmt.Errorf("backend %q missing from base report", backendID)
		}
		candidateBackend, ok := candidateIndex[backendID]
		if !ok {
			return MultiBackendEvalComparison{}, fmt.Errorf("backend %q missing from candidate report", backendID)
		}

		comparison, err := CompareReports(
			RoutingEvalReport{
				Target:  base.Target,
				Suite:   base.Suite,
				Backend: baseBackend.Backend,
				Summary: baseBackend.Summary,
				Results: baseBackend.Results,
			},
			RoutingEvalReport{
				Target:  candidate.Target,
				Suite:   candidate.Suite,
				Backend: candidateBackend.Backend,
				Summary: candidateBackend.Summary,
				Results: candidateBackend.Results,
			},
		)
		if err != nil {
			return MultiBackendEvalComparison{}, fmt.Errorf("compare backend %q: %w", backendID, err)
		}

		perBackend = append(perBackend, BackendEvalComparison{
			Backend:    candidateBackend.Backend,
			Base:       comparison.Base,
			Candidate:  comparison.Candidate,
			Comparison: comparison,
		})
		backends = append(backends, candidateBackend.Backend)
	}

	caseDeltas, widened, narrowed, err := compareMultiBackendCaseDeltas(base, candidate, backendIDs)
	if err != nil {
		return MultiBackendEvalComparison{}, err
	}
	summary := summarizeMultiBackendComparison(perBackend, widened, narrowed)

	return MultiBackendEvalComparison{
		Base: RoutingEvalSideSummary{
			Target: base.Target,
		},
		Candidate: RoutingEvalSideSummary{
			Target: candidate.Target,
		},
		Suite:                 base.Suite,
		Backends:              backends,
		AggregateSummary:      summary,
		PerBackend:            perBackend,
		DifferingCases:        caseDeltas,
		WidenedDisagreements:  widened,
		NarrowedDisagreements: narrowed,
	}, nil
}

func compareMultiBackendCaseDeltas(base, candidate MultiBackendEvalReport, backendIDs []string) ([]MultiBackendEvalCaseDelta, []MultiBackendEvalCaseDelta, []MultiBackendEvalCaseDelta, error) {
	baseCases, err := indexMultiBackendCases(base, backendIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	candidateCases, err := indexMultiBackendCases(candidate, backendIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	caseIDs := make([]string, 0, len(baseCases))
	for caseID := range baseCases {
		if _, ok := candidateCases[caseID]; !ok {
			return nil, nil, nil, fmt.Errorf("case %q missing from candidate multi-backend report", caseID)
		}
		caseIDs = append(caseIDs, caseID)
	}
	slices.Sort(caseIDs)

	deltas := make([]MultiBackendEvalCaseDelta, 0)
	widened := make([]MultiBackendEvalCaseDelta, 0)
	narrowed := make([]MultiBackendEvalCaseDelta, 0)

	for _, caseID := range caseIDs {
		baseDelta := baseCases[caseID]
		candidateDelta := candidateCases[caseID]
		changedBackends := make([]MultiBackendEvalCaseBackendChange, 0)

		for _, backendID := range backendIDs {
			baseOutcome := baseDelta.outcomes[backendID]
			candidateOutcome := candidateDelta.outcomes[backendID]
			if baseOutcome.Passed == candidateOutcome.Passed &&
				baseOutcome.ActualTrigger == candidateOutcome.ActualTrigger &&
				baseOutcome.FailureKind == candidateOutcome.FailureKind {
				continue
			}
			changedBackends = append(changedBackends, MultiBackendEvalCaseBackendChange{
				BackendID:              backendID,
				BackendName:            candidateOutcome.BackendName,
				BasePassed:             baseOutcome.Passed,
				CandidatePassed:        candidateOutcome.Passed,
				BaseFailureKind:        baseOutcome.FailureKind,
				CandidateFailureKind:   candidateOutcome.FailureKind,
				BaseActualTrigger:      baseOutcome.ActualTrigger,
				CandidateActualTrigger: candidateOutcome.ActualTrigger,
			})
		}
		if len(changedBackends) == 0 {
			continue
		}

		caseDelta := MultiBackendEvalCaseDelta{
			ID:                    caseID,
			Label:                 candidateDelta.meta.Label,
			Prompt:                candidateDelta.meta.Prompt,
			Profile:               candidateDelta.meta.Profile,
			Tags:                  append([]string(nil), candidateDelta.meta.Tags...),
			Expectation:           candidateDelta.meta.Expectation,
			BaseDisagreement:      baseDelta.meta.Disagreement,
			CandidateDisagreement: candidateDelta.meta.Disagreement,
			ChangedBackends:       changedBackends,
		}
		deltas = append(deltas, caseDelta)
		if !baseDelta.meta.Disagreement && candidateDelta.meta.Disagreement {
			widened = append(widened, caseDelta)
		}
		if baseDelta.meta.Disagreement && !candidateDelta.meta.Disagreement {
			narrowed = append(narrowed, caseDelta)
		}
	}

	return deltas, widened, narrowed, nil
}

type indexedCaseMeta struct {
	Label        string
	Prompt       string
	Profile      string
	Tags         []string
	Expectation  RoutingExpectation
	Disagreement bool
}

type indexedCase struct {
	meta     indexedCaseMeta
	outcomes map[string]MultiBackendCaseOutcome
}

func indexMultiBackendCases(report MultiBackendEvalReport, backendIDs []string) (map[string]indexedCase, error) {
	index := make(map[string]indexedCase)

	for _, item := range report.DifferingCases {
		entry := indexedCase{
			outcomes: make(map[string]MultiBackendCaseOutcome),
		}
		passedCount := 0
		for _, outcome := range item.Outcomes {
			entry.outcomes[outcome.BackendID] = outcome
			if outcome.Passed {
				passedCount++
			}
		}
		entry.meta = indexedCaseMeta{
			Label:        item.Label,
			Prompt:       item.Prompt,
			Profile:      item.Profile,
			Tags:         append([]string(nil), item.Tags...),
			Expectation:  item.Expectation,
			Disagreement: passedCount > 0 && passedCount < len(item.Outcomes),
		}
		index[item.ID] = entry
	}

	for _, backend := range report.Backends {
		for _, result := range backend.Results {
			entry, ok := index[result.ID]
			if !ok {
				entry = indexedCase{
					outcomes: make(map[string]MultiBackendCaseOutcome),
				}
				entry.meta = indexedCaseMeta{
					Label:        result.Label,
					Prompt:       result.Prompt,
					Profile:      result.Profile,
					Tags:         append([]string(nil), result.Tags...),
					Expectation:  result.Expectation,
					Disagreement: false,
				}
				index[result.ID] = entry
			}
			entry.outcomes[backend.Backend.ID] = MultiBackendCaseOutcome{
				BackendID:     backend.Backend.ID,
				BackendName:   backend.Backend.Name,
				Passed:        result.Passed,
				ActualTrigger: result.ActualTrigger,
				FailureKind:   result.FailureKind,
				Reason:        result.Reason,
			}
			index[result.ID] = entry
		}
	}

	for caseID, entry := range index {
		passedCount := 0
		for _, backendID := range backendIDs {
			outcome, ok := entry.outcomes[backendID]
			if !ok {
				return nil, fmt.Errorf("backend %q missing case %q", backendID, caseID)
			}
			if outcome.Passed {
				passedCount++
			}
		}
		entry.meta.Disagreement = passedCount > 0 && passedCount < len(backendIDs)
		index[caseID] = entry
	}

	return index, nil
}

func summarizeMultiBackendComparison(perBackend []BackendEvalComparison, widened, narrowed []MultiBackendEvalCaseDelta) MultiBackendEvalComparisonSummary {
	summary := MultiBackendEvalComparisonSummary{
		BackendCount:              len(perBackend),
		WidenedDisagreementCount:  len(widened),
		NarrowedDisagreementCount: len(narrowed),
	}

	for _, backend := range perBackend {
		switch backend.Comparison.Summary.Overall {
		case ComparisonImproved:
			summary.ImprovedBackendCount++
			if len(summary.NotableImprovements) < 5 {
				summary.NotableImprovements = append(summary.NotableImprovements, fmt.Sprintf("%s improved by %.0fpp", backend.Backend.Name, backend.Comparison.Summary.MetricsDelta.PassRate*100))
			}
		case ComparisonRegressed:
			summary.RegressedBackendCount++
			if len(summary.HighPriorityRegressions) < 5 {
				summary.HighPriorityRegressions = append(summary.HighPriorityRegressions, fmt.Sprintf("%s regressed by %.0fpp", backend.Backend.Name, backend.Comparison.Summary.MetricsDelta.PassRate*100))
			}
		case ComparisonMixed:
			summary.MixedBackendCount++
		default:
			summary.UnchangedBackendCount++
		}
		for _, area := range backend.Comparison.Summary.RegressionAreas {
			if !slices.Contains(summary.BackendRegressionAreas, area) {
				summary.BackendRegressionAreas = append(summary.BackendRegressionAreas, area)
			}
		}
		for _, area := range backend.Comparison.Summary.ImprovementAreas {
			if !slices.Contains(summary.BackendImprovementAreas, area) {
				summary.BackendImprovementAreas = append(summary.BackendImprovementAreas, area)
			}
		}
	}

	switch {
	case summary.RegressedBackendCount > 0 && summary.ImprovedBackendCount > 0:
		summary.Overall = ComparisonMixed
		summary.Summary = "The candidate improves measured routing on some backends and regresses on others."
	case summary.RegressedBackendCount > 0 || summary.MixedBackendCount > 0:
		if summary.ImprovedBackendCount > 0 {
			summary.Overall = ComparisonMixed
			summary.Summary = "The candidate improves some backend-specific routing outcomes but introduces regressions elsewhere."
		} else {
			summary.Overall = ComparisonRegressed
			summary.Summary = "The candidate regresses measured routing on one or more selected backends."
		}
	case summary.ImprovedBackendCount > 0:
		summary.Overall = ComparisonImproved
		summary.Summary = "The candidate improves measured routing across the selected backends without introducing backend-specific regressions."
	default:
		summary.Overall = ComparisonUnchanged
		summary.Summary = "Firety did not detect measurable multi-backend routing changes between the two versions."
	}

	if len(widened) > 0 && len(summary.HighPriorityRegressions) < 5 {
		summary.HighPriorityRegressions = append(summary.HighPriorityRegressions, fmt.Sprintf("%d case(s) widened backend disagreement", len(widened)))
	}
	if len(narrowed) > 0 && len(summary.NotableImprovements) < 5 {
		summary.NotableImprovements = append(summary.NotableImprovements, fmt.Sprintf("%d case(s) narrowed backend disagreement", len(narrowed)))
	}

	return summary
}
