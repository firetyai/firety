package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
)

const SkillLintCompareArtifactSchemaVersion = "1"

type SkillLintCompareArtifactOptions struct {
	Format      string
	Profile     string
	Strictness  string
	FailOn      string
	Explain     bool
	RoutingRisk bool
}

type SkillLintCompareArtifact struct {
	SchemaVersion    string                                   `json:"schema_version"`
	ArtifactType     string                                   `json:"artifact_type"`
	Tool             SkillLintArtifactTool                    `json:"tool"`
	Run              SkillLintCompareArtifactRun              `json:"run"`
	Base             SkillLintArtifactSummary                 `json:"base"`
	Candidate        SkillLintArtifactSummary                 `json:"candidate"`
	Comparison       lint.ReportComparisonSummary             `json:"comparison"`
	AddedFindings    []SkillLintArtifactFinding               `json:"added_findings,omitempty"`
	RemovedFindings  []SkillLintArtifactFinding               `json:"removed_findings,omitempty"`
	ChangedFindings  []SkillLintCompareArtifactChangedFinding `json:"changed_findings,omitempty"`
	CategoryDeltas   []lint.ComparisonCategoryDelta           `json:"category_deltas,omitempty"`
	RoutingRiskDelta *lint.RoutingRiskDelta                   `json:"routing_risk_delta,omitempty"`
	RuleCatalog      []SkillLintArtifactRule                  `json:"rule_catalog"`
	Fingerprint      string                                   `json:"fingerprint,omitempty"`
}

type SkillLintCompareArtifactRun struct {
	BaseTarget      string `json:"base_target"`
	CandidateTarget string `json:"candidate_target"`
	Profile         string `json:"profile"`
	Strictness      string `json:"strictness"`
	FailOn          string `json:"fail_on"`
	Explain         bool   `json:"explain"`
	RoutingRisk     bool   `json:"routing_risk"`
	ExitCode        int    `json:"exit_code"`
	StdoutFormat    string `json:"stdout_format"`
}

type SkillLintCompareArtifactChangedFinding struct {
	RuleID              string `json:"rule_id"`
	Severity            string `json:"severity"`
	Category            string `json:"category,omitempty"`
	Path                string `json:"path"`
	Message             string `json:"message"`
	BaseSeverity        string `json:"base_severity"`
	CandidateSeverity   string `json:"candidate_severity"`
	BaseLine            *int   `json:"base_line,omitempty"`
	CandidateLine       *int   `json:"candidate_line,omitempty"`
	WhyItMatters        string `json:"why_it_matters,omitempty"`
	WhatGoodLooksLike   string `json:"what_good_looks_like,omitempty"`
	ImprovementHint     string `json:"improvement_hint,omitempty"`
	GuidanceProfile     string `json:"guidance_profile,omitempty"`
	ProfileSpecificHint string `json:"profile_specific_hint,omitempty"`
	TargetingPosture    string `json:"targeting_posture,omitempty"`
	Fixability          string `json:"fixability"`
	ProfileAware        bool   `json:"profile_aware"`
	LineAware           bool   `json:"line_aware"`
}

func BuildSkillLintCompareArtifact(version app.VersionInfo, result service.SkillCompareResult, options SkillLintCompareArtifactOptions, exitCode int) SkillLintCompareArtifact {
	strictness := lint.Strictness(options.Strictness)
	baseContext := lint.NewExplainContext(result.BaseReport.Findings, options.Profile, strictness)
	candidateContext := lint.NewExplainContext(result.CandidateReport.Findings, options.Profile, strictness)

	referencedRuleIDs := make(map[string]struct{})
	addedFindings := make([]SkillLintArtifactFinding, 0, len(result.Comparison.AddedFindings))
	removedFindings := make([]SkillLintArtifactFinding, 0, len(result.Comparison.RemovedFindings))
	changedFindings := make([]SkillLintCompareArtifactChangedFinding, 0, len(result.Comparison.ChangedFindings))

	for _, finding := range result.Comparison.AddedFindings {
		rule, ok := lint.FindRule(finding.RuleID)
		if !ok {
			continue
		}
		referencedRuleIDs[rule.ID] = struct{}{}
		addedFindings = append(addedFindings, artifactFindingFromComparison(finding, rule, candidateContext, options.Explain))
	}
	for _, finding := range result.Comparison.RemovedFindings {
		rule, ok := lint.FindRule(finding.RuleID)
		if !ok {
			continue
		}
		referencedRuleIDs[rule.ID] = struct{}{}
		removedFindings = append(removedFindings, artifactFindingFromComparison(finding, rule, baseContext, false))
	}
	for _, finding := range result.Comparison.ChangedFindings {
		rule, ok := lint.FindRule(finding.RuleID)
		if !ok {
			continue
		}
		referencedRuleIDs[rule.ID] = struct{}{}
		changedFindings = append(changedFindings, artifactChangedFindingFromComparison(finding, rule, candidateContext, options.Explain))
	}

	ruleCatalog := make([]SkillLintArtifactRule, 0, len(referencedRuleIDs))
	for _, rule := range lint.AllRules() {
		if _, ok := referencedRuleIDs[rule.ID]; !ok {
			continue
		}
		ruleCatalog = append(ruleCatalog, SkillLintArtifactRule{
			ID:               rule.ID,
			Slug:             rule.Slug,
			Category:         string(rule.Category),
			DefaultSeverity:  string(rule.Severity),
			StrictSeverity:   string(rule.StrictSeverity),
			PedanticSeverity: string(rule.PedanticSeverity),
			Title:            rule.Title,
			Description:      rule.Description,
			ProfileAware:     rule.ProfileAware,
			LineAware:        rule.LineAware,
			Fixability:       string(rule.Fixability),
		})
	}

	artifact := SkillLintCompareArtifact{
		SchemaVersion: SkillLintCompareArtifactSchemaVersion,
		ArtifactType:  "firety.skill-lint-compare",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillLintCompareArtifactRun{
			BaseTarget:      result.BaseReport.Target,
			CandidateTarget: result.CandidateReport.Target,
			Profile:         options.Profile,
			Strictness:      strictness.DisplayName(),
			FailOn:          options.FailOn,
			Explain:         options.Explain,
			RoutingRisk:     options.RoutingRisk,
			ExitCode:        exitCode,
			StdoutFormat:    compareOptionsOutputFormatDefault(options),
		},
		Base:            compareArtifactSummary(result.BaseReport, options.FailOn),
		Candidate:       compareArtifactSummary(result.CandidateReport, options.FailOn),
		Comparison:      result.Comparison.Summary,
		AddedFindings:   addedFindings,
		RemovedFindings: removedFindings,
		ChangedFindings: changedFindings,
		CategoryDeltas:  result.Comparison.CategoryDeltas,
		RuleCatalog:     ruleCatalog,
	}
	if options.RoutingRisk {
		routingDelta := result.Comparison.RoutingRiskDelta
		artifact.RoutingRiskDelta = &routingDelta
	}
	artifact.Fingerprint = compareArtifactFingerprint(artifact)

	return artifact
}

func WriteSkillLintCompareArtifact(path string, artifact SkillLintCompareArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalSkillLintCompareArtifact(artifact)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func marshalSkillLintCompareArtifact(artifact SkillLintCompareArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func artifactFindingFromComparison(finding lint.ComparisonFinding, rule lint.Rule, ctx lint.ExplainContext, explain bool) SkillLintArtifactFinding {
	item := SkillLintArtifactFinding{
		RuleID:       finding.RuleID,
		Severity:     string(finding.Severity),
		Category:     string(rule.Category),
		Path:         finding.Path,
		Message:      finding.Message,
		Fixability:   string(rule.Fixability),
		ProfileAware: rule.ProfileAware,
		LineAware:    rule.LineAware,
	}
	if finding.Line != nil {
		item.Line = finding.Line
	}
	if !explain {
		return item
	}

	explanation := rule.Explain(ctx)
	item.WhyItMatters = explanation.WhyItMatters
	item.WhatGoodLooksLike = explanation.WhatGoodLooksLike
	item.ImprovementHint = explanation.ImprovementHint
	item.GuidanceProfile = explanation.GuidanceProfile
	item.ProfileSpecificHint = explanation.ProfileSpecificHint
	item.FixHint = explanation.FixHint
	item.AutofixAvailable = explanation.AutofixAvailable
	if explanation.TargetingPosture != "" {
		item.TargetingPosture = string(explanation.TargetingPosture)
	}

	return item
}

func artifactChangedFindingFromComparison(finding lint.ComparisonChangedFinding, rule lint.Rule, ctx lint.ExplainContext, explain bool) SkillLintCompareArtifactChangedFinding {
	item := SkillLintCompareArtifactChangedFinding{
		RuleID:            finding.RuleID,
		Severity:          string(finding.CandidateSeverity),
		Category:          string(rule.Category),
		Path:              finding.Path,
		Message:           finding.Message,
		BaseSeverity:      string(finding.BaseSeverity),
		CandidateSeverity: string(finding.CandidateSeverity),
		BaseLine:          finding.BaseLine,
		CandidateLine:     finding.CandidateLine,
		Fixability:        string(rule.Fixability),
		ProfileAware:      rule.ProfileAware,
		LineAware:         rule.LineAware,
	}
	if !explain {
		return item
	}

	explanation := rule.Explain(ctx)
	item.WhyItMatters = explanation.WhyItMatters
	item.WhatGoodLooksLike = explanation.WhatGoodLooksLike
	item.ImprovementHint = explanation.ImprovementHint
	item.GuidanceProfile = explanation.GuidanceProfile
	item.ProfileSpecificHint = explanation.ProfileSpecificHint
	if explanation.TargetingPosture != "" {
		item.TargetingPosture = string(explanation.TargetingPosture)
	}

	return item
}

func compareArtifactSummary(report lint.Report, failOn string) SkillLintArtifactSummary {
	return SkillLintArtifactSummary{
		Valid:            !report.HasErrors(),
		PassesFailPolicy: !shouldFailForArtifact(report, failOn),
		ErrorCount:       report.ErrorCount(),
		WarningCount:     report.WarningCount(),
		FindingCount:     len(report.Findings),
		SeverityCounts: SkillLintArtifactCounts{
			Errors:   report.ErrorCount(),
			Warnings: report.WarningCount(),
		},
	}
}

func compareArtifactFingerprint(artifact SkillLintCompareArtifact) string {
	type fingerprintInput struct {
		SchemaVersion   string                                   `json:"schema_version"`
		BaseTarget      string                                   `json:"base_target"`
		CandidateTarget string                                   `json:"candidate_target"`
		Profile         string                                   `json:"profile"`
		Strictness      string                                   `json:"strictness"`
		FailOn          string                                   `json:"fail_on"`
		Explain         bool                                     `json:"explain"`
		RoutingRisk     *lint.RoutingRiskDelta                   `json:"routing_risk_delta,omitempty"`
		Comparison      lint.ReportComparisonSummary             `json:"comparison"`
		AddedFindings   []SkillLintArtifactFinding               `json:"added_findings,omitempty"`
		RemovedFindings []SkillLintArtifactFinding               `json:"removed_findings,omitempty"`
		ChangedFindings []SkillLintCompareArtifactChangedFinding `json:"changed_findings,omitempty"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion:   artifact.SchemaVersion,
		BaseTarget:      artifact.Run.BaseTarget,
		CandidateTarget: artifact.Run.CandidateTarget,
		Profile:         artifact.Run.Profile,
		Strictness:      artifact.Run.Strictness,
		FailOn:          artifact.Run.FailOn,
		Explain:         artifact.Run.Explain,
		RoutingRisk:     artifact.RoutingRiskDelta,
		Comparison:      artifact.Comparison,
		AddedFindings:   artifact.AddedFindings,
		RemovedFindings: artifact.RemovedFindings,
		ChangedFindings: artifact.ChangedFindings,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func compareOptionsOutputFormatDefault(options SkillLintCompareArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
