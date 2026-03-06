package eval

import "fmt"

type MultiBackendEvalReport struct {
	Target         string                      `json:"target"`
	Suite          RoutingEvalSuiteInfo        `json:"suite"`
	Backends       []BackendEvalReport         `json:"backends"`
	Summary        MultiBackendEvalSummary     `json:"summary"`
	DifferingCases []MultiBackendDifferingCase `json:"differing_cases,omitempty"`
}

type BackendEvalReport struct {
	Backend RoutingEvalBackendInfo  `json:"backend"`
	Summary RoutingEvalSummary      `json:"summary"`
	Results []RoutingEvalCaseResult `json:"results"`
}

type MultiBackendEvalSummary struct {
	BackendCount             int      `json:"backend_count"`
	TotalCases               int      `json:"total_cases"`
	DifferingCaseCount       int      `json:"differing_case_count"`
	StrongestBackend         string   `json:"strongest_backend,omitempty"`
	WeakestBackend           string   `json:"weakest_backend,omitempty"`
	BackendSpecificStrengths []string `json:"backend_specific_strengths,omitempty"`
	BackendSpecificMisses    []string `json:"backend_specific_misses,omitempty"`
	Summary                  string   `json:"summary"`
}

type MultiBackendDifferingCase struct {
	ID          string                    `json:"id"`
	Label       string                    `json:"label,omitempty"`
	Prompt      string                    `json:"prompt"`
	Profile     string                    `json:"profile,omitempty"`
	Tags        []string                  `json:"tags,omitempty"`
	Expectation RoutingExpectation        `json:"expectation"`
	Outcomes    []MultiBackendCaseOutcome `json:"outcomes"`
}

type MultiBackendCaseOutcome struct {
	BackendID     string             `json:"backend_id"`
	BackendName   string             `json:"backend_name"`
	Passed        bool               `json:"passed"`
	ActualTrigger bool               `json:"actual_trigger"`
	FailureKind   RoutingFailureKind `json:"failure_kind,omitempty"`
	Reason        string             `json:"reason,omitempty"`
}

func BuildMultiBackendEvalReport(target string, reports []RoutingEvalReport) (MultiBackendEvalReport, error) {
	if len(reports) < 2 {
		return MultiBackendEvalReport{}, fmt.Errorf("multi-backend eval requires at least two backend reports")
	}

	base := reports[0]
	caseOrder := make([]string, 0, len(base.Results))
	baseCases := make(map[string]RoutingEvalCaseResult, len(base.Results))
	for _, result := range base.Results {
		caseOrder = append(caseOrder, result.ID)
		baseCases[result.ID] = result
	}

	backendReports := make([]BackendEvalReport, 0, len(reports))
	reportIndexes := make([]map[string]RoutingEvalCaseResult, 0, len(reports))
	for _, report := range reports {
		if report.Suite.SchemaVersion != base.Suite.SchemaVersion || report.Suite.Name != base.Suite.Name || report.Suite.CaseCount != base.Suite.CaseCount {
			return MultiBackendEvalReport{}, fmt.Errorf("routing eval reports do not share the same suite identity")
		}

		index := make(map[string]RoutingEvalCaseResult, len(report.Results))
		for _, result := range report.Results {
			index[result.ID] = result
		}
		if len(index) != len(baseCases) {
			return MultiBackendEvalReport{}, fmt.Errorf("routing eval reports do not contain the same case ids")
		}
		for caseID := range baseCases {
			if _, ok := index[caseID]; !ok {
				return MultiBackendEvalReport{}, fmt.Errorf("routing eval case %q is missing from backend %q", caseID, report.Backend.ID)
			}
		}

		backendReports = append(backendReports, BackendEvalReport{
			Backend: report.Backend,
			Summary: report.Summary,
			Results: report.Results,
		})
		reportIndexes = append(reportIndexes, index)
	}

	differingCases := make([]MultiBackendDifferingCase, 0)
	uniquePassCounts := make(map[string]int, len(reports))
	uniqueFailCounts := make(map[string]int, len(reports))

	for _, caseID := range caseOrder {
		baseResult := reportIndexes[0][caseID]
		outcomes := make([]MultiBackendCaseOutcome, 0, len(reportIndexes))
		passedCount := 0
		failedCount := 0
		lastPassedBackendID := ""
		lastFailedBackendID := ""

		for idx, index := range reportIndexes {
			result := index[caseID]
			outcomes = append(outcomes, MultiBackendCaseOutcome{
				BackendID:     backendReports[idx].Backend.ID,
				BackendName:   backendReports[idx].Backend.Name,
				Passed:        result.Passed,
				ActualTrigger: result.ActualTrigger,
				FailureKind:   result.FailureKind,
				Reason:        result.Reason,
			})

			if result.Passed {
				passedCount++
				lastPassedBackendID = backendReports[idx].Backend.ID
			} else {
				failedCount++
				lastFailedBackendID = backendReports[idx].Backend.ID
			}
		}

		if passedCount == 1 && failedCount == len(reportIndexes)-1 {
			uniquePassCounts[lastPassedBackendID]++
		}
		if failedCount == 1 && passedCount == len(reportIndexes)-1 {
			uniqueFailCounts[lastFailedBackendID]++
		}

		if passedCount == 0 || failedCount == 0 {
			continue
		}

		differingCases = append(differingCases, MultiBackendDifferingCase{
			ID:          caseID,
			Label:       baseResult.Label,
			Prompt:      baseResult.Prompt,
			Profile:     baseResult.Profile,
			Tags:        append([]string(nil), baseResult.Tags...),
			Expectation: baseResult.Expectation,
			Outcomes:    outcomes,
		})
	}

	summary := summarizeMultiBackendEval(backendReports, differingCases, uniquePassCounts, uniqueFailCounts)

	return MultiBackendEvalReport{
		Target:         target,
		Suite:          base.Suite,
		Backends:       backendReports,
		Summary:        summary,
		DifferingCases: differingCases,
	}, nil
}

func summarizeMultiBackendEval(reports []BackendEvalReport, differingCases []MultiBackendDifferingCase, uniquePassCounts, uniqueFailCounts map[string]int) MultiBackendEvalSummary {
	summary := MultiBackendEvalSummary{
		BackendCount:       len(reports),
		TotalCases:         reports[0].Summary.Total,
		DifferingCaseCount: len(differingCases),
	}

	strongest := reports[0]
	weakest := reports[0]
	for _, report := range reports[1:] {
		if report.Summary.PassRate > strongest.Summary.PassRate || (report.Summary.PassRate == strongest.Summary.PassRate && report.Backend.Name < strongest.Backend.Name) {
			strongest = report
		}
		if report.Summary.PassRate < weakest.Summary.PassRate || (report.Summary.PassRate == weakest.Summary.PassRate && report.Backend.Name < weakest.Backend.Name) {
			weakest = report
		}
	}
	summary.StrongestBackend = strongest.Backend.Name
	summary.WeakestBackend = weakest.Backend.Name
	summary.Summary = fmt.Sprintf("%d backend(s) evaluated across %d case(s), with %d differing case(s).", summary.BackendCount, summary.TotalCases, summary.DifferingCaseCount)

	for _, report := range reports {
		if count := uniquePassCounts[report.Backend.ID]; count > 0 {
			summary.BackendSpecificStrengths = append(summary.BackendSpecificStrengths, fmt.Sprintf("%s uniquely passed %d case(s).", report.Backend.Name, count))
		}
		if count := uniqueFailCounts[report.Backend.ID]; count > 0 {
			summary.BackendSpecificMisses = append(summary.BackendSpecificMisses, fmt.Sprintf("%s uniquely failed %d case(s).", report.Backend.Name, count))
		}
	}

	return summary
}
