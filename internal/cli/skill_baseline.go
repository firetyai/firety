package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/baseline"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const skillBaselineCompareJSONSchemaVersion = "1"

type skillBaselineSaveOptions struct {
	output         string
	profile        string
	strictness     string
	suite          string
	runner         string
	backends       []string
	inputArtifacts []string
}

type skillBaselineCompareOptions struct {
	baseline string
	format   string
	failOn   string
	runner   string
	backends []string
	artifact string
}

func newSkillBaselineCommand(application *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "Manage saved Firety baseline snapshots",
		Long:  "Save, update, and compare against explicit Firety baseline snapshots for long-lived regression workflows.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newSkillBaselineSaveCommand(application))
	cmd.AddCommand(newSkillBaselineUpdateCommand(application))
	cmd.AddCommand(newSkillBaselineCompareCommand(application))

	return cmd
}

func newSkillBaselineSaveCommand(application *app.App) *cobra.Command {
	options := skillBaselineSaveOptions{
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
	}

	cmd := &cobra.Command{
		Use:   "save [path]",
		Short: "Save a Firety baseline snapshot",
		Long:  "Save a baseline snapshot artifact from a fresh Firety run or from existing compatible artifacts.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := executeSkillBaselineSave(cmd.OutOrStdout(), application, args, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&options.output, "output", "", "Path to write the baseline snapshot artifact")
	cmd.Flags().StringVar(&options.profile, "profile", string(service.SkillLintProfileGeneric), "Routing profile for fresh baseline snapshots")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness for fresh baseline snapshots")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Path to the local routing eval suite JSON file for fresh baseline snapshots")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Path to the local routing eval runner executable for fresh single-backend baseline snapshots")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection in the form "<id>" or "<id>=/path/to/runner"; repeat for fresh multi-backend baseline snapshots`)
	cmd.Flags().StringArrayVar(&options.inputArtifacts, "input-artifact", nil, "Create the baseline snapshot from existing compatible Firety artifacts instead of rerunning analysis")

	return cmd
}

func newSkillBaselineUpdateCommand(application *app.App) *cobra.Command {
	options := skillBaselineSaveOptions{
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
	}
	var baselinePath string

	cmd := &cobra.Command{
		Use:   "update [path]",
		Short: "Update an existing Firety baseline snapshot",
		Long:  "Rebuild and overwrite an existing baseline snapshot artifact using the current skill version or current compatible artifacts.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if baselinePath == "" {
				return newExitError(ExitCodeRuntime, fmt.Errorf("baseline path must not be empty"))
			}
			options.output = baselinePath
			if err := executeSkillBaselineSave(cmd.OutOrStdout(), application, args, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&baselinePath, "baseline", "", "Existing baseline snapshot path to overwrite")
	cmd.Flags().StringVar(&options.profile, "profile", string(service.SkillLintProfileGeneric), "Routing profile for fresh baseline snapshots")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness for fresh baseline snapshots")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Path to the local routing eval suite JSON file for fresh baseline snapshots")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Path to the local routing eval runner executable for fresh single-backend baseline snapshots")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection in the form "<id>" or "<id>=/path/to/runner"; repeat for fresh multi-backend baseline snapshots`)
	cmd.Flags().StringArrayVar(&options.inputArtifacts, "input-artifact", nil, "Create the baseline snapshot from existing compatible Firety artifacts instead of rerunning analysis")

	return cmd
}

func newSkillBaselineCompareCommand(application *app.App) *cobra.Command {
	options := skillBaselineCompareOptions{
		format: skillLintFormatText,
		failOn: skillLintFailOnErrors,
	}

	cmd := &cobra.Command{
		Use:   "compare [path]",
		Short: "Compare the current skill against a saved baseline",
		Long:  "Compare the current skill against an explicit saved Firety baseline snapshot instead of supplying a second live path.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			target := "."
			if len(args) == 1 {
				target = args[0]
			}

			backendSelections, err := parseSkillEvalBackendSelections(options.backends)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			result, err := application.Services.SkillBaseline.Compare(target, service.SkillBaselineCompareOptions{
				BaselinePath:      options.baseline,
				Runner:            options.runner,
				BackendSelections: backendSelections,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillBaselineCompareReport(cmd.OutOrStdout(), result, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := skillBaselineCompareExitCode(result, options.failOn)
			if options.artifact != "" {
				compareArtifact := artifact.BuildSkillBaselineCompareArtifact(application.Version, result.Comparison, artifact.SkillBaselineCompareArtifactOptions{
					Format:       options.format,
					BaselinePath: options.baseline,
				}, exitCode)
				if err := artifact.WriteSkillBaselineCompareArtifact(options.artifact, compareArtifact); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}
			if exitCode == ExitCodeLint {
				return newExitError(ExitCodeLint, nil)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.baseline, "baseline", "", "Path to the saved baseline snapshot artifact")
	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	cmd.Flags().StringVar(&options.failOn, "fail-on", skillLintFailOnErrors, "Fail policy based on the current skill: errors or warnings")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Override runner path when the baseline contains single-backend eval context")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Override backend selection in the form "<id>" or "<id>=/path/to/runner" when the baseline contains multi-backend eval context`)
	cmd.Flags().StringVar(&options.artifact, "artifact", "", "Write a versioned machine-readable baseline compare artifact to the given file path")

	return cmd
}

func (o skillBaselineSaveOptions) Validate() error {
	if o.output == "" {
		return fmt.Errorf("output path must not be empty")
	}
	if o.output == "-" {
		return fmt.Errorf(`output path "-" is not supported; choose a file path`)
	}
	if len(o.backends) > 0 {
		if len(o.backends) < 2 {
			return fmt.Errorf("multi-backend baseline snapshots require at least two --backend values")
		}
		if o.runner != "" {
			return fmt.Errorf("--runner cannot be combined with --backend; use either single-backend or multi-backend baseline capture")
		}
	}
	if len(o.inputArtifacts) > 0 && (o.runner != "" || len(o.backends) > 0 || o.suite != "" || o.profile != string(service.SkillLintProfileGeneric) || o.strictness != string(lint.StrictnessDefault)) {
		return fmt.Errorf("artifact-based baseline save cannot be combined with fresh-run profile, strictness, suite, runner, or backend flags")
	}
	if _, err := service.ParseSkillLintProfile(o.profile); err != nil {
		return err
	}
	if _, err := lint.ParseStrictness(o.strictness); err != nil {
		return err
	}
	return nil
}

func (o skillBaselineCompareOptions) Validate() error {
	if o.baseline == "" {
		return fmt.Errorf("baseline path must not be empty")
	}
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
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	if len(o.backends) == 1 {
		return fmt.Errorf("baseline compare requires at least two --backend values when overriding multi-backend context")
	}
	if len(o.backends) > 0 && o.runner != "" {
		return fmt.Errorf("--runner cannot be combined with --backend")
	}
	return nil
}

func writeSkillBaselineSaveText(w io.Writer, snapshot baseline.Snapshot, outputPath string, fromArtifacts bool) error {
	action := "Saved"
	if fromArtifacts {
		action = "Saved"
	}
	if _, err := fmt.Fprintf(w, "%s baseline snapshot to %s\n", action, outputPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", snapshot.Summary.Summary); err != nil {
		return err
	}
	return nil
}

func executeSkillBaselineSave(w io.Writer, application *app.App, args []string, options skillBaselineSaveOptions) error {
	if err := options.Validate(); err != nil {
		return err
	}

	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	if len(options.inputArtifacts) > 0 && len(args) == 0 {
		target = ""
	}

	return runSkillBaselineSave(w, application, target, options)
}

func runSkillBaselineSave(w io.Writer, application *app.App, target string, options skillBaselineSaveOptions) error {
	profile, err := service.ParseSkillLintProfile(options.profile)
	if err != nil {
		return err
	}
	strictness, err := lint.ParseStrictness(options.strictness)
	if err != nil {
		return err
	}
	backendSelections, err := parseSkillEvalBackendSelections(options.backends)
	if err != nil {
		return err
	}

	result, err := application.Services.SkillBaseline.Save(target, service.SkillBaselineSaveOptions{
		Profile:           profile,
		Strictness:        strictness,
		SuitePath:         options.suite,
		Runner:            options.runner,
		BackendSelections: backendSelections,
		InputArtifacts:    options.inputArtifacts,
	})
	if err != nil {
		return err
	}

	snapshotArtifact := artifact.BuildSkillBaselineSnapshotArtifact(application.Version, result.Snapshot)
	if err := artifact.WriteSkillBaselineSnapshotArtifact(options.output, snapshotArtifact); err != nil {
		return err
	}

	return writeSkillBaselineSaveText(w, result.Snapshot, options.output, len(options.inputArtifacts) > 0)
}

func writeSkillBaselineCompareReport(w io.Writer, result service.SkillBaselineCompareResult, options skillBaselineCompareOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillBaselineCompareText(w, result)
	case skillLintFormatJSON:
		return writeSkillBaselineCompareJSON(w, result, options.baseline)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillBaselineCompareText(w io.Writer, result service.SkillBaselineCompareResult) error {
	if _, err := fmt.Fprintf(w, "Baseline: %s\n", result.Baseline.Context.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Current: %s\n", result.Current.Context.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Overall: %s\n", strings.ToUpper(string(result.Comparison.Summary.Overall))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", result.Comparison.Summary.Summary); err != nil {
		return err
	}
	if len(result.Comparison.Summary.Components) > 0 {
		if _, err := fmt.Fprintln(w, "Components:"); err != nil {
			return err
		}
		for _, component := range result.Comparison.Summary.Components {
			if _, err := fmt.Fprintf(w, "- %s: %s\n", component.Title, component.Summary); err != nil {
				return err
			}
		}
	}
	if len(result.Comparison.Summary.HighPriorityRegressions) > 0 {
		if _, err := fmt.Fprintln(w, "Top regressions:"); err != nil {
			return err
		}
		for _, item := range result.Comparison.Summary.HighPriorityRegressions {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(result.Comparison.Summary.NotableImprovements) > 0 {
		if _, err := fmt.Fprintln(w, "Notable improvements:"); err != nil {
			return err
		}
		for _, item := range result.Comparison.Summary.NotableImprovements {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if result.Comparison.Summary.UpdateRecommendation != "" {
		if _, err := fmt.Fprintf(w, "Recommendation: %s\n", result.Comparison.Summary.UpdateRecommendation); err != nil {
			return err
		}
	}
	return nil
}

func writeSkillBaselineCompareJSON(w io.Writer, result service.SkillBaselineCompareResult, baselinePath string) error {
	payload := struct {
		SchemaVersion string              `json:"schema_version"`
		BaselinePath  string              `json:"baseline_path"`
		Comparison    baseline.Comparison `json:"comparison"`
	}{
		SchemaVersion: skillBaselineCompareJSONSchemaVersion,
		BaselinePath:  baselinePath,
		Comparison:    result.Comparison,
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func skillBaselineCompareExitCode(result service.SkillBaselineCompareResult, failOn string) int {
	if shouldFailSkillLint(result.Current.LintReport, failOn) {
		return ExitCodeLint
	}
	if result.Current.EvalReport != nil && result.Current.EvalReport.Summary.Failed > 0 {
		return ExitCodeLint
	}
	if result.Current.MultiBackendEval != nil {
		for _, backend := range result.Current.MultiBackendEval.Backends {
			if backend.Summary.Failed > 0 {
				return ExitCodeLint
			}
		}
	}
	return ExitCodeOK
}
