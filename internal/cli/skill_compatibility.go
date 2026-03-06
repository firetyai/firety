package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/compatibility"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const skillCompatibilityJSONSchemaVersion = "1"

type skillCompatibilityOptions struct {
	format         string
	profiles       []string
	strictness     string
	suite          string
	backends       []string
	inputArtifacts []string
	artifact       string
}

func newSkillCompatibilityCommand(application *app.App) *cobra.Command {
	options := skillCompatibilityOptions{
		format:     skillLintFormatText,
		strictness: string(lint.StrictnessDefault),
	}

	cmd := &cobra.Command{
		Use:   "compatibility [path]",
		Short: "Summarize skill compatibility and support posture",
		Long:  "Synthesize Firety lint, routing, and backend evidence into a concise compatibility matrix and support-posture summary.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			if len(options.inputArtifacts) > 0 && len(args) == 0 {
				target = ""
			}

			strictness, err := lint.ParseStrictness(options.strictness)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			profiles, err := service.ParseSkillLintProfiles(options.profiles)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			backends, err := parseSkillEvalBackendSelections(options.backends)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			result, err := application.Services.SkillCompatibility.Analyze(target, service.SkillCompatibilityOptions{
				Profiles:       profiles,
				Strictness:     strictness,
				SuitePath:      options.suite,
				Backends:       backends,
				InputArtifacts: append([]string(nil), options.inputArtifacts...),
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillCompatibilityReport(cmd.OutOrStdout(), result.Report, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if options.artifact != "" {
				value := artifact.BuildSkillCompatibilityArtifact(application.Version, result.Report, artifact.SkillCompatibilityArtifactOptions{
					Format:         options.format,
					Target:         target,
					Profiles:       append([]string(nil), options.profiles...),
					Strictness:     strictness.DisplayName(),
					SuitePath:      options.suite,
					Backends:       append([]string(nil), options.backends...),
					InputArtifacts: append([]string(nil), options.inputArtifacts...),
				})
				if err := artifact.WriteSkillCompatibilityArtifact(options.artifact, value); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	cmd.Flags().StringArrayVar(&options.profiles, "profile", nil, "Compatibility profile to evaluate; repeat for multiple profiles")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness: default, strict, or pedantic")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Path to the local routing eval suite JSON file for backend evidence")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection in the form "<id>" or "<id>=/path/to/runner"; repeat for backend compatibility checks`)
	cmd.Flags().StringArrayVar(&options.inputArtifacts, "input-artifact", nil, "Use existing compatible Firety artifacts instead of rerunning analysis")
	cmd.Flags().StringVar(&options.artifact, "artifact", "", "Write a versioned machine-readable compatibility artifact to the given file path")

	return cmd
}

func (o skillCompatibilityOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	if _, err := lint.ParseStrictness(o.strictness); err != nil {
		return err
	}
	if _, err := service.ParseSkillLintProfiles(o.profiles); err != nil {
		return err
	}
	if len(o.inputArtifacts) > 0 && (len(o.backends) > 0 || o.suite != "" || len(o.profiles) > 0 || o.strictness != string(lint.StrictnessDefault)) {
		return fmt.Errorf("artifact-based compatibility analysis cannot be combined with fresh-run profile, strictness, suite, or backend flags")
	}
	return nil
}

func writeSkillCompatibilityReport(w io.Writer, report compatibility.Report, options skillCompatibilityOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillCompatibilityText(w, report)
	case skillLintFormatJSON:
		return writeSkillCompatibilityJSON(w, report)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillCompatibilityText(w io.Writer, report compatibility.Report) error {
	if report.Target != "" {
		if _, err := fmt.Fprintf(w, "Target: %s\n", report.Target); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Support posture: %s\n", strings.ToUpper(string(report.SupportPosture))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Evidence: %s\n", strings.ToUpper(string(report.EvidenceLevel))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", report.Summary); err != nil {
		return err
	}
	if len(report.Profiles) > 0 {
		if _, err := fmt.Fprintln(w, "Profiles:"); err != nil {
			return err
		}
		for _, profile := range report.Profiles {
			if _, err := fmt.Fprintf(w, "- %s: %s\n", profile.DisplayName, profile.Summary); err != nil {
				return err
			}
		}
	}
	if len(report.Backends) > 0 {
		if _, err := fmt.Fprintln(w, "Backends:"); err != nil {
			return err
		}
		for _, backend := range report.Backends {
			if _, err := fmt.Fprintf(w, "- %s: %s\n", backend.BackendName, backend.Summary); err != nil {
				return err
			}
		}
	}
	if len(report.Blockers) > 0 {
		if _, err := fmt.Fprintln(w, "Top blockers:"); err != nil {
			return err
		}
		for _, blocker := range report.Blockers {
			if _, err := fmt.Fprintf(w, "- %s\n", blocker); err != nil {
				return err
			}
		}
	}
	if len(report.Strengths) > 0 {
		if _, err := fmt.Fprintln(w, "Strengths:"); err != nil {
			return err
		}
		for _, strength := range report.Strengths {
			if _, err := fmt.Fprintf(w, "- %s\n", strength); err != nil {
				return err
			}
		}
	}
	if report.RecommendedPositioning != "" {
		if _, err := fmt.Fprintf(w, "Recommended positioning: %s\n", report.RecommendedPositioning); err != nil {
			return err
		}
	}
	return nil
}

func writeSkillCompatibilityJSON(w io.Writer, report compatibility.Report) error {
	payload := struct {
		SchemaVersion string               `json:"schema_version"`
		Report        compatibility.Report `json:"report"`
	}{
		SchemaVersion: skillCompatibilityJSONSchemaVersion,
		Report:        report,
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
