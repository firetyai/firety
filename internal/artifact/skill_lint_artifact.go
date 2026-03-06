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

const SkillLintArtifactSchemaVersion = "1"

type SkillLintArtifactOptions struct {
	Format      string
	Profile     string
	Strictness  string
	FailOn      string
	Explain     bool
	RoutingRisk bool
	Fix         bool
}

type SkillLintArtifact struct {
	SchemaVersion string                        `json:"schema_version"`
	ArtifactType  string                        `json:"artifact_type"`
	Tool          SkillLintArtifactTool         `json:"tool"`
	Run           SkillLintArtifactRun          `json:"run"`
	Summary       SkillLintArtifactSummary      `json:"summary"`
	Findings      []SkillLintArtifactFinding    `json:"findings"`
	RoutingRisk   *lint.RoutingRiskSummary      `json:"routing_risk,omitempty"`
	ActionAreas   []lint.ActionArea             `json:"action_areas,omitempty"`
	RuleCatalog   []SkillLintArtifactRule       `json:"rule_catalog"`
	AppliedFixes  []SkillLintArtifactAppliedFix `json:"applied_fixes,omitempty"`
	Fingerprint   string                        `json:"fingerprint,omitempty"`
}

type SkillLintArtifactTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"build_date,omitempty"`
}

type SkillLintArtifactRun struct {
	Target       string `json:"target"`
	Profile      string `json:"profile"`
	Strictness   string `json:"strictness"`
	FailOn       string `json:"fail_on"`
	Explain      bool   `json:"explain"`
	RoutingRisk  bool   `json:"routing_risk"`
	Fix          bool   `json:"fix"`
	ExitCode     int    `json:"exit_code"`
	StdoutFormat string `json:"stdout_format"`
}

type SkillLintArtifactSummary struct {
	Valid            bool                    `json:"valid"`
	PassesFailPolicy bool                    `json:"passes_fail_policy"`
	ErrorCount       int                     `json:"error_count"`
	WarningCount     int                     `json:"warning_count"`
	FindingCount     int                     `json:"finding_count"`
	AppliedFixCount  int                     `json:"applied_fix_count"`
	SeverityCounts   SkillLintArtifactCounts `json:"severity_counts"`
	StrictnessNote   string                  `json:"strictness_note,omitempty"`
	PortabilityNote  string                  `json:"portability_note,omitempty"`
}

type SkillLintArtifactCounts struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

type SkillLintArtifactFinding struct {
	RuleID              string `json:"rule_id"`
	Severity            string `json:"severity"`
	Category            string `json:"category,omitempty"`
	Path                string `json:"path"`
	Line                *int   `json:"line,omitempty"`
	Message             string `json:"message"`
	WhyItMatters        string `json:"why_it_matters,omitempty"`
	WhatGoodLooksLike   string `json:"what_good_looks_like,omitempty"`
	ImprovementHint     string `json:"improvement_hint,omitempty"`
	GuidanceProfile     string `json:"guidance_profile,omitempty"`
	ProfileSpecificHint string `json:"profile_specific_hint,omitempty"`
	TargetingPosture    string `json:"targeting_posture,omitempty"`
	FixHint             string `json:"fix_hint,omitempty"`
	AutofixAvailable    bool   `json:"autofix_available,omitempty"`
	Fixability          string `json:"fixability"`
	ProfileAware        bool   `json:"profile_aware"`
	LineAware           bool   `json:"line_aware"`
}

type SkillLintArtifactRule struct {
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	Category         string `json:"category"`
	DefaultSeverity  string `json:"default_severity"`
	StrictSeverity   string `json:"strict_severity,omitempty"`
	PedanticSeverity string `json:"pedantic_severity,omitempty"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	ProfileAware     bool   `json:"profile_aware"`
	LineAware        bool   `json:"line_aware"`
	Fixability       string `json:"fixability"`
}

type SkillLintArtifactAppliedFix struct {
	RuleID     string `json:"rule_id"`
	Path       string `json:"path"`
	Message    string `json:"message"`
	Fixability string `json:"fixability"`
}

func BuildSkillLintArtifact(version app.VersionInfo, report lint.Report, fixResult service.SkillFixResult, options SkillLintArtifactOptions, exitCode int) SkillLintArtifact {
	strictness := lint.Strictness(options.Strictness)
	explainContext := lint.NewExplainContext(report.Findings, options.Profile, strictness)
	actionAreas := lint.SummarizeActionAreas(report.Findings)
	portabilityNote := lint.PortabilitySummary(explainContext)
	strictnessNote := lint.StrictnessSummary(strictness)
	var routingRisk *lint.RoutingRiskSummary
	if options.RoutingRisk {
		summary := lint.SummarizeRoutingRisk(report.Findings)
		routingRisk = &summary
	}

	findings := make([]SkillLintArtifactFinding, 0, len(report.Findings))
	referencedRuleIDs := make(map[string]struct{}, len(report.Findings)+len(fixResult.Applied))
	for _, finding := range report.Findings {
		rule, ok := lint.FindRule(finding.RuleID)
		if !ok {
			continue
		}
		referencedRuleIDs[rule.ID] = struct{}{}

		explanation := rule.Explain(explainContext)
		item := SkillLintArtifactFinding{
			RuleID:           finding.RuleID,
			Severity:         string(finding.Severity),
			Category:         string(rule.Category),
			Path:             finding.Path,
			Message:          finding.Message,
			Fixability:       string(rule.Fixability),
			ProfileAware:     rule.ProfileAware,
			LineAware:        rule.LineAware,
			AutofixAvailable: explanation.AutofixAvailable,
		}
		if finding.Line > 0 {
			item.Line = &finding.Line
		}
		if options.Explain {
			item.WhyItMatters = explanation.WhyItMatters
			item.WhatGoodLooksLike = explanation.WhatGoodLooksLike
			item.ImprovementHint = explanation.ImprovementHint
			item.GuidanceProfile = explanation.GuidanceProfile
			item.ProfileSpecificHint = explanation.ProfileSpecificHint
			item.FixHint = explanation.FixHint
			if explanation.TargetingPosture != "" {
				item.TargetingPosture = string(explanation.TargetingPosture)
			}
		}

		findings = append(findings, item)
	}

	appliedFixes := make([]SkillLintArtifactAppliedFix, 0, len(fixResult.Applied))
	for _, fix := range fixResult.Applied {
		referencedRuleIDs[fix.Rule.ID] = struct{}{}
		appliedFixes = append(appliedFixes, SkillLintArtifactAppliedFix{
			RuleID:     fix.Rule.ID,
			Path:       fix.Path,
			Message:    fix.Message,
			Fixability: string(fix.Rule.Fixability),
		})
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

	artifact := SkillLintArtifact{
		SchemaVersion: SkillLintArtifactSchemaVersion,
		ArtifactType:  "firety.skill-lint",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillLintArtifactRun{
			Target:       report.Target,
			Profile:      options.Profile,
			Strictness:   strictness.DisplayName(),
			FailOn:       options.FailOn,
			Explain:      options.Explain,
			RoutingRisk:  options.RoutingRisk,
			Fix:          options.Fix,
			ExitCode:     exitCode,
			StdoutFormat: optionsOutputFormatDefault(options),
		},
		Summary: SkillLintArtifactSummary{
			Valid:            !report.HasErrors(),
			PassesFailPolicy: !shouldFailForArtifact(report, options.FailOn),
			ErrorCount:       report.ErrorCount(),
			WarningCount:     report.WarningCount(),
			FindingCount:     len(report.Findings),
			AppliedFixCount:  len(fixResult.Applied),
			SeverityCounts: SkillLintArtifactCounts{
				Errors:   report.ErrorCount(),
				Warnings: report.WarningCount(),
			},
			StrictnessNote:  strictnessNote,
			PortabilityNote: portabilityNote,
		},
		Findings:     findings,
		RoutingRisk:  routingRisk,
		ActionAreas:  actionAreas,
		RuleCatalog:  ruleCatalog,
		AppliedFixes: appliedFixes,
	}
	artifact.Fingerprint = artifactFingerprint(artifact)

	return artifact
}

func WriteSkillLintArtifact(path string, artifact SkillLintArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalSkillLintArtifact(artifact)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func marshalSkillLintArtifact(artifact SkillLintArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func artifactFingerprint(artifact SkillLintArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                        `json:"schema_version"`
		Target        string                        `json:"target"`
		Profile       string                        `json:"profile"`
		Strictness    string                        `json:"strictness"`
		FailOn        string                        `json:"fail_on"`
		Explain       bool                          `json:"explain"`
		RoutingRisk   *lint.RoutingRiskSummary      `json:"routing_risk,omitempty"`
		Fix           bool                          `json:"fix"`
		Findings      []SkillLintArtifactFinding    `json:"findings"`
		AppliedFixes  []SkillLintArtifactAppliedFix `json:"applied_fixes,omitempty"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: artifact.SchemaVersion,
		Target:        artifact.Run.Target,
		Profile:       artifact.Run.Profile,
		Strictness:    artifact.Run.Strictness,
		FailOn:        artifact.Run.FailOn,
		Explain:       artifact.Run.Explain,
		RoutingRisk:   artifact.RoutingRisk,
		Fix:           artifact.Run.Fix,
		Findings:      artifact.Findings,
		AppliedFixes:  artifact.AppliedFixes,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func shouldFailForArtifact(report lint.Report, failOn string) bool {
	switch failOn {
	case "warnings":
		return len(report.Findings) > 0
	default:
		return report.HasErrors()
	}
}

func optionsOutputFormatDefault(options SkillLintArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
