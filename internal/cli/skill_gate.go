package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const skillGateJSONSchemaVersion = "1"

type skillGateOptions struct {
	format                      string
	base                        string
	profile                     string
	strictness                  string
	suite                       string
	runner                      string
	backends                    []string
	artifact                    string
	inputArtifacts              []string
	maxLintErrors               int
	maxLintWarnings             int
	maxRoutingRisk              string
	minEvalPassRate             float64
	maxFalsePositives           int
	maxFalseNegatives           int
	minPerBackendPassRate       float64
	maxBackendDisagreementRate  float64
	maxPassRateRegression       float64
	maxFalsePositiveIncrease    int
	maxFalseNegativeIncrease    int
	maxWidenedDisagreements     int
	failOnNewErrors             bool
	failOnNewPortabilityRegress bool
}

func newSkillGateCommand(application *app.App) *cobra.Command {
	options := skillGateOptions{
		format:                     skillLintFormatText,
		profile:                    string(service.SkillLintProfileGeneric),
		strictness:                 string(lint.StrictnessDefault),
		maxLintErrors:              -1,
		maxLintWarnings:            -1,
		minEvalPassRate:            -1,
		maxFalsePositives:          -1,
		maxFalseNegatives:          -1,
		minPerBackendPassRate:      -1,
		maxBackendDisagreementRate: -1,
		maxPassRateRegression:      -1,
		maxFalsePositiveIncrease:   -1,
		maxFalseNegativeIncrease:   -1,
		maxWidenedDisagreements:    -1,
	}

	cmd := &cobra.Command{
		Use:   "gate [path]",
		Short: "Evaluate a deterministic Firety quality gate",
		Long:  "Evaluate lint, eval, compare, and multi-backend evidence against a focused set of explicit CI or release criteria.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			target := ""
			if len(args) == 1 {
				target = args[0]
			} else if len(options.inputArtifacts) == 0 {
				target = "."
			}

			profile, err := service.ParseSkillLintProfile(options.profile)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			strictness, err := lint.ParseStrictness(options.strictness)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			backendSelections, err := parseSkillGateBackendSelections(options.backends)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			criteria, err := options.Criteria()
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			result, err := application.Services.SkillGate.Evaluate(target, service.SkillGateOptions{
				BasePath:          options.base,
				Profile:           profile,
				Strictness:        strictness,
				SuitePath:         options.suite,
				Runner:            options.runner,
				BackendSelections: backendSelections,
				InputArtifacts:    options.inputArtifacts,
				Criteria:          criteria,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillGateReport(cmd.OutOrStdout(), result.Gate, target, options.base, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if result.Gate.Decision == gate.DecisionFail {
				exitCode = ExitCodeLint
			}

			if options.artifact != "" {
				gateArtifact := artifact.BuildSkillGateArtifact(application.Version, result.Gate, artifact.SkillGateArtifactOptions{
					Format:         options.format,
					Target:         target,
					BaseTarget:     options.base,
					Profile:        options.profile,
					Strictness:     strictness.DisplayName(),
					SuitePath:      options.suite,
					Runner:         options.runner,
					Backends:       append([]string(nil), options.backends...),
					InputArtifacts: append([]string(nil), options.inputArtifacts...),
				}, exitCode)
				if err := artifact.WriteSkillGateArtifact(options.artifact, gateArtifact); err != nil {
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
	cmd.Flags().StringVar(&options.base, "base", "", "Base skill directory for compare-aware gating")
	cmd.Flags().StringVar(&options.profile, "profile", string(service.SkillLintProfileGeneric), "Routing profile: generic, codex, claude-code, copilot, or cursor")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness: default, strict, or pedantic")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Path to the local routing eval suite JSON file")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Path to the local routing eval runner executable")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection in the form "<id>" or "<id>=/path/to/runner"; repeat for multiple backends`)
	cmd.Flags().StringVar(&options.artifact, "artifact", "", "Write a versioned machine-readable gate artifact to the given file path")
	cmd.Flags().StringArrayVar(&options.inputArtifacts, "input-artifact", nil, "Use one or more existing Firety artifacts as gate evidence")
	cmd.Flags().IntVar(&options.maxLintErrors, "max-lint-errors", -1, "Maximum allowed lint errors (-1 disables; default blocks on lint errors)")
	cmd.Flags().IntVar(&options.maxLintWarnings, "max-lint-warnings", -1, "Maximum allowed lint warnings (-1 disables)")
	cmd.Flags().StringVar(&options.maxRoutingRisk, "max-routing-risk", "", "Maximum allowed routing risk: low, medium, or high")
	cmd.Flags().Float64Var(&options.minEvalPassRate, "min-eval-pass-rate", -1, "Minimum required eval pass rate as a percentage from 0 to 100 (-1 disables; default requires 100 when eval evidence is present)")
	cmd.Flags().IntVar(&options.maxFalsePositives, "max-false-positives", -1, "Maximum allowed false positives (-1 disables)")
	cmd.Flags().IntVar(&options.maxFalseNegatives, "max-false-negatives", -1, "Maximum allowed false negatives (-1 disables)")
	cmd.Flags().Float64Var(&options.minPerBackendPassRate, "min-per-backend-pass-rate", -1, "Minimum required per-backend pass rate as a percentage from 0 to 100 (-1 disables; default requires 100 when multi-backend evidence is present)")
	cmd.Flags().Float64Var(&options.maxBackendDisagreementRate, "max-backend-disagreement-rate", -1, "Maximum allowed backend disagreement rate as a percentage from 0 to 100 (-1 disables)")
	cmd.Flags().Float64Var(&options.maxPassRateRegression, "max-pass-rate-regression", -1, "Maximum allowed pass-rate regression versus base, in percentage points from 0 to 100 (-1 disables)")
	cmd.Flags().IntVar(&options.maxFalsePositiveIncrease, "max-false-positive-increase", -1, "Maximum allowed false-positive increase versus base (-1 disables)")
	cmd.Flags().IntVar(&options.maxFalseNegativeIncrease, "max-false-negative-increase", -1, "Maximum allowed false-negative increase versus base (-1 disables)")
	cmd.Flags().IntVar(&options.maxWidenedDisagreements, "max-widened-disagreements", -1, "Maximum allowed widened backend disagreements versus base (-1 disables)")
	cmd.Flags().BoolVar(&options.failOnNewErrors, "fail-on-new-errors", false, "Fail when the candidate introduces new or escalated error findings")
	cmd.Flags().BoolVar(&options.failOnNewPortabilityRegress, "fail-on-new-portability-regressions", false, "Fail when the candidate introduces new or escalated portability regressions")

	return cmd
}

func (o skillGateOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
	if _, err := service.ParseSkillLintProfile(o.profile); err != nil {
		return err
	}
	if _, err := lint.ParseStrictness(o.strictness); err != nil {
		return err
	}
	if len(o.backends) > 0 {
		if len(o.backends) < 2 {
			return fmt.Errorf("multi-backend gating requires at least two --backend values")
		}
		if o.runner != "" {
			return fmt.Errorf("--runner cannot be combined with --backend; use either single-backend eval evidence or multi-backend evidence")
		}
		if o.profile != string(service.SkillLintProfileGeneric) {
			return fmt.Errorf("--profile cannot be combined with --backend; multi-backend gating uses each backend's profile affinity")
		}
	}
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	if o.maxRoutingRisk != "" {
		if _, err := parseRoutingRiskLevel(o.maxRoutingRisk); err != nil {
			return err
		}
	}
	for _, item := range []struct {
		name  string
		value int
	}{
		{name: "max lint errors", value: o.maxLintErrors},
		{name: "max lint warnings", value: o.maxLintWarnings},
		{name: "max false positives", value: o.maxFalsePositives},
		{name: "max false negatives", value: o.maxFalseNegatives},
		{name: "max false positive increase", value: o.maxFalsePositiveIncrease},
		{name: "max false negative increase", value: o.maxFalseNegativeIncrease},
		{name: "max widened disagreements", value: o.maxWidenedDisagreements},
	} {
		if item.value < -1 {
			return fmt.Errorf("%s must be -1 or greater", item.name)
		}
	}
	for _, item := range []struct {
		name  string
		value float64
	}{
		{name: "min eval pass rate", value: o.minEvalPassRate},
		{name: "min per-backend pass rate", value: o.minPerBackendPassRate},
		{name: "max backend disagreement rate", value: o.maxBackendDisagreementRate},
		{name: "max pass rate regression", value: o.maxPassRateRegression},
	} {
		if item.value < -1 || item.value > 100 {
			return fmt.Errorf("%s must be between 0 and 100, or -1 to disable", item.name)
		}
	}
	return nil
}

func (o skillGateOptions) Criteria() (gate.Criteria, error) {
	criteria := gate.Criteria{}
	if o.maxLintErrors >= 0 {
		criteria.MaxLintErrors = intPointer(o.maxLintErrors)
	}
	if o.maxLintWarnings >= 0 {
		criteria.MaxLintWarnings = intPointer(o.maxLintWarnings)
	}
	if o.maxRoutingRisk != "" {
		level, err := parseRoutingRiskLevel(o.maxRoutingRisk)
		if err != nil {
			return gate.Criteria{}, err
		}
		criteria.MaxRoutingRisk = &level
	}
	if o.minEvalPassRate >= 0 {
		criteria.MinEvalPassRate = floatPointer(o.minEvalPassRate / 100)
	}
	if o.maxFalsePositives >= 0 {
		criteria.MaxFalsePositives = intPointer(o.maxFalsePositives)
	}
	if o.maxFalseNegatives >= 0 {
		criteria.MaxFalseNegatives = intPointer(o.maxFalseNegatives)
	}
	if o.minPerBackendPassRate >= 0 {
		criteria.MinPerBackendPassRate = floatPointer(o.minPerBackendPassRate / 100)
	}
	if o.maxBackendDisagreementRate >= 0 {
		criteria.MaxBackendDisagreementRate = floatPointer(o.maxBackendDisagreementRate / 100)
	}
	if o.maxPassRateRegression >= 0 {
		criteria.MaxPassRateRegression = floatPointer(o.maxPassRateRegression / 100)
	}
	if o.maxFalsePositiveIncrease >= 0 {
		criteria.MaxFalsePositiveIncrease = intPointer(o.maxFalsePositiveIncrease)
	}
	if o.maxFalseNegativeIncrease >= 0 {
		criteria.MaxFalseNegativeIncrease = intPointer(o.maxFalseNegativeIncrease)
	}
	if o.maxWidenedDisagreements >= 0 {
		criteria.MaxWidenedDisagreements = intPointer(o.maxWidenedDisagreements)
	}
	criteria.FailOnNewErrors = o.failOnNewErrors
	criteria.FailOnNewPortabilityRegress = o.failOnNewPortabilityRegress
	return criteria, nil
}

func writeSkillGateReport(w io.Writer, result gate.Result, target, base string, options skillGateOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillGateText(w, result, target, base)
	case skillLintFormatJSON:
		return writeSkillGateJSON(w, result, target, base, options)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillGateText(w io.Writer, result gate.Result, target, base string) error {
	if target != "" {
		if _, err := fmt.Fprintf(w, "Target: %s\n", target); err != nil {
			return err
		}
	}
	if base != "" {
		if _, err := fmt.Fprintf(w, "Base: %s\n", base); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Decision: %s\n", strings.ToUpper(string(result.Decision))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", result.Summary); err != nil {
		return err
	}
	if result.SupportingMetrics.Lint != nil {
		if _, err := fmt.Fprintf(w, "Lint: %d error(s), %d warning(s)\n", result.SupportingMetrics.Lint.ErrorCount, result.SupportingMetrics.Lint.WarningCount); err != nil {
			return err
		}
		if result.SupportingMetrics.Lint.RoutingRisk != nil {
			if _, err := fmt.Fprintf(w, "Routing risk: %s\n", strings.ToUpper(string(*result.SupportingMetrics.Lint.RoutingRisk))); err != nil {
				return err
			}
		}
	}
	if result.SupportingMetrics.Eval != nil {
		if _, err := fmt.Fprintf(
			w,
			"Eval: %d failed, %d false positive(s), %d false negative(s), %.0f%% pass rate\n",
			result.SupportingMetrics.Eval.Failed,
			result.SupportingMetrics.Eval.FalsePositives,
			result.SupportingMetrics.Eval.FalseNegatives,
			result.SupportingMetrics.Eval.PassRate*100,
		); err != nil {
			return err
		}
	}
	if result.SupportingMetrics.MultiBackend != nil {
		if _, err := fmt.Fprintf(w, "Multi-backend: %d backend(s)", result.SupportingMetrics.MultiBackend.BackendCount); err != nil {
			return err
		}
		if result.SupportingMetrics.MultiBackend.DisagreementRate != nil {
			if _, err := fmt.Fprintf(w, ", %.0f%% disagreement", *result.SupportingMetrics.MultiBackend.DisagreementRate*100); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if len(result.BlockingReasons) > 0 {
		if _, err := fmt.Fprintln(w, "Blocking reasons:"); err != nil {
			return err
		}
		for _, reason := range result.BlockingReasons {
			if _, err := fmt.Fprintf(w, "- %s\n", reason.Summary); err != nil {
				return err
			}
		}
	}
	if len(result.Warnings) > 0 {
		if _, err := fmt.Fprintln(w, "Warnings:"); err != nil {
			return err
		}
		for _, reason := range result.Warnings {
			if _, err := fmt.Fprintf(w, "- %s\n", reason.Summary); err != nil {
				return err
			}
		}
	}
	if len(result.PerBackendResults) > 0 {
		if _, err := fmt.Fprintln(w, "Per backend:"); err != nil {
			return err
		}
		for _, backend := range result.PerBackendResults {
			if _, err := fmt.Fprintf(w, "- %s: %s (%.0f%% pass rate)\n", backend.BackendName, strings.ToUpper(string(backend.Decision)), backend.PassRate*100); err != nil {
				return err
			}
		}
	}
	if result.NextAction != "" {
		if _, err := fmt.Fprintf(w, "Next action: %s\n", result.NextAction); err != nil {
			return err
		}
	}
	return nil
}

func writeSkillGateJSON(w io.Writer, result gate.Result, target, base string, options skillGateOptions) error {
	payload := struct {
		SchemaVersion string      `json:"schema_version"`
		Target        string      `json:"target,omitempty"`
		BaseTarget    string      `json:"base_target,omitempty"`
		Profile       string      `json:"profile"`
		Strictness    string      `json:"strictness"`
		Result        gate.Result `json:"result"`
	}{
		SchemaVersion: skillGateJSONSchemaVersion,
		Target:        target,
		BaseTarget:    base,
		Profile:       options.profile,
		Strictness:    options.strictness,
		Result:        result,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func parseRoutingRiskLevel(raw string) (lint.RoutingRiskLevel, error) {
	switch lint.RoutingRiskLevel(raw) {
	case lint.RoutingRiskLow, lint.RoutingRiskMedium, lint.RoutingRiskHigh:
		return lint.RoutingRiskLevel(raw), nil
	default:
		return "", fmt.Errorf("invalid max-routing-risk value %q: must be one of low, medium, high", raw)
	}
}

func parseSkillGateBackendSelections(values []string) ([]service.SkillEvalBackendSelection, error) {
	if len(values) == 0 {
		return nil, nil
	}
	return parseSkillEvalBackendSelections(values)
}

func intPointer(value int) *int {
	return &value
}

func floatPointer(value float64) *float64 {
	return &value
}
