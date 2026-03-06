package service

import (
	"fmt"
	"path/filepath"

	domaineval "github.com/firety/firety/internal/domain/eval"
)

type SkillEvalCompareResult struct {
	BaseReport      domaineval.RoutingEvalReport
	CandidateReport domaineval.RoutingEvalReport
	Comparison      domaineval.RoutingEvalComparison
}

type SkillEvalMultiCompareResult struct {
	BaseReport      domaineval.MultiBackendEvalReport
	CandidateReport domaineval.MultiBackendEvalReport
	Comparison      domaineval.MultiBackendEvalComparison
}

type SkillEvalCompareService struct {
	eval SkillEvalService
}

func NewSkillEvalCompareService(evalService SkillEvalService) SkillEvalCompareService {
	return SkillEvalCompareService{eval: evalService}
}

func (s SkillEvalCompareService) Compare(basePath, candidatePath string, options SkillEvalOptions) (SkillEvalCompareResult, error) {
	suitePath := options.SuitePath
	if suitePath == "" {
		suitePath = filepath.Join(basePath, defaultRoutingEvalSuiteRelativePath)
	}

	sharedOptions := options
	sharedOptions.SuitePath = suitePath

	baseReport, err := s.eval.Evaluate(basePath, sharedOptions)
	if err != nil {
		return SkillEvalCompareResult{}, err
	}

	candidateReport, err := s.eval.Evaluate(candidatePath, sharedOptions)
	if err != nil {
		return SkillEvalCompareResult{}, err
	}

	comparison, err := domaineval.CompareReports(baseReport, candidateReport)
	if err != nil {
		return SkillEvalCompareResult{}, fmt.Errorf("compare eval reports: %w", err)
	}

	return SkillEvalCompareResult{
		BaseReport:      baseReport,
		CandidateReport: candidateReport,
		Comparison:      comparison,
	}, nil
}

func (s SkillEvalCompareService) CompareAcrossBackends(basePath, candidatePath, suitePath string, selections []SkillEvalBackendSelection) (SkillEvalMultiCompareResult, error) {
	if len(selections) < 2 {
		return SkillEvalMultiCompareResult{}, fmt.Errorf("multi-backend eval compare requires at least two backend selections")
	}

	if suitePath == "" {
		suitePath = filepath.Join(basePath, defaultRoutingEvalSuiteRelativePath)
	}

	baseReport, err := s.eval.EvaluateAcrossBackends(basePath, suitePath, selections)
	if err != nil {
		return SkillEvalMultiCompareResult{}, err
	}

	candidateReport, err := s.eval.EvaluateAcrossBackends(candidatePath, suitePath, selections)
	if err != nil {
		return SkillEvalMultiCompareResult{}, err
	}

	comparison, err := domaineval.CompareMultiBackendReports(baseReport, candidateReport)
	if err != nil {
		return SkillEvalMultiCompareResult{}, fmt.Errorf("compare multi-backend eval reports: %w", err)
	}

	return SkillEvalMultiCompareResult{
		BaseReport:      baseReport,
		CandidateReport: candidateReport,
		Comparison:      comparison,
	}, nil
}
