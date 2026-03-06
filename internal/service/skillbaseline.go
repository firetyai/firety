package service

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/firety/firety/internal/domain/baseline"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type SkillBaselineSaveOptions struct {
	Profile           SkillLintProfile
	Strictness        lint.Strictness
	SuitePath         string
	Runner            string
	BackendSelections []SkillEvalBackendSelection
	InputArtifacts    []string
}

type SkillBaselineCompareOptions struct {
	BaselinePath      string
	Runner            string
	BackendSelections []SkillEvalBackendSelection
}

type SkillBaselineSaveResult struct {
	Snapshot baseline.Snapshot
}

type SkillBaselineCompareResult struct {
	Baseline   baseline.Snapshot
	Current    baseline.Snapshot
	Comparison baseline.Comparison
}

type SkillBaselineService struct {
	linter SkillLinter
	eval   SkillEvalService
}

func NewSkillBaselineService(linter SkillLinter, eval SkillEvalService) SkillBaselineService {
	return SkillBaselineService{
		linter: linter,
		eval:   eval,
	}
}

func (s SkillBaselineService) Save(target string, options SkillBaselineSaveOptions) (SkillBaselineSaveResult, error) {
	if len(options.InputArtifacts) > 0 {
		snapshot, err := loadBaselineSnapshotFromArtifacts(options.InputArtifacts)
		if err != nil {
			return SkillBaselineSaveResult{}, err
		}
		return SkillBaselineSaveResult{Snapshot: snapshot}, nil
	}

	lintReport, err := s.linter.LintWithProfileAndStrictness(target, options.Profile, options.Strictness)
	if err != nil {
		return SkillBaselineSaveResult{}, err
	}

	context := baseline.SnapshotContext{
		Target:     target,
		Profile:    string(options.Profile),
		Strictness: options.Strictness.DisplayName(),
	}

	if len(options.BackendSelections) > 0 {
		report, err := s.eval.EvaluateAcrossBackends(target, options.SuitePath, options.BackendSelections)
		if err != nil {
			return SkillBaselineSaveResult{}, err
		}
		context.SuitePath = report.Suite.Path
		context.Backends = baselineSelectionsFromService(options.BackendSelections)
		return SkillBaselineSaveResult{
			Snapshot: baseline.BuildSnapshot(context, lintReport, nil, &report),
		}, nil
	}

	if options.Runner != "" {
		report, err := s.eval.Evaluate(target, SkillEvalOptions{
			SuitePath: options.SuitePath,
			Profile:   options.Profile,
			Runner:    options.Runner,
		})
		if err != nil {
			return SkillBaselineSaveResult{}, err
		}
		context.SuitePath = report.Suite.Path
		context.Runner = options.Runner
		return SkillBaselineSaveResult{
			Snapshot: baseline.BuildSnapshot(context, lintReport, &report, nil),
		}, nil
	}

	return SkillBaselineSaveResult{
		Snapshot: baseline.BuildSnapshot(context, lintReport, nil, nil),
	}, nil
}

func (s SkillBaselineService) Compare(target string, options SkillBaselineCompareOptions) (SkillBaselineCompareResult, error) {
	snapshot, err := loadBaselineSnapshotArtifact(options.BaselinePath)
	if err != nil {
		return SkillBaselineCompareResult{}, err
	}

	strictness, err := lint.ParseStrictness(snapshot.Context.Strictness)
	if err != nil {
		return SkillBaselineCompareResult{}, fmt.Errorf("baseline strictness: %w", err)
	}
	profile, err := ParseSkillLintProfile(snapshot.Context.Profile)
	if err != nil {
		return SkillBaselineCompareResult{}, fmt.Errorf("baseline profile: %w", err)
	}

	currentLint, err := s.linter.LintWithProfileAndStrictness(target, profile, strictness)
	if err != nil {
		return SkillBaselineCompareResult{}, err
	}
	currentContext := snapshot.Context
	currentContext.Target = target

	var currentEval *domaineval.RoutingEvalReport
	var currentMulti *domaineval.MultiBackendEvalReport
	var evalComparison *domaineval.RoutingEvalComparison
	var multiComparison *domaineval.MultiBackendEvalComparison

	if snapshot.EvalReport != nil {
		runner := options.Runner
		if runner == "" {
			runner = snapshot.Context.Runner
		}
		if runner == "" {
			return SkillBaselineCompareResult{}, fmt.Errorf("baseline contains eval results but no runner is available; pass --runner to compare against this baseline")
		}

		report, err := s.eval.Evaluate(target, SkillEvalOptions{
			SuitePath: snapshot.Context.SuitePath,
			Profile:   profile,
			Runner:    runner,
		})
		if err != nil {
			return SkillBaselineCompareResult{}, err
		}
		comparison, err := domaineval.CompareReports(*snapshot.EvalReport, report)
		if err != nil {
			return SkillBaselineCompareResult{}, fmt.Errorf("compare current eval against baseline: %w", err)
		}
		currentEval = &report
		evalComparison = &comparison
		currentContext.SuitePath = report.Suite.Path
		currentContext.Runner = runner
	}

	if snapshot.MultiBackendEval != nil {
		selections := options.BackendSelections
		if len(selections) == 0 {
			selections = serviceSelectionsFromBaseline(snapshot.Context.Backends)
		}
		if len(selections) < 2 {
			return SkillBaselineCompareResult{}, fmt.Errorf("baseline contains multi-backend results but no backend runners are available; pass repeated --backend values to compare against this baseline")
		}

		report, err := s.eval.EvaluateAcrossBackends(target, snapshot.Context.SuitePath, selections)
		if err != nil {
			return SkillBaselineCompareResult{}, err
		}
		comparison, err := domaineval.CompareMultiBackendReports(*snapshot.MultiBackendEval, report)
		if err != nil {
			return SkillBaselineCompareResult{}, fmt.Errorf("compare current multi-backend eval against baseline: %w", err)
		}
		currentMulti = &report
		multiComparison = &comparison
		currentContext.SuitePath = report.Suite.Path
		currentContext.Backends = baselineSelectionsFromService(selections)
	}

	current := baseline.BuildSnapshot(currentContext, currentLint, currentEval, currentMulti)
	lintComparison := lint.CompareReports(snapshot.LintReport, currentLint)
	comparison := baseline.SummarizeComparison(current, snapshot, &lintComparison, evalComparison, multiComparison)

	return SkillBaselineCompareResult{
		Baseline:   snapshot,
		Current:    current,
		Comparison: comparison,
	}, nil
}

func loadBaselineSnapshotFromArtifacts(paths []string) (baseline.Snapshot, error) {
	var snapshot baseline.Snapshot
	var hasLint bool
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return baseline.Snapshot{}, err
		}

		var envelope struct {
			ArtifactType string `json:"artifact_type"`
		}
		if err := json.Unmarshal(content, &envelope); err != nil {
			return baseline.Snapshot{}, fmt.Errorf("parse artifact envelope: %w", err)
		}

		switch envelope.ArtifactType {
		case "firety.skill-lint":
			var value gateSkillLintArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return baseline.Snapshot{}, err
			}
			if hasLint {
				return baseline.Snapshot{}, fmt.Errorf("duplicate lint artifact in baseline input")
			}
			snapshot.Context.Target = value.Run.Target
			snapshot.Context.Profile = value.Run.Profile
			snapshot.Context.Strictness = value.Run.Strictness
			snapshot.LintReport = lint.Report{
				Target:   value.Run.Target,
				Findings: lintFindingsFromArtifact(value.Findings),
			}
			routingRisk := value.RoutingRisk
			if routingRisk == nil {
				summary := lint.SummarizeRoutingRisk(snapshot.LintReport.Findings)
				routingRisk = &summary
			}
			snapshot.RoutingRisk = *routingRisk
			hasLint = true
		case "firety.skill-analysis":
			var value gateSkillAnalysisArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return baseline.Snapshot{}, err
			}
			if hasLint {
				return baseline.Snapshot{}, fmt.Errorf("duplicate lint artifact in baseline input")
			}
			snapshot.Context.Target = value.Run.Target
			snapshot.Context.Profile = value.Run.Profile
			snapshot.Context.Strictness = value.Run.Strictness
			snapshot.LintReport = lint.Report{
				Target:   value.Run.Target,
				Findings: lintFindingsFromArtifact(value.Lint.Findings),
			}
			if value.Lint.RoutingRisk != nil {
				snapshot.RoutingRisk = *value.Lint.RoutingRisk
			} else {
				snapshot.RoutingRisk = lint.SummarizeRoutingRisk(snapshot.LintReport.Findings)
			}
			evalReport := domaineval.RoutingEvalReport{
				Target:  value.Run.Target,
				Suite:   value.Eval.Suite,
				Backend: value.Eval.Backend,
				Summary: value.Eval.Summary,
				Results: value.Eval.Results,
			}
			snapshot.EvalReport = &evalReport
			snapshot.Context.SuitePath = value.Eval.Suite.Path
			snapshot.Context.Runner = value.Run.Runner
			hasLint = true
		case "firety.skill-routing-eval":
			var value gateSkillEvalArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return baseline.Snapshot{}, err
			}
			report := domaineval.RoutingEvalReport{
				Target:  value.Run.Target,
				Suite:   value.Suite,
				Backend: value.Backend,
				Summary: value.Summary,
				Results: value.Results,
			}
			snapshot.EvalReport = &report
			snapshot.Context.SuitePath = value.Suite.Path
			snapshot.Context.Runner = value.Run.Runner
		case "firety.skill-routing-eval-multi":
			var value gateSkillEvalMultiArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return baseline.Snapshot{}, err
			}
			report := domaineval.MultiBackendEvalReport{
				Target:         value.Run.Target,
				Suite:          value.Suite,
				Backends:       value.Results,
				Summary:        value.Summary,
				DifferingCases: value.DifferingCases,
			}
			snapshot.MultiBackendEval = &report
			snapshot.Context.SuitePath = value.Suite.Path
			snapshot.Context.Backends = baselineSelectionsFromReports(value.Results)
		default:
			return baseline.Snapshot{}, fmt.Errorf("artifact %s has unsupported type %q for baseline snapshots", path, envelope.ArtifactType)
		}
	}

	if !hasLint {
		return baseline.Snapshot{}, fmt.Errorf("baseline snapshot requires lint evidence")
	}

	if snapshot.RoutingRisk.OverallRisk == "" {
		snapshot.RoutingRisk = lint.SummarizeRoutingRisk(snapshot.LintReport.Findings)
	}
	if snapshot.Context.Profile == "" {
		snapshot.Context.Profile = string(SkillLintProfileGeneric)
	}
	if snapshot.Context.Strictness == "" {
		snapshot.Context.Strictness = lint.StrictnessDefault.DisplayName()
	}
	snapshot.Summary = baseline.BuildSnapshot(snapshot.Context, snapshot.LintReport, snapshot.EvalReport, snapshot.MultiBackendEval).Summary
	return snapshot, nil
}

func loadBaselineSnapshotArtifact(path string) (baseline.Snapshot, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return baseline.Snapshot{}, err
	}

	var payload struct {
		ArtifactType string            `json:"artifact_type"`
		Snapshot     baseline.Snapshot `json:"snapshot"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return baseline.Snapshot{}, fmt.Errorf("parse baseline snapshot artifact: %w", err)
	}
	if payload.ArtifactType != "firety.skill-baseline" {
		return baseline.Snapshot{}, fmt.Errorf("artifact %s has unsupported type %q for baseline loading", path, payload.ArtifactType)
	}
	return payload.Snapshot, nil
}

func baselineSelectionsFromService(values []SkillEvalBackendSelection) []baseline.BackendSelection {
	out := make([]baseline.BackendSelection, 0, len(values))
	for _, value := range values {
		out = append(out, baseline.BackendSelection{
			ID:     value.ID,
			Runner: value.Runner,
		})
	}
	return out
}

func serviceSelectionsFromBaseline(values []baseline.BackendSelection) []SkillEvalBackendSelection {
	out := make([]SkillEvalBackendSelection, 0, len(values))
	for _, value := range values {
		out = append(out, SkillEvalBackendSelection{
			ID:     value.ID,
			Runner: value.Runner,
		})
	}
	return out
}

func baselineSelectionsFromReports(values []domaineval.BackendEvalReport) []baseline.BackendSelection {
	out := make([]baseline.BackendSelection, 0, len(values))
	for _, value := range values {
		out = append(out, baseline.BackendSelection{
			ID: value.Backend.ID,
		})
	}
	slices.SortStableFunc(out, func(left, right baseline.BackendSelection) int {
		if left.ID < right.ID {
			return -1
		}
		if left.ID > right.ID {
			return 1
		}
		return 0
	})
	return out
}

func lintFindingsFromArtifact(findings []gateSkillLintArtifactFinding) []lint.Finding {
	out := make([]lint.Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, lint.Finding{
			RuleID:   finding.RuleID,
			Severity: lint.Severity(finding.Severity),
			Path:     finding.Path,
			Line:     derefLine(finding.Line),
			Message:  finding.Message,
		})
	}
	return out
}
