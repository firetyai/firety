package service

import (
	"github.com/firety/firety/internal/domain/analysis"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type SkillPlanOptions struct {
	Profile           SkillLintProfile
	Strictness        lint.Strictness
	SuitePath         string
	Runner            string
	BackendSelections []SkillEvalBackendSelection
}

type SkillPlanResult struct {
	LintReport       lint.Report
	RoutingRisk      lint.RoutingRiskSummary
	ActionAreas      []lint.ActionArea
	EvalReport       *domaineval.RoutingEvalReport
	Correlation      analysis.LintEvalCorrelation
	MultiBackendEval *domaineval.MultiBackendEvalReport
	Plan             analysis.ImprovementPlan
}

type SkillPlanService struct {
	linter SkillLinter
	eval   SkillEvalService
}

func NewSkillPlanService(linter SkillLinter, evalService SkillEvalService) SkillPlanService {
	return SkillPlanService{
		linter: linter,
		eval:   evalService,
	}
}

func (s SkillPlanService) Build(target string, options SkillPlanOptions) (SkillPlanResult, error) {
	lintReport, err := s.linter.LintWithProfileAndStrictness(target, options.Profile, options.Strictness)
	if err != nil {
		return SkillPlanResult{}, err
	}

	result := SkillPlanResult{
		LintReport:  lintReport,
		RoutingRisk: lint.SummarizeRoutingRisk(lintReport.Findings),
		ActionAreas: lint.SummarizeActionAreas(lintReport.Findings),
	}

	if len(options.BackendSelections) > 0 {
		multiReport, err := s.eval.EvaluateAcrossBackends(target, options.SuitePath, options.BackendSelections)
		if err != nil {
			return SkillPlanResult{}, err
		}
		result.MultiBackendEval = &multiReport
	} else if shouldRunSingleEval(options) {
		evalReport, err := s.eval.Evaluate(target, SkillEvalOptions{
			SuitePath: options.SuitePath,
			Profile:   options.Profile,
			Runner:    options.Runner,
		})
		if err != nil {
			return SkillPlanResult{}, err
		}
		result.EvalReport = &evalReport
		result.Correlation = analysis.CorrelateLintAndEval(lintReport.Findings, evalReport)
	}

	result.Plan = analysis.BuildImprovementPlan(analysis.ImprovementPlanEvidence{
		Findings:         lintReport.Findings,
		RoutingRisk:      result.RoutingRisk,
		ActionAreas:      result.ActionAreas,
		Correlation:      result.Correlation,
		EvalReport:       result.EvalReport,
		MultiBackendEval: result.MultiBackendEval,
	})

	return result, nil
}

func shouldRunSingleEval(options SkillPlanOptions) bool {
	return options.Runner != ""
}
