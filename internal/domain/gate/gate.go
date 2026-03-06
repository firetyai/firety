package gate

import (
	"fmt"
	"slices"
	"sort"

	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type Decision string

const (
	DecisionPass Decision = "pass"
	DecisionFail Decision = "fail"
)

type Criteria struct {
	MaxLintErrors               *int                   `json:"max_lint_errors,omitempty"`
	MaxLintWarnings             *int                   `json:"max_lint_warnings,omitempty"`
	MaxRoutingRisk              *lint.RoutingRiskLevel `json:"max_routing_risk,omitempty"`
	MinEvalPassRate             *float64               `json:"min_eval_pass_rate,omitempty"`
	MaxFalsePositives           *int                   `json:"max_false_positives,omitempty"`
	MaxFalseNegatives           *int                   `json:"max_false_negatives,omitempty"`
	MinPerBackendPassRate       *float64               `json:"min_per_backend_pass_rate,omitempty"`
	MaxBackendDisagreementRate  *float64               `json:"max_backend_disagreement_rate,omitempty"`
	MaxPassRateRegression       *float64               `json:"max_pass_rate_regression,omitempty"`
	MaxFalsePositiveIncrease    *int                   `json:"max_false_positive_increase,omitempty"`
	MaxFalseNegativeIncrease    *int                   `json:"max_false_negative_increase,omitempty"`
	MaxWidenedDisagreements     *int                   `json:"max_widened_disagreements,omitempty"`
	FailOnNewErrors             bool                   `json:"fail_on_new_errors,omitempty"`
	FailOnNewPortabilityRegress bool                   `json:"fail_on_new_portability_regressions,omitempty"`
}

type LintFindingRef struct {
	RuleID   string        `json:"rule_id"`
	Category lint.Category `json:"category,omitempty"`
	Severity lint.Severity `json:"severity"`
}

type LintChangedFindingRef struct {
	RuleID            string        `json:"rule_id"`
	Category          lint.Category `json:"category,omitempty"`
	BaseSeverity      lint.Severity `json:"base_severity"`
	CandidateSeverity lint.Severity `json:"candidate_severity"`
}

type LintCurrentEvidence struct {
	Target       string                   `json:"target"`
	ErrorCount   int                      `json:"error_count"`
	WarningCount int                      `json:"warning_count"`
	RuleIDs      []string                 `json:"rule_ids,omitempty"`
	RoutingRisk  *lint.RoutingRiskSummary `json:"routing_risk,omitempty"`
}

type LintCompareEvidence struct {
	BaseTarget       string                       `json:"base_target"`
	CandidateTarget  string                       `json:"candidate_target"`
	Summary          lint.ReportComparisonSummary `json:"summary"`
	AddedFindings    []LintFindingRef             `json:"added_findings,omitempty"`
	ChangedFindings  []LintChangedFindingRef      `json:"changed_findings,omitempty"`
	RoutingRiskDelta *lint.RoutingRiskDelta       `json:"routing_risk_delta,omitempty"`
}

type EvalCurrentEvidence struct {
	Target               string                            `json:"target"`
	Suite                domaineval.RoutingEvalSuiteInfo   `json:"suite"`
	Backend              domaineval.RoutingEvalBackendInfo `json:"backend"`
	Summary              domaineval.RoutingEvalSummary     `json:"summary"`
	FailedCaseIDs        []string                          `json:"failed_case_ids,omitempty"`
	FalsePositiveCaseIDs []string                          `json:"false_positive_case_ids,omitempty"`
	FalseNegativeCaseIDs []string                          `json:"false_negative_case_ids,omitempty"`
}

type EvalCompareEvidence struct {
	BaseTarget      string                           `json:"base_target"`
	CandidateTarget string                           `json:"candidate_target"`
	Comparison      domaineval.RoutingEvalComparison `json:"comparison"`
}

type MultiBackendCurrentEvidence struct {
	Target           string                             `json:"target"`
	Suite            domaineval.RoutingEvalSuiteInfo    `json:"suite"`
	Summary          domaineval.MultiBackendEvalSummary `json:"summary"`
	Backends         []domaineval.BackendEvalReport     `json:"backends"`
	DisagreementRate *float64                           `json:"disagreement_rate,omitempty"`
	DifferingCaseIDs []string                           `json:"differing_case_ids,omitempty"`
}

type MultiBackendCompareEvidence struct {
	BaseTarget      string                                `json:"base_target"`
	CandidateTarget string                                `json:"candidate_target"`
	Comparison      domaineval.MultiBackendEvalComparison `json:"comparison"`
}

type Evidence struct {
	LintCurrent         *LintCurrentEvidence         `json:"lint_current,omitempty"`
	LintCompare         *LintCompareEvidence         `json:"lint_compare,omitempty"`
	EvalCurrent         *EvalCurrentEvidence         `json:"eval_current,omitempty"`
	EvalCompare         *EvalCompareEvidence         `json:"eval_compare,omitempty"`
	MultiBackendCurrent *MultiBackendCurrentEvidence `json:"multi_backend_current,omitempty"`
	MultiBackendCompare *MultiBackendCompareEvidence `json:"multi_backend_compare,omitempty"`
}

type Reason struct {
	Code               string   `json:"code"`
	Title              string   `json:"title"`
	Summary            string   `json:"summary"`
	RelatedRuleIDs     []string `json:"related_rule_ids,omitempty"`
	RelatedEvalCaseIDs []string `json:"related_eval_case_ids,omitempty"`
	RelatedBackendIDs  []string `json:"related_backend_ids,omitempty"`
}

type LintMetrics struct {
	ErrorCount   int                    `json:"error_count"`
	WarningCount int                    `json:"warning_count"`
	RoutingRisk  *lint.RoutingRiskLevel `json:"routing_risk,omitempty"`
}

type EvalMetrics struct {
	Backend        string  `json:"backend,omitempty"`
	Total          int     `json:"total"`
	Failed         int     `json:"failed"`
	FalsePositives int     `json:"false_positives"`
	FalseNegatives int     `json:"false_negatives"`
	PassRate       float64 `json:"pass_rate"`
}

type BackendMetrics struct {
	BackendID      string  `json:"backend_id"`
	BackendName    string  `json:"backend_name"`
	PassRate       float64 `json:"pass_rate"`
	FalsePositives int     `json:"false_positives"`
	FalseNegatives int     `json:"false_negatives"`
	Failed         int     `json:"failed"`
}

type MultiBackendMetrics struct {
	BackendCount       int              `json:"backend_count"`
	TotalCases         int              `json:"total_cases"`
	DisagreementRate   *float64         `json:"disagreement_rate,omitempty"`
	DifferingCaseCount int              `json:"differing_case_count,omitempty"`
	Backends           []BackendMetrics `json:"backends,omitempty"`
}

type CompareMetrics struct {
	LintOverall               string   `json:"lint_overall,omitempty"`
	EvalOverall               string   `json:"eval_overall,omitempty"`
	MultiBackendOverall       string   `json:"multi_backend_overall,omitempty"`
	PassRateRegression        *float64 `json:"pass_rate_regression,omitempty"`
	FalsePositiveIncrease     *int     `json:"false_positive_increase,omitempty"`
	FalseNegativeIncrease     *int     `json:"false_negative_increase,omitempty"`
	NewErrorFindings          int      `json:"new_error_findings,omitempty"`
	NewPortabilityRegressions int      `json:"new_portability_regressions,omitempty"`
	WidenedDisagreements      *int     `json:"widened_disagreements,omitempty"`
}

type Metrics struct {
	Lint         *LintMetrics         `json:"lint,omitempty"`
	Eval         *EvalMetrics         `json:"eval,omitempty"`
	MultiBackend *MultiBackendMetrics `json:"multi_backend,omitempty"`
	Compare      *CompareMetrics      `json:"compare,omitempty"`
}

type BackendResult struct {
	BackendID       string   `json:"backend_id"`
	BackendName     string   `json:"backend_name"`
	Decision        Decision `json:"decision"`
	PassRate        float64  `json:"pass_rate"`
	FalsePositives  int      `json:"false_positives"`
	FalseNegatives  int      `json:"false_negatives"`
	Failed          int      `json:"failed"`
	BlockingReasons []string `json:"blocking_reasons,omitempty"`
}

type CompareContext struct {
	BaseTarget          string `json:"base_target,omitempty"`
	CandidateTarget     string `json:"candidate_target,omitempty"`
	LintOverall         string `json:"lint_overall,omitempty"`
	EvalOverall         string `json:"eval_overall,omitempty"`
	MultiBackendOverall string `json:"multi_backend_overall,omitempty"`
}

type Result struct {
	Decision          Decision        `json:"decision"`
	Summary           string          `json:"summary"`
	Criteria          Criteria        `json:"criteria"`
	BlockingReasons   []Reason        `json:"blocking_reasons,omitempty"`
	Warnings          []Reason        `json:"warnings,omitempty"`
	SupportingMetrics Metrics         `json:"supporting_metrics,omitempty"`
	PerBackendResults []BackendResult `json:"per_backend_results,omitempty"`
	CompareContext    *CompareContext `json:"compare_context,omitempty"`
	NextAction        string          `json:"next_action,omitempty"`
}

func Evaluate(criteria Criteria, evidence Evidence) (Result, error) {
	if err := validateCriteria(criteria); err != nil {
		return Result{}, err
	}

	result := Result{
		Decision: DecisionPass,
		Criteria: criteria,
	}

	lintCurrent := evidence.LintCurrent
	evalCurrent := evidence.EvalCurrent
	multiCurrent := evidence.MultiBackendCurrent
	if evalCurrent == nil && evidence.EvalCompare != nil {
		evalCurrent = evalCurrentFromCompare(*evidence.EvalCompare)
	}
	if multiCurrent == nil && evidence.MultiBackendCompare != nil {
		multiCurrent = multiBackendCurrentFromCompare(*evidence.MultiBackendCompare)
	}

	result.SupportingMetrics = buildMetrics(lintCurrent, evalCurrent, multiCurrent, evidence)
	result.PerBackendResults = evaluatePerBackend(criteria, multiCurrent)
	result.CompareContext = buildCompareContext(evidence)

	if criteria.MaxLintErrors != nil {
		if lintCurrent == nil {
			return Result{}, fmt.Errorf("max lint errors criterion requires lint evidence")
		}
		if lintCurrent.ErrorCount > *criteria.MaxLintErrors {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:           "gate.lint-errors-exceeded",
				Title:          "Lint errors exceed policy",
				Summary:        fmt.Sprintf("The candidate has %d lint error(s), above the allowed maximum of %d.", lintCurrent.ErrorCount, *criteria.MaxLintErrors),
				RelatedRuleIDs: firstStrings(lintCurrent.RuleIDs, 5),
			})
		}
	}

	if criteria.MaxLintWarnings != nil {
		if lintCurrent == nil {
			return Result{}, fmt.Errorf("max lint warnings criterion requires lint evidence")
		}
		if lintCurrent.WarningCount > *criteria.MaxLintWarnings {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:           "gate.lint-warnings-exceeded",
				Title:          "Lint warnings exceed policy",
				Summary:        fmt.Sprintf("The candidate has %d lint warning(s), above the allowed maximum of %d.", lintCurrent.WarningCount, *criteria.MaxLintWarnings),
				RelatedRuleIDs: firstStrings(lintCurrent.RuleIDs, 5),
			})
		}
	}

	if criteria.MaxRoutingRisk != nil {
		if lintCurrent == nil || lintCurrent.RoutingRisk == nil {
			return Result{}, fmt.Errorf("max routing risk criterion requires routing-risk evidence")
		}
		if routingRiskRank(lintCurrent.RoutingRisk.OverallRisk) > routingRiskRank(*criteria.MaxRoutingRisk) {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:  "gate.routing-risk-too-high",
				Title: "Routing risk exceeds policy",
				Summary: fmt.Sprintf(
					"The candidate routing risk is %s, above the allowed maximum of %s.",
					lintCurrent.RoutingRisk.OverallRisk,
					*criteria.MaxRoutingRisk,
				),
				RelatedRuleIDs: contributingRuleIDs(lintCurrent.RoutingRisk),
			})
		}
	}

	if criteria.MinEvalPassRate != nil {
		if evalCurrent == nil {
			return Result{}, fmt.Errorf("min eval pass rate criterion requires eval evidence")
		}
		if evalCurrent.Summary.PassRate < *criteria.MinEvalPassRate {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:               "gate.eval-pass-rate-too-low",
				Title:              "Eval pass rate is below policy",
				Summary:            fmt.Sprintf("The candidate pass rate is %.0f%%, below the required minimum of %.0f%%.", evalCurrent.Summary.PassRate*100, *criteria.MinEvalPassRate*100),
				RelatedEvalCaseIDs: firstStrings(evalCurrent.FailedCaseIDs, 5),
			})
		}
	}

	if criteria.MaxFalsePositives != nil {
		if evalCurrent == nil {
			return Result{}, fmt.Errorf("max false positives criterion requires eval evidence")
		}
		if evalCurrent.Summary.FalsePositives > *criteria.MaxFalsePositives {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:               "gate.false-positives-exceeded",
				Title:              "False positives exceed policy",
				Summary:            fmt.Sprintf("The candidate has %d false positive(s), above the allowed maximum of %d.", evalCurrent.Summary.FalsePositives, *criteria.MaxFalsePositives),
				RelatedEvalCaseIDs: firstStrings(evalCurrent.FalsePositiveCaseIDs, 5),
			})
		}
	}

	if criteria.MaxFalseNegatives != nil {
		if evalCurrent == nil {
			return Result{}, fmt.Errorf("max false negatives criterion requires eval evidence")
		}
		if evalCurrent.Summary.FalseNegatives > *criteria.MaxFalseNegatives {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:               "gate.false-negatives-exceeded",
				Title:              "False negatives exceed policy",
				Summary:            fmt.Sprintf("The candidate has %d false negative(s), above the allowed maximum of %d.", evalCurrent.Summary.FalseNegatives, *criteria.MaxFalseNegatives),
				RelatedEvalCaseIDs: firstStrings(evalCurrent.FalseNegativeCaseIDs, 5),
			})
		}
	}

	if criteria.MinPerBackendPassRate != nil {
		if multiCurrent == nil {
			return Result{}, fmt.Errorf("min per-backend pass rate criterion requires multi-backend eval evidence")
		}
		for _, backend := range result.PerBackendResults {
			if backend.PassRate >= *criteria.MinPerBackendPassRate {
				continue
			}
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:              "gate.backend-pass-rate-too-low",
				Title:             "Per-backend pass rate is below policy",
				Summary:           fmt.Sprintf("%s has a %.0f%% pass rate, below the required minimum of %.0f%%.", backend.BackendName, backend.PassRate*100, *criteria.MinPerBackendPassRate*100),
				RelatedBackendIDs: []string{backend.BackendID},
			})
		}
	}

	if criteria.MaxBackendDisagreementRate != nil {
		if multiCurrent == nil || multiCurrent.DisagreementRate == nil {
			return Result{}, fmt.Errorf("max backend disagreement rate criterion requires current multi-backend disagreement evidence")
		}
		if *multiCurrent.DisagreementRate > *criteria.MaxBackendDisagreementRate {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:               "gate.backend-disagreement-too-high",
				Title:              "Backend disagreement exceeds policy",
				Summary:            fmt.Sprintf("Backend disagreement is %.0f%%, above the allowed maximum of %.0f%%.", *multiCurrent.DisagreementRate*100, *criteria.MaxBackendDisagreementRate*100),
				RelatedEvalCaseIDs: firstStrings(multiCurrent.DifferingCaseIDs, 5),
			})
		}
	}

	if criteria.MaxPassRateRegression != nil {
		if evidence.EvalCompare == nil {
			return Result{}, fmt.Errorf("max pass rate regression criterion requires eval compare evidence")
		}
		regression := -evidence.EvalCompare.Comparison.Summary.MetricsDelta.PassRate
		if regression > *criteria.MaxPassRateRegression {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:               "gate.pass-rate-regression-too-high",
				Title:              "Measured routing regressed too much",
				Summary:            fmt.Sprintf("Pass rate regressed by %.0f percentage points, above the allowed maximum regression of %.0f percentage points.", regression*100, *criteria.MaxPassRateRegression*100),
				RelatedEvalCaseIDs: caseIDsFromChanges(evidence.EvalCompare.Comparison.FlippedToFail, 5),
			})
		}
	}

	if criteria.MaxFalsePositiveIncrease != nil {
		if evidence.EvalCompare == nil {
			return Result{}, fmt.Errorf("max false positive increase criterion requires eval compare evidence")
		}
		increase := evidence.EvalCompare.Comparison.Summary.MetricsDelta.FalsePositives
		if increase > *criteria.MaxFalsePositiveIncrease {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:               "gate.false-positive-increase-too-high",
				Title:              "False positives regressed too much",
				Summary:            fmt.Sprintf("False positives increased by %d, above the allowed maximum increase of %d.", increase, *criteria.MaxFalsePositiveIncrease),
				RelatedEvalCaseIDs: caseIDsFromFailureKind(evidence.EvalCompare.Comparison.FlippedToFail, domaineval.RoutingFalsePositive, 5),
			})
		}
	}

	if criteria.MaxFalseNegativeIncrease != nil {
		if evidence.EvalCompare == nil {
			return Result{}, fmt.Errorf("max false negative increase criterion requires eval compare evidence")
		}
		increase := evidence.EvalCompare.Comparison.Summary.MetricsDelta.FalseNegatives
		if increase > *criteria.MaxFalseNegativeIncrease {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:               "gate.false-negative-increase-too-high",
				Title:              "False negatives regressed too much",
				Summary:            fmt.Sprintf("False negatives increased by %d, above the allowed maximum increase of %d.", increase, *criteria.MaxFalseNegativeIncrease),
				RelatedEvalCaseIDs: caseIDsFromFailureKind(evidence.EvalCompare.Comparison.FlippedToFail, domaineval.RoutingFalseNegative, 5),
			})
		}
	}

	if criteria.MaxWidenedDisagreements != nil {
		if evidence.MultiBackendCompare == nil {
			return Result{}, fmt.Errorf("max widened disagreements criterion requires multi-backend compare evidence")
		}
		widened := len(evidence.MultiBackendCompare.Comparison.WidenedDisagreements)
		if widened > *criteria.MaxWidenedDisagreements {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:               "gate.widened-disagreements-too-high",
				Title:              "Backend disagreement widened too much",
				Summary:            fmt.Sprintf("%d case(s) widened backend disagreement, above the allowed maximum of %d.", widened, *criteria.MaxWidenedDisagreements),
				RelatedEvalCaseIDs: multiCaseIDs(evidence.MultiBackendCompare.Comparison.WidenedDisagreements, 5),
			})
		}
	}

	if criteria.FailOnNewErrors {
		if evidence.LintCompare == nil {
			return Result{}, fmt.Errorf("fail on new errors criterion requires lint compare evidence")
		}
		newErrors := newErrorRuleIDs(*evidence.LintCompare)
		if len(newErrors) > 0 {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:           "gate.new-error-findings",
				Title:          "New error findings were introduced",
				Summary:        fmt.Sprintf("The candidate introduces %d new or escalated error finding(s).", len(newErrors)),
				RelatedRuleIDs: firstStrings(newErrors, 5),
			})
		}
	}

	if criteria.FailOnNewPortabilityRegress {
		if evidence.LintCompare == nil {
			return Result{}, fmt.Errorf("fail on new portability regressions criterion requires lint compare evidence")
		}
		portability := newPortabilityRegressionRuleIDs(*evidence.LintCompare)
		if len(portability) > 0 {
			result.BlockingReasons = append(result.BlockingReasons, Reason{
				Code:           "gate.new-portability-regressions",
				Title:          "New portability regressions were introduced",
				Summary:        fmt.Sprintf("The candidate introduces %d new or escalated portability regression(s).", len(portability)),
				RelatedRuleIDs: firstStrings(portability, 5),
			})
		}
	}

	appendNonBlockingWarnings(&result, evidence)

	if len(result.BlockingReasons) > 0 {
		result.Decision = DecisionFail
		result.Summary = fmt.Sprintf("Gate failed: %d blocking criterion violation(s).", len(result.BlockingReasons))
		result.NextAction = fmt.Sprintf("Review the first blocking issue: %s.", result.BlockingReasons[0].Title)
		return result, nil
	}

	result.Summary = "Gate passed: no blocking criteria were exceeded."
	if len(result.Warnings) > 0 {
		result.NextAction = fmt.Sprintf("Review the first warning: %s.", result.Warnings[0].Title)
	}

	return result, nil
}

func validateCriteria(criteria Criteria) error {
	for _, item := range []struct {
		name  string
		value *int
	}{
		{name: "max lint errors", value: criteria.MaxLintErrors},
		{name: "max lint warnings", value: criteria.MaxLintWarnings},
		{name: "max false positives", value: criteria.MaxFalsePositives},
		{name: "max false negatives", value: criteria.MaxFalseNegatives},
		{name: "max false positive increase", value: criteria.MaxFalsePositiveIncrease},
		{name: "max false negative increase", value: criteria.MaxFalseNegativeIncrease},
		{name: "max widened disagreements", value: criteria.MaxWidenedDisagreements},
	} {
		if item.value != nil && *item.value < 0 {
			return fmt.Errorf("%s must not be negative", item.name)
		}
	}
	for _, item := range []struct {
		name  string
		value *float64
	}{
		{name: "min eval pass rate", value: criteria.MinEvalPassRate},
		{name: "min per-backend pass rate", value: criteria.MinPerBackendPassRate},
		{name: "max backend disagreement rate", value: criteria.MaxBackendDisagreementRate},
		{name: "max pass rate regression", value: criteria.MaxPassRateRegression},
	} {
		if item.value != nil && (*item.value < 0 || *item.value > 1) {
			return fmt.Errorf("%s must be between 0 and 1", item.name)
		}
	}
	return nil
}

func evalCurrentFromCompare(compare EvalCompareEvidence) *EvalCurrentEvidence {
	return &EvalCurrentEvidence{
		Target:  compare.CandidateTarget,
		Suite:   compare.Comparison.Suite,
		Backend: compare.Comparison.Backend,
		Summary: compare.Comparison.Candidate.Summary,
	}
}

func multiBackendCurrentFromCompare(compare MultiBackendCompareEvidence) *MultiBackendCurrentEvidence {
	backends := make([]domaineval.BackendEvalReport, 0, len(compare.Comparison.PerBackend))
	for _, backend := range compare.Comparison.PerBackend {
		backends = append(backends, domaineval.BackendEvalReport{
			Backend: backend.Backend,
			Summary: backend.Candidate.Summary,
		})
	}
	return &MultiBackendCurrentEvidence{
		Target:   compare.CandidateTarget,
		Suite:    compare.Comparison.Suite,
		Summary:  domaineval.MultiBackendEvalSummary{BackendCount: len(backends), TotalCases: compare.Comparison.Suite.CaseCount},
		Backends: backends,
	}
}

func buildMetrics(lintCurrent *LintCurrentEvidence, evalCurrent *EvalCurrentEvidence, multiCurrent *MultiBackendCurrentEvidence, evidence Evidence) Metrics {
	metrics := Metrics{}
	if lintCurrent != nil {
		lintMetrics := LintMetrics{
			ErrorCount:   lintCurrent.ErrorCount,
			WarningCount: lintCurrent.WarningCount,
		}
		if lintCurrent.RoutingRisk != nil {
			level := lintCurrent.RoutingRisk.OverallRisk
			lintMetrics.RoutingRisk = &level
		}
		metrics.Lint = &lintMetrics
	}
	if evalCurrent != nil {
		metrics.Eval = &EvalMetrics{
			Backend:        evalCurrent.Backend.Name,
			Total:          evalCurrent.Summary.Total,
			Failed:         evalCurrent.Summary.Failed,
			FalsePositives: evalCurrent.Summary.FalsePositives,
			FalseNegatives: evalCurrent.Summary.FalseNegatives,
			PassRate:       evalCurrent.Summary.PassRate,
		}
	}
	if multiCurrent != nil {
		multi := &MultiBackendMetrics{
			BackendCount:       multiCurrent.Summary.BackendCount,
			TotalCases:         multiCurrent.Summary.TotalCases,
			DifferingCaseCount: len(multiCurrent.DifferingCaseIDs),
		}
		if multiCurrent.DisagreementRate != nil {
			rate := *multiCurrent.DisagreementRate
			multi.DisagreementRate = &rate
		}
		for _, backend := range multiCurrent.Backends {
			multi.Backends = append(multi.Backends, BackendMetrics{
				BackendID:      backend.Backend.ID,
				BackendName:    backend.Backend.Name,
				PassRate:       backend.Summary.PassRate,
				FalsePositives: backend.Summary.FalsePositives,
				FalseNegatives: backend.Summary.FalseNegatives,
				Failed:         backend.Summary.Failed,
			})
		}
		metrics.MultiBackend = multi
	}
	if evidence.LintCompare != nil || evidence.EvalCompare != nil || evidence.MultiBackendCompare != nil {
		compare := &CompareMetrics{}
		if evidence.LintCompare != nil {
			compare.LintOverall = string(evidence.LintCompare.Summary.Overall)
			compare.NewErrorFindings = len(newErrorRuleIDs(*evidence.LintCompare))
			compare.NewPortabilityRegressions = len(newPortabilityRegressionRuleIDs(*evidence.LintCompare))
		}
		if evidence.EvalCompare != nil {
			compare.EvalOverall = string(evidence.EvalCompare.Comparison.Summary.Overall)
			regression := -evidence.EvalCompare.Comparison.Summary.MetricsDelta.PassRate
			if regression > 0 {
				compare.PassRateRegression = &regression
			}
			falsePositiveIncrease := evidence.EvalCompare.Comparison.Summary.MetricsDelta.FalsePositives
			if falsePositiveIncrease > 0 {
				compare.FalsePositiveIncrease = &falsePositiveIncrease
			}
			falseNegativeIncrease := evidence.EvalCompare.Comparison.Summary.MetricsDelta.FalseNegatives
			if falseNegativeIncrease > 0 {
				compare.FalseNegativeIncrease = &falseNegativeIncrease
			}
		}
		if evidence.MultiBackendCompare != nil {
			compare.MultiBackendOverall = string(evidence.MultiBackendCompare.Comparison.AggregateSummary.Overall)
			widened := len(evidence.MultiBackendCompare.Comparison.WidenedDisagreements)
			compare.WidenedDisagreements = &widened
		}
		metrics.Compare = compare
	}
	return metrics
}

func evaluatePerBackend(criteria Criteria, current *MultiBackendCurrentEvidence) []BackendResult {
	if current == nil {
		return nil
	}

	results := make([]BackendResult, 0, len(current.Backends))
	for _, backend := range current.Backends {
		item := BackendResult{
			BackendID:      backend.Backend.ID,
			BackendName:    backend.Backend.Name,
			Decision:       DecisionPass,
			PassRate:       backend.Summary.PassRate,
			FalsePositives: backend.Summary.FalsePositives,
			FalseNegatives: backend.Summary.FalseNegatives,
			Failed:         backend.Summary.Failed,
		}
		if criteria.MinPerBackendPassRate != nil && backend.Summary.PassRate < *criteria.MinPerBackendPassRate {
			item.Decision = DecisionFail
			item.BlockingReasons = append(item.BlockingReasons, fmt.Sprintf("Pass rate %.0f%% is below the required minimum of %.0f%%.", backend.Summary.PassRate*100, *criteria.MinPerBackendPassRate*100))
		}
		results = append(results, item)
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Decision != results[j].Decision {
			return results[i].Decision == DecisionFail
		}
		if results[i].PassRate != results[j].PassRate {
			return results[i].PassRate < results[j].PassRate
		}
		return results[i].BackendID < results[j].BackendID
	})

	return results
}

func buildCompareContext(evidence Evidence) *CompareContext {
	ctx := &CompareContext{}
	if evidence.LintCompare != nil {
		ctx.BaseTarget = evidence.LintCompare.BaseTarget
		ctx.CandidateTarget = evidence.LintCompare.CandidateTarget
		ctx.LintOverall = string(evidence.LintCompare.Summary.Overall)
	}
	if evidence.EvalCompare != nil {
		if ctx.BaseTarget == "" {
			ctx.BaseTarget = evidence.EvalCompare.BaseTarget
			ctx.CandidateTarget = evidence.EvalCompare.CandidateTarget
		}
		ctx.EvalOverall = string(evidence.EvalCompare.Comparison.Summary.Overall)
	}
	if evidence.MultiBackendCompare != nil {
		if ctx.BaseTarget == "" {
			ctx.BaseTarget = evidence.MultiBackendCompare.BaseTarget
			ctx.CandidateTarget = evidence.MultiBackendCompare.CandidateTarget
		}
		ctx.MultiBackendOverall = string(evidence.MultiBackendCompare.Comparison.AggregateSummary.Overall)
	}
	if ctx.BaseTarget == "" && ctx.CandidateTarget == "" && ctx.LintOverall == "" && ctx.EvalOverall == "" && ctx.MultiBackendOverall == "" {
		return nil
	}
	return ctx
}

func appendNonBlockingWarnings(result *Result, evidence Evidence) {
	if evidence.LintCompare != nil && evidence.LintCompare.Summary.Overall == lint.ComparisonRegressed {
		result.Warnings = append(result.Warnings, Reason{
			Code:    "gate.lint-regression-present",
			Title:   "Lint quality regressed",
			Summary: evidence.LintCompare.Summary.Summary,
		})
	}
	if evidence.EvalCompare != nil && evidence.EvalCompare.Comparison.Summary.Overall == domaineval.ComparisonRegressed {
		result.Warnings = append(result.Warnings, Reason{
			Code:               "gate.eval-regression-present",
			Title:              "Measured routing regressed",
			Summary:            evidence.EvalCompare.Comparison.Summary.Summary,
			RelatedEvalCaseIDs: caseIDsFromChanges(evidence.EvalCompare.Comparison.FlippedToFail, 5),
		})
	}
	if evidence.MultiBackendCompare != nil && evidence.MultiBackendCompare.Comparison.AggregateSummary.Overall == domaineval.ComparisonRegressed {
		result.Warnings = append(result.Warnings, Reason{
			Code:               "gate.multi-backend-regression-present",
			Title:              "Backend coverage regressed",
			Summary:            evidence.MultiBackendCompare.Comparison.AggregateSummary.Summary,
			RelatedEvalCaseIDs: multiCaseIDs(evidence.MultiBackendCompare.Comparison.WidenedDisagreements, 5),
		})
	}
}

func routingRiskRank(level lint.RoutingRiskLevel) int {
	switch level {
	case lint.RoutingRiskLow:
		return 0
	case lint.RoutingRiskMedium:
		return 1
	case lint.RoutingRiskHigh:
		return 2
	default:
		return -1
	}
}

func contributingRuleIDs(summary *lint.RoutingRiskSummary) []string {
	if summary == nil {
		return nil
	}
	ids := make([]string, 0)
	for _, area := range summary.RiskAreas {
		for _, ruleID := range area.ContributingRuleIDs {
			if !slices.Contains(ids, ruleID) {
				ids = append(ids, ruleID)
			}
		}
	}
	slices.Sort(ids)
	return ids
}

func caseIDsFromFailureKind(changes []domaineval.RoutingEvalCaseChange, kind domaineval.RoutingFailureKind, limit int) []string {
	ids := make([]string, 0, len(changes))
	for _, change := range changes {
		if change.CandidateFailureKind == kind {
			ids = append(ids, change.ID)
		}
	}
	return firstStrings(ids, limit)
}

func caseIDsFromChanges(changes []domaineval.RoutingEvalCaseChange, limit int) []string {
	ids := make([]string, 0, len(changes))
	for _, change := range changes {
		ids = append(ids, change.ID)
	}
	return firstStrings(ids, limit)
}

func multiCaseIDs(changes []domaineval.MultiBackendEvalCaseDelta, limit int) []string {
	ids := make([]string, 0, len(changes))
	for _, change := range changes {
		ids = append(ids, change.ID)
	}
	return firstStrings(ids, limit)
}

func newErrorRuleIDs(compare LintCompareEvidence) []string {
	ruleIDs := make([]string, 0)
	for _, finding := range compare.AddedFindings {
		if finding.Severity == lint.SeverityError {
			ruleIDs = append(ruleIDs, finding.RuleID)
		}
	}
	for _, finding := range compare.ChangedFindings {
		if finding.CandidateSeverity == lint.SeverityError && finding.BaseSeverity != lint.SeverityError {
			ruleIDs = append(ruleIDs, finding.RuleID)
		}
	}
	return uniqueSorted(ruleIDs)
}

func newPortabilityRegressionRuleIDs(compare LintCompareEvidence) []string {
	ruleIDs := make([]string, 0)
	for _, finding := range compare.AddedFindings {
		if finding.Category == lint.CategoryPortability {
			ruleIDs = append(ruleIDs, finding.RuleID)
		}
	}
	for _, finding := range compare.ChangedFindings {
		if finding.Category == lint.CategoryPortability && severityRank(finding.CandidateSeverity) > severityRank(finding.BaseSeverity) {
			ruleIDs = append(ruleIDs, finding.RuleID)
		}
	}
	return uniqueSorted(ruleIDs)
}

func severityRank(severity lint.Severity) int {
	switch severity {
	case lint.SeverityError:
		return 2
	case lint.SeverityWarning:
		return 1
	default:
		return 0
	}
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func firstStrings(values []string, limit int) []string {
	if len(values) == 0 {
		return nil
	}
	if len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}
