package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/analysis"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const skillPlanJSONSchemaVersion = "1"

type skillPlanOptions struct {
	format     string
	failOn     string
	profile    string
	strictness string
	suite      string
	runner     string
	backends   []string
	artifact   string
}

func newSkillPlanCommand(application *app.App) *cobra.Command {
	options := skillPlanOptions{
		format:     skillLintFormatText,
		failOn:     skillLintFailOnErrors,
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
	}

	cmd := &cobra.Command{
		Use:   "plan [path]",
		Short: "Build a short prioritized skill improvement plan",
		Long:  "Synthesize Firety lint findings, optional eval misses, correlations, and backend differences into a concise prioritized remediation plan.",
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

			var backendSelections []service.SkillEvalBackendSelection
			if len(options.backends) > 0 {
				backendSelections, err = parseSkillEvalBackendSelections(options.backends)
				if err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}

			result, err := application.Services.SkillPlan.Build(target, service.SkillPlanOptions{
				Profile:           profile,
				Strictness:        strictness,
				SuitePath:         options.suite,
				Runner:            options.runner,
				BackendSelections: backendSelections,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillPlanReport(cmd.OutOrStdout(), result, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if shouldFailSkillLint(result.LintReport, options.failOn) {
				exitCode = ExitCodeLint
			}
			if result.EvalReport != nil && result.EvalReport.Summary.Failed > 0 {
				exitCode = ExitCodeLint
			}
			if result.MultiBackendEval != nil {
				for _, backend := range result.MultiBackendEval.Backends {
					if backend.Summary.Failed > 0 {
						exitCode = ExitCodeLint
						break
					}
				}
			}

			if options.artifact != "" {
				planArtifact := artifact.BuildSkillPlanArtifact(application.Version, result, artifact.SkillPlanArtifactOptions{
					Format:     options.format,
					Profile:    options.profile,
					Strictness: options.strictness,
					FailOn:     options.failOn,
					Suite:      options.suite,
				}, exitCode)
				if err := artifact.WriteSkillPlanArtifact(options.artifact, planArtifact); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}

			if exitCode == ExitCodeLint {
				return newExitError(ExitCodeLint, nil)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	cmd.Flags().StringVar(&options.failOn, "fail-on", skillLintFailOnErrors, "Fail policy: errors or warnings")
	cmd.Flags().StringVar(&options.profile, "profile", string(service.SkillLintProfileGeneric), "Routing profile: generic, codex, claude-code, copilot, or cursor")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness: default, strict, or pedantic")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Path to the local routing eval suite JSON file")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Path to the local routing eval runner executable")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection for multi-backend evidence, in the form "<id>" or "<id>=/path/to/runner"; repeat the flag for multiple backends`)
	cmd.Flags().StringVar(&options.artifact, "artifact", "", "Write a versioned machine-readable plan artifact to the given file path")

	return cmd
}

func (o skillPlanOptions) Validate() error {
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
	if len(o.backends) > 0 {
		if len(o.backends) < 2 {
			return fmt.Errorf("multi-backend planning requires at least two --backend values; omit --backend for lint-only or single-run planning")
		}
		if o.runner != "" {
			return fmt.Errorf("--runner cannot be combined with --backend in plan mode")
		}
		if o.profile != string(service.SkillLintProfileGeneric) {
			return fmt.Errorf("--profile cannot be combined with --backend in plan mode; multi-backend evidence uses each backend's profile affinity")
		}
	} else {
		if _, err := service.ParseSkillLintProfile(o.profile); err != nil {
			return err
		}
	}
	if _, err := lint.ParseStrictness(o.strictness); err != nil {
		return err
	}
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	return nil
}

func writeSkillPlanReport(w io.Writer, result service.SkillPlanResult, options skillPlanOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillPlanText(w, result)
	case skillLintFormatJSON:
		return writeSkillPlanJSON(w, result, options)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillPlanText(w io.Writer, result service.SkillPlanResult) error {
	if _, err := fmt.Fprintf(w, "Target: %s\n", result.LintReport.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Plan summary: %s\n", result.Plan.Summary); err != nil {
		return err
	}
	if !result.Plan.HasPriorities() {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Top priorities:"); err != nil {
		return err
	}
	for _, item := range result.Plan.Priorities {
		if _, err := fmt.Fprintf(w, "- %s\n", item.Title); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Why: %s\n", item.WhyItMatters); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Improve: %s\n", item.WhatToImprove); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Impact: %s\n", joinStrings(item.ImpactAreas)); err != nil {
			return err
		}
		if evidence := item.EvidenceSummary(); evidence != "" {
			if _, err := fmt.Fprintf(w, "  Evidence: %s\n", evidence); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeSkillPlanJSON(w io.Writer, result service.SkillPlanResult, options skillPlanOptions) error {
	payload := skillPlanJSONReport{
		SchemaVersion: skillPlanJSONSchemaVersion,
		Target:        result.LintReport.Target,
		Profile:       options.profile,
		Strictness:    lint.Strictness(options.strictness).DisplayName(),
		FailOn:        options.failOn,
		LintSummary: skillPlanJSONLintSummary{
			ErrorCount:   result.LintReport.ErrorCount(),
			WarningCount: result.LintReport.WarningCount(),
		},
		RoutingRisk: result.RoutingRisk,
		ActionAreas: result.ActionAreas,
		Plan:        result.Plan,
	}
	if result.EvalReport != nil {
		payload.EvalSummary = &result.EvalReport.Summary
	}
	if result.Correlation.HasEvidence() || result.Correlation.Summary != "" {
		payload.Correlation = &result.Correlation
	}
	if result.MultiBackendEval != nil {
		payload.MultiBackendEval = &result.MultiBackendEval.Summary
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

type skillPlanJSONReport struct {
	SchemaVersion    string                              `json:"schema_version"`
	Target           string                              `json:"target"`
	Profile          string                              `json:"profile"`
	Strictness       string                              `json:"strictness"`
	FailOn           string                              `json:"fail_on"`
	LintSummary      skillPlanJSONLintSummary            `json:"lint_summary"`
	RoutingRisk      lint.RoutingRiskSummary             `json:"routing_risk"`
	ActionAreas      []lint.ActionArea                   `json:"action_areas,omitempty"`
	EvalSummary      *domaineval.RoutingEvalSummary      `json:"eval_summary,omitempty"`
	Correlation      *analysis.LintEvalCorrelation       `json:"correlation,omitempty"`
	MultiBackendEval *domaineval.MultiBackendEvalSummary `json:"multi_backend_eval,omitempty"`
	Plan             analysis.ImprovementPlan            `json:"plan"`
}

type skillPlanJSONLintSummary struct {
	ErrorCount   int `json:"error_count"`
	WarningCount int `json:"warning_count"`
}

func joinStrings(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	default:
		return values[0] + ", " + values[1] + func() string {
			if len(values) == 2 {
				return ""
			}
			return ", " + values[2]
		}()
	}
}
