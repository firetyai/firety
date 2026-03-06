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

const (
	skillLintFormatText  = "text"
	skillLintFormatJSON  = "json"
	skillLintFormatSARIF = "sarif"

	skillLintFailOnErrors   = "errors"
	skillLintFailOnWarnings = "warnings"
)

type skillLintOptions struct {
	format      string
	failOn      string
	profile     string
	strictness  string
	artifact    string
	explain     bool
	routingRisk bool
	quiet       bool
	noSummary   bool
	fix         bool
}

type skillRulesOptions struct {
	format string
}

func newSkillCommand(application *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Work with reusable skills",
		Long:  "Inspect and validate reusable skill directories.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newSkillLintCommand(application))
	cmd.AddCommand(newSkillBaselineCommand(application))
	cmd.AddCommand(newSkillCompatibilityCommand(application))
	cmd.AddCommand(newSkillPlanCommand(application))
	cmd.AddCommand(newSkillAnalyzeCommand(application))
	cmd.AddCommand(newSkillEvalCommand(application))
	cmd.AddCommand(newSkillEvalCompareCommand(application))
	cmd.AddCommand(newSkillGateCommand(application))
	cmd.AddCommand(newSkillCompareCommand(application))
	cmd.AddCommand(newSkillRenderCommand())
	cmd.AddCommand(newSkillRulesCommand())

	return cmd
}

func newSkillRulesCommand() *cobra.Command {
	options := skillRulesOptions{
		format: skillLintFormatText,
	}

	cmd := &cobra.Command{
		Use:   "rules",
		Short: "List Firety lint rules",
		Long:  "List the authoritative Firety skill lint rule catalog in text or JSON format.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillRuleCatalog(cmd.OutOrStdout(), options); err != nil {
				return newExitError(ExitCodeRuntime, err)
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

	return cmd
}

func newSkillLintCommand(application *app.App) *cobra.Command {
	options := skillLintOptions{
		format:     skillLintFormatText,
		failOn:     skillLintFailOnErrors,
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
	}

	cmd := &cobra.Command{
		Use:   "lint [path]",
		Short: "Lint a local skill directory",
		Long:  "Lint a local skill directory and report errors and warnings for SKILL.md and referenced local files.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			target := "."
			if len(args) == 1 {
				target = args[0]
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

			var fixResult service.SkillFixResult
			if options.fix {
				fixResult, err = application.Services.SkillFix.Apply(target)
				if err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}

			report, err := application.Services.SkillLint.LintWithProfileAndStrictness(target, profile, strictness)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillLintReport(cmd.OutOrStdout(), report, fixResult, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if shouldFailSkillLint(report, options.failOn) {
				exitCode = ExitCodeLint
			}

			if options.artifact != "" {
				lintArtifact := artifact.BuildSkillLintArtifact(application.Version, report, fixResult, artifact.SkillLintArtifactOptions{
					Format:      options.format,
					Profile:     options.profile,
					Strictness:  options.strictness,
					FailOn:      options.failOn,
					Explain:     options.explain,
					RoutingRisk: options.routingRisk,
					Fix:         options.fix,
				}, exitCode)
				if err := artifact.WriteSkillLintArtifact(options.artifact, lintArtifact); err != nil {
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
		"Output format: text, json, or sarif",
	)
	cmd.Flags().StringVar(
		&options.failOn,
		"fail-on",
		skillLintFailOnErrors,
		"Fail policy: errors or warnings",
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
		"Write a versioned machine-readable lint artifact to the given file path",
	)
	cmd.Flags().BoolVar(
		&options.explain,
		"explain",
		false,
		"Augment findings with deterministic rule-aware guidance",
	)
	cmd.Flags().BoolVar(
		&options.routingRisk,
		"routing-risk",
		false,
		"Summarize the most important trigger and routing weaknesses",
	)
	cmd.Flags().BoolVar(
		&options.quiet,
		"quiet",
		false,
		"Reduce text output noise",
	)
	cmd.Flags().BoolVar(
		&options.noSummary,
		"no-summary",
		false,
		"Suppress the final summary line in text output",
	)
	cmd.Flags().BoolVar(
		&options.fix,
		"fix",
		false,
		"Apply safe, low-risk automatic fixes before linting the final file state",
	)

	return cmd
}

func (o skillLintOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON, skillLintFormatSARIF:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s, %s", o.format, skillLintFormatText, skillLintFormatJSON, skillLintFormatSARIF)
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

func (o skillRulesOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
		return nil
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func shouldFailSkillLint(report lint.Report, failOn string) bool {
	switch failOn {
	case skillLintFailOnWarnings:
		return len(report.Findings) > 0
	default:
		return report.HasErrors()
	}
}

func writeSkillLintReport(w io.Writer, report lint.Report, fixResult service.SkillFixResult, options skillLintOptions) error {
	explainContext := lint.NewExplainContext(report.Findings, options.profile, lint.Strictness(options.strictness))

	switch options.format {
	case skillLintFormatText:
		return writeSkillLintText(w, report, fixResult, options, explainContext)
	case skillLintFormatJSON:
		return writeSkillLintJSON(w, report, fixResult, options.explain, options.routingRisk, explainContext)
	case skillLintFormatSARIF:
		return writeSkillLintSARIF(w, report)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s, %s", options.format, skillLintFormatText, skillLintFormatJSON, skillLintFormatSARIF)
	}
}

func writeSkillLintText(w io.Writer, report lint.Report, fixResult service.SkillFixResult, options skillLintOptions, explainContext lint.ExplainContext) error {
	if !options.quiet {
		if _, err := fmt.Fprintf(w, "Target: %s\n", report.Target); err != nil {
			return err
		}
		if options.strictness != string(lint.StrictnessDefault) {
			if _, err := fmt.Fprintf(w, "Strictness: %s\n", options.strictness); err != nil {
				return err
			}
		}
	}

	if len(fixResult.Applied) > 0 {
		if _, err := fmt.Fprintf(w, "Applied %d fix(es)\n", len(fixResult.Applied)); err != nil {
			return err
		}
		for _, fix := range fixResult.Applied {
			if _, err := fmt.Fprintf(w, "FIXED [%s] %s (%s)\n", fix.Rule.ID, fix.Message, fix.Path); err != nil {
				return err
			}
		}
	}

	if len(report.Findings) == 0 {
		if options.quiet {
			if options.routingRisk {
				return writeRoutingRiskSection(w, report.Findings)
			}
			return nil
		}

		if _, err := fmt.Fprintln(w, "OK: no lint findings"); err != nil {
			return err
		}
		if options.noSummary {
			if options.routingRisk {
				return writeRoutingRiskSection(w, report.Findings)
			}
			return nil
		}
		if _, err := fmt.Fprintln(w, "Summary: 0 error(s), 0 warning(s)"); err != nil {
			return err
		}
		if options.routingRisk {
			return writeRoutingRiskSection(w, report.Findings)
		}
		return nil
	}

	for _, finding := range report.Findings {
		rule, _ := lint.FindRule(finding.RuleID)
		explanation := rule.Explain(explainContext)
		location := formatFindingLocation(finding)
		if _, err := fmt.Fprintf(
			w,
			"%s [%s] %s%s\n",
			strings.ToUpper(string(finding.Severity)),
			finding.RuleID,
			finding.Message,
			location,
		); err != nil {
			return err
		}
		if options.explain {
			if _, err := fmt.Fprintf(w, "  Why: %s\n", explanation.WhyItMatters); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "  Improve: %s\n", explanation.ImprovementHint); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "  Good: %s\n", explanation.WhatGoodLooksLike); err != nil {
				return err
			}
			if explanation.ProfileSpecificHint != "" {
				if _, err := fmt.Fprintf(w, "  Profile hint: %s\n", explanation.ProfileSpecificHint); err != nil {
					return err
				}
			}
			if explanation.FixHint != "" {
				if _, err := fmt.Fprintf(w, "  Fix hint: %s\n", explanation.FixHint); err != nil {
					return err
				}
			}
		}
	}

	if options.noSummary {
		if options.routingRisk {
			return writeRoutingRiskSection(w, report.Findings)
		}
		return nil
	}

	_, err := fmt.Fprintf(
		w,
		"Summary: %d error(s), %d warning(s)\n",
		report.ErrorCount(),
		report.WarningCount(),
	)
	if err != nil {
		return err
	}

	if options.routingRisk {
		if err := writeRoutingRiskSection(w, report.Findings); err != nil {
			return err
		}
	}

	if !options.explain {
		return nil
	}

	actionAreas := lint.SummarizeActionAreas(report.Findings)
	if len(actionAreas) == 0 {
		return nil
	}

	if _, err := fmt.Fprintln(w, "How to improve this skill:"); err != nil {
		return err
	}
	for _, area := range actionAreas {
		if _, err := fmt.Fprintf(w, "- %s: %s\n", area.Title, area.Summary); err != nil {
			return err
		}
	}
	if strictnessSummary := lint.StrictnessSummary(explainContext.Strictness); strictnessSummary != "" {
		if _, err := fmt.Fprintf(w, "Strictness guidance: %s\n", strictnessSummary); err != nil {
			return err
		}
	}
	if portabilitySummary := lint.PortabilitySummary(explainContext); portabilitySummary != "" {
		if _, err := fmt.Fprintf(w, "Portability guidance: %s\n", portabilitySummary); err != nil {
			return err
		}
	}

	return nil
}

func writeSkillLintJSON(w io.Writer, report lint.Report, fixResult service.SkillFixResult, explain bool, routingRisk bool, explainContext lint.ExplainContext) error {
	payload := skillLintJSONReport{
		Target:       report.Target,
		Valid:        !report.HasErrors(),
		ErrorCount:   report.ErrorCount(),
		WarningCount: report.WarningCount(),
		Strictness:   explainContext.Strictness.DisplayName(),
		Findings:     make([]skillLintJSONFinding, 0, len(report.Findings)),
	}
	if explain {
		payload.Explain = true
	}
	if routingRisk {
		routingRiskSummary := lint.SummarizeRoutingRisk(report.Findings)
		payload.RoutingRisk = &routingRiskSummary
	}
	if len(fixResult.Applied) > 0 {
		payload.AppliedFixCount = len(fixResult.Applied)
		payload.AppliedFixes = make([]skillLintJSONFix, 0, len(fixResult.Applied))
		for _, fix := range fixResult.Applied {
			payload.AppliedFixes = append(payload.AppliedFixes, skillLintJSONFix{
				RuleID:  fix.Rule.ID,
				Path:    fix.Path,
				Message: fix.Message,
			})
		}
	}

	for _, finding := range report.Findings {
		rule, _ := lint.FindRule(finding.RuleID)
		explanation := rule.Explain(explainContext)
		item := skillLintJSONFinding{
			RuleID:   finding.RuleID,
			Severity: string(finding.Severity),
			Path:     finding.Path,
			Message:  finding.Message,
		}
		if finding.Line > 0 {
			item.Line = &finding.Line
		}
		if explain {
			item.Category = string(explanation.Category)
			item.WhyItMatters = explanation.WhyItMatters
			item.WhatGoodLooksLike = explanation.WhatGoodLooksLike
			item.ImprovementHint = explanation.ImprovementHint
			item.GuidanceProfile = explanation.GuidanceProfile
			item.ProfileSpecificHint = explanation.ProfileSpecificHint
			if explanation.TargetingPosture != "" {
				item.TargetingPosture = string(explanation.TargetingPosture)
			}
			if explanation.FixHint != "" {
				item.FixHint = explanation.FixHint
			}
			item.AutofixAvailable = explanation.AutofixAvailable
		}

		payload.Findings = append(payload.Findings, item)
	}

	return encodeSkillLintJSON(w, payload)
}

func encodeSkillLintJSON(w io.Writer, payload skillLintJSONReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	return encoder.Encode(payload)
}

func writeRoutingRiskSection(w io.Writer, findings []lint.Finding) error {
	routingRisk := lint.SummarizeRoutingRisk(findings)
	if _, err := fmt.Fprintf(w, "Routing risk: %s\n", strings.ToUpper(string(routingRisk.OverallRisk))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Routing summary: %s\n", routingRisk.Summary); err != nil {
		return err
	}
	if len(routingRisk.RiskAreas) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Top routing risk areas:"); err != nil {
		return err
	}
	for _, area := range routingRisk.RiskAreas {
		if _, err := fmt.Fprintf(w, "- %s: %s\n", area.Title, area.Summary); err != nil {
			return err
		}
	}
	if len(routingRisk.PriorityActions) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Routing improvement priorities:"); err != nil {
		return err
	}
	for _, action := range routingRisk.PriorityActions {
		if _, err := fmt.Fprintf(w, "- %s\n", action); err != nil {
			return err
		}
	}

	return nil
}

func writeSkillLintSARIF(w io.Writer, report lint.Report) error {
	sarifRules := make([]sarifRule, 0, len(lint.AllRules()))
	for _, rule := range lint.AllRules() {
		sarifRules = append(sarifRules, sarifRule{
			ID: rule.ID,
			ShortDescription: sarifMessage{
				Text: rule.Title,
			},
			FullDescription: &sarifMessage{
				Text: rule.Description,
			},
			DefaultConfiguration: sarifDefaultConfiguration{
				Level: sarifLevel(rule.Severity),
			},
		})
	}

	results := make([]sarifResult, 0, len(report.Findings))
	for _, finding := range report.Findings {
		result := sarifResult{
			RuleID: finding.RuleID,
			Level:  sarifLevel(finding.Severity),
			Message: sarifMessage{
				Text: finding.Message,
			},
		}

		if finding.Path != "" {
			location := sarifLocation{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{
						URI: finding.Path,
					},
				},
			}
			if finding.Line > 0 {
				location.PhysicalLocation.Region = &sarifRegion{
					StartLine: finding.Line,
				}
			}

			result.Locations = []sarifLocation{location}
		}

		results = append(results, result)
	}

	payload := sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{
			{
				Tool: sarifTool{
					Driver: sarifDriver{
						Name:  "firety",
						Rules: sarifRules,
					},
				},
				Results: results,
			},
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	return encoder.Encode(payload)
}

func writeSkillRuleCatalog(w io.Writer, options skillRulesOptions) error {
	switch options.format {
	case skillLintFormatText:
		_, err := io.WriteString(w, lint.TextCatalog())
		return err
	case skillLintFormatJSON:
		payload := skillRulesJSONCatalog{
			Rules: lint.AllRules(),
		}

		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

type skillLintJSONReport struct {
	Target          string                   `json:"target"`
	Valid           bool                     `json:"valid"`
	ErrorCount      int                      `json:"error_count"`
	WarningCount    int                      `json:"warning_count"`
	Strictness      string                   `json:"strictness,omitempty"`
	Explain         bool                     `json:"explain,omitempty"`
	AppliedFixCount int                      `json:"applied_fix_count,omitempty"`
	AppliedFixes    []skillLintJSONFix       `json:"applied_fixes,omitempty"`
	RoutingRisk     *lint.RoutingRiskSummary `json:"routing_risk,omitempty"`
	Findings        []skillLintJSONFinding   `json:"findings"`
}

type skillLintJSONFinding struct {
	RuleID              string `json:"rule_id"`
	Severity            string `json:"severity"`
	Path                string `json:"path"`
	Message             string `json:"message"`
	Line                *int   `json:"line,omitempty"`
	Category            string `json:"category,omitempty"`
	WhyItMatters        string `json:"why_it_matters,omitempty"`
	WhatGoodLooksLike   string `json:"what_good_looks_like,omitempty"`
	ImprovementHint     string `json:"improvement_hint,omitempty"`
	GuidanceProfile     string `json:"guidance_profile,omitempty"`
	ProfileSpecificHint string `json:"profile_specific_hint,omitempty"`
	TargetingPosture    string `json:"targeting_posture,omitempty"`
	FixHint             string `json:"fix_hint,omitempty"`
	AutofixAvailable    bool   `json:"autofix_available,omitempty"`
}

type skillLintJSONFix struct {
	RuleID  string `json:"rule_id"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID                   string                    `json:"id"`
	ShortDescription     sarifMessage              `json:"shortDescription"`
	FullDescription      *sarifMessage             `json:"fullDescription,omitempty"`
	DefaultConfiguration sarifDefaultConfiguration `json:"defaultConfiguration"`
}

type skillRulesJSONCatalog struct {
	Rules []lint.Rule `json:"rules"`
}

type sarifDefaultConfiguration struct {
	Level string `json:"level"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

func sarifLevel(severity lint.Severity) string {
	switch severity {
	case lint.SeverityError:
		return "error"
	default:
		return "warning"
	}
}

func formatFindingLocation(finding lint.Finding) string {
	if finding.Path == "" {
		return ""
	}

	if finding.Line > 0 {
		return fmt.Sprintf(" (%s:%d)", finding.Path, finding.Line)
	}

	return fmt.Sprintf(" (%s)", finding.Path)
}
