package service

import (
	"github.com/firety/firety/internal/domain/analysis"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type SkillAnalyzeOptions struct {
	Profile    SkillLintProfile
	Strictness lint.Strictness
	SuitePath  string
	Runner     string
}

type SkillAnalyzeResult struct {
	LintReport  lint.Report
	EvalReport  domaineval.RoutingEvalReport
	RoutingRisk lint.RoutingRiskSummary
	ActionAreas []lint.ActionArea
	Correlation analysis.LintEvalCorrelation
}

type SkillAnalyzeService struct {
	linter SkillLinter
	eval   SkillEvalService
}

func NewSkillAnalyzeService(linter SkillLinter, evalService SkillEvalService) SkillAnalyzeService {
	return SkillAnalyzeService{
		linter: linter,
		eval:   evalService,
	}
}

func (s SkillAnalyzeService) Analyze(target string, options SkillAnalyzeOptions) (SkillAnalyzeResult, error) {
	lintReport, err := s.linter.LintWithProfileAndStrictness(target, options.Profile, options.Strictness)
	if err != nil {
		return SkillAnalyzeResult{}, err
	}

	evalReport, err := s.eval.Evaluate(target, SkillEvalOptions{
		SuitePath: options.SuitePath,
		Profile:   options.Profile,
		Runner:    options.Runner,
	})
	if err != nil {
		return SkillAnalyzeResult{}, err
	}

	return SkillAnalyzeResult{
		LintReport:  lintReport,
		EvalReport:  evalReport,
		RoutingRisk: lint.SummarizeRoutingRisk(lintReport.Findings),
		ActionAreas: lint.SummarizeActionAreas(lintReport.Findings),
		Correlation: analysis.CorrelateLintAndEval(lintReport.Findings, evalReport),
	}, nil
}
