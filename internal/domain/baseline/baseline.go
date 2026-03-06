package baseline

import (
	"fmt"
	"slices"
	"strings"

	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type Scope string
type Outcome string

const (
	ScopeLint             Scope = "lint"
	ScopeLintEval         Scope = "lint+eval"
	ScopeLintMultiBackend Scope = "lint+multi-backend-eval"

	OutcomeImproved  Outcome = "improved"
	OutcomeRegressed Outcome = "regressed"
	OutcomeMixed     Outcome = "mixed"
	OutcomeUnchanged Outcome = "unchanged"
)

type BackendSelection struct {
	ID     string `json:"id"`
	Runner string `json:"runner,omitempty"`
}

type SnapshotContext struct {
	Target     string             `json:"target"`
	Profile    string             `json:"profile"`
	Strictness string             `json:"strictness"`
	SuitePath  string             `json:"suite_path,omitempty"`
	Runner     string             `json:"runner,omitempty"`
	Backends   []BackendSelection `json:"backends,omitempty"`
}

type SnapshotSummary struct {
	Scope               Scope                 `json:"scope"`
	Summary             string                `json:"summary"`
	ErrorCount          int                   `json:"error_count"`
	WarningCount        int                   `json:"warning_count"`
	RoutingRisk         lint.RoutingRiskLevel `json:"routing_risk"`
	HasEval             bool                  `json:"has_eval"`
	HasMultiBackendEval bool                  `json:"has_multi_backend_eval"`
	EvalPassRate        *float64              `json:"eval_pass_rate,omitempty"`
	BackendCount        int                   `json:"backend_count,omitempty"`
	DisagreementCount   int                   `json:"backend_disagreement_count,omitempty"`
}

type Snapshot struct {
	Context          SnapshotContext                    `json:"context"`
	Summary          SnapshotSummary                    `json:"summary"`
	LintReport       lint.Report                        `json:"lint_report"`
	RoutingRisk      lint.RoutingRiskSummary            `json:"routing_risk"`
	EvalReport       *domaineval.RoutingEvalReport      `json:"eval_report,omitempty"`
	MultiBackendEval *domaineval.MultiBackendEvalReport `json:"multi_backend_eval,omitempty"`
}

type ComponentStatus struct {
	Key     string  `json:"key"`
	Title   string  `json:"title"`
	Outcome Outcome `json:"outcome"`
	Summary string  `json:"summary"`
}

type ComparisonSummary struct {
	Overall                 Outcome           `json:"overall"`
	Summary                 string            `json:"summary"`
	Components              []ComponentStatus `json:"components,omitempty"`
	HighPriorityRegressions []string          `json:"high_priority_regressions,omitempty"`
	NotableImprovements     []string          `json:"notable_improvements,omitempty"`
	UpdateRecommendation    string            `json:"update_recommendation,omitempty"`
}

type Comparison struct {
	BaselineTarget         string                                 `json:"baseline_target"`
	CurrentTarget          string                                 `json:"current_target"`
	BaselineContext        SnapshotContext                        `json:"baseline_context"`
	CurrentSummary         SnapshotSummary                        `json:"current_summary"`
	Summary                ComparisonSummary                      `json:"summary"`
	LintComparison         *lint.ReportComparison                 `json:"lint_comparison,omitempty"`
	EvalComparison         *domaineval.RoutingEvalComparison      `json:"eval_comparison,omitempty"`
	MultiBackendComparison *domaineval.MultiBackendEvalComparison `json:"multi_backend_comparison,omitempty"`
}

func BuildSnapshot(context SnapshotContext, lintReport lint.Report, evalReport *domaineval.RoutingEvalReport, multiBackend *domaineval.MultiBackendEvalReport) Snapshot {
	routingRisk := lint.SummarizeRoutingRisk(lintReport.Findings)
	summary := summarizeSnapshot(lintReport, routingRisk, evalReport, multiBackend)
	return Snapshot{
		Context:          context,
		Summary:          summary,
		LintReport:       lintReport,
		RoutingRisk:      routingRisk,
		EvalReport:       evalReport,
		MultiBackendEval: multiBackend,
	}
}

func summarizeSnapshot(lintReport lint.Report, routingRisk lint.RoutingRiskSummary, evalReport *domaineval.RoutingEvalReport, multiBackend *domaineval.MultiBackendEvalReport) SnapshotSummary {
	summary := SnapshotSummary{
		Scope:        ScopeLint,
		ErrorCount:   lintReport.ErrorCount(),
		WarningCount: lintReport.WarningCount(),
		RoutingRisk:  routingRisk.OverallRisk,
		Summary:      fmt.Sprintf("%d lint error(s), %d warning(s), routing risk %s.", lintReport.ErrorCount(), lintReport.WarningCount(), routingRisk.OverallRisk),
	}
	if evalReport != nil {
		summary.Scope = ScopeLintEval
		summary.HasEval = true
		passRate := evalReport.Summary.PassRate
		summary.EvalPassRate = &passRate
		summary.Summary = fmt.Sprintf("%d lint error(s), %d warning(s), routing risk %s, eval pass rate %.0f%%.", lintReport.ErrorCount(), lintReport.WarningCount(), routingRisk.OverallRisk, passRate*100)
	}
	if multiBackend != nil {
		summary.Scope = ScopeLintMultiBackend
		summary.HasMultiBackendEval = true
		summary.BackendCount = multiBackend.Summary.BackendCount
		summary.DisagreementCount = multiBackend.Summary.DifferingCaseCount
		summary.Summary = fmt.Sprintf("%d lint error(s), %d warning(s), routing risk %s, %d backend(s), %d differing case(s).", lintReport.ErrorCount(), lintReport.WarningCount(), routingRisk.OverallRisk, multiBackend.Summary.BackendCount, multiBackend.Summary.DifferingCaseCount)
	}
	return summary
}

func SummarizeComparison(current Snapshot, baseline Snapshot, lintCompare *lint.ReportComparison, evalCompare *domaineval.RoutingEvalComparison, multiCompare *domaineval.MultiBackendEvalComparison) Comparison {
	components := make([]ComponentStatus, 0, 3)
	regressions := make([]string, 0, 6)
	improvements := make([]string, 0, 6)
	hasRegression := false
	hasImprovement := false

	if lintCompare != nil {
		outcome := Outcome(lintCompare.Summary.Overall)
		components = append(components, ComponentStatus{
			Key:     "lint",
			Title:   "Lint quality",
			Outcome: outcome,
			Summary: lintCompare.Summary.Summary,
		})
		regressions = appendPrefixed(regressions, "Lint", lintCompare.Summary.HighPriorityRegressions)
		improvements = appendPrefixed(improvements, "Lint", lintCompare.Summary.NotableImprovements)
		hasRegression = hasRegression || outcome == OutcomeRegressed || outcome == OutcomeMixed
		hasImprovement = hasImprovement || outcome == OutcomeImproved || outcome == OutcomeMixed
	}

	if evalCompare != nil {
		outcome := Outcome(evalCompare.Summary.Overall)
		components = append(components, ComponentStatus{
			Key:     "eval",
			Title:   "Measured routing",
			Outcome: outcome,
			Summary: evalCompare.Summary.Summary,
		})
		regressions = appendPrefixed(regressions, "Eval", evalCompare.Summary.HighPriorityRegressions)
		improvements = appendPrefixed(improvements, "Eval", evalCompare.Summary.NotableImprovements)
		hasRegression = hasRegression || outcome == OutcomeRegressed || outcome == OutcomeMixed
		hasImprovement = hasImprovement || outcome == OutcomeImproved || outcome == OutcomeMixed
	}

	if multiCompare != nil {
		outcome := Outcome(multiCompare.AggregateSummary.Overall)
		components = append(components, ComponentStatus{
			Key:     "multi-backend",
			Title:   "Multi-backend routing",
			Outcome: outcome,
			Summary: multiCompare.AggregateSummary.Summary,
		})
		regressions = appendPrefixed(regressions, "Multi-backend", multiCompare.AggregateSummary.HighPriorityRegressions)
		improvements = appendPrefixed(improvements, "Multi-backend", multiCompare.AggregateSummary.NotableImprovements)
		hasRegression = hasRegression || outcome == OutcomeRegressed || outcome == OutcomeMixed
		hasImprovement = hasImprovement || outcome == OutcomeImproved || outcome == OutcomeMixed
	}

	sortComponentStatuses(components)
	regressions = uniqueFirst(regressions, 5)
	improvements = uniqueFirst(improvements, 5)

	summary := ComparisonSummary{
		Components:              components,
		HighPriorityRegressions: regressions,
		NotableImprovements:     improvements,
	}
	switch {
	case hasRegression && hasImprovement:
		summary.Overall = OutcomeMixed
		summary.Summary = "The current skill improves some quality areas but also regresses versus the saved baseline."
	case hasRegression:
		summary.Overall = OutcomeRegressed
		summary.Summary = "The current skill regresses versus the saved baseline."
	case hasImprovement:
		summary.Overall = OutcomeImproved
		summary.Summary = "The current skill improves versus the saved baseline without introducing new regressions."
	default:
		summary.Overall = OutcomeUnchanged
		summary.Summary = "Firety did not detect meaningful quality changes versus the saved baseline."
	}

	if summary.Overall == OutcomeImproved || summary.Overall == OutcomeUnchanged {
		summary.UpdateRecommendation = "Consider updating the baseline after review if this version is the new accepted reference."
	} else {
		summary.UpdateRecommendation = "Review the highest-priority regressions before updating the baseline."
	}

	return Comparison{
		BaselineTarget:         baseline.Context.Target,
		CurrentTarget:          current.Context.Target,
		BaselineContext:        baseline.Context,
		CurrentSummary:         current.Summary,
		Summary:                summary,
		LintComparison:         lintCompare,
		EvalComparison:         evalCompare,
		MultiBackendComparison: multiCompare,
	}
}

func appendPrefixed(target []string, prefix string, values []string) []string {
	for _, value := range values {
		target = append(target, fmt.Sprintf("%s: %s", prefix, value))
	}
	return target
}

func uniqueFirst(values []string, limit int) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) == limit {
			break
		}
	}
	return out
}

func sortComponentStatuses(values []ComponentStatus) {
	order := map[string]int{
		"lint":          0,
		"eval":          1,
		"multi-backend": 2,
	}
	slices.SortStableFunc(values, func(left, right ComponentStatus) int {
		if order[left.Key] != order[right.Key] {
			return order[left.Key] - order[right.Key]
		}
		return strings.Compare(left.Key, right.Key)
	})
}
