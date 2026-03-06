package cli

import (
	"fmt"
	"io"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/firety/firety/internal/trustreport"
	"github.com/spf13/cobra"
)

type publishReportOptions struct {
	output         string
	inputArtifacts []string
	inputPacks     []string
	profile        string
	strictness     string
	failOn         string
	explain        bool
	routingRisk    bool
	runner         string
	suite          string
	backends       []string
	includePlan    bool
	includeGate    bool
}

func newPublishCommand(application *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Build publishable Firety outputs",
		Long:  "Generate static Firety outputs for releases, offline review, and publishing workflows without requiring a hosted service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newPublishReportCommand(application))
	return cmd
}

func newPublishReportCommand(application *app.App) *cobra.Command {
	options := publishReportOptions{
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
		failOn:     "errors",
	}

	cmd := &cobra.Command{
		Use:   "report [path]",
		Short: "Build a static Firety trust-report bundle",
		Long:  "Generate a deterministic static Firety trust-report bundle from fresh analysis, saved artifacts, or existing evidence packs.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(args); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			target := ""
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
			backends, err := parseSkillEvalBackendSelections(options.backends)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			builder := trustreport.NewBuilder(application)
			result, err := builder.Build(target, trustreport.BuildOptions{
				OutputDir:         options.output,
				InputArtifacts:    append([]string(nil), options.inputArtifacts...),
				InputPacks:        append([]string(nil), options.inputPacks...),
				Profile:           profile,
				Strictness:        strictness,
				FailOn:            options.failOn,
				Explain:           options.explain,
				RoutingRisk:       options.routingRisk,
				Runner:            options.runner,
				SuitePath:         options.suite,
				BackendSelections: backends,
				IncludePlan:       options.includePlan,
				IncludeGate:       options.includeGate,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writePublishReportText(cmd.OutOrStdout(), result); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.output, "output", "", "Write the trust report to this directory")
	cmd.Flags().StringArrayVar(&options.inputArtifacts, "input-artifact", nil, "Existing Firety artifact to publish; repeat for multiple artifacts")
	cmd.Flags().StringArrayVar(&options.inputPacks, "input-pack", nil, "Existing Firety evidence pack to publish; repeat for multiple packs")
	cmd.Flags().StringVar(&options.profile, "profile", string(service.SkillLintProfileGeneric), "Lint profile for fresh trust-report generation")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness for fresh trust-report generation")
	cmd.Flags().StringVar(&options.failOn, "fail-on", "errors", "Lint fail policy for fresh trust-report generation: errors or warnings")
	cmd.Flags().BoolVar(&options.explain, "explain", false, "Include explain-mode metadata in fresh trust-report evidence")
	cmd.Flags().BoolVar(&options.routingRisk, "routing-risk", false, "Include routing-risk summaries in fresh trust-report evidence")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Runner path for fresh single-backend eval evidence")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Routing eval suite path for fresh eval evidence")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection for fresh multi-backend eval evidence, in the form "<id>" or "<id>=/path/to/runner"`)
	cmd.Flags().BoolVar(&options.includePlan, "include-plan", false, "Include a derived improvement-plan artifact in fresh trust reports")
	cmd.Flags().BoolVar(&options.includeGate, "include-gate", false, "Include a derived quality-gate artifact in fresh trust reports")

	return cmd
}

func (o publishReportOptions) Validate(args []string) error {
	if o.output == "" {
		return fmt.Errorf("--output is required")
	}
	if o.output == "-" {
		return fmt.Errorf(`output path "-" is not supported; choose a directory path`)
	}
	hasInputs := len(o.inputArtifacts) > 0 || len(o.inputPacks) > 0
	if hasInputs && len(args) > 0 {
		return fmt.Errorf("trust report accepts either a target path or existing artifact/pack inputs, not both")
	}
	if !hasInputs && len(args) == 0 {
		return fmt.Errorf("trust report requires a target path or at least one --input-artifact/--input-pack value")
	}
	if o.failOn != "errors" && o.failOn != "warnings" {
		return fmt.Errorf(`invalid fail-on %q: must be one of errors, warnings`, o.failOn)
	}
	if len(o.backends) > 0 && o.runner != "" {
		return fmt.Errorf("--runner cannot be combined with --backend")
	}
	return nil
}

func writePublishReportText(w io.Writer, result trustreport.Result) error {
	if _, err := fmt.Fprintf(w, "Trust report: %s\n", result.OutputDir); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Entrypoint: index.html"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", result.Manifest.Summary); err != nil {
		return err
	}
	if result.Manifest.SupportPosture != "" {
		if _, err := fmt.Fprintf(w, "Support posture: %s\n", result.Manifest.SupportPosture); err != nil {
			return err
		}
	}
	if result.Manifest.QualityGateDecision != "" {
		if _, err := fmt.Fprintf(w, "Quality gate: %s\n", result.Manifest.QualityGateDecision); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Pages: %d\n", len(result.Manifest.Pages)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Artifacts: %d\n", len(result.Manifest.Artifacts)); err != nil {
		return err
	}
	if len(result.Manifest.RecommendedEntrypoints) > 0 {
		if _, err := fmt.Fprintln(w, "Read first:"); err != nil {
			return err
		}
		for _, item := range result.Manifest.RecommendedEntrypoints {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	return nil
}
