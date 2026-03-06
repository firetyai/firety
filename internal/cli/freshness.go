package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/firety/firety/internal/freshness"
	"github.com/spf13/cobra"
)

const freshnessInspectJSONSchemaVersion = "1"

type freshnessInspectOptions struct {
	format            string
	maxAge            time.Duration
	maxEvalAge        time.Duration
	maxMultiEvalAge   time.Duration
	maxBenchmarkAge   time.Duration
	maxAttestationAge time.Duration
	maxReportAge      time.Duration
}

func newFreshnessCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "freshness",
		Short: "Inspect evidence freshness and recertification needs",
		Long:  "Inspect saved Firety artifacts, evidence packs, and trust reports to determine whether they are still current enough to trust or should be recertified.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newFreshnessInspectCommand())
	return cmd
}

func newFreshnessInspectCommand() *cobra.Command {
	defaults := freshness.DefaultOptions()
	options := freshnessInspectOptions{
		format:            skillLintFormatText,
		maxAge:            defaults.MaxAge,
		maxEvalAge:        defaults.MaxEvalAge,
		maxMultiEvalAge:   defaults.MaxMultiEvalAge,
		maxBenchmarkAge:   defaults.MaxBenchmarkAge,
		maxAttestationAge: defaults.MaxAttestationAge,
		maxReportAge:      defaults.MaxReportAge,
	}

	cmd := &cobra.Command{
		Use:   "inspect <artifact-or-pack-or-report>",
		Short: "Inspect freshness and recertification status for a saved Firety output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			report, err := freshness.Inspect(args[0], options.toFreshnessOptions())
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeFreshnessReport(cmd.OutOrStdout(), report, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	cmd.Flags().DurationVar(&options.maxAge, "max-age", defaults.MaxAge, "Maximum age for general saved evidence")
	cmd.Flags().DurationVar(&options.maxEvalAge, "max-eval-age", defaults.MaxEvalAge, "Maximum age for eval-backed evidence")
	cmd.Flags().DurationVar(&options.maxMultiEvalAge, "max-multi-eval-age", defaults.MaxMultiEvalAge, "Maximum age for multi-backend eval evidence")
	cmd.Flags().DurationVar(&options.maxBenchmarkAge, "max-benchmark-age", defaults.MaxBenchmarkAge, "Maximum age for benchmark evidence")
	cmd.Flags().DurationVar(&options.maxAttestationAge, "max-attestation-age", defaults.MaxAttestationAge, "Maximum age for attestations")
	cmd.Flags().DurationVar(&options.maxReportAge, "max-report-age", defaults.MaxReportAge, "Maximum age for evidence packs and trust reports")
	return cmd
}

func (o freshnessInspectOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
	return o.toFreshnessOptions().Validate()
}

func (o freshnessInspectOptions) toFreshnessOptions() freshness.Options {
	return freshness.Options{
		Now:               time.Now().UTC(),
		MaxAge:            o.maxAge,
		MaxEvalAge:        o.maxEvalAge,
		MaxMultiEvalAge:   o.maxMultiEvalAge,
		MaxBenchmarkAge:   o.maxBenchmarkAge,
		MaxAttestationAge: o.maxAttestationAge,
		MaxReportAge:      o.maxReportAge,
	}
}

func writeFreshnessReport(w io.Writer, report freshness.Report, options freshnessInspectOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeFreshnessText(w, report)
	case skillLintFormatJSON:
		payload := struct {
			SchemaVersion string           `json:"schema_version"`
			Freshness     freshness.Report `json:"freshness"`
		}{
			SchemaVersion: freshnessInspectJSONSchemaVersion,
			Freshness:     report,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q", options.format)
	}
}

func writeFreshnessText(w io.Writer, report freshness.Report) error {
	lines := []string{
		fmt.Sprintf("Subject: %s", report.Subject.Path),
		fmt.Sprintf("Kind: %s", report.Subject.Kind),
		fmt.Sprintf("Type: %s", report.Subject.Type),
		fmt.Sprintf("Status: %s", strings.ToUpper(string(report.FreshnessStatus))),
		fmt.Sprintf("Generated at: %s", report.GeneratedAt),
		fmt.Sprintf("Age: %s", report.AgeSummary),
	}
	if len(report.StaleComponents) > 0 {
		items := make([]string, 0, len(report.StaleComponents))
		for _, item := range report.StaleComponents {
			items = append(items, fmt.Sprintf("%s (%s)", item.Type, item.AgeSummary))
		}
		lines = append(lines, "Stale components: "+strings.Join(items, "; "))
	}
	if len(report.CaveatComponents) > 0 {
		items := make([]string, 0, len(report.CaveatComponents))
		for _, item := range report.CaveatComponents {
			items = append(items, fmt.Sprintf("%s (%s)", item.Type, item.AgeSummary))
		}
		lines = append(lines, "Caveat components: "+strings.Join(items, "; "))
	}
	if len(report.Caveats) > 0 {
		lines = append(lines, "Caveats: "+strings.Join(report.Caveats, "; "))
	}
	if len(report.IntendedUseSuitability) > 0 {
		useLines := make([]string, 0, len(report.IntendedUseSuitability))
		for _, item := range report.IntendedUseSuitability {
			useLines = append(useLines, fmt.Sprintf("%s=%s", item.Use, item.Status))
		}
		lines = append(lines, "Use suitability: "+strings.Join(useLines, "; "))
	}
	if len(report.RecertificationActions) > 0 {
		lines = append(lines, "Recertify by: "+strings.Join(report.RecertificationActions, "; "))
	}
	_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n")
	return err
}
