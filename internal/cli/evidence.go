package cli

import (
	"fmt"
	"io"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/evidencepack"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

type evidencePackOptions struct {
	output               string
	inputArtifacts       []string
	profile              string
	strictness           string
	failOn               string
	explain              bool
	routingRisk          bool
	runner               string
	suite                string
	backends             []string
	includePlan          bool
	includeCompatibility bool
	includeGate          bool
}

func newEvidenceCommand(application *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: "Package Firety quality evidence for offline review and publishing",
		Long:  "Generate deterministic Firety evidence packs from fresh analysis or existing artifacts for CI, releases, and offline review.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newEvidencePackCommand(application))

	return cmd
}

func newEvidencePackCommand(application *app.App) *cobra.Command {
	options := evidencePackOptions{
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
		failOn:     "errors",
	}

	cmd := &cobra.Command{
		Use:   "pack [path]",
		Short: "Build a deterministic Firety evidence pack",
		Long:  "Build a deterministic Firety evidence pack directory containing selected artifacts, rendered summaries, and a manifest for offline review or publishing workflows.",
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

			builder := evidencepack.NewBuilder(application)
			result, err := builder.Build(target, evidencepack.PackOptions{
				OutputDir:            options.output,
				InputArtifacts:       append([]string(nil), options.inputArtifacts...),
				Profile:              profile,
				Strictness:           strictness,
				FailOn:               options.failOn,
				Explain:              options.explain,
				RoutingRisk:          options.routingRisk,
				Runner:               options.runner,
				SuitePath:            options.suite,
				BackendSelections:    backends,
				IncludePlan:          options.includePlan,
				IncludeCompatibility: options.includeCompatibility,
				IncludeGate:          options.includeGate,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeEvidencePackText(cmd.OutOrStdout(), result); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.output, "output", "", "Write the evidence pack to this directory")
	cmd.Flags().StringArrayVar(&options.inputArtifacts, "input-artifact", nil, "Existing Firety artifact to package; repeat for multiple artifacts")
	cmd.Flags().StringVar(&options.profile, "profile", string(service.SkillLintProfileGeneric), "Lint profile for fresh pack generation")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness for fresh pack generation")
	cmd.Flags().StringVar(&options.failOn, "fail-on", "errors", "Lint fail policy for fresh pack generation: errors or warnings")
	cmd.Flags().BoolVar(&options.explain, "explain", false, "Include explain-mode metadata in fresh lint artifacts")
	cmd.Flags().BoolVar(&options.routingRisk, "routing-risk", false, "Include routing-risk summaries in fresh lint artifacts")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Runner path for fresh single-backend eval evidence")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Routing eval suite path for fresh eval evidence")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection for fresh multi-backend eval evidence, in the form "<id>" or "<id>=/path/to/runner"`)
	cmd.Flags().BoolVar(&options.includePlan, "include-plan", false, "Include a derived improvement-plan artifact in fresh packs")
	cmd.Flags().BoolVar(&options.includeCompatibility, "include-compatibility", false, "Include a derived compatibility artifact")
	cmd.Flags().BoolVar(&options.includeGate, "include-gate", false, "Include a derived quality-gate artifact")

	return cmd
}

func (o evidencePackOptions) Validate(args []string) error {
	if o.output == "" {
		return fmt.Errorf("--output is required")
	}
	if o.output == "-" {
		return fmt.Errorf(`output path "-" is not supported; choose a directory path`)
	}
	if len(o.inputArtifacts) > 0 && len(args) > 0 {
		return fmt.Errorf("evidence pack accepts either a target path or --input-artifact values, not both")
	}
	if len(o.inputArtifacts) == 0 && len(args) == 0 {
		return fmt.Errorf("evidence pack requires a target path or at least one --input-artifact value")
	}
	if o.failOn != "errors" && o.failOn != "warnings" {
		return fmt.Errorf(`invalid fail-on %q: must be one of errors, warnings`, o.failOn)
	}
	if len(o.backends) > 0 && o.runner != "" {
		return fmt.Errorf("--runner cannot be combined with --backend")
	}
	return nil
}

func writeEvidencePackText(w io.Writer, result evidencepack.Result) error {
	if _, err := fmt.Fprintf(w, "Evidence pack: %s\n", result.OutputDir); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Source: %s\n", result.Manifest.Source); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", result.Manifest.ReviewSummary); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Artifacts: %d\n", len(result.Manifest.Artifacts)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Reports: %d\n", len(result.Manifest.Reports)); err != nil {
		return err
	}
	if len(result.Manifest.RecommendedEntrypoints) > 0 {
		if _, err := fmt.Fprintln(w, "Review first:"); err != nil {
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
