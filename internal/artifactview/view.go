package artifactview

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/firety/firety/internal/artifact"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/render"
)

type Inspection struct {
	Path                 string   `json:"path"`
	ArtifactType         string   `json:"artifact_type"`
	SchemaVersion        string   `json:"schema_version"`
	Origin               string   `json:"origin"`
	Target               string   `json:"target,omitempty"`
	BaseTarget           string   `json:"base_target,omitempty"`
	CandidateTarget      string   `json:"candidate_target,omitempty"`
	Summary              string   `json:"summary,omitempty"`
	Context              []string `json:"context,omitempty"`
	SupportedRenderModes []string `json:"supported_render_modes,omitempty"`
	ComparableTo         []string `json:"comparable_to,omitempty"`
}

type CompareResult struct {
	SchemaVersion           string                                 `json:"schema_version"`
	ArtifactType            string                                 `json:"artifact_type"`
	BasePath                string                                 `json:"base_path"`
	CandidatePath           string                                 `json:"candidate_path"`
	Overall                 string                                 `json:"overall"`
	Summary                 string                                 `json:"summary"`
	HighPriorityRegressions []string                               `json:"high_priority_regressions,omitempty"`
	NotableImprovements     []string                               `json:"notable_improvements,omitempty"`
	LintComparison          *lint.ReportComparison                 `json:"lint_comparison,omitempty"`
	EvalComparison          *domaineval.RoutingEvalComparison      `json:"eval_comparison,omitempty"`
	MultiBackendComparison  *domaineval.MultiBackendEvalComparison `json:"multi_backend_comparison,omitempty"`
}

type envelope struct {
	ArtifactType  string `json:"artifact_type"`
	SchemaVersion string `json:"schema_version"`
}

type loadedArtifact struct {
	Path    string
	Inspect Inspection
	Value   any
}

func Inspect(path string) (Inspection, error) {
	loaded, err := load(path)
	if err != nil {
		return Inspection{}, err
	}
	return loaded.Inspect, nil
}

func Render(path string, mode render.Mode) (string, error) {
	loaded, err := load(path)
	if err != nil {
		return "", err
	}
	if len(loaded.Inspect.SupportedRenderModes) == 0 {
		return "", fmt.Errorf("artifact type %q cannot be rendered", loaded.Inspect.ArtifactType)
	}
	return render.RenderArtifact(path, mode)
}

func Compare(basePath, candidatePath string) (CompareResult, error) {
	base, err := load(basePath)
	if err != nil {
		return CompareResult{}, err
	}
	candidate, err := load(candidatePath)
	if err != nil {
		return CompareResult{}, err
	}
	if base.Inspect.ArtifactType != candidate.Inspect.ArtifactType {
		return CompareResult{}, fmt.Errorf("artifact types are incompatible for compare: %q vs %q", base.Inspect.ArtifactType, candidate.Inspect.ArtifactType)
	}
	if !slices.Contains(base.Inspect.ComparableTo, candidate.Inspect.ArtifactType) {
		return CompareResult{}, fmt.Errorf("artifact type %q does not support artifact-to-artifact compare", base.Inspect.ArtifactType)
	}

	switch base.Inspect.ArtifactType {
	case "firety.skill-lint":
		baseValue := base.Value.(artifact.SkillLintArtifact)
		candidateValue := candidate.Value.(artifact.SkillLintArtifact)
		comparison := lint.CompareReports(lintReportFromArtifact(baseValue), lintReportFromArtifact(candidateValue))
		return CompareResult{
			SchemaVersion:           "1",
			ArtifactType:            base.Inspect.ArtifactType,
			BasePath:                basePath,
			CandidatePath:           candidatePath,
			Overall:                 string(comparison.Summary.Overall),
			Summary:                 comparison.Summary.Summary,
			HighPriorityRegressions: comparison.Summary.HighPriorityRegressions,
			NotableImprovements:     comparison.Summary.NotableImprovements,
			LintComparison:          &comparison,
		}, nil
	case "firety.skill-routing-eval":
		baseValue := base.Value.(artifact.SkillEvalArtifact)
		candidateValue := candidate.Value.(artifact.SkillEvalArtifact)
		comparison, err := domaineval.CompareReports(evalReportFromArtifact(baseValue), evalReportFromArtifact(candidateValue))
		if err != nil {
			return CompareResult{}, err
		}
		return CompareResult{
			SchemaVersion:           "1",
			ArtifactType:            base.Inspect.ArtifactType,
			BasePath:                basePath,
			CandidatePath:           candidatePath,
			Overall:                 string(comparison.Summary.Overall),
			Summary:                 comparison.Summary.Summary,
			HighPriorityRegressions: comparison.Summary.HighPriorityRegressions,
			NotableImprovements:     comparison.Summary.NotableImprovements,
			EvalComparison:          &comparison,
		}, nil
	case "firety.skill-routing-eval-multi":
		baseValue := base.Value.(artifact.SkillEvalMultiArtifact)
		candidateValue := candidate.Value.(artifact.SkillEvalMultiArtifact)
		comparison, err := domaineval.CompareMultiBackendReports(multiReportFromArtifact(baseValue), multiReportFromArtifact(candidateValue))
		if err != nil {
			return CompareResult{}, err
		}
		return CompareResult{
			SchemaVersion:           "1",
			ArtifactType:            base.Inspect.ArtifactType,
			BasePath:                basePath,
			CandidatePath:           candidatePath,
			Overall:                 string(comparison.AggregateSummary.Overall),
			Summary:                 comparison.AggregateSummary.Summary,
			HighPriorityRegressions: comparison.AggregateSummary.HighPriorityRegressions,
			NotableImprovements:     comparison.AggregateSummary.NotableImprovements,
			MultiBackendComparison:  &comparison,
		}, nil
	default:
		return CompareResult{}, fmt.Errorf("artifact type %q does not support artifact-to-artifact compare", base.Inspect.ArtifactType)
	}
}

func load(path string) (loadedArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return loadedArtifact{}, err
	}

	var header envelope
	if err := json.Unmarshal(data, &header); err != nil {
		return loadedArtifact{}, fmt.Errorf("parse artifact envelope: %w", err)
	}
	if header.ArtifactType == "" {
		return loadedArtifact{}, fmt.Errorf("artifact %s is missing artifact_type", path)
	}
	if header.SchemaVersion != "1" {
		return loadedArtifact{}, fmt.Errorf("artifact %s has unsupported schema version %q", path, header.SchemaVersion)
	}

	switch header.ArtifactType {
	case "firety.skill-attestation":
		var value artifact.SkillAttestationArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillAttestation(path, value)}, nil
	case "firety.skill-lint":
		var value artifact.SkillLintArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillLint(path, value)}, nil
	case "firety.skill-lint-compare":
		var value artifact.SkillLintCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillLintCompare(path, value)}, nil
	case "firety.skill-routing-eval":
		var value artifact.SkillEvalArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillEval(path, value)}, nil
	case "firety.skill-routing-eval-compare":
		var value artifact.SkillEvalCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillEvalCompare(path, value)}, nil
	case "firety.skill-routing-eval-multi":
		var value artifact.SkillEvalMultiArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillEvalMulti(path, value)}, nil
	case "firety.skill-routing-eval-compare-multi":
		var value artifact.SkillEvalMultiCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillEvalMultiCompare(path, value)}, nil
	case "firety.skill-analysis":
		var value artifact.SkillAnalysisArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillAnalysis(path, value)}, nil
	case "firety.skill-improvement-plan":
		var value artifact.SkillPlanArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillPlan(path, value)}, nil
	case "firety.skill-quality-gate":
		var value artifact.SkillGateArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillGate(path, value)}, nil
	case "firety.skill-baseline":
		var value artifact.SkillBaselineSnapshotArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillBaseline(path, value)}, nil
	case "firety.skill-baseline-compare":
		var value artifact.SkillBaselineCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillBaselineCompare(path, value)}, nil
	case "firety.skill-compatibility":
		var value artifact.SkillCompatibilityArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectSkillCompatibility(path, value)}, nil
	case "firety.benchmark-report":
		var value artifact.BenchmarkArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return loadedArtifact{}, err
		}
		return loadedArtifact{Path: path, Value: value, Inspect: inspectBenchmark(path, value)}, nil
	default:
		return loadedArtifact{}, fmt.Errorf("unsupported artifact type %q", header.ArtifactType)
	}
}

func inspectSkillAttestation(path string, value artifact.SkillAttestationArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill attest",
		Target:               firstNonEmpty(value.Run.Target, value.Attestation.Target),
		Summary:              value.Attestation.Summary,
		Context:              nonEmptyStrings(fmt.Sprintf("support posture %s", value.Attestation.SupportPosture), fmt.Sprintf("evidence %s", value.Attestation.EvidenceLevel)),
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillLint(path string, value artifact.SkillLintArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill lint",
		Target:               value.Run.Target,
		Summary:              fmt.Sprintf("%d error(s), %d warning(s), %d finding(s).", value.Summary.ErrorCount, value.Summary.WarningCount, value.Summary.FindingCount),
		Context:              nonEmptyStrings(fmt.Sprintf("profile %s", value.Run.Profile), fmt.Sprintf("strictness %s", value.Run.Strictness)),
		SupportedRenderModes: renderModes(),
		ComparableTo:         []string{"firety.skill-lint"},
	}
}

func inspectSkillLintCompare(path string, value artifact.SkillLintCompareArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill compare",
		BaseTarget:           value.Run.BaseTarget,
		CandidateTarget:      value.Run.CandidateTarget,
		Summary:              value.Comparison.Summary,
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillEval(path string, value artifact.SkillEvalArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill eval",
		Target:               value.Run.Target,
		Summary:              fmt.Sprintf("%d passed, %d failed, %.0f%% pass rate.", value.Summary.Passed, value.Summary.Failed, value.Summary.PassRate*100),
		Context:              nonEmptyStrings(fmt.Sprintf("backend %s", value.Backend.Name), fmt.Sprintf("profile %s", value.Run.Profile), fmt.Sprintf("suite %s", value.Suite.Name)),
		SupportedRenderModes: renderModes(),
		ComparableTo:         []string{"firety.skill-routing-eval"},
	}
}

func inspectSkillEvalCompare(path string, value artifact.SkillEvalCompareArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill eval-compare",
		BaseTarget:           value.Run.BaseTarget,
		CandidateTarget:      value.Run.CandidateTarget,
		Summary:              value.Comparison.Summary,
		Context:              nonEmptyStrings(fmt.Sprintf("backend %s", value.Backend.Name), fmt.Sprintf("suite %s", value.Suite.Name)),
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillEvalMulti(path string, value artifact.SkillEvalMultiArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill eval",
		Target:               value.Run.Target,
		Summary:              value.Summary.Summary,
		Context:              []string{fmt.Sprintf("%d backend(s)", value.Summary.BackendCount), fmt.Sprintf("suite %s", value.Suite.Name)},
		SupportedRenderModes: renderModes(),
		ComparableTo:         []string{"firety.skill-routing-eval-multi"},
	}
}

func inspectSkillEvalMultiCompare(path string, value artifact.SkillEvalMultiCompareArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill eval-compare",
		BaseTarget:           value.Run.BaseTarget,
		CandidateTarget:      value.Run.CandidateTarget,
		Summary:              value.AggregateSummary.Summary,
		Context:              []string{fmt.Sprintf("%d backend(s)", len(value.Backends)), fmt.Sprintf("suite %s", value.Suite.Name)},
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillAnalysis(path string, value artifact.SkillAnalysisArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill analyze",
		Target:               value.Run.Target,
		Summary:              value.Correlation.Summary,
		Context:              nonEmptyStrings(fmt.Sprintf("profile %s", value.Run.Profile), fmt.Sprintf("strictness %s", value.Run.Strictness), fmt.Sprintf("suite %s", value.Eval.Suite.Name)),
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillPlan(path string, value artifact.SkillPlanArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill plan",
		Target:               value.Run.Target,
		Summary:              value.Plan.Summary,
		Context:              nonEmptyStrings(fmt.Sprintf("profile %s", value.Run.Profile), fmt.Sprintf("strictness %s", value.Run.Strictness)),
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillGate(path string, value artifact.SkillGateArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill gate",
		Target:               value.Run.Target,
		BaseTarget:           firstNonEmpty(value.Run.BaseTarget, value.Run.BaselinePath),
		Summary:              value.Result.Summary,
		Context:              nonEmptyStrings(fmt.Sprintf("profile %s", value.Run.Profile), fmt.Sprintf("strictness %s", value.Run.Strictness)),
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillBaseline(path string, value artifact.SkillBaselineSnapshotArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill baseline save",
		Target:               value.Snapshot.Context.Target,
		Summary:              value.Snapshot.Summary.Summary,
		Context:              nonEmptyStrings(fmt.Sprintf("profile %s", value.Snapshot.Context.Profile), fmt.Sprintf("strictness %s", value.Snapshot.Context.Strictness)),
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillBaselineCompare(path string, value artifact.SkillBaselineCompareArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill baseline compare",
		BaseTarget:           value.Comparison.BaselineTarget,
		CandidateTarget:      value.Comparison.CurrentTarget,
		Summary:              value.Comparison.Summary.Summary,
		SupportedRenderModes: renderModes(),
	}
}

func inspectSkillCompatibility(path string, value artifact.SkillCompatibilityArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety skill compatibility",
		Target:               firstNonEmpty(value.Run.Target, value.Report.Target),
		Summary:              value.Report.Summary,
		Context:              nonEmptyStrings(fmt.Sprintf("support posture %s", value.Report.SupportPosture), fmt.Sprintf("evidence %s", value.Report.EvidenceLevel)),
		SupportedRenderModes: renderModes(),
	}
}

func inspectBenchmark(path string, value artifact.BenchmarkArtifact) Inspection {
	return Inspection{
		Path:                 path,
		ArtifactType:         value.ArtifactType,
		SchemaVersion:        value.SchemaVersion,
		Origin:               "firety benchmark run",
		Summary:              value.Summary.Summary,
		Context:              []string{fmt.Sprintf("suite %s", value.Suite.Name), fmt.Sprintf("%d fixture(s)", value.Suite.FixtureCount)},
		SupportedRenderModes: renderModes(),
	}
}

func lintReportFromArtifact(value artifact.SkillLintArtifact) lint.Report {
	findings := make([]lint.Finding, 0, len(value.Findings))
	for _, item := range value.Findings {
		line := 0
		if item.Line != nil {
			line = *item.Line
		}
		findings = append(findings, lint.Finding{
			RuleID:   item.RuleID,
			Severity: lint.Severity(item.Severity),
			Path:     item.Path,
			Line:     line,
			Message:  item.Message,
		})
	}
	return lint.Report{
		Target:   value.Run.Target,
		Findings: findings,
	}
}

func evalReportFromArtifact(value artifact.SkillEvalArtifact) domaineval.RoutingEvalReport {
	return domaineval.RoutingEvalReport{
		Target:  value.Run.Target,
		Suite:   value.Suite,
		Backend: value.Backend,
		Summary: value.Summary,
		Results: value.Results,
	}
}

func multiReportFromArtifact(value artifact.SkillEvalMultiArtifact) domaineval.MultiBackendEvalReport {
	return domaineval.MultiBackendEvalReport{
		Target:         value.Run.Target,
		Suite:          value.Suite,
		Backends:       value.Results,
		Summary:        value.Summary,
		DifferingCases: value.DifferingCases,
	}
}

func renderModes() []string {
	return []string{string(render.ModePRComment), string(render.ModeCISummary), string(render.ModeFullReport)}
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
