package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/artifactview"
	"github.com/firety/firety/internal/render"
	"github.com/spf13/cobra"
)

const artifactInspectJSONSchemaVersion = "1"
const artifactCompareJSONSchemaVersion = "1"

type artifactInspectOptions struct {
	format string
}

type artifactRenderOptions struct {
	mode string
}

type artifactCompareOptions struct {
	format string
}

func newArtifactCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Inspect and render saved Firety artifacts",
		Long:  "Work with saved Firety artifacts for offline inspection, rendering, and compatible artifact-to-artifact comparison.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newArtifactInspectCommand())
	cmd.AddCommand(newArtifactRenderCommand())
	cmd.AddCommand(newArtifactCompareCommand())

	return cmd
}

func newArtifactInspectCommand() *cobra.Command {
	options := artifactInspectOptions{format: skillLintFormatText}

	cmd := &cobra.Command{
		Use:   "inspect <artifact>",
		Short: "Inspect a saved Firety artifact",
		Long:  "Inspect a saved Firety artifact, validate its schema and type, and summarize what Firety can do with it.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			info, err := artifactview.Inspect(args[0])
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeArtifactInspectReport(cmd.OutOrStdout(), info, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	return cmd
}

func newArtifactRenderCommand() *cobra.Command {
	options := artifactRenderOptions{mode: string(render.ModeFullReport)}

	cmd := &cobra.Command{
		Use:   "render <artifact>",
		Short: "Render a saved Firety artifact",
		Long:  "Render a saved Firety artifact into a PR comment, CI summary, or full report without rerunning analysis.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			output, err := artifactview.Render(args[0], render.Mode(options.mode))
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), output); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.mode, "render", string(render.ModeFullReport), "Render mode: pr-comment, ci-summary, or full-report")
	return cmd
}

func newArtifactCompareCommand() *cobra.Command {
	options := artifactCompareOptions{format: skillLintFormatText}

	cmd := &cobra.Command{
		Use:   "compare <base-artifact> <candidate-artifact>",
		Short: "Compare two compatible Firety artifacts",
		Long:  "Compare two compatible saved Firety artifacts without rerunning analysis. The first version supports lint, single-backend eval, and multi-backend eval artifacts.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			result, err := artifactview.Compare(args[0], args[1])
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeArtifactCompareReport(cmd.OutOrStdout(), result, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	return cmd
}

func (o artifactInspectOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
		return nil
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func (o artifactRenderOptions) Validate() error {
	_, err := render.ParseMode(o.mode)
	return err
}

func (o artifactCompareOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
		return nil
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeArtifactInspectReport(w io.Writer, info artifactview.Inspection, options artifactInspectOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeArtifactInspectText(w, info)
	case skillLintFormatJSON:
		payload := struct {
			SchemaVersion string                  `json:"schema_version"`
			Inspection    artifactview.Inspection `json:"inspection"`
		}{
			SchemaVersion: artifactInspectJSONSchemaVersion,
			Inspection:    info,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q", options.format)
	}
}

func writeArtifactInspectText(w io.Writer, info artifactview.Inspection) error {
	if _, err := fmt.Fprintf(w, "Artifact: %s\n", info.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Type: %s\n", info.ArtifactType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Schema version: %s\n", info.SchemaVersion); err != nil {
		return err
	}
	if info.Origin != "" {
		if _, err := fmt.Fprintf(w, "Origin: %s\n", info.Origin); err != nil {
			return err
		}
	}
	if info.Target != "" {
		if _, err := fmt.Fprintf(w, "Target: %s\n", info.Target); err != nil {
			return err
		}
	}
	if info.BaseTarget != "" {
		if _, err := fmt.Fprintf(w, "Base: %s\n", info.BaseTarget); err != nil {
			return err
		}
	}
	if info.CandidateTarget != "" {
		if _, err := fmt.Fprintf(w, "Candidate: %s\n", info.CandidateTarget); err != nil {
			return err
		}
	}
	if info.Summary != "" {
		if _, err := fmt.Fprintf(w, "Summary: %s\n", info.Summary); err != nil {
			return err
		}
	}
	if len(info.Context) > 0 {
		if _, err := fmt.Fprintf(w, "Context: %s\n", strings.Join(info.Context, "; ")); err != nil {
			return err
		}
	}
	if len(info.SupportedRenderModes) > 0 {
		if _, err := fmt.Fprintf(w, "Can render: %s\n", strings.Join(info.SupportedRenderModes, ", ")); err != nil {
			return err
		}
	}
	if len(info.ComparableTo) > 0 {
		if _, err := fmt.Fprintf(w, "Can compare with: %s\n", strings.Join(info.ComparableTo, ", ")); err != nil {
			return err
		}
	}
	return nil
}

func writeArtifactCompareReport(w io.Writer, result artifactview.CompareResult, options artifactCompareOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeArtifactCompareText(w, result)
	case skillLintFormatJSON:
		payload := struct {
			SchemaVersion string                     `json:"schema_version"`
			Comparison    artifactview.CompareResult `json:"comparison"`
		}{
			SchemaVersion: artifactCompareJSONSchemaVersion,
			Comparison:    result,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q", options.format)
	}
}

func writeArtifactCompareText(w io.Writer, result artifactview.CompareResult) error {
	if _, err := fmt.Fprintf(w, "Artifact type: %s\n", result.ArtifactType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Base artifact: %s\n", result.BasePath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Candidate artifact: %s\n", result.CandidatePath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Overall: %s\n", strings.ToUpper(result.Overall)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", result.Summary); err != nil {
		return err
	}
	if len(result.HighPriorityRegressions) > 0 {
		if _, err := fmt.Fprintln(w, "Top regressions:"); err != nil {
			return err
		}
		for _, item := range result.HighPriorityRegressions {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(result.NotableImprovements) > 0 {
		if _, err := fmt.Fprintln(w, "Notable improvements:"); err != nil {
			return err
		}
		for _, item := range result.NotableImprovements {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	return nil
}
