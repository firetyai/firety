package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/analysis"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const skillAnalyzeJSONSchemaVersion = "1"

type skillAnalyzeOptions struct {
	format     string
	failOn     string
	profile    string
	strictness string
	suite      string
	runner     string
	artifact   string
}

func newSkillAnalyzeCommand(application *app.App) *cobra.Command {
	options := skillAnalyzeOptions{
		format:     skillLintFormatText,
		failOn:     skillLintFailOnErrors,
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
	}

	cmd := &cobra.Command{
		Use:   "analyze [path]",
		Short: "Correlate lint findings with measured routing eval misses",
		Long:  "Run both Firety skill lint and measured routing evals for one skill, then summarize likely contributors to false positives and false negatives.",
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

			result, err := application.Services.SkillAnalyze.Analyze(target, service.SkillAnalyzeOptions{
				Profile:    profile,
				Strictness: strictness,
				SuitePath:  options.suite,
				Runner:     options.runner,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillAnalyzeReport(cmd.OutOrStdout(), result, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if shouldFailSkillLint(result.LintReport, options.failOn) || result.EvalReport.Summary.Failed > 0 {
				exitCode = ExitCodeLint
			}

			if options.artifact != "" {
				analyzeArtifact := artifact.BuildSkillAnalysisArtifact(application.Version, result, artifact.SkillAnalysisArtifactOptions{
					Format:     options.format,
					Profile:    options.profile,
					Strictness: options.strictness,
					FailOn:     options.failOn,
					Suite:      result.EvalReport.Suite.Path,
					Runner:     options.runner,
				}, exitCode)
				if err := artifact.WriteSkillAnalysisArtifact(options.artifact, analyzeArtifact); err != nil {
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
		"Fail policy: errors or warnings",
	)
	cmd.Flags().StringVar(
		&options.profile,
		"profile",
		string(service.SkillLintProfileGeneric),
		"Routing profile: generic, codex, claude-code, copilot, or cursor",
	)
	cmd.Flags().StringVar(
		&options.strictness,
		"strictness",
		string(lint.StrictnessDefault),
		"Lint strictness: default, strict, or pedantic",
	)
	cmd.Flags().StringVar(
		&options.suite,
		"suite",
		"",
		"Path to the local routing eval suite JSON file (defaults to evals/routing.json inside the skill directory)",
	)
	cmd.Flags().StringVar(
		&options.runner,
		"runner",
		"",
		"Path to the local routing eval runner executable (defaults to FIRETY_SKILL_EVAL_RUNNER)",
	)
	cmd.Flags().StringVar(
		&options.artifact,
		"artifact",
		"",
		"Write a versioned machine-readable analysis artifact to the given file path",
	)

	return cmd
}

func (o skillAnalyzeOptions) Validate() error {
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

func writeSkillAnalyzeReport(w io.Writer, result service.SkillAnalyzeResult, options skillAnalyzeOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillAnalyzeText(w, result, options)
	case skillLintFormatJSON:
		return writeSkillAnalyzeJSON(w, result, options)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillAnalyzeText(w io.Writer, result service.SkillAnalyzeResult, options skillAnalyzeOptions) error {
	if _, err := fmt.Fprintf(w, "Target: %s\n", result.LintReport.Target); err != nil {
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
	if _, err := fmt.Fprintf(
		w,
		"Lint summary: %d error(s), %d warning(s)\n",
		result.LintReport.ErrorCount(),
		result.LintReport.WarningCount(),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"Eval summary: %d passed, %d failed, %d false positive(s), %d false negative(s), %.0f%% pass rate\n",
		result.EvalReport.Summary.Passed,
		result.EvalReport.Summary.Failed,
		result.EvalReport.Summary.FalsePositives,
		result.EvalReport.Summary.FalseNegatives,
		result.EvalReport.Summary.PassRate*100,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Routing risk: %s\n", strings.ToUpper(string(result.RoutingRisk.OverallRisk))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Routing summary: %s\n", result.RoutingRisk.Summary); err != nil {
		return err
	}

	if len(result.EvalReport.Summary.NotableMisses) > 0 {
		if _, err := fmt.Fprintln(w, "Notable misses:"); err != nil {
			return err
		}
		for _, miss := range result.EvalReport.Summary.NotableMisses {
			if _, err := fmt.Fprintf(
				w,
				"- [%s] expected %s for %q, got trigger=%t\n",
				miss.ID,
				miss.Expectation,
				miss.Prompt,
				miss.ActualTrigger,
			); err != nil {
				return err
			}
		}
	}

	if !result.Correlation.HasEvidence() {
		if result.Correlation.Summary != "" {
			if _, err := fmt.Fprintf(w, "Correlation: %s\n", result.Correlation.Summary); err != nil {
				return err
			}
		}
		return nil
	}

	if _, err := fmt.Fprintln(w, "Lint/eval correlation:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- %s\n", result.Correlation.Summary); err != nil {
		return err
	}
	for _, group := range result.Correlation.MissGroups {
		if _, err := fmt.Fprintf(w, "- %s: %s\n", group.Title, group.Summary); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Eval cases: %s\n", strings.Join(group.SupportingEvalCaseIDs, ", ")); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Likely contributors: %s\n", contributorTitleList(group.LikelyContributors)); err != nil {
			return err
		}
	}
	if len(result.Correlation.PriorityActions) > 0 {
		if _, err := fmt.Fprintln(w, "Top improvement priorities:"); err != nil {
			return err
		}
		for _, action := range result.Correlation.PriorityActions {
			if _, err := fmt.Fprintf(w, "- %s\n", action); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeSkillAnalyzeJSON(w io.Writer, result service.SkillAnalyzeResult, options skillAnalyzeOptions) error {
	payload := skillAnalyzeJSONReport{
		SchemaVersion: skillAnalyzeJSONSchemaVersion,
		Target:        result.LintReport.Target,
		Profile:       options.profile,
		Strictness:    lint.Strictness(options.strictness).DisplayName(),
		FailOn:        options.failOn,
		Lint: skillAnalyzeJSONLint{
			Valid:        !result.LintReport.HasErrors(),
			ErrorCount:   result.LintReport.ErrorCount(),
			WarningCount: result.LintReport.WarningCount(),
			Findings:     make([]skillLintJSONFinding, 0, len(result.LintReport.Findings)),
			RoutingRisk:  &result.RoutingRisk,
			ActionAreas:  result.ActionAreas,
		},
		Eval: skillAnalyzeJSONEval{
			Suite:   result.EvalReport.Suite,
			Backend: result.EvalReport.Backend,
			Summary: result.EvalReport.Summary,
			Results: result.EvalReport.Results,
		},
	}
	if result.Correlation.Summary != "" || len(result.Correlation.MissGroups) > 0 {
		payload.Correlation = &result.Correlation
	}

	for _, finding := range result.LintReport.Findings {
		item := skillLintJSONFinding{
			RuleID:   finding.RuleID,
			Severity: string(finding.Severity),
			Path:     finding.Path,
			Message:  finding.Message,
		}
		if finding.Line > 0 {
			item.Line = &finding.Line
		}
		payload.Lint.Findings = append(payload.Lint.Findings, item)
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func contributorTitleList(contributors []analysis.LintEvalContributor) string {
	titles := make([]string, 0, len(contributors))
	for _, contributor := range contributors {
		titles = append(titles, contributor.Title)
	}
	return strings.Join(titles, ", ")
}

type skillAnalyzeJSONReport struct {
	SchemaVersion string                        `json:"schema_version"`
	Target        string                        `json:"target"`
	Profile       string                        `json:"profile"`
	Strictness    string                        `json:"strictness"`
	FailOn        string                        `json:"fail_on"`
	Lint          skillAnalyzeJSONLint          `json:"lint"`
	Eval          skillAnalyzeJSONEval          `json:"eval"`
	Correlation   *analysis.LintEvalCorrelation `json:"correlation,omitempty"`
}

type skillAnalyzeJSONLint struct {
	Valid        bool                     `json:"valid"`
	ErrorCount   int                      `json:"error_count"`
	WarningCount int                      `json:"warning_count"`
	RoutingRisk  *lint.RoutingRiskSummary `json:"routing_risk,omitempty"`
	ActionAreas  []lint.ActionArea        `json:"action_areas,omitempty"`
	Findings     []skillLintJSONFinding   `json:"findings"`
}

type skillAnalyzeJSONEval struct {
	Suite   domaineval.RoutingEvalSuiteInfo    `json:"suite"`
	Backend domaineval.RoutingEvalBackendInfo  `json:"backend"`
	Summary domaineval.RoutingEvalSummary      `json:"summary"`
	Results []domaineval.RoutingEvalCaseResult `json:"results"`
}
