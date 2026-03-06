package render

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/benchmark"
	"github.com/firety/firety/internal/domain/analysis"
	"github.com/firety/firety/internal/domain/attestation"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/gate"
	workspacepkg "github.com/firety/firety/internal/domain/workspace"
)

type Mode string

const (
	ModePRComment  Mode = "pr-comment"
	ModeCISummary  Mode = "ci-summary"
	ModeFullReport Mode = "full-report"
)

type artifactEnvelope struct {
	ArtifactType string `json:"artifact_type"`
}

func ParseMode(raw string) (Mode, error) {
	switch Mode(raw) {
	case ModePRComment, ModeCISummary, ModeFullReport:
		return Mode(raw), nil
	default:
		return "", fmt.Errorf("invalid render mode %q: must be one of %s, %s, %s", raw, ModePRComment, ModeCISummary, ModeFullReport)
	}
}

func RenderArtifact(path string, mode Mode) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var envelope artifactEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("parse artifact envelope: %w", err)
	}

	switch envelope.ArtifactType {
	case "firety.skill-attestation":
		var value artifact.SkillAttestationArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderAttestationArtifact(path, value, mode), nil
	case "firety.skill-compatibility":
		var value artifact.SkillCompatibilityArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderCompatibilityArtifact(path, value, mode), nil
	case "firety.skill-readiness":
		var value artifact.SkillReadinessArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderReadinessArtifact(path, value, mode), nil
	case "firety.workspace-report":
		var value artifact.WorkspaceReportArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderWorkspaceReportArtifact(path, value, mode), nil
	case "firety.workspace-change-scope":
		var value artifact.WorkspaceChangeScopeArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderWorkspaceChangeScopeArtifact(path, value, mode), nil
	case "firety.skill-baseline":
		var value artifact.SkillBaselineSnapshotArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderBaselineSnapshotArtifact(path, value, mode), nil
	case "firety.skill-baseline-compare":
		var value artifact.SkillBaselineCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderBaselineCompareArtifact(path, value, mode), nil
	case "firety.skill-quality-gate":
		var value artifact.SkillGateArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderGateArtifact(path, value, mode), nil
	case "firety.benchmark-report":
		var value artifact.BenchmarkArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderBenchmarkArtifact(path, value, mode), nil
	case "firety.skill-improvement-plan":
		var value artifact.SkillPlanArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderPlanArtifact(path, value, mode), nil
	case "firety.skill-analysis":
		var value artifact.SkillAnalysisArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderAnalysisArtifact(path, value, mode), nil
	case "firety.skill-lint-compare":
		var value artifact.SkillLintCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderLintCompareArtifact(path, value, mode), nil
	case "firety.skill-routing-eval-compare":
		var value artifact.SkillEvalCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderEvalCompareArtifact(path, value, mode), nil
	case "firety.skill-routing-eval-compare-multi":
		var value artifact.SkillEvalMultiCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderEvalMultiCompareArtifact(path, value, mode), nil
	case "firety.skill-routing-eval-multi":
		var value artifact.SkillEvalMultiArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderEvalMultiArtifact(path, value, mode), nil
	case "firety.skill-lint":
		var value artifact.SkillLintArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderLintArtifact(path, value, mode), nil
	case "firety.skill-routing-eval":
		var value artifact.SkillEvalArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return renderEvalArtifact(path, value, mode), nil
	default:
		return "", fmt.Errorf("unsupported artifact type %q", envelope.ArtifactType)
	}
}

func renderAttestationArtifact(path string, value artifact.SkillAttestationArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Skill Attestation")
	writeLine(&b, fmt.Sprintf("Support posture: %s", string(value.Attestation.SupportPosture)))
	writeLine(&b, fmt.Sprintf("Evidence: %s", string(value.Attestation.EvidenceLevel)))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Attestation.Summary))
	if value.Attestation.QualityGate != nil {
		writeLine(&b, fmt.Sprintf("Quality gate: %s", strings.ToUpper(value.Attestation.QualityGate.Decision)))
	}
	if len(value.Attestation.Claims) > 0 {
		writeSectionHeader(&b, mode, "Claims")
		for _, item := range firstClaimStatements(value.Attestation.Claims, 3) {
			writeBullet(&b, item)
		}
	}
	if len(value.Attestation.Limitations) > 0 && mode != ModePRComment {
		writeSectionHeader(&b, mode, "Known limitations")
		for _, item := range firstStrings(value.Attestation.Limitations, 3) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		if len(value.Attestation.RecommendedConsumerReadingOrder) > 0 {
			writeSectionHeader(&b, mode, "Read next")
			for _, item := range value.Attestation.RecommendedConsumerReadingOrder {
				writeBullet(&b, item)
			}
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func firstClaimStatements(claims []attestation.Claim, limit int) []string {
	values := make([]string, 0, min(len(claims), limit))
	for _, claim := range claims {
		values = append(values, claim.Statement)
		if len(values) == limit {
			break
		}
	}
	return values
}

func renderCompatibilityArtifact(path string, value artifact.SkillCompatibilityArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Compatibility")
	writeLine(&b, fmt.Sprintf("Support posture: %s", string(value.Report.SupportPosture)))
	writeLine(&b, fmt.Sprintf("Evidence: %s", string(value.Report.EvidenceLevel)))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Report.Summary))
	if len(value.Report.Blockers) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstStrings(value.Report.Blockers, 3) {
			writeBullet(&b, item)
		}
	}
	if mode != ModePRComment && len(value.Report.Strengths) > 0 {
		writeSectionHeader(&b, mode, "Strengths")
		for _, item := range firstStrings(value.Report.Strengths, 3) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		if len(value.Report.Profiles) > 0 {
			writeSectionHeader(&b, mode, "Profiles")
			for _, profile := range value.Report.Profiles {
				writeBullet(&b, fmt.Sprintf("%s: %s", profile.DisplayName, profile.Summary))
			}
		}
		if len(value.Report.Backends) > 0 {
			writeSectionHeader(&b, mode, "Backends")
			for _, backend := range value.Report.Backends {
				writeBullet(&b, fmt.Sprintf("%s: %s", backend.BackendName, backend.Summary))
			}
		}
		if value.Report.RecommendedPositioning != "" {
			writeSectionHeader(&b, mode, "Positioning")
			writeBullet(&b, value.Report.RecommendedPositioning)
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderReadinessArtifact(path string, value artifact.SkillReadinessArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Readiness")
	writeLine(&b, fmt.Sprintf("Decision: %s", strings.ToUpper(string(value.Readiness.Decision))))
	writeLine(&b, fmt.Sprintf("Context: %s", value.Readiness.PublishContext))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Readiness.Summary))
	if len(value.Readiness.Blockers) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range value.Readiness.Blockers[:min(len(value.Readiness.Blockers), 3)] {
			writeBullet(&b, item.Summary)
		}
	}
	if mode != ModePRComment && len(value.Readiness.Caveats) > 0 {
		writeSectionHeader(&b, mode, "Caveats")
		for _, item := range value.Readiness.Caveats[:min(len(value.Readiness.Caveats), 3)] {
			writeBullet(&b, item.Summary)
		}
	}
	if mode == ModeFullReport {
		if len(value.Readiness.RecommendedActions) > 0 {
			writeSectionHeader(&b, mode, "Next actions")
			for _, item := range value.Readiness.RecommendedActions {
				writeBullet(&b, item)
			}
		}
		writeSectionHeader(&b, mode, "Publish surfaces")
		writeBullet(&b, fmt.Sprintf("Attestation: %s", value.Readiness.AttestationSuitability.Summary))
		writeBullet(&b, fmt.Sprintf("Trust report: %s", value.Readiness.TrustReportSuitability.Summary))
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderWorkspaceReportArtifact(path string, value artifact.WorkspaceReportArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Workspace Report")
	writeLine(&b, fmt.Sprintf("Workspace: %s", value.Report.WorkspaceRoot))
	writeLine(&b, fmt.Sprintf("Skills: %d", value.Report.Summary.SkillCount))
	writeLine(&b, fmt.Sprintf("Lint totals: %d error(s), %d warning(s)", value.Report.Summary.TotalLintErrors, value.Report.Summary.TotalLintWarnings))
	if value.Report.Gate != nil {
		writeLine(&b, fmt.Sprintf("Workspace gate: %s", strings.ToUpper(string(value.Report.Gate.Decision))))
	}
	writeLine(&b, fmt.Sprintf("Readiness: ready=%d, caveats=%d, not-ready=%d, insufficient=%d",
		value.Report.Summary.ReadySkills,
		value.Report.Summary.ReadyWithCaveatsSkills,
		value.Report.Summary.NotReadySkills,
		value.Report.Summary.InsufficientEvidenceSkills,
	))
	if len(value.Report.Summary.WorkspaceBlockers) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstStrings(value.Report.Summary.WorkspaceBlockers, 3) {
			writeBullet(&b, item)
		}
	}
	if mode != ModePRComment && len(value.Report.Summary.TopPriorities) > 0 {
		writeSectionHeader(&b, mode, "Top priorities")
		for _, item := range firstStrings(value.Report.Summary.TopPriorities, 5) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		writeSectionHeader(&b, mode, "Per skill")
		for _, skill := range value.Report.Skills {
			summary := skill.Lint.Summary
			if skill.Readiness != nil {
				summary = fmt.Sprintf("%s; readiness=%s", summary, skill.Readiness.Decision)
			}
			writeBullet(&b, fmt.Sprintf("%s: %s", skill.Skill.Name, summary))
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderWorkspaceChangeScopeArtifact(path string, value artifact.WorkspaceChangeScopeArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Workspace Change Scope")
	writeLine(&b, fmt.Sprintf("Workspace: %s", value.Scope.WorkspaceRoot))
	writeLine(&b, fmt.Sprintf("Diff: %s", value.Scope.DiffContext.Summary))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Scope.Summary))
	if len(value.Scope.DirectlyChangedSkills) > 0 {
		writeSectionHeader(&b, mode, "Directly changed")
		for _, skill := range firstSkillNames(value.Scope.DirectlyChangedSkills, 5) {
			writeBullet(&b, skill)
		}
	}
	if len(value.Scope.ImpactedSkills) > 0 && mode != ModePRComment {
		writeSectionHeader(&b, mode, "Impacted")
		for _, skill := range firstSkillNames(value.Scope.ImpactedSkills, 5) {
			writeBullet(&b, skill)
		}
	}
	if len(value.Scope.Caveats) > 0 && mode == ModeFullReport {
		writeSectionHeader(&b, mode, "Caveats")
		for _, caveat := range firstStrings(value.Scope.Caveats, 5) {
			writeBullet(&b, caveat)
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func firstSkillNames(skills []workspacepkg.SkillRef, limit int) []string {
	values := make([]string, 0, min(len(skills), limit))
	for _, skill := range skills {
		values = append(values, skill.Name)
		if len(values) == limit {
			break
		}
	}
	return values
}

func renderBaselineSnapshotArtifact(path string, value artifact.SkillBaselineSnapshotArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Baseline Snapshot")
	writeLine(&b, fmt.Sprintf("Target: %s", value.Snapshot.Context.Target))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Snapshot.Summary.Summary))
	writeLine(&b, fmt.Sprintf("Scope: %s", value.Snapshot.Summary.Scope))
	if value.Snapshot.Context.Profile != "" || value.Snapshot.Context.Strictness != "" {
		writeLine(&b, fmt.Sprintf("Context: profile %s, strictness %s", emptyDefault(value.Snapshot.Context.Profile, "generic"), emptyDefault(value.Snapshot.Context.Strictness, "default")))
	}
	if mode == ModeFullReport {
		if value.Snapshot.Context.SuitePath != "" {
			writeLine(&b, fmt.Sprintf("Suite: %s", value.Snapshot.Context.SuitePath))
		}
		if len(value.Snapshot.Context.Backends) > 0 {
			writeSectionHeader(&b, mode, "Backends")
			for _, backend := range value.Snapshot.Context.Backends {
				label := backend.ID
				if backend.Runner != "" {
					label = fmt.Sprintf("%s (%s)", backend.ID, backend.Runner)
				}
				writeBullet(&b, label)
			}
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderBaselineCompareArtifact(path string, value artifact.SkillBaselineCompareArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Baseline Compare")
	writeLine(&b, fmt.Sprintf("Status: %s", compareStatus(string(value.Comparison.Summary.Overall))))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Comparison.Summary.Summary))
	if len(value.Comparison.Summary.HighPriorityRegressions) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstStrings(value.Comparison.Summary.HighPriorityRegressions, 3) {
			writeBullet(&b, item)
		}
	}
	if len(value.Comparison.Summary.NotableImprovements) > 0 && mode != ModePRComment {
		writeSectionHeader(&b, mode, "Improvements")
		for _, item := range firstStrings(value.Comparison.Summary.NotableImprovements, 3) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		writeSectionHeader(&b, mode, "Components")
		for _, component := range value.Comparison.Summary.Components {
			writeBullet(&b, fmt.Sprintf("%s: %s", component.Title, component.Summary))
		}
		if value.Comparison.Summary.UpdateRecommendation != "" {
			writeSectionHeader(&b, mode, "Recommendation")
			writeBullet(&b, value.Comparison.Summary.UpdateRecommendation)
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderBenchmarkArtifact(path string, value artifact.BenchmarkArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Benchmark Health")
	writeLine(&b, fmt.Sprintf("Status: %s", benchmarkStatusLabel(value)))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Summary.Summary))
	writeLine(&b, fmt.Sprintf("Suite: %s v%s (%d fixture(s))", value.Suite.Name, value.Suite.Version, value.Suite.FixtureCount))
	if mode != ModePRComment {
		writeLine(&b, fmt.Sprintf("Counts: %d passed, %d failed", value.Summary.PassedFixtures, value.Summary.FailedFixtures))
	}
	if len(value.Summary.NotableRegressions) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstStrings(value.Summary.NotableRegressions, 4) {
			writeBullet(&b, item)
		}
	}
	if mode != ModePRComment && len(value.Categories) > 0 {
		writeSectionHeader(&b, mode, "Category overview")
		for _, item := range firstBenchmarkCategorySummaries(value.Categories, 5) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		if len(value.Summary.ConfidenceSignals) > 0 {
			writeSectionHeader(&b, mode, "Confidence signals")
			for _, item := range firstStrings(value.Summary.ConfidenceSignals, 4) {
				writeBullet(&b, item)
			}
		}
		if len(value.Summary.NotableNoise) > 0 {
			writeSectionHeader(&b, mode, "Noisy areas")
			for _, item := range firstStrings(value.Summary.NotableNoise, 4) {
				writeBullet(&b, item)
			}
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderGateArtifact(path string, value artifact.SkillGateArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Quality Gate")
	writeLine(&b, fmt.Sprintf("Status: %s", strings.ToUpper(string(value.Result.Decision))))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Result.Summary))
	if len(value.Result.BlockingReasons) > 0 {
		writeSectionHeader(&b, mode, "Blocking reasons")
		for _, item := range firstGateReasonSummaries(value.Result.BlockingReasons, 4) {
			writeBullet(&b, item)
		}
	}
	if len(value.Result.Warnings) > 0 && mode != ModePRComment {
		writeSectionHeader(&b, mode, "Warnings")
		for _, item := range firstGateReasonSummaries(value.Result.Warnings, 3) {
			writeBullet(&b, item)
		}
	}
	if value.Result.NextAction != "" {
		writeSectionHeader(&b, mode, "Next action")
		writeBullet(&b, value.Result.NextAction)
	}
	if mode == ModeFullReport {
		if value.Run.BaselinePath != "" {
			writeLine(&b, fmt.Sprintf("Baseline: %s", value.Run.BaselinePath))
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderPlanArtifact(path string, value artifact.SkillPlanArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Skill Quality")
	writeLine(&b, fmt.Sprintf("Status: %s", statusLabel(value.Run.ExitCode)))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Plan.Summary))
	if mode != ModePRComment {
		writeLine(&b, fmt.Sprintf("Lint: %d error(s), %d warning(s)", value.LintSummary.ErrorCount, value.LintSummary.WarningCount))
		if value.EvalSummary != nil {
			writeLine(&b, fmt.Sprintf("Eval: %d passed, %d failed, %d false positive(s), %d false negative(s)", value.EvalSummary.Passed, value.EvalSummary.Failed, value.EvalSummary.FalsePositives, value.EvalSummary.FalseNegatives))
		}
		if value.MultiBackendEval != nil {
			writeLine(&b, fmt.Sprintf("Backends: %d backend(s), %d differing case(s)", value.MultiBackendEval.BackendCount, value.MultiBackendEval.DifferingCaseCount))
		}
	}
	writeSectionHeader(&b, mode, "Review first")
	writePlanItems(&b, value.Plan, mode)
	if mode != ModePRComment {
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderAnalysisArtifact(path string, value artifact.SkillAnalysisArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Skill Analysis")
	writeLine(&b, fmt.Sprintf("Status: %s", statusLabel(value.Run.ExitCode)))
	writeLine(&b, fmt.Sprintf("Lint: %d error(s), %d warning(s)", value.Lint.Summary.ErrorCount, value.Lint.Summary.WarningCount))
	writeLine(&b, fmt.Sprintf("Eval: %d passed, %d failed, %d false positive(s), %d false negative(s)", value.Eval.Summary.Passed, value.Eval.Summary.Failed, value.Eval.Summary.FalsePositives, value.Eval.Summary.FalseNegatives))
	if value.Correlation.Summary != "" {
		writeLine(&b, fmt.Sprintf("Correlation: %s", value.Correlation.Summary))
	}
	if value.Lint.RoutingRisk != nil {
		writeLine(&b, fmt.Sprintf("Routing risk: %s", strings.ToUpper(string(value.Lint.RoutingRisk.OverallRisk))))
	}
	if len(value.Correlation.PriorityActions) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstStrings(value.Correlation.PriorityActions, 3) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		writeSectionHeader(&b, mode, "Likely contributors")
		for _, group := range value.Correlation.MissGroups {
			writeBullet(&b, fmt.Sprintf("%s: %s", group.Title, group.Summary))
		}
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderLintCompareArtifact(path string, value artifact.SkillLintCompareArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Skill Compare")
	writeLine(&b, fmt.Sprintf("Status: %s", compareStatus(string(value.Comparison.Overall))))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Comparison.Summary))
	writeLine(&b, fmt.Sprintf("Candidate: %d error(s), %d warning(s)", value.Candidate.ErrorCount, value.Candidate.WarningCount))
	if len(value.Comparison.HighPriorityRegressions) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstStrings(value.Comparison.HighPriorityRegressions, 3) {
			writeBullet(&b, item)
		}
	}
	if len(value.Comparison.NotableImprovements) > 0 && mode != ModePRComment {
		writeSectionHeader(&b, mode, "Improvements")
		for _, item := range firstStrings(value.Comparison.NotableImprovements, 3) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderEvalCompareArtifact(path string, value artifact.SkillEvalCompareArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Routing Eval Compare")
	writeLine(&b, fmt.Sprintf("Status: %s", compareStatus(string(value.Comparison.Overall))))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Comparison.Summary))
	writeLine(&b, fmt.Sprintf("Pass rate delta: %+0.fpp", value.Comparison.MetricsDelta.PassRate*100))
	writeLine(&b, fmt.Sprintf("False positives delta: %+d", value.Comparison.MetricsDelta.FalsePositives))
	writeLine(&b, fmt.Sprintf("False negatives delta: %+d", value.Comparison.MetricsDelta.FalseNegatives))
	if len(value.Comparison.HighPriorityRegressions) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstStrings(value.Comparison.HighPriorityRegressions, 3) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderEvalMultiArtifact(path string, value artifact.SkillEvalMultiArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Multi-Backend Eval")
	writeLine(&b, fmt.Sprintf("Status: %s", statusLabel(value.Run.ExitCode)))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.Summary.Summary))
	writeLine(&b, fmt.Sprintf("Strongest backend: %s", value.Summary.StrongestBackend))
	writeLine(&b, fmt.Sprintf("Weakest backend: %s", value.Summary.WeakestBackend))
	if len(value.Summary.BackendSpecificMisses) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstStrings(value.Summary.BackendSpecificMisses, 3) {
			writeBullet(&b, item)
		}
	}
	if mode != ModePRComment && len(value.DifferingCases) > 0 {
		writeSectionHeader(&b, mode, "Differing cases")
		for _, item := range firstDifferingCaseSummaries(value, 3) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderEvalMultiCompareArtifact(path string, value artifact.SkillEvalMultiCompareArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Multi-Backend Eval Compare")
	writeLine(&b, fmt.Sprintf("Status: %s", compareStatus(string(value.AggregateSummary.Overall))))
	writeLine(&b, fmt.Sprintf("Summary: %s", value.AggregateSummary.Summary))
	if mode != ModePRComment {
		writeLine(&b, fmt.Sprintf("Backends: %d", value.AggregateSummary.BackendCount))
	}
	writeSectionHeader(&b, mode, "Review first")
	for _, item := range firstStrings(value.AggregateSummary.HighPriorityRegressions, 3) {
		writeBullet(&b, item)
	}
	if mode != ModePRComment {
		if len(value.WidenedDisagreements) > 0 {
			writeSectionHeader(&b, mode, "Widened disagreements")
			for _, item := range firstMultiCompareCaseSummaries(value.WidenedDisagreements, 3) {
				writeBullet(&b, item)
			}
		}
		if len(value.NarrowedDisagreements) > 0 {
			writeSectionHeader(&b, mode, "Narrowed disagreements")
			for _, item := range firstMultiCompareCaseSummaries(value.NarrowedDisagreements, 3) {
				writeBullet(&b, item)
			}
		}
	}
	if mode == ModeFullReport {
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderLintArtifact(path string, value artifact.SkillLintArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Skill Lint")
	writeLine(&b, fmt.Sprintf("Status: %s", statusLabel(value.Run.ExitCode)))
	writeLine(&b, fmt.Sprintf("Lint: %d error(s), %d warning(s)", value.Summary.ErrorCount, value.Summary.WarningCount))
	if value.RoutingRisk != nil {
		writeLine(&b, fmt.Sprintf("Routing risk: %s", strings.ToUpper(string(value.RoutingRisk.OverallRisk))))
	}
	if len(value.Findings) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, finding := range firstLintFindingSummaries(value.Findings, 3) {
			writeBullet(&b, finding)
		}
	}
	if mode == ModeFullReport {
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderEvalArtifact(path string, value artifact.SkillEvalArtifact, mode Mode) string {
	var b strings.Builder
	writeTitle(&b, mode, "Firety Routing Eval")
	writeLine(&b, fmt.Sprintf("Status: %s", statusLabel(value.Run.ExitCode)))
	writeLine(&b, fmt.Sprintf("Summary: %d passed, %d failed, %d false positive(s), %d false negative(s)", value.Summary.Passed, value.Summary.Failed, value.Summary.FalsePositives, value.Summary.FalseNegatives))
	if len(value.Summary.NotableMisses) > 0 {
		writeSectionHeader(&b, mode, "Review first")
		for _, item := range firstEvalMissSummaries(value.Summary.NotableMisses, 3) {
			writeBullet(&b, item)
		}
	}
	if mode == ModeFullReport {
		writeLine(&b, fmt.Sprintf("Artifact: %s", path))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func emptyDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func writeTitle(b *strings.Builder, mode Mode, title string) {
	switch mode {
	case ModePRComment:
		b.WriteString("## " + title + "\n")
	case ModeCISummary:
		b.WriteString("### " + title + "\n")
	default:
		b.WriteString("# " + title + "\n")
	}
}

func writeSectionHeader(b *strings.Builder, mode Mode, title string) {
	switch mode {
	case ModePRComment, ModeCISummary:
		b.WriteString("\n**" + title + "**\n")
	default:
		b.WriteString("\n## " + title + "\n")
	}
}

func writeLine(b *strings.Builder, line string) {
	b.WriteString(line + "\n")
}

func writeBullet(b *strings.Builder, line string) {
	b.WriteString("- " + line + "\n")
}

func writePlanItems(b *strings.Builder, plan analysis.ImprovementPlan, mode Mode) {
	for _, item := range firstPlanItems(plan.Priorities, 5) {
		writeBullet(b, item.Title)
		if mode == ModePRComment {
			continue
		}
		writeLine(b, "  Why: "+item.WhyItMatters)
		writeLine(b, "  Improve: "+item.WhatToImprove)
		writeLine(b, "  Impact: "+strings.Join(item.ImpactAreas, ", "))
		if evidence := item.EvidenceSummary(); evidence != "" {
			writeLine(b, "  Evidence: "+evidence)
		}
	}
}

func statusLabel(exitCode int) string {
	if exitCode == 0 {
		return "good shape"
	}
	return "attention needed"
}

func compareStatus(overall string) string {
	switch overall {
	case "improved":
		return "improved"
	case "regressed":
		return "regressed"
	case "mixed":
		return "mixed"
	default:
		return "unchanged"
	}
}

func benchmarkStatusLabel(value artifact.BenchmarkArtifact) string {
	if value.Summary.FailedFixtures == 0 {
		return "healthy"
	}
	if value.Summary.PassedFixtures == 0 {
		return "regressed"
	}
	return "attention needed"
}

func firstStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstPlanItems(values []analysis.ImprovementPlanItem, limit int) []analysis.ImprovementPlanItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstLintFindingSummaries(findings []artifact.SkillLintArtifactFinding, limit int) []string {
	items := make([]string, 0, limit)
	for _, finding := range findings {
		items = append(items, fmt.Sprintf("%s: %s", finding.RuleID, finding.Message))
		if len(items) == limit {
			break
		}
	}
	return items
}

func firstEvalMissSummaries(misses []domaineval.RoutingEvalCaseResult, limit int) []string {
	items := make([]string, 0, limit)
	for _, miss := range misses {
		summary := miss.ID
		if miss.Label != "" {
			summary = miss.Label
		}
		if miss.FailureKind != "" {
			summary = fmt.Sprintf("%s (%s)", summary, miss.FailureKind)
		}
		items = append(items, summary)
		if len(items) == limit {
			break
		}
	}
	return items
}

func firstDifferingCaseSummaries(value artifact.SkillEvalMultiArtifact, limit int) []string {
	items := make([]string, 0, limit)
	for _, item := range value.DifferingCases {
		items = append(items, fmt.Sprintf("%s: %s", item.ID, item.Prompt))
		if len(items) == limit {
			break
		}
	}
	return items
}

func firstBenchmarkCategorySummaries(values []benchmark.CategorySummary, limit int) []string {
	items := make([]string, 0, limit)
	for _, item := range values {
		items = append(items, fmt.Sprintf("%s: %d/%d passed", item.CategoryLabel, item.Passed, item.FixtureCount))
		if len(items) == limit {
			break
		}
	}
	return items
}

func firstMultiCompareCaseSummaries(values []domaineval.MultiBackendEvalCaseDelta, limit int) []string {
	items := make([]string, 0, limit)
	for _, item := range values {
		items = append(items, fmt.Sprintf("%s: %s", item.ID, item.Prompt))
		if len(items) == limit {
			break
		}
	}
	return items
}

func firstGateReasonSummaries(values []gate.Reason, limit int) []string {
	items := make([]string, 0, limit)
	for _, item := range values {
		items = append(items, item.Summary)
		if len(items) == limit {
			break
		}
	}
	return items
}
