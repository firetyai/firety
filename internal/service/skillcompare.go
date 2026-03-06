package service

import "github.com/firety/firety/internal/domain/lint"

type SkillCompareResult struct {
	BaseReport      lint.Report
	CandidateReport lint.Report
	Comparison      lint.ReportComparison
}

type SkillCompareService struct {
	linter SkillLinter
}

func NewSkillCompareService(linter SkillLinter) SkillCompareService {
	return SkillCompareService{linter: linter}
}

func (s SkillCompareService) Compare(basePath, candidatePath string, profile SkillLintProfile, strictness lint.Strictness) (SkillCompareResult, error) {
	baseReport, err := s.linter.LintWithProfileAndStrictness(basePath, profile, strictness)
	if err != nil {
		return SkillCompareResult{}, err
	}

	candidateReport, err := s.linter.LintWithProfileAndStrictness(candidatePath, profile, strictness)
	if err != nil {
		return SkillCompareResult{}, err
	}

	return SkillCompareResult{
		BaseReport:      baseReport,
		CandidateReport: candidateReport,
		Comparison:      lint.CompareReports(baseReport, candidateReport),
	}, nil
}
