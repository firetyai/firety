package service

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/firety/firety/internal/domain/baseline"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/lint"
)

type SkillGateOptions struct {
	BasePath          string
	BaselinePath      string
	Profile           SkillLintProfile
	Strictness        lint.Strictness
	SuitePath         string
	Runner            string
	BackendSelections []SkillEvalBackendSelection
	InputArtifacts    []string
	Criteria          gate.Criteria
}

type SkillGateResult struct {
	Gate     gate.Result
	Evidence gate.Evidence
}

type SkillGateService struct {
	linter      SkillLinter
	lintCompare SkillCompareService
	eval        SkillEvalService
	evalCompare SkillEvalCompareService
}

func NewSkillGateService(linter SkillLinter, lintCompare SkillCompareService, eval SkillEvalService, evalCompare SkillEvalCompareService) SkillGateService {
	return SkillGateService{
		linter:      linter,
		lintCompare: lintCompare,
		eval:        eval,
		evalCompare: evalCompare,
	}
}

func (s SkillGateService) Evaluate(target string, options SkillGateOptions) (SkillGateResult, error) {
	evidence, err := s.loadGateArtifacts(options.InputArtifacts)
	if err != nil {
		return SkillGateResult{}, err
	}

	if target != "" {
		if err := s.loadFreshEvidence(&evidence, target, options); err != nil {
			return SkillGateResult{}, err
		}
	}

	if isEmptyGateEvidence(evidence) {
		return SkillGateResult{}, fmt.Errorf("quality gate requires a target path or at least one supported input artifact")
	}

	criteria := applyDefaultGateCriteria(options.Criteria, evidence)
	result, err := gate.Evaluate(criteria, evidence)
	if err != nil {
		return SkillGateResult{}, err
	}

	return SkillGateResult{
		Gate:     result,
		Evidence: evidence,
	}, nil
}

func (s SkillGateService) loadFreshEvidence(evidence *gate.Evidence, target string, options SkillGateOptions) error {
	if options.BaselinePath != "" {
		return s.loadBaselineEvidence(evidence, target, options)
	}

	if options.BasePath != "" {
		compareResult, err := s.lintCompare.Compare(options.BasePath, target, options.Profile, options.Strictness)
		if err != nil {
			return err
		}
		if err := assignGateLintCurrent(evidence, lintCurrentEvidenceFromReport(compareResult.CandidateReport), "fresh lint"); err != nil {
			return err
		}
		if err := assignGateLintCompare(evidence, lintCompareEvidenceFromResult(compareResult), "fresh lint compare"); err != nil {
			return err
		}
	} else {
		report, err := s.linter.LintWithProfileAndStrictness(target, options.Profile, options.Strictness)
		if err != nil {
			return err
		}
		if err := assignGateLintCurrent(evidence, lintCurrentEvidenceFromReport(report), "fresh lint"); err != nil {
			return err
		}
	}

	if len(options.BackendSelections) > 0 {
		if options.BasePath != "" {
			result, err := s.evalCompare.CompareAcrossBackends(options.BasePath, target, options.SuitePath, options.BackendSelections)
			if err != nil {
				return err
			}
			if err := assignGateMultiCurrent(evidence, multiBackendCurrentEvidenceFromReport(result.CandidateReport), "fresh multi-backend eval"); err != nil {
				return err
			}
			if err := assignGateMultiCompare(evidence, multiBackendCompareEvidenceFromResult(result), "fresh multi-backend eval compare"); err != nil {
				return err
			}
			return nil
		}

		report, err := s.eval.EvaluateAcrossBackends(target, options.SuitePath, options.BackendSelections)
		if err != nil {
			return err
		}
		if err := assignGateMultiCurrent(evidence, multiBackendCurrentEvidenceFromReport(report), "fresh multi-backend eval"); err != nil {
			return err
		}
		return nil
	}

	if options.Runner != "" {
		if options.BasePath != "" {
			result, err := s.evalCompare.Compare(options.BasePath, target, SkillEvalOptions{
				SuitePath: options.SuitePath,
				Profile:   options.Profile,
				Runner:    options.Runner,
			})
			if err != nil {
				return err
			}
			if err := assignGateEvalCurrent(evidence, evalCurrentEvidenceFromReport(result.CandidateReport), "fresh eval"); err != nil {
				return err
			}
			if err := assignGateEvalCompare(evidence, evalCompareEvidenceFromResult(result), "fresh eval compare"); err != nil {
				return err
			}
			return nil
		}

		report, err := s.eval.Evaluate(target, SkillEvalOptions{
			SuitePath: options.SuitePath,
			Profile:   options.Profile,
			Runner:    options.Runner,
		})
		if err != nil {
			return err
		}
		if err := assignGateEvalCurrent(evidence, evalCurrentEvidenceFromReport(report), "fresh eval"); err != nil {
			return err
		}
	}

	return nil
}

func (s SkillGateService) loadBaselineEvidence(evidence *gate.Evidence, target string, options SkillGateOptions) error {
	result, err := NewSkillBaselineService(s.linter, s.eval).Compare(target, SkillBaselineCompareOptions{
		BaselinePath:      options.BaselinePath,
		Runner:            options.Runner,
		BackendSelections: options.BackendSelections,
	})
	if err != nil {
		return err
	}

	if err := assignGateLintCurrent(evidence, lintCurrentEvidenceFromReport(result.Current.LintReport), "baseline lint"); err != nil {
		return err
	}
	if err := assignGateLintCompare(evidence, lintCompareEvidenceFromBaselineComparison(result.Comparison), "baseline lint compare"); err != nil {
		return err
	}

	if result.Current.EvalReport != nil {
		if err := assignGateEvalCurrent(evidence, evalCurrentEvidenceFromReport(*result.Current.EvalReport), "baseline eval"); err != nil {
			return err
		}
	}
	if result.Comparison.EvalComparison != nil {
		if err := assignGateEvalCompare(evidence, evalCompareEvidenceFromBaselineComparison(result.Comparison), "baseline eval compare"); err != nil {
			return err
		}
	}

	if result.Current.MultiBackendEval != nil {
		if err := assignGateMultiCurrent(evidence, multiBackendCurrentEvidenceFromReport(*result.Current.MultiBackendEval), "baseline multi-backend eval"); err != nil {
			return err
		}
	}
	if result.Comparison.MultiBackendComparison != nil {
		if err := assignGateMultiCompare(evidence, multiBackendCompareEvidenceFromBaselineComparison(result.Comparison), "baseline multi-backend compare"); err != nil {
			return err
		}
	}

	return nil
}

func (s SkillGateService) loadGateArtifacts(paths []string) (gate.Evidence, error) {
	var evidence gate.Evidence
	for _, path := range paths {
		if err := s.loadGateArtifact(&evidence, path); err != nil {
			return gate.Evidence{}, err
		}
	}
	return evidence, nil
}

func (s SkillGateService) loadGateArtifact(evidence *gate.Evidence, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var envelope gateArtifactEnvelope
	if err := json.Unmarshal(content, &envelope); err != nil {
		return fmt.Errorf("parse artifact envelope: %w", err)
	}

	switch envelope.ArtifactType {
	case "firety.skill-lint":
		var value gateSkillLintArtifact
		if err := json.Unmarshal(content, &value); err != nil {
			return err
		}
		return assignGateLintCurrent(evidence, lintCurrentEvidenceFromArtifact(value), path)
	case "firety.skill-analysis":
		var value gateSkillAnalysisArtifact
		if err := json.Unmarshal(content, &value); err != nil {
			return err
		}
		if err := assignGateLintCurrent(evidence, lintCurrentEvidenceFromAnalysisArtifact(value), path); err != nil {
			return err
		}
		return assignGateEvalCurrent(evidence, evalCurrentEvidenceFromAnalysisArtifact(value), path)
	case "firety.skill-lint-compare":
		var value gateSkillLintCompareArtifact
		if err := json.Unmarshal(content, &value); err != nil {
			return err
		}
		if err := assignGateLintCurrent(evidence, lintCurrentEvidenceFromCompareArtifact(value), path); err != nil {
			return err
		}
		return assignGateLintCompare(evidence, lintCompareEvidenceFromArtifact(value), path)
	case "firety.skill-routing-eval":
		var value gateSkillEvalArtifact
		if err := json.Unmarshal(content, &value); err != nil {
			return err
		}
		return assignGateEvalCurrent(evidence, evalCurrentEvidenceFromArtifact(value), path)
	case "firety.skill-routing-eval-compare":
		var value gateSkillEvalCompareArtifact
		if err := json.Unmarshal(content, &value); err != nil {
			return err
		}
		if err := assignGateEvalCurrent(evidence, evalCurrentEvidenceFromCompareArtifact(value), path); err != nil {
			return err
		}
		return assignGateEvalCompare(evidence, evalCompareEvidenceFromArtifact(value), path)
	case "firety.skill-routing-eval-multi":
		var value gateSkillEvalMultiArtifact
		if err := json.Unmarshal(content, &value); err != nil {
			return err
		}
		return assignGateMultiCurrent(evidence, multiBackendCurrentEvidenceFromArtifact(value), path)
	case "firety.skill-routing-eval-compare-multi":
		var value gateSkillEvalMultiCompareArtifact
		if err := json.Unmarshal(content, &value); err != nil {
			return err
		}
		if err := assignGateMultiCurrent(evidence, multiBackendCurrentEvidenceFromCompareArtifact(value), path); err != nil {
			return err
		}
		return assignGateMultiCompare(evidence, multiBackendCompareEvidenceFromArtifact(value), path)
	case "firety.skill-baseline-compare":
		var value gateSkillBaselineCompareArtifact
		if err := json.Unmarshal(content, &value); err != nil {
			return err
		}
		if err := assignGateLintCurrent(evidence, lintCurrentEvidenceFromBaselineCompareArtifact(value), path); err != nil {
			return err
		}
		if lintCompare := lintCompareEvidenceFromBaselineCompareArtifact(value); lintCompare != nil {
			if err := assignGateLintCompare(evidence, lintCompare, path); err != nil {
				return err
			}
		}
		if evalCurrent := evalCurrentEvidenceFromBaselineCompareArtifact(value); evalCurrent != nil {
			if err := assignGateEvalCurrent(evidence, evalCurrent, path); err != nil {
				return err
			}
		}
		if evalCompare := evalCompareEvidenceFromBaselineCompareArtifact(value); evalCompare != nil {
			if err := assignGateEvalCompare(evidence, evalCompare, path); err != nil {
				return err
			}
		}
		if multiCurrent := multiBackendCurrentEvidenceFromBaselineCompareArtifact(value); multiCurrent != nil {
			if err := assignGateMultiCurrent(evidence, multiCurrent, path); err != nil {
				return err
			}
		}
		if multiCompare := multiBackendCompareEvidenceFromBaselineCompareArtifact(value); multiCompare != nil {
			if err := assignGateMultiCompare(evidence, multiCompare, path); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("artifact %s has unsupported type %q for quality gating", path, envelope.ArtifactType)
	}
}

func applyDefaultGateCriteria(criteria gate.Criteria, evidence gate.Evidence) gate.Criteria {
	if criteria.MaxLintErrors == nil && (evidence.LintCurrent != nil || evidence.LintCompare != nil) {
		criteria.MaxLintErrors = intPointer(0)
	}
	if criteria.MinEvalPassRate == nil && (evidence.EvalCurrent != nil || evidence.EvalCompare != nil) {
		criteria.MinEvalPassRate = floatPointer(1.0)
	}
	if criteria.MinPerBackendPassRate == nil && (evidence.MultiBackendCurrent != nil || evidence.MultiBackendCompare != nil) {
		criteria.MinPerBackendPassRate = floatPointer(1.0)
	}
	return criteria
}

func lintCurrentEvidenceFromReport(report lint.Report) *gate.LintCurrentEvidence {
	summary := lint.SummarizeRoutingRisk(report.Findings)
	return &gate.LintCurrentEvidence{
		Target:       report.Target,
		ErrorCount:   report.ErrorCount(),
		WarningCount: report.WarningCount(),
		RuleIDs:      uniqueFindingRuleIDs(report.Findings),
		RoutingRisk:  &summary,
	}
}

func lintCompareEvidenceFromResult(result SkillCompareResult) *gate.LintCompareEvidence {
	return &gate.LintCompareEvidence{
		BaseTarget:       result.BaseReport.Target,
		CandidateTarget:  result.CandidateReport.Target,
		Summary:          result.Comparison.Summary,
		AddedFindings:    gateFindingRefsFromComparison(result.Comparison.AddedFindings),
		ChangedFindings:  gateChangedFindingRefsFromComparison(result.Comparison.ChangedFindings),
		RoutingRiskDelta: &result.Comparison.RoutingRiskDelta,
	}
}

func lintCompareEvidenceFromBaselineComparison(compare baseline.Comparison) *gate.LintCompareEvidence {
	if compare.LintComparison == nil {
		return nil
	}
	return &gate.LintCompareEvidence{
		BaseTarget:       compare.BaselineTarget,
		CandidateTarget:  compare.CurrentTarget,
		Summary:          compare.LintComparison.Summary,
		AddedFindings:    gateFindingRefsFromComparison(compare.LintComparison.AddedFindings),
		ChangedFindings:  gateChangedFindingRefsFromComparison(compare.LintComparison.ChangedFindings),
		RoutingRiskDelta: &compare.LintComparison.RoutingRiskDelta,
	}
}

func evalCurrentEvidenceFromReport(report domaineval.RoutingEvalReport) *gate.EvalCurrentEvidence {
	return &gate.EvalCurrentEvidence{
		Target:               report.Target,
		Suite:                report.Suite,
		Backend:              report.Backend,
		Summary:              report.Summary,
		FailedCaseIDs:        failedCaseIDs(report.Results),
		FalsePositiveCaseIDs: failedCaseIDsByKind(report.Results, domaineval.RoutingFalsePositive),
		FalseNegativeCaseIDs: failedCaseIDsByKind(report.Results, domaineval.RoutingFalseNegative),
	}
}

func evalCompareEvidenceFromResult(result SkillEvalCompareResult) *gate.EvalCompareEvidence {
	return &gate.EvalCompareEvidence{
		BaseTarget:      result.BaseReport.Target,
		CandidateTarget: result.CandidateReport.Target,
		Comparison:      result.Comparison,
	}
}

func evalCompareEvidenceFromBaselineComparison(compare baseline.Comparison) *gate.EvalCompareEvidence {
	if compare.EvalComparison == nil {
		return nil
	}
	return &gate.EvalCompareEvidence{
		BaseTarget:      compare.BaselineTarget,
		CandidateTarget: compare.CurrentTarget,
		Comparison:      *compare.EvalComparison,
	}
}

func multiBackendCurrentEvidenceFromReport(report domaineval.MultiBackendEvalReport) *gate.MultiBackendCurrentEvidence {
	rate := disagreementRate(report.Summary.TotalCases, len(report.DifferingCases))
	return &gate.MultiBackendCurrentEvidence{
		Target:           report.Target,
		Suite:            report.Suite,
		Summary:          report.Summary,
		Backends:         report.Backends,
		DisagreementRate: &rate,
		DifferingCaseIDs: differingCaseIDs(report.DifferingCases),
	}
}

func multiBackendCompareEvidenceFromResult(result SkillEvalMultiCompareResult) *gate.MultiBackendCompareEvidence {
	return &gate.MultiBackendCompareEvidence{
		BaseTarget:      result.BaseReport.Target,
		CandidateTarget: result.CandidateReport.Target,
		Comparison:      result.Comparison,
	}
}

func multiBackendCompareEvidenceFromBaselineComparison(compare baseline.Comparison) *gate.MultiBackendCompareEvidence {
	if compare.MultiBackendComparison == nil {
		return nil
	}
	return &gate.MultiBackendCompareEvidence{
		BaseTarget:      compare.BaselineTarget,
		CandidateTarget: compare.CurrentTarget,
		Comparison:      *compare.MultiBackendComparison,
	}
}

func lintCurrentEvidenceFromArtifact(value gateSkillLintArtifact) *gate.LintCurrentEvidence {
	routingRisk := value.RoutingRisk
	if routingRisk == nil {
		summary := synthesizeRoutingRisk(value.Findings)
		routingRisk = &summary
	}
	return &gate.LintCurrentEvidence{
		Target:       value.Run.Target,
		ErrorCount:   value.Summary.ErrorCount,
		WarningCount: value.Summary.WarningCount,
		RuleIDs:      uniqueArtifactFindingRuleIDs(value.Findings),
		RoutingRisk:  routingRisk,
	}
}

func lintCurrentEvidenceFromBaselineCompareArtifact(value gateSkillBaselineCompareArtifact) *gate.LintCurrentEvidence {
	evidence := &gate.LintCurrentEvidence{
		Target:       value.Comparison.CurrentTarget,
		ErrorCount:   value.Comparison.CurrentSummary.ErrorCount,
		WarningCount: value.Comparison.CurrentSummary.WarningCount,
	}
	if value.Comparison.CurrentSummary.RoutingRisk != "" {
		evidence.RoutingRisk = &lint.RoutingRiskSummary{
			OverallRisk: value.Comparison.CurrentSummary.RoutingRisk,
		}
	}
	return evidence
}

func lintCurrentEvidenceFromAnalysisArtifact(value gateSkillAnalysisArtifact) *gate.LintCurrentEvidence {
	routingRisk := value.Lint.RoutingRisk
	if routingRisk == nil {
		summary := synthesizeRoutingRisk(value.Lint.Findings)
		routingRisk = &summary
	}
	return &gate.LintCurrentEvidence{
		Target:       value.Run.Target,
		ErrorCount:   value.Lint.Summary.ErrorCount,
		WarningCount: value.Lint.Summary.WarningCount,
		RuleIDs:      uniqueArtifactFindingRuleIDs(value.Lint.Findings),
		RoutingRisk:  routingRisk,
	}
}

func lintCurrentEvidenceFromCompareArtifact(value gateSkillLintCompareArtifact) *gate.LintCurrentEvidence {
	current := &gate.LintCurrentEvidence{
		Target:       value.Run.CandidateTarget,
		ErrorCount:   value.Candidate.ErrorCount,
		WarningCount: value.Candidate.WarningCount,
	}
	if value.RoutingRiskDelta != nil {
		current.RoutingRisk = &lint.RoutingRiskSummary{
			OverallRisk: value.RoutingRiskDelta.CandidateOverallRisk,
		}
	}
	return current
}

func lintCompareEvidenceFromArtifact(value gateSkillLintCompareArtifact) *gate.LintCompareEvidence {
	return &gate.LintCompareEvidence{
		BaseTarget:       value.Run.BaseTarget,
		CandidateTarget:  value.Run.CandidateTarget,
		Summary:          value.Comparison,
		AddedFindings:    gateFindingRefsFromArtifact(value.AddedFindings),
		ChangedFindings:  gateChangedFindingRefsFromArtifact(value.ChangedFindings),
		RoutingRiskDelta: value.RoutingRiskDelta,
	}
}

func lintCompareEvidenceFromBaselineCompareArtifact(value gateSkillBaselineCompareArtifact) *gate.LintCompareEvidence {
	if value.Comparison.LintComparison == nil {
		return nil
	}
	return &gate.LintCompareEvidence{
		BaseTarget:       value.Comparison.BaselineTarget,
		CandidateTarget:  value.Comparison.CurrentTarget,
		Summary:          value.Comparison.LintComparison.Summary,
		AddedFindings:    gateFindingRefsFromComparison(value.Comparison.LintComparison.AddedFindings),
		ChangedFindings:  gateChangedFindingRefsFromComparison(value.Comparison.LintComparison.ChangedFindings),
		RoutingRiskDelta: &value.Comparison.LintComparison.RoutingRiskDelta,
	}
}

func evalCurrentEvidenceFromArtifact(value gateSkillEvalArtifact) *gate.EvalCurrentEvidence {
	return &gate.EvalCurrentEvidence{
		Target:               value.Run.Target,
		Suite:                value.Suite,
		Backend:              value.Backend,
		Summary:              value.Summary,
		FailedCaseIDs:        failedCaseIDs(value.Results),
		FalsePositiveCaseIDs: failedCaseIDsByKind(value.Results, domaineval.RoutingFalsePositive),
		FalseNegativeCaseIDs: failedCaseIDsByKind(value.Results, domaineval.RoutingFalseNegative),
	}
}

func evalCurrentEvidenceFromBaselineCompareArtifact(value gateSkillBaselineCompareArtifact) *gate.EvalCurrentEvidence {
	if value.Comparison.EvalComparison == nil {
		return nil
	}
	compare := value.Comparison.EvalComparison
	return &gate.EvalCurrentEvidence{
		Target:               value.Comparison.CurrentTarget,
		Suite:                compare.Suite,
		Backend:              compare.Backend,
		Summary:              compare.Candidate.Summary,
		FailedCaseIDs:        changedCaseIDs(compare.ChangedCases),
		FalsePositiveCaseIDs: changedCaseIDsByKind(compare.FlippedToFail, domaineval.RoutingFalsePositive),
		FalseNegativeCaseIDs: changedCaseIDsByKind(compare.FlippedToFail, domaineval.RoutingFalseNegative),
	}
}

func evalCurrentEvidenceFromAnalysisArtifact(value gateSkillAnalysisArtifact) *gate.EvalCurrentEvidence {
	return &gate.EvalCurrentEvidence{
		Target:               value.Run.Target,
		Suite:                value.Eval.Suite,
		Backend:              value.Eval.Backend,
		Summary:              value.Eval.Summary,
		FailedCaseIDs:        failedCaseIDs(value.Eval.Results),
		FalsePositiveCaseIDs: failedCaseIDsByKind(value.Eval.Results, domaineval.RoutingFalsePositive),
		FalseNegativeCaseIDs: failedCaseIDsByKind(value.Eval.Results, domaineval.RoutingFalseNegative),
	}
}

func evalCurrentEvidenceFromCompareArtifact(value gateSkillEvalCompareArtifact) *gate.EvalCurrentEvidence {
	return &gate.EvalCurrentEvidence{
		Target:  value.Run.CandidateTarget,
		Suite:   value.Suite,
		Backend: value.Backend,
		Summary: value.Candidate.Summary,
	}
}

func evalCompareEvidenceFromArtifact(value gateSkillEvalCompareArtifact) *gate.EvalCompareEvidence {
	return &gate.EvalCompareEvidence{
		BaseTarget:      value.Run.BaseTarget,
		CandidateTarget: value.Run.CandidateTarget,
		Comparison: domaineval.RoutingEvalComparison{
			Base:            value.Base,
			Candidate:       value.Candidate,
			Suite:           value.Suite,
			Backend:         value.Backend,
			Summary:         value.Comparison,
			FlippedToFail:   value.FlippedToFail,
			FlippedToPass:   value.FlippedToPass,
			ChangedCases:    value.ChangedCases,
			ByProfileDeltas: value.ByProfileDeltas,
			ByTagDeltas:     value.ByTagDeltas,
		},
	}
}

func evalCompareEvidenceFromBaselineCompareArtifact(value gateSkillBaselineCompareArtifact) *gate.EvalCompareEvidence {
	if value.Comparison.EvalComparison == nil {
		return nil
	}
	return &gate.EvalCompareEvidence{
		BaseTarget:      value.Comparison.BaselineTarget,
		CandidateTarget: value.Comparison.CurrentTarget,
		Comparison:      *value.Comparison.EvalComparison,
	}
}

func multiBackendCurrentEvidenceFromArtifact(value gateSkillEvalMultiArtifact) *gate.MultiBackendCurrentEvidence {
	rate := disagreementRate(value.Summary.TotalCases, len(value.DifferingCases))
	return &gate.MultiBackendCurrentEvidence{
		Target:           value.Run.Target,
		Suite:            value.Suite,
		Summary:          value.Summary,
		Backends:         value.Results,
		DisagreementRate: &rate,
		DifferingCaseIDs: differingCaseIDs(value.DifferingCases),
	}
}

func multiBackendCurrentEvidenceFromBaselineCompareArtifact(value gateSkillBaselineCompareArtifact) *gate.MultiBackendCurrentEvidence {
	if value.Comparison.MultiBackendComparison == nil {
		return nil
	}
	compare := value.Comparison.MultiBackendComparison
	backends := make([]domaineval.BackendEvalReport, 0, len(compare.PerBackend))
	for _, backend := range compare.PerBackend {
		backends = append(backends, domaineval.BackendEvalReport{
			Backend: backend.Backend,
			Summary: backend.Candidate.Summary,
		})
	}
	rate := disagreementRate(compare.Candidate.Summary.Total, len(compare.DifferingCases))
	return &gate.MultiBackendCurrentEvidence{
		Target:           value.Comparison.CurrentTarget,
		Suite:            compare.Suite,
		Summary:          domaineval.MultiBackendEvalSummary{BackendCount: len(backends), TotalCases: compare.Candidate.Summary.Total, DifferingCaseCount: len(compare.DifferingCases)},
		Backends:         backends,
		DisagreementRate: &rate,
		DifferingCaseIDs: differingCaseDeltaIDs(compare.DifferingCases),
	}
}

func multiBackendCurrentEvidenceFromCompareArtifact(value gateSkillEvalMultiCompareArtifact) *gate.MultiBackendCurrentEvidence {
	backends := make([]domaineval.BackendEvalReport, 0, len(value.PerBackendDeltas))
	for _, backend := range value.PerBackendDeltas {
		backends = append(backends, domaineval.BackendEvalReport{
			Backend: backend.Backend,
			Summary: backend.Candidate.Summary,
		})
	}
	return &gate.MultiBackendCurrentEvidence{
		Target:   value.Run.CandidateTarget,
		Suite:    value.Suite,
		Summary:  domaineval.MultiBackendEvalSummary{BackendCount: len(backends), TotalCases: value.Suite.CaseCount},
		Backends: backends,
	}
}

func multiBackendCompareEvidenceFromArtifact(value gateSkillEvalMultiCompareArtifact) *gate.MultiBackendCompareEvidence {
	return &gate.MultiBackendCompareEvidence{
		BaseTarget:      value.Run.BaseTarget,
		CandidateTarget: value.Run.CandidateTarget,
		Comparison: domaineval.MultiBackendEvalComparison{
			Base:                  value.Base,
			Candidate:             value.Candidate,
			Suite:                 value.Suite,
			Backends:              value.Backends,
			AggregateSummary:      value.AggregateSummary,
			PerBackend:            value.PerBackendDeltas,
			DifferingCases:        value.DifferingCases,
			WidenedDisagreements:  value.WidenedDisagreements,
			NarrowedDisagreements: value.NarrowedDisagreements,
		},
	}
}

func multiBackendCompareEvidenceFromBaselineCompareArtifact(value gateSkillBaselineCompareArtifact) *gate.MultiBackendCompareEvidence {
	if value.Comparison.MultiBackendComparison == nil {
		return nil
	}
	return &gate.MultiBackendCompareEvidence{
		BaseTarget:      value.Comparison.BaselineTarget,
		CandidateTarget: value.Comparison.CurrentTarget,
		Comparison:      *value.Comparison.MultiBackendComparison,
	}
}

func assignGateLintCurrent(evidence *gate.Evidence, current *gate.LintCurrentEvidence, source string) error {
	if current == nil {
		return nil
	}
	if evidence.LintCurrent != nil {
		return fmt.Errorf("duplicate lint evidence provided (%s)", source)
	}
	evidence.LintCurrent = current
	return nil
}

func assignGateLintCompare(evidence *gate.Evidence, compare *gate.LintCompareEvidence, source string) error {
	if compare == nil {
		return nil
	}
	if evidence.LintCompare != nil {
		return fmt.Errorf("duplicate lint compare evidence provided (%s)", source)
	}
	evidence.LintCompare = compare
	return nil
}

func assignGateEvalCurrent(evidence *gate.Evidence, current *gate.EvalCurrentEvidence, source string) error {
	if current == nil {
		return nil
	}
	if evidence.EvalCurrent != nil {
		return fmt.Errorf("duplicate eval evidence provided (%s)", source)
	}
	evidence.EvalCurrent = current
	return nil
}

func assignGateEvalCompare(evidence *gate.Evidence, compare *gate.EvalCompareEvidence, source string) error {
	if compare == nil {
		return nil
	}
	if evidence.EvalCompare != nil {
		return fmt.Errorf("duplicate eval compare evidence provided (%s)", source)
	}
	evidence.EvalCompare = compare
	return nil
}

func assignGateMultiCurrent(evidence *gate.Evidence, current *gate.MultiBackendCurrentEvidence, source string) error {
	if current == nil {
		return nil
	}
	if evidence.MultiBackendCurrent != nil {
		return fmt.Errorf("duplicate multi-backend eval evidence provided (%s)", source)
	}
	evidence.MultiBackendCurrent = current
	return nil
}

func assignGateMultiCompare(evidence *gate.Evidence, compare *gate.MultiBackendCompareEvidence, source string) error {
	if compare == nil {
		return nil
	}
	if evidence.MultiBackendCompare != nil {
		return fmt.Errorf("duplicate multi-backend compare evidence provided (%s)", source)
	}
	evidence.MultiBackendCompare = compare
	return nil
}

func gateFindingRefsFromComparison(findings []lint.ComparisonFinding) []gate.LintFindingRef {
	out := make([]gate.LintFindingRef, 0, len(findings))
	for _, finding := range findings {
		out = append(out, gate.LintFindingRef{
			RuleID:   finding.RuleID,
			Category: finding.Category,
			Severity: finding.Severity,
		})
	}
	return out
}

func gateChangedFindingRefsFromComparison(findings []lint.ComparisonChangedFinding) []gate.LintChangedFindingRef {
	out := make([]gate.LintChangedFindingRef, 0, len(findings))
	for _, finding := range findings {
		out = append(out, gate.LintChangedFindingRef{
			RuleID:            finding.RuleID,
			Category:          finding.Category,
			BaseSeverity:      finding.BaseSeverity,
			CandidateSeverity: finding.CandidateSeverity,
		})
	}
	return out
}

func gateFindingRefsFromArtifact(findings []gateSkillLintCompareFinding) []gate.LintFindingRef {
	out := make([]gate.LintFindingRef, 0, len(findings))
	for _, finding := range findings {
		out = append(out, gate.LintFindingRef{
			RuleID:   finding.RuleID,
			Category: lint.Category(finding.Category),
			Severity: lint.Severity(finding.Severity),
		})
	}
	return out
}

func gateChangedFindingRefsFromArtifact(findings []gateSkillLintCompareChanged) []gate.LintChangedFindingRef {
	out := make([]gate.LintChangedFindingRef, 0, len(findings))
	for _, finding := range findings {
		out = append(out, gate.LintChangedFindingRef{
			RuleID:            finding.RuleID,
			Category:          lint.Category(finding.Category),
			BaseSeverity:      lint.Severity(finding.BaseSeverity),
			CandidateSeverity: lint.Severity(finding.CandidateSeverity),
		})
	}
	return out
}

func uniqueFindingRuleIDs(findings []lint.Finding) []string {
	values := make([]string, 0, len(findings))
	seen := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		if _, ok := seen[finding.RuleID]; ok {
			continue
		}
		seen[finding.RuleID] = struct{}{}
		values = append(values, finding.RuleID)
	}
	slices.Sort(values)
	return values
}

func uniqueArtifactFindingRuleIDs(findings []gateSkillLintArtifactFinding) []string {
	values := make([]string, 0, len(findings))
	seen := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		if _, ok := seen[finding.RuleID]; ok {
			continue
		}
		seen[finding.RuleID] = struct{}{}
		values = append(values, finding.RuleID)
	}
	slices.Sort(values)
	return values
}

func synthesizeRoutingRisk(findings []gateSkillLintArtifactFinding) lint.RoutingRiskSummary {
	synthesized := make([]lint.Finding, 0, len(findings))
	for _, finding := range findings {
		synthesized = append(synthesized, lint.Finding{
			RuleID:   finding.RuleID,
			Severity: lint.Severity(finding.Severity),
			Path:     finding.Path,
			Line:     derefLine(finding.Line),
			Message:  finding.Message,
		})
	}
	return lint.SummarizeRoutingRisk(synthesized)
}

func failedCaseIDs(results []domaineval.RoutingEvalCaseResult) []string {
	values := make([]string, 0)
	for _, result := range results {
		if !result.Passed {
			values = append(values, result.ID)
		}
	}
	return values
}

func failedCaseIDsByKind(results []domaineval.RoutingEvalCaseResult, kind domaineval.RoutingFailureKind) []string {
	values := make([]string, 0)
	for _, result := range results {
		if !result.Passed && result.FailureKind == kind {
			values = append(values, result.ID)
		}
	}
	return values
}

func changedCaseIDs(changes []domaineval.RoutingEvalCaseChange) []string {
	values := make([]string, 0, len(changes))
	for _, change := range changes {
		values = append(values, change.ID)
	}
	return values
}

func changedCaseIDsByKind(changes []domaineval.RoutingEvalCaseChange, kind domaineval.RoutingFailureKind) []string {
	values := make([]string, 0, len(changes))
	for _, change := range changes {
		if change.CandidateFailureKind == kind {
			values = append(values, change.ID)
		}
	}
	return values
}

func disagreementRate(total, differing int) float64 {
	if total == 0 {
		return 0
	}
	return float64(differing) / float64(total)
}

func differingCaseIDs(cases []domaineval.MultiBackendDifferingCase) []string {
	values := make([]string, 0, len(cases))
	for _, item := range cases {
		values = append(values, item.ID)
	}
	return values
}

func differingCaseDeltaIDs(cases []domaineval.MultiBackendEvalCaseDelta) []string {
	values := make([]string, 0, len(cases))
	for _, item := range cases {
		values = append(values, item.ID)
	}
	return values
}

func isEmptyGateEvidence(evidence gate.Evidence) bool {
	return evidence.LintCurrent == nil &&
		evidence.LintCompare == nil &&
		evidence.EvalCurrent == nil &&
		evidence.EvalCompare == nil &&
		evidence.MultiBackendCurrent == nil &&
		evidence.MultiBackendCompare == nil
}

func intPointer(value int) *int {
	return &value
}

func floatPointer(value float64) *float64 {
	return &value
}

func derefLine(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
