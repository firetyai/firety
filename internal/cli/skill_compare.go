package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const skillCompareJSONSchemaVersion = "1"

type skillCompareOptions struct {
	format      string
	failOn      string
	profile     string
	strictness  string
	artifact    string
	explain     bool
	routingRisk bool
}

func newSkillCompareCommand(application *app.App) *cobra.Command {
	options := skillCompareOptions{
		format:     skillLintFormatText,
		failOn:     skillLintFailOnErrors,
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
	}

	cmd := &cobra.Command{
		Use:   "compare <base> <candidate>",
		Short: "Compare lint quality between two skill directories",
		Long:  "Compare Firety lint results for two versions of a skill directory and summarize quality improvements or regressions.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			profile, err := service.ParseSkillLintProfile(options.profile)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			strictness, err := lint.ParseStrictness(options.strictness)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			options.strictness = string(strictness)

			result, err := application.Services.SkillCompare.Compare(args[0], args[1], profile, strictness)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillCompareReport(cmd.OutOrStdout(), result, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if shouldFailSkillLint(result.CandidateReport, options.failOn) {
				exitCode = ExitCodeLint
			}

			if options.artifact != "" {
				compareArtifact := artifact.BuildSkillLintCompareArtifact(application.Version, result, artifact.SkillLintCompareArtifactOptions{
					Format:      options.format,
					Profile:     options.profile,
					Strictness:  options.strictness,
					FailOn:      options.failOn,
					Explain:     options.explain,
					RoutingRisk: options.routingRisk,
				}, exitCode)
				if err := artifact.WriteSkillLintCompareArtifact(options.artifact, compareArtifact); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}

			if exitCode == ExitCodeLint {
				return newExitError(ExitCodeLint, nil)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(
		&options.format,
		"format",
		skillLintFormatText,
		"Output format: text or json",
	)
	cmd.Flags().StringVar(
		&options.failOn,
		"fail-on",
		skillLintFailOnErrors,
		"Fail policy based on the candidate skill: errors or warnings",
	)
	cmd.Flags().StringVar(
		&options.profile,
		"profile",
		string(service.SkillLintProfileGeneric),
		"Portability profile: generic, codex, claude-code, copilot, or cursor",
	)
	cmd.Flags().StringVar(
		&options.strictness,
		"strictness",
		string(lint.StrictnessDefault),
		"Lint strictness: default, strict, or pedantic",
	)
	cmd.Flags().StringVar(
		&options.artifact,
		"artifact",
		"",
		"Write a versioned machine-readable compare artifact to the given file path",
	)
	cmd.Flags().BoolVar(
		&options.explain,
		"explain",
		false,
		"Augment changed findings with deterministic rule-aware guidance",
	)
	cmd.Flags().BoolVar(
		&options.routingRisk,
		"routing-risk",
		false,
		"Include a focused routing-risk delta derived from the two lint runs",
	)

	return cmd
}

func (o skillCompareOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}

	switch o.failOn {
	case skillLintFailOnErrors, skillLintFailOnWarnings:
	default:
		return fmt.Errorf("invalid fail-on value %q: must be one of %s, %s", o.failOn, skillLintFailOnErrors, skillLintFailOnWarnings)
	}

	if _, err := service.ParseSkillLintProfile(o.profile); err != nil {
		return err
	}
	if _, err := lint.ParseStrictness(o.strictness); err != nil {
		return err
	}
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	return nil
}

func writeSkillCompareReport(w io.Writer, result service.SkillCompareResult, options skillCompareOptions) error {
	baseContext := lint.NewExplainContext(result.BaseReport.Findings, options.profile, lint.Strictness(options.strictness))
	candidateContext := lint.NewExplainContext(result.CandidateReport.Findings, options.profile, lint.Strictness(options.strictness))

	switch options.format {
	case skillLintFormatText:
		return writeSkillCompareText(w, result, options, baseContext, candidateContext)
	case skillLintFormatJSON:
		return writeSkillCompareJSON(w, result, options, baseContext, candidateContext)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillCompareText(w io.Writer, result service.SkillCompareResult, options skillCompareOptions, baseContext, candidateContext lint.ExplainContext) error {
	comparison := result.Comparison

	if _, err := fmt.Fprintf(w, "Base: %s\n", comparison.Base.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Candidate: %s\n", comparison.Candidate.Target); err != nil {
		return err
	}
	if options.profile != string(service.SkillLintProfileGeneric) {
		if _, err := fmt.Fprintf(w, "Profile: %s\n", options.profile); err != nil {
			return err
		}
	}
	if options.strictness != string(lint.StrictnessDefault) {
		if _, err := fmt.Fprintf(w, "Strictness: %s\n", options.strictness); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Overall: %s\n", strings.ToUpper(string(comparison.Summary.Overall))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", comparison.Summary.Summary); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"Counts: %d error(s), %d warning(s) -> %d error(s), %d warning(s)\n",
		comparison.Base.ErrorCount,
		comparison.Base.WarningCount,
		comparison.Candidate.ErrorCount,
		comparison.Candidate.WarningCount,
	); err != nil {
		return err
	}

	if len(comparison.Summary.HighPriorityRegressions) > 0 {
		if _, err := fmt.Fprintln(w, "Top review priorities:"); err != nil {
			return err
		}
		for _, item := range comparison.Summary.HighPriorityRegressions {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(comparison.Summary.NotableImprovements) > 0 {
		if _, err := fmt.Fprintln(w, "Notable improvements:"); err != nil {
			return err
		}
		for _, item := range comparison.Summary.NotableImprovements {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}

	if options.routingRisk {
		if err := writeRoutingRiskDeltaSection(w, comparison.RoutingRiskDelta); err != nil {
			return err
		}
	}

	if len(comparison.AddedFindings) > 0 {
		if _, err := fmt.Fprintf(w, "Added findings (%d):\n", len(comparison.AddedFindings)); err != nil {
			return err
		}
		if err := writeComparisonFindings(w, comparison.AddedFindings, candidateContext, options.explain, 8); err != nil {
			return err
		}
	}

	if len(comparison.ChangedFindings) > 0 {
		if _, err := fmt.Fprintf(w, "Severity changes (%d):\n", len(comparison.ChangedFindings)); err != nil {
			return err
		}
		if err := writeChangedComparisonFindings(w, comparison.ChangedFindings, candidateContext, options.explain, 8); err != nil {
			return err
		}
	}

	if len(comparison.RemovedFindings) > 0 {
		if _, err := fmt.Fprintf(w, "Resolved findings (%d):\n", len(comparison.RemovedFindings)); err != nil {
			return err
		}
		if err := writeComparisonFindings(w, comparison.RemovedFindings, baseContext, false, 8); err != nil {
			return err
		}
	}

	if len(comparison.Summary.RegressionAreas) > 0 {
		if _, err := fmt.Fprintln(w, "Regression areas:"); err != nil {
			return err
		}
		for _, area := range comparison.Summary.RegressionAreas {
			if _, err := fmt.Fprintf(w, "- %s\n", area); err != nil {
				return err
			}
		}
	}
	if len(comparison.Summary.ImprovementAreas) > 0 {
		if _, err := fmt.Fprintln(w, "Improvement areas:"); err != nil {
			return err
		}
		for _, area := range comparison.Summary.ImprovementAreas {
			if _, err := fmt.Fprintf(w, "- %s\n", area); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeComparisonFindings(w io.Writer, findings []lint.ComparisonFinding, explainContext lint.ExplainContext, explain bool, limit int) error {
	visible := findings
	if len(visible) > limit {
		visible = visible[:limit]
	}

	for _, finding := range visible {
		location := formatComparisonFindingLocation(finding.Path, finding.Line)
		if _, err := fmt.Fprintf(w, "- %s [%s] %s%s\n", strings.ToUpper(string(finding.Severity)), finding.RuleID, finding.Message, location); err != nil {
			return err
		}
		if !explain {
			continue
		}

		rule, ok := lint.FindRule(finding.RuleID)
		if !ok {
			continue
		}
		explanation := rule.Explain(explainContext)
		if _, err := fmt.Fprintf(w, "  Why: %s\n", explanation.WhyItMatters); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Improve: %s\n", explanation.ImprovementHint); err != nil {
			return err
		}
		if explanation.ProfileSpecificHint != "" {
			if _, err := fmt.Fprintf(w, "  Profile hint: %s\n", explanation.ProfileSpecificHint); err != nil {
				return err
			}
		}
	}

	if len(findings) > len(visible) {
		if _, err := fmt.Fprintf(w, "- ... %d more finding change(s)\n", len(findings)-len(visible)); err != nil {
			return err
		}
	}

	return nil
}

func writeChangedComparisonFindings(w io.Writer, findings []lint.ComparisonChangedFinding, explainContext lint.ExplainContext, explain bool, limit int) error {
	visible := findings
	if len(visible) > limit {
		visible = visible[:limit]
	}

	for _, finding := range visible {
		location := formatChangedComparisonFindingLocation(finding)
		if _, err := fmt.Fprintf(
			w,
			"- [%s] %s changed from %s to %s%s\n",
			finding.RuleID,
			finding.Message,
			strings.ToUpper(string(finding.BaseSeverity)),
			strings.ToUpper(string(finding.CandidateSeverity)),
			location,
		); err != nil {
			return err
		}
		if !explain {
			continue
		}

		rule, ok := lint.FindRule(finding.RuleID)
		if !ok {
			continue
		}
		explanation := rule.Explain(explainContext)
		if _, err := fmt.Fprintf(w, "  Why: %s\n", explanation.WhyItMatters); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Improve: %s\n", explanation.ImprovementHint); err != nil {
			return err
		}
		if explanation.ProfileSpecificHint != "" {
			if _, err := fmt.Fprintf(w, "  Profile hint: %s\n", explanation.ProfileSpecificHint); err != nil {
				return err
			}
		}
	}

	if len(findings) > len(visible) {
		if _, err := fmt.Fprintf(w, "- ... %d more severity change(s)\n", len(findings)-len(visible)); err != nil {
			return err
		}
	}

	return nil
}

func writeRoutingRiskDeltaSection(w io.Writer, delta lint.RoutingRiskDelta) error {
	if _, err := fmt.Fprintf(
		w,
		"Routing risk: %s (%s -> %s)\n",
		strings.ToUpper(string(delta.Status)),
		delta.BaseOverallRisk,
		delta.CandidateOverallRisk,
	); err != nil {
		return err
	}

	if len(delta.AddedRiskAreas) > 0 {
		if _, err := fmt.Fprintln(w, "Added routing risk areas:"); err != nil {
			return err
		}
		for _, area := range delta.AddedRiskAreas {
			if _, err := fmt.Fprintf(w, "- %s\n", area); err != nil {
				return err
			}
		}
	}
	if len(delta.RemovedRiskAreas) > 0 {
		if _, err := fmt.Fprintln(w, "Resolved routing risk areas:"); err != nil {
			return err
		}
		for _, area := range delta.RemovedRiskAreas {
			if _, err := fmt.Fprintf(w, "- %s\n", area); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeSkillCompareJSON(w io.Writer, result service.SkillCompareResult, options skillCompareOptions, baseContext, candidateContext lint.ExplainContext) error {
	payload := skillCompareJSONReport{
		SchemaVersion:   skillCompareJSONSchemaVersion,
		Profile:         options.profile,
		Strictness:      lint.Strictness(options.strictness).DisplayName(),
		Explain:         options.explain,
		RoutingRisk:     options.routingRisk,
		Base:            result.Comparison.Base,
		Candidate:       result.Comparison.Candidate,
		Summary:         result.Comparison.Summary,
		CategoryDeltas:  result.Comparison.CategoryDeltas,
		AddedFindings:   make([]skillCompareJSONFinding, 0, len(result.Comparison.AddedFindings)),
		RemovedFindings: make([]skillCompareJSONFinding, 0, len(result.Comparison.RemovedFindings)),
		ChangedFindings: make([]skillCompareJSONChangedFinding, 0, len(result.Comparison.ChangedFindings)),
	}

	if options.routingRisk {
		payload.RoutingRiskDelta = &result.Comparison.RoutingRiskDelta
	}

	for _, finding := range result.Comparison.AddedFindings {
		payload.AddedFindings = append(payload.AddedFindings, buildSkillCompareJSONFinding(finding, candidateContext, options.explain))
	}
	for _, finding := range result.Comparison.RemovedFindings {
		payload.RemovedFindings = append(payload.RemovedFindings, buildSkillCompareJSONFinding(finding, baseContext, false))
	}
	for _, finding := range result.Comparison.ChangedFindings {
		payload.ChangedFindings = append(payload.ChangedFindings, buildSkillCompareJSONChangedFinding(finding, candidateContext, options.explain))
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	return encoder.Encode(payload)
}

func buildSkillCompareJSONFinding(finding lint.ComparisonFinding, explainContext lint.ExplainContext, explain bool) skillCompareJSONFinding {
	item := skillCompareJSONFinding{
		RuleID:   finding.RuleID,
		Category: string(finding.Category),
		Severity: string(finding.Severity),
		Path:     finding.Path,
		Message:  finding.Message,
	}
	if finding.Line != nil {
		item.Line = finding.Line
	}
	if !explain {
		return item
	}

	rule, ok := lint.FindRule(finding.RuleID)
	if !ok {
		return item
	}
	explanation := rule.Explain(explainContext)
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

func buildSkillCompareJSONChangedFinding(finding lint.ComparisonChangedFinding, explainContext lint.ExplainContext, explain bool) skillCompareJSONChangedFinding {
	item := skillCompareJSONChangedFinding{
		RuleID:            finding.RuleID,
		Category:          string(finding.Category),
		Path:              finding.Path,
		Message:           finding.Message,
		BaseSeverity:      string(finding.BaseSeverity),
		CandidateSeverity: string(finding.CandidateSeverity),
		BaseLine:          finding.BaseLine,
		CandidateLine:     finding.CandidateLine,
	}
	if !explain {
		return item
	}

	rule, ok := lint.FindRule(finding.RuleID)
	if !ok {
		return item
	}
	explanation := rule.Explain(explainContext)
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

func formatComparisonFindingLocation(path string, line *int) string {
	if path == "" {
		return ""
	}
	if line == nil {
		return fmt.Sprintf(" (%s)", path)
	}
	return fmt.Sprintf(" (%s:%d)", path, *line)
}

func formatChangedComparisonFindingLocation(finding lint.ComparisonChangedFinding) string {
	if finding.Path == "" {
		return ""
	}
	if finding.CandidateLine != nil {
		return fmt.Sprintf(" (%s:%d)", finding.Path, *finding.CandidateLine)
	}
	if finding.BaseLine != nil {
		return fmt.Sprintf(" (%s:%d)", finding.Path, *finding.BaseLine)
	}
	return fmt.Sprintf(" (%s)", finding.Path)
}

type skillCompareJSONReport struct {
	SchemaVersion    string                           `json:"schema_version"`
	Profile          string                           `json:"profile"`
	Strictness       string                           `json:"strictness"`
	Explain          bool                             `json:"explain,omitempty"`
	RoutingRisk      bool                             `json:"routing_risk,omitempty"`
	Base             lint.ComparisonSideSummary       `json:"base"`
	Candidate        lint.ComparisonSideSummary       `json:"candidate"`
	Summary          lint.ReportComparisonSummary     `json:"summary"`
	CategoryDeltas   []lint.ComparisonCategoryDelta   `json:"category_deltas,omitempty"`
	RoutingRiskDelta *lint.RoutingRiskDelta           `json:"routing_risk_delta,omitempty"`
	AddedFindings    []skillCompareJSONFinding        `json:"added_findings,omitempty"`
	RemovedFindings  []skillCompareJSONFinding        `json:"removed_findings,omitempty"`
	ChangedFindings  []skillCompareJSONChangedFinding `json:"changed_findings,omitempty"`
}

type skillCompareJSONFinding struct {
	RuleID              string `json:"rule_id"`
	Category            string `json:"category,omitempty"`
	Severity            string `json:"severity"`
	Path                string `json:"path"`
	Line                *int   `json:"line,omitempty"`
	Message             string `json:"message"`
	WhyItMatters        string `json:"why_it_matters,omitempty"`
	WhatGoodLooksLike   string `json:"what_good_looks_like,omitempty"`
	ImprovementHint     string `json:"improvement_hint,omitempty"`
	GuidanceProfile     string `json:"guidance_profile,omitempty"`
	ProfileSpecificHint string `json:"profile_specific_hint,omitempty"`
	TargetingPosture    string `json:"targeting_posture,omitempty"`
}

type skillCompareJSONChangedFinding struct {
	RuleID              string `json:"rule_id"`
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
}
