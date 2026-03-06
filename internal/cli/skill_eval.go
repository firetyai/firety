package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const skillEvalJSONSchemaVersion = "1"

type skillEvalOptions struct {
	format   string
	profile  string
	suite    string
	runner   string
	backends []string
	artifact string
}

func newSkillEvalCommand(application *app.App) *cobra.Command {
	options := skillEvalOptions{
		format:  skillLintFormatText,
		profile: string(service.SkillLintProfileGeneric),
	}

	cmd := &cobra.Command{
		Use:   "eval [path]",
		Short: "Measure routing behavior against a local eval suite",
		Long:  "Run measured routing evals for a local skill directory using a local JSON eval suite and a configured runner backend.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			target := "."
			if len(args) == 1 {
				target = args[0]
			}

			if len(options.backends) > 0 {
				selections, err := parseSkillEvalBackendSelections(options.backends)
				if err != nil {
					return newExitError(ExitCodeRuntime, err)
				}

				report, err := application.Services.SkillEval.EvaluateAcrossBackends(target, options.suite, selections)
				if err != nil {
					return newExitError(ExitCodeRuntime, err)
				}

				if err := writeSkillEvalMultiBackendReport(cmd.OutOrStdout(), report, options); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}

				exitCode := ExitCodeOK
				for _, backend := range report.Backends {
					if backend.Summary.Failed > 0 {
						exitCode = ExitCodeLint
						break
					}
				}

				if options.artifact != "" {
					evalArtifact := artifact.BuildSkillEvalMultiArtifact(application.Version, report, artifact.SkillEvalMultiArtifactOptions{
						Format: options.format,
						Suite:  report.Suite.Path,
					}, exitCode)
					if err := artifact.WriteSkillEvalMultiArtifact(options.artifact, evalArtifact); err != nil {
						return newExitError(ExitCodeRuntime, err)
					}
				}

				if exitCode == ExitCodeLint {
					return newExitError(ExitCodeLint, nil)
				}
				return nil
			}

			profile, err := service.ParseSkillLintProfile(options.profile)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			report, err := application.Services.SkillEval.Evaluate(target, service.SkillEvalOptions{
				SuitePath: options.suite,
				Profile:   profile,
				Runner:    options.runner,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillEvalReport(cmd.OutOrStdout(), report, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if report.Summary.Failed > 0 {
				exitCode = ExitCodeLint
			}

			if options.artifact != "" {
				evalArtifact := artifact.BuildSkillEvalArtifact(application.Version, report, artifact.SkillEvalArtifactOptions{
					Format:  options.format,
					Profile: options.profile,
					Suite:   report.Suite.Path,
					Runner:  options.runner,
				}, exitCode)
				if err := artifact.WriteSkillEvalArtifact(options.artifact, evalArtifact); err != nil {
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
		&options.profile,
		"profile",
		string(service.SkillLintProfileGeneric),
		"Routing profile: generic, codex, claude-code, copilot, or cursor",
	)
	cmd.Flags().StringVar(
		&options.suite,
		"suite",
		"",
		"Path to the local routing eval suite JSON file (defaults to evals/routing.json inside the skill directory)",
	)
	cmd.Flags().StringVar(
		&options.runner,
		"runner",
		"",
		"Path to the local routing eval runner executable (defaults to FIRETY_SKILL_EVAL_RUNNER)",
	)
	cmd.Flags().StringArrayVar(
		&options.backends,
		"backend",
		nil,
		`Backend selection for multi-backend eval, in the form "<id>" or "<id>=/path/to/runner"; repeat the flag for multiple backends`,
	)
	cmd.Flags().StringVar(
		&options.artifact,
		"artifact",
		"",
		"Write a versioned machine-readable eval artifact to the given file path",
	)

	return cmd
}

func (o skillEvalOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}

	if len(o.backends) > 0 {
		if len(o.backends) < 2 {
			return fmt.Errorf("multi-backend eval requires at least two --backend values; use --runner for single-backend eval")
		}
		if o.runner != "" {
			return fmt.Errorf("--runner cannot be combined with --backend; use either single-backend mode or multi-backend mode")
		}
		if o.profile != string(service.SkillLintProfileGeneric) {
			return fmt.Errorf("--profile cannot be combined with --backend; multi-backend eval uses each backend's profile affinity")
		}
	} else {
		if _, err := service.ParseSkillLintProfile(o.profile); err != nil {
			return err
		}
	}
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	return nil
}

func writeSkillEvalReport(w io.Writer, report domaineval.RoutingEvalReport, options skillEvalOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillEvalText(w, report)
	case skillLintFormatJSON:
		return writeSkillEvalJSON(w, report, options.profile)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillEvalText(w io.Writer, report domaineval.RoutingEvalReport) error {
	if _, err := fmt.Fprintf(w, "Target: %s\n", report.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Suite: %s (%d case(s))\n", report.Suite.Name, report.Suite.CaseCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Backend: %s\n", report.Backend.Name); err != nil {
		return err
	}
	if report.Profile != "" && report.Profile != string(service.SkillLintProfileGeneric) {
		if _, err := fmt.Fprintf(w, "Profile: %s\n", report.Profile); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(
		w,
		"Summary: %d passed, %d failed, %d false positive(s), %d false negative(s), %.0f%% pass rate\n",
		report.Summary.Passed,
		report.Summary.Failed,
		report.Summary.FalsePositives,
		report.Summary.FalseNegatives,
		report.Summary.PassRate*100,
	); err != nil {
		return err
	}

	if len(report.Summary.ByProfile) > 0 {
		if _, err := fmt.Fprintln(w, "By profile:"); err != nil {
			return err
		}
		for _, breakdown := range report.Summary.ByProfile {
			if _, err := fmt.Fprintf(w, "- %s: %d/%d passed\n", breakdown.Key, breakdown.Passed, breakdown.Total); err != nil {
				return err
			}
		}
	}

	if len(report.Summary.ByTag) > 0 {
		if _, err := fmt.Fprintln(w, "By tag:"); err != nil {
			return err
		}
		for _, breakdown := range report.Summary.ByTag {
			if _, err := fmt.Fprintf(w, "- %s: %d/%d passed\n", breakdown.Key, breakdown.Passed, breakdown.Total); err != nil {
				return err
			}
		}
	}

	if len(report.Summary.NotableMisses) == 0 {
		if _, err := fmt.Fprintln(w, "Notable misses: none"); err != nil {
			return err
		}
		return nil
	}

	if _, err := fmt.Fprintln(w, "Notable misses:"); err != nil {
		return err
	}
	for _, result := range report.Summary.NotableMisses {
		if _, err := fmt.Fprintf(
			w,
			"- [%s] expected %s for %q, got trigger=%t\n",
			result.ID,
			result.Expectation,
			result.Prompt,
			result.ActualTrigger,
		); err != nil {
			return err
		}
		if result.Reason != "" {
			if _, err := fmt.Fprintf(w, "  Reason: %s\n", result.Reason); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeSkillEvalJSON(w io.Writer, report domaineval.RoutingEvalReport, profile string) error {
	payload := skillEvalJSONReport{
		SchemaVersion: skillEvalJSONSchemaVersion,
		Target:        report.Target,
		Profile:       profile,
		Suite:         report.Suite,
		Backend:       report.Backend,
		Summary:       report.Summary,
		Results:       report.Results,
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

type skillEvalJSONReport struct {
	SchemaVersion string                             `json:"schema_version"`
	Target        string                             `json:"target"`
	Profile       string                             `json:"profile"`
	Suite         domaineval.RoutingEvalSuiteInfo    `json:"suite"`
	Backend       domaineval.RoutingEvalBackendInfo  `json:"backend"`
	Summary       domaineval.RoutingEvalSummary      `json:"summary"`
	Results       []domaineval.RoutingEvalCaseResult `json:"results"`
}

type skillEvalMultiBackendJSONReport struct {
	SchemaVersion  string                                 `json:"schema_version"`
	Target         string                                 `json:"target"`
	Suite          domaineval.RoutingEvalSuiteInfo        `json:"suite"`
	Backends       []domaineval.BackendEvalReport         `json:"backends"`
	Summary        domaineval.MultiBackendEvalSummary     `json:"summary"`
	DifferingCases []domaineval.MultiBackendDifferingCase `json:"differing_cases,omitempty"`
}

func parseSkillEvalBackendSelections(values []string) ([]service.SkillEvalBackendSelection, error) {
	selections := make([]service.SkillEvalBackendSelection, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		id, runner, ok := strings.Cut(value, "=")
		if !ok {
			id = value
			runner = ""
		}
		if id == "" {
			return nil, fmt.Errorf("backend selection %q is invalid; expected <id> or <id>=/path/to/runner", value)
		}
		definition, found := domaineval.FindBackendDefinition(id)
		if !found {
			return nil, fmt.Errorf("unsupported backend %q", id)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("backend %q was selected more than once", id)
		}
		seen[id] = struct{}{}

		selections = append(selections, service.SkillEvalBackendSelection{
			ID:     definition.ID,
			Runner: runner,
		})
	}

	return selections, nil
}

func writeSkillEvalMultiBackendReport(w io.Writer, report domaineval.MultiBackendEvalReport, options skillEvalOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillEvalMultiBackendText(w, report)
	case skillLintFormatJSON:
		return writeSkillEvalMultiBackendJSON(w, report)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillEvalMultiBackendText(w io.Writer, report domaineval.MultiBackendEvalReport) error {
	if _, err := fmt.Fprintf(w, "Target: %s\n", report.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Suite: %s (%d case(s))\n", report.Suite.Name, report.Suite.CaseCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", report.Summary.Summary); err != nil {
		return err
	}
	if report.Summary.StrongestBackend != "" {
		if _, err := fmt.Fprintf(w, "Strongest backend: %s\n", report.Summary.StrongestBackend); err != nil {
			return err
		}
	}
	if report.Summary.WeakestBackend != "" {
		if _, err := fmt.Fprintf(w, "Weakest backend: %s\n", report.Summary.WeakestBackend); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "Per-backend results:"); err != nil {
		return err
	}
	for _, backend := range report.Backends {
		if _, err := fmt.Fprintf(
			w,
			"- %s: %d passed, %d failed, %d false positive(s), %d false negative(s), %.0f%% pass rate\n",
			backend.Backend.Name,
			backend.Summary.Passed,
			backend.Summary.Failed,
			backend.Summary.FalsePositives,
			backend.Summary.FalseNegatives,
			backend.Summary.PassRate*100,
		); err != nil {
			return err
		}
	}

	if len(report.Summary.BackendSpecificStrengths) > 0 {
		if _, err := fmt.Fprintln(w, "Backend-specific strengths:"); err != nil {
			return err
		}
		for _, item := range report.Summary.BackendSpecificStrengths {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(report.Summary.BackendSpecificMisses) > 0 {
		if _, err := fmt.Fprintln(w, "Backend-specific misses:"); err != nil {
			return err
		}
		for _, item := range report.Summary.BackendSpecificMisses {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}

	if len(report.DifferingCases) == 0 {
		if _, err := fmt.Fprintln(w, "Differing cases: none"); err != nil {
			return err
		}
		return nil
	}

	if _, err := fmt.Fprintln(w, "Differing cases:"); err != nil {
		return err
	}
	for _, item := range report.DifferingCases {
		if _, err := fmt.Fprintf(w, "- [%s] expected %s for %q\n", item.ID, item.Expectation, item.Prompt); err != nil {
			return err
		}
		for _, outcome := range item.Outcomes {
			if _, err := fmt.Fprintf(w, "  %s: passed=%t trigger=%t", outcome.BackendName, outcome.Passed, outcome.ActualTrigger); err != nil {
				return err
			}
			if outcome.FailureKind != "" {
				if _, err := fmt.Fprintf(w, " (%s)", outcome.FailureKind); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeSkillEvalMultiBackendJSON(w io.Writer, report domaineval.MultiBackendEvalReport) error {
	payload := skillEvalMultiBackendJSONReport{
		SchemaVersion:  skillEvalJSONSchemaVersion,
		Target:         report.Target,
		Suite:          report.Suite,
		Backends:       report.Backends,
		Summary:        report.Summary,
		DifferingCases: report.DifferingCases,
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
