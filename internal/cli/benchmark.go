package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/benchmark"
	"github.com/firety/firety/internal/render"
	"github.com/spf13/cobra"
)

const benchmarkJSONSchemaVersion = "1"

type benchmarkRunOptions struct {
	format   string
	artifact string
}

type benchmarkRenderOptions struct {
	mode string
}

func newBenchmarkCommand(application *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run and render Firety's built-in benchmark corpus",
		Long:  "Run Firety's built-in benchmark corpus and render benchmark artifacts for maintainer review and future public reporting.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newBenchmarkRunCommand(application))
	cmd.AddCommand(newBenchmarkRenderCommand())

	return cmd
}

func newBenchmarkRunCommand(application *app.App) *cobra.Command {
	options := benchmarkRunOptions{
		format: skillLintFormatText,
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run Firety's built-in benchmark corpus",
		Long:  "Run the built-in Firety benchmark corpus and summarize benchmark health, stability, and fixture coverage.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			report, err := application.Services.Benchmark.RunBuiltIn()
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if report.Summary.FailedFixtures > 0 {
				exitCode = ExitCodeLint
			}

			benchmarkArtifact := artifact.BuildBenchmarkArtifact(application.Version, report, artifact.BenchmarkArtifactOptions{
				Format: options.format,
			}, exitCode)

			if err := writeBenchmarkReport(cmd.OutOrStdout(), report, benchmarkArtifact, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if options.artifact != "" {
				if err := artifact.WriteBenchmarkArtifact(options.artifact, benchmarkArtifact); err != nil {
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
		&options.artifact,
		"artifact",
		"",
		"Write a versioned machine-readable benchmark artifact to the given file path",
	)

	return cmd
}

func newBenchmarkRenderCommand() *cobra.Command {
	options := benchmarkRenderOptions{
		mode: string(render.ModeFullReport),
	}

	cmd := &cobra.Command{
		Use:   "render <artifact>",
		Short: "Render a benchmark artifact into a reviewer-friendly report",
		Long:  "Render an existing Firety benchmark artifact into a PR comment, CI summary, or fuller public-facing report.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			output, err := render.RenderArtifact(args[0], render.Mode(options.mode))
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), output); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&options.mode,
		"render",
		string(render.ModeFullReport),
		"Render mode: pr-comment, ci-summary, or full-report",
	)

	return cmd
}

func (o benchmarkRunOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	return nil
}

func (o benchmarkRenderOptions) Validate() error {
	if _, err := render.ParseMode(o.mode); err != nil {
		return err
	}
	return nil
}

func writeBenchmarkReport(w io.Writer, report benchmark.Report, benchmarkArtifact artifact.BenchmarkArtifact, options benchmarkRunOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeBenchmarkText(w, report, options.artifact)
	case skillLintFormatJSON:
		return writeBenchmarkJSON(w, report, benchmarkArtifact)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeBenchmarkText(w io.Writer, report benchmark.Report, artifactPath string) error {
	if _, err := fmt.Fprintln(w, "Firety benchmark health"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Suite: %s v%s (%d fixture(s))\n", report.Suite.Name, report.Suite.Version, report.Suite.FixtureCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Status: %s\n", benchmarkStatus(report)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", report.Summary.Summary); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Counts: %d passed, %d failed\n", report.Summary.PassedFixtures, report.Summary.FailedFixtures); err != nil {
		return err
	}

	if len(report.Categories) > 0 {
		if _, err := fmt.Fprintln(w, "Category overview:"); err != nil {
			return err
		}
		for _, category := range report.Categories {
			if _, err := fmt.Fprintf(w, "- %s: %d/%d passed\n", category.CategoryLabel, category.Passed, category.FixtureCount); err != nil {
				return err
			}
		}
	}

	if len(report.Summary.NotableRegressions) > 0 {
		if _, err := fmt.Fprintln(w, "Review first:"); err != nil {
			return err
		}
		for _, item := range report.Summary.NotableRegressions {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}

	if len(report.Summary.ConfidenceSignals) > 0 {
		if _, err := fmt.Fprintln(w, "Confidence signals:"); err != nil {
			return err
		}
		for _, item := range report.Summary.ConfidenceSignals {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}

	if artifactPath != "" {
		if _, err := fmt.Fprintf(w, "Artifact: %s\n", artifactPath); err != nil {
			return err
		}
	}

	return nil
}

func writeBenchmarkJSON(w io.Writer, report benchmark.Report, benchmarkArtifact artifact.BenchmarkArtifact) error {
	payload := struct {
		SchemaVersion string                      `json:"schema_version"`
		Suite         benchmark.SuiteInfo         `json:"suite"`
		Summary       benchmark.Summary           `json:"summary"`
		Categories    []benchmark.CategorySummary `json:"categories"`
		Fixtures      []benchmark.FixtureResult   `json:"fixtures"`
		Fingerprint   string                      `json:"fingerprint"`
	}{
		SchemaVersion: benchmarkJSONSchemaVersion,
		Suite:         report.Suite,
		Summary:       report.Summary,
		Categories:    report.Categories,
		Fixtures:      report.Fixtures,
		Fingerprint:   benchmarkArtifact.Fingerprint,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func benchmarkStatus(report benchmark.Report) string {
	if report.Summary.FailedFixtures == 0 {
		return "healthy"
	}
	if report.Summary.PassedFixtures == 0 {
		return "regressed"
	}
	return "attention needed"
}
