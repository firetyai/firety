package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/provenance"
	"github.com/spf13/cobra"
)

const provenanceInspectJSONSchemaVersion = "1"
const provenanceCompareJSONSchemaVersion = "1"

type provenanceInspectOptions struct {
	format string
}

type provenanceCompareOptions struct {
	format string
}

func newProvenanceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provenance",
		Short: "Inspect Firety reproducibility and comparability metadata",
		Long:  "Inspect saved Firety artifacts, evidence packs, and trust reports for reproducibility metadata and explicit comparability checks.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newProvenanceInspectCommand())
	cmd.AddCommand(newProvenanceCompareCommand())
	return cmd
}

func newProvenanceInspectCommand() *cobra.Command {
	options := provenanceInspectOptions{format: skillLintFormatText}

	cmd := &cobra.Command{
		Use:   "inspect <artifact-or-pack-or-report>",
		Short: "Inspect provenance for a saved Firety output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			info, err := provenance.Inspect(args[0])
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeProvenanceInspectReport(cmd.OutOrStdout(), info, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	return cmd
}

func newProvenanceCompareCommand() *cobra.Command {
	options := provenanceCompareOptions{format: skillLintFormatText}

	cmd := &cobra.Command{
		Use:   "compare <base> <candidate>",
		Short: "Compare provenance for two saved Firety outputs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			result, err := provenance.ComparePaths(args[0], args[1])
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeProvenanceCompareReport(cmd.OutOrStdout(), result, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	return cmd
}

func (o provenanceInspectOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
		return nil
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func (o provenanceCompareOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
		return nil
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeProvenanceInspectReport(w io.Writer, info provenance.Inspection, options provenanceInspectOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeProvenanceInspectText(w, info)
	case skillLintFormatJSON:
		payload := struct {
			SchemaVersion string                `json:"schema_version"`
			Inspection    provenance.Inspection `json:"inspection"`
		}{
			SchemaVersion: provenanceInspectJSONSchemaVersion,
			Inspection:    info,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q", options.format)
	}
}

func writeProvenanceCompareReport(w io.Writer, result provenance.Comparison, options provenanceCompareOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeProvenanceCompareText(w, result)
	case skillLintFormatJSON:
		payload := struct {
			SchemaVersion string                `json:"schema_version"`
			Comparison    provenance.Comparison `json:"comparison"`
		}{
			SchemaVersion: provenanceCompareJSONSchemaVersion,
			Comparison:    result,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q", options.format)
	}
}

func writeProvenanceInspectText(w io.Writer, info provenance.Inspection) error {
	lines := []string{
		fmt.Sprintf("Path: %s", info.Path),
		fmt.Sprintf("Kind: %s", info.Kind),
		fmt.Sprintf("Type: %s", info.Type),
		fmt.Sprintf("Schema version: %s", info.SchemaVersion),
	}
	if info.Summary != "" {
		lines = append(lines, fmt.Sprintf("Summary: %s", info.Summary))
	}
	if info.Provenance.CommandOrigin != "" {
		lines = append(lines, fmt.Sprintf("Command origin: %s", info.Provenance.CommandOrigin))
	}
	if info.Provenance.FiretyVersion != "" {
		version := info.Provenance.FiretyVersion
		if info.Provenance.FiretyCommit != "" {
			version += " (" + info.Provenance.FiretyCommit + ")"
		}
		lines = append(lines, fmt.Sprintf("Firety: %s", version))
	}
	if info.Provenance.Target != "" {
		lines = append(lines, fmt.Sprintf("Target: %s", info.Provenance.Target))
	}
	if info.Provenance.TargetFingerprint != "" {
		lines = append(lines, fmt.Sprintf("Target fingerprint: %s", info.Provenance.TargetFingerprint))
	}
	context := provenanceContextLines(info.Provenance)
	lines = append(lines, context...)
	lines = append(lines,
		fmt.Sprintf("Suitable for baseline: %t", info.SuitableForBaseline),
		fmt.Sprintf("Suitable for comparison: %t", info.SuitableForComparison),
	)
	if len(info.Provenance.ComparabilityNotes) > 0 {
		lines = append(lines, "Comparability notes: "+strings.Join(info.Provenance.ComparabilityNotes, "; "))
	}
	if len(info.Provenance.ReproducibilityNotes) > 0 {
		lines = append(lines, "Reproducibility notes: "+strings.Join(info.Provenance.ReproducibilityNotes, "; "))
	}
	_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n")
	return err
}

func writeProvenanceCompareText(w io.Writer, result provenance.Comparison) error {
	lines := []string{
		fmt.Sprintf("Base: %s", result.BasePath),
		fmt.Sprintf("Candidate: %s", result.CandidatePath),
		fmt.Sprintf("Status: %s", strings.ToUpper(string(result.Status))),
		fmt.Sprintf("Summary: %s", result.Summary),
	}
	if len(result.SharedContext) > 0 {
		lines = append(lines, "Shared context: "+strings.Join(result.SharedContext, "; "))
	}
	if len(result.Reasons) > 0 {
		lines = append(lines, "Reasons: "+strings.Join(result.Reasons, "; "))
	}
	if result.RerunRecommendation != "" {
		lines = append(lines, "Recommendation: "+result.RerunRecommendation)
	}
	_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n")
	return err
}

func provenanceContextLines(record provenance.Record) []string {
	lines := make([]string, 0, 8)
	if record.Profile != "" {
		lines = append(lines, "Profile: "+record.Profile)
	}
	if record.Strictness != "" {
		lines = append(lines, "Strictness: "+record.Strictness)
	}
	if record.FailOn != "" {
		lines = append(lines, "Fail-on policy: "+record.FailOn)
	}
	if record.SuitePath != "" {
		lines = append(lines, "Suite: "+record.SuitePath)
	}
	if len(record.Backends) > 0 {
		lines = append(lines, "Backends: "+strings.Join(record.Backends, ", "))
	}
	if len(record.InputArtifacts) > 0 {
		lines = append(lines, "Input artifacts: "+strings.Join(record.InputArtifacts, ", "))
	}
	if len(record.InputPacks) > 0 {
		lines = append(lines, "Input packs: "+strings.Join(record.InputPacks, ", "))
	}
	if len(record.ArtifactDependencies) > 0 {
		lines = append(lines, "Artifact dependencies: "+strings.Join(record.ArtifactDependencies, ", "))
	}
	return lines
}
