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

const skillEvalCompareJSONSchemaVersion = "1"

type skillEvalCompareOptions struct {
	format   string
	profile  string
	suite    string
	runner   string
	backends []string
	artifact string
}

func newSkillEvalCompareCommand(application *app.App) *cobra.Command {
	options := skillEvalCompareOptions{
		format:  skillLintFormatText,
		profile: string(service.SkillLintProfileGeneric),
	}

	cmd := &cobra.Command{
		Use:   "eval-compare <base> <candidate>",
		Short: "Compare measured routing eval results for two skill directories",
		Long:  "Run the same routing eval suite against two skill directories and summarize measured improvements or regressions.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if len(options.backends) > 0 {
				selections, err := parseSkillEvalBackendSelections(options.backends)
				if err != nil {
					return newExitError(ExitCodeRuntime, err)
				}

				result, err := application.Services.SkillEvalCompare.CompareAcrossBackends(args[0], args[1], options.suite, selections)
				if err != nil {
					return newExitError(ExitCodeRuntime, err)
				}

				if err := writeSkillEvalMultiCompareReport(cmd.OutOrStdout(), result, options); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}

				exitCode := ExitCodeOK
				for _, backend := range result.CandidateReport.Backends {
					if backend.Summary.Failed > 0 {
						exitCode = ExitCodeLint
						break
					}
				}

				if options.artifact != "" {
					evalArtifact := artifact.BuildSkillEvalMultiCompareArtifact(application.Version, result, artifact.SkillEvalMultiCompareArtifactOptions{
						Format: options.format,
						Suite:  result.BaseReport.Suite.Path,
					}, exitCode)
					if err := artifact.WriteSkillEvalMultiCompareArtifact(options.artifact, evalArtifact); err != nil {
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

			result, err := application.Services.SkillEvalCompare.Compare(args[0], args[1], service.SkillEvalOptions{
				SuitePath: options.suite,
				Profile:   profile,
				Runner:    options.runner,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillEvalCompareReport(cmd.OutOrStdout(), result, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if result.CandidateReport.Summary.Failed > 0 {
				exitCode = ExitCodeLint
			}

			if options.artifact != "" {
				evalArtifact := artifact.BuildSkillEvalCompareArtifact(application.Version, result, artifact.SkillEvalCompareArtifactOptions{
					Format:  options.format,
					Profile: options.profile,
					Suite:   result.BaseReport.Suite.Path,
					Runner:  options.runner,
				}, exitCode)
				if err := artifact.WriteSkillEvalCompareArtifact(options.artifact, evalArtifact); err != nil {
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
		"Path to the local routing eval suite JSON file (defaults to evals/routing.json inside the base skill directory)",
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
		`Backend selection for multi-backend compare, in the form "<id>" or "<id>=/path/to/runner"; repeat the flag for multiple backends`,
	)
	cmd.Flags().StringVar(
		&options.artifact,
		"artifact",
		"",
		"Write a versioned machine-readable eval compare artifact to the given file path",
	)

	return cmd
}

func (o skillEvalCompareOptions) Validate() error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}

	if len(o.backends) > 0 {
		if len(o.backends) < 2 {
			return fmt.Errorf("multi-backend eval compare requires at least two --backend values; use --runner for single-backend compare")
		}
		if o.runner != "" {
			return fmt.Errorf("--runner cannot be combined with --backend; use either single-backend compare or multi-backend compare")
		}
		if o.profile != string(service.SkillLintProfileGeneric) {
			return fmt.Errorf("--profile cannot be combined with --backend; multi-backend compare uses each backend's profile affinity")
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

func writeSkillEvalMultiCompareReport(w io.Writer, result service.SkillEvalMultiCompareResult, options skillEvalCompareOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillEvalMultiCompareText(w, result)
	case skillLintFormatJSON:
		return writeSkillEvalMultiCompareJSON(w, result)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillEvalCompareReport(w io.Writer, result service.SkillEvalCompareResult, options skillEvalCompareOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillEvalCompareText(w, result)
	case skillLintFormatJSON:
		return writeSkillEvalCompareJSON(w, result, options.profile)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillEvalCompareText(w io.Writer, result service.SkillEvalCompareResult) error {
	comparison := result.Comparison
	if _, err := fmt.Fprintf(w, "Base: %s\n", comparison.Base.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Candidate: %s\n", comparison.Candidate.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Suite: %s (%d case(s))\n", comparison.Suite.Name, comparison.Suite.CaseCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Backend: %s\n", comparison.Backend.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Overall: %s\n", strings.ToUpper(string(comparison.Summary.Overall))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", comparison.Summary.Summary); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"Pass rate: %.0f%% -> %.0f%% (%+.0fpp)\n",
		comparison.Base.Summary.PassRate*100,
		comparison.Candidate.Summary.PassRate*100,
		comparison.Summary.MetricsDelta.PassRate*100,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"False positives: %d -> %d (%+d)\n",
		comparison.Base.Summary.FalsePositives,
		comparison.Candidate.Summary.FalsePositives,
		comparison.Summary.MetricsDelta.FalsePositives,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"False negatives: %d -> %d (%+d)\n",
		comparison.Base.Summary.FalseNegatives,
		comparison.Candidate.Summary.FalseNegatives,
		comparison.Summary.MetricsDelta.FalseNegatives,
	); err != nil {
		return err
	}

	if len(comparison.Summary.HighPriorityRegressions) > 0 {
		if _, err := fmt.Fprintln(w, "Top regressions to review:"); err != nil {
			return err
		}
		for _, item := range comparison.Summary.HighPriorityRegressions {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(comparison.Summary.NotableImprovements) > 0 {
		if _, err := fmt.Fprintln(w, "Notable improvements:"); err != nil {
			return err
		}
		for _, item := range comparison.Summary.NotableImprovements {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}

	if len(comparison.FlippedToFail) > 0 {
		if _, err := fmt.Fprintf(w, "Flipped to fail (%d):\n", len(comparison.FlippedToFail)); err != nil {
			return err
		}
		if err := writeSkillEvalCaseChanges(w, comparison.FlippedToFail, 6); err != nil {
			return err
		}
	}
	if len(comparison.FlippedToPass) > 0 {
		if _, err := fmt.Fprintf(w, "Flipped to pass (%d):\n", len(comparison.FlippedToPass)); err != nil {
			return err
		}
		if err := writeSkillEvalCaseChanges(w, comparison.FlippedToPass, 6); err != nil {
			return err
		}
	}
	if len(comparison.ChangedCases) > 0 {
		if _, err := fmt.Fprintf(w, "Changed failing cases (%d):\n", len(comparison.ChangedCases)); err != nil {
			return err
		}
		if err := writeSkillEvalCaseChanges(w, comparison.ChangedCases, 6); err != nil {
			return err
		}
	}
	if len(comparison.Summary.RegressionAreas) > 0 {
		if _, err := fmt.Fprintln(w, "Regression areas:"); err != nil {
			return err
		}
		for _, area := range comparison.Summary.RegressionAreas {
			if _, err := fmt.Fprintf(w, "- %s\n", area); err != nil {
				return err
			}
		}
	}
	if len(comparison.Summary.ImprovementAreas) > 0 {
		if _, err := fmt.Fprintln(w, "Improvement areas:"); err != nil {
			return err
		}
		for _, area := range comparison.Summary.ImprovementAreas {
			if _, err := fmt.Fprintf(w, "- %s\n", area); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeSkillEvalMultiCompareText(w io.Writer, result service.SkillEvalMultiCompareResult) error {
	comparison := result.Comparison
	if _, err := fmt.Fprintf(w, "Base: %s\n", comparison.Base.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Candidate: %s\n", comparison.Candidate.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Suite: %s (%d case(s))\n", comparison.Suite.Name, comparison.Suite.CaseCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Overall: %s\n", strings.ToUpper(string(comparison.AggregateSummary.Overall))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", comparison.AggregateSummary.Summary); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Per-backend deltas:"); err != nil {
		return err
	}
	for _, backend := range comparison.PerBackend {
		if _, err := fmt.Fprintf(
			w,
			"- %s: %s, %.0f%% -> %.0f%% (%+.0fpp), false positives %+d, false negatives %+d\n",
			backend.Backend.Name,
			strings.ToUpper(string(backend.Comparison.Summary.Overall)),
			backend.Base.Summary.PassRate*100,
			backend.Candidate.Summary.PassRate*100,
			backend.Comparison.Summary.MetricsDelta.PassRate*100,
			backend.Comparison.Summary.MetricsDelta.FalsePositives,
			backend.Comparison.Summary.MetricsDelta.FalseNegatives,
		); err != nil {
			return err
		}
	}
	if len(comparison.AggregateSummary.HighPriorityRegressions) > 0 {
		if _, err := fmt.Fprintln(w, "Top regressions to review:"); err != nil {
			return err
		}
		for _, item := range comparison.AggregateSummary.HighPriorityRegressions {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(comparison.AggregateSummary.NotableImprovements) > 0 {
		if _, err := fmt.Fprintln(w, "Notable improvements:"); err != nil {
			return err
		}
		for _, item := range comparison.AggregateSummary.NotableImprovements {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(comparison.WidenedDisagreements) > 0 {
		if _, err := fmt.Fprintf(w, "Widened disagreements (%d):\n", len(comparison.WidenedDisagreements)); err != nil {
			return err
		}
		if err := writeSkillEvalMultiCompareCaseDeltas(w, comparison.WidenedDisagreements, 4); err != nil {
			return err
		}
	}
	if len(comparison.NarrowedDisagreements) > 0 {
		if _, err := fmt.Fprintf(w, "Narrowed disagreements (%d):\n", len(comparison.NarrowedDisagreements)); err != nil {
			return err
		}
		if err := writeSkillEvalMultiCompareCaseDeltas(w, comparison.NarrowedDisagreements, 4); err != nil {
			return err
		}
	}
	return nil
}

func writeSkillEvalCaseChanges(w io.Writer, changes []domaineval.RoutingEvalCaseChange, limit int) error {
	visible := changes
	if len(visible) > limit {
		visible = visible[:limit]
	}
	for _, change := range visible {
		if _, err := fmt.Fprintf(
			w,
			"- [%s] %q (base=%t, candidate=%t, base_failure=%s, candidate_failure=%s)\n",
			change.ID,
			change.Prompt,
			change.BasePassed,
			change.CandidatePassed,
			change.BaseFailureKind,
			change.CandidateFailureKind,
		); err != nil {
			return err
		}
	}
	if len(changes) > len(visible) {
		if _, err := fmt.Fprintf(w, "- ... %d more changed case(s)\n", len(changes)-len(visible)); err != nil {
			return err
		}
	}
	return nil
}

func writeSkillEvalCompareJSON(w io.Writer, result service.SkillEvalCompareResult, profile string) error {
	payload := skillEvalCompareJSONReport{
		SchemaVersion:   skillEvalCompareJSONSchemaVersion,
		Profile:         profile,
		Base:            result.Comparison.Base,
		Candidate:       result.Comparison.Candidate,
		Suite:           result.Comparison.Suite,
		Backend:         result.Comparison.Backend,
		Summary:         result.Comparison.Summary,
		FlippedToFail:   result.Comparison.FlippedToFail,
		FlippedToPass:   result.Comparison.FlippedToPass,
		ChangedCases:    result.Comparison.ChangedCases,
		ByProfileDeltas: result.Comparison.ByProfileDeltas,
		ByTagDeltas:     result.Comparison.ByTagDeltas,
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func writeSkillEvalMultiCompareJSON(w io.Writer, result service.SkillEvalMultiCompareResult) error {
	payload := struct {
		SchemaVersion         string                                       `json:"schema_version"`
		Base                  domaineval.RoutingEvalSideSummary            `json:"base"`
		Candidate             domaineval.RoutingEvalSideSummary            `json:"candidate"`
		Suite                 domaineval.RoutingEvalSuiteInfo              `json:"suite"`
		Backends              []domaineval.RoutingEvalBackendInfo          `json:"backends"`
		AggregateSummary      domaineval.MultiBackendEvalComparisonSummary `json:"aggregate_summary"`
		PerBackendDeltas      []domaineval.BackendEvalComparison           `json:"per_backend_deltas"`
		DifferingCases        []domaineval.MultiBackendEvalCaseDelta       `json:"differing_cases,omitempty"`
		WidenedDisagreements  []domaineval.MultiBackendEvalCaseDelta       `json:"widened_disagreements,omitempty"`
		NarrowedDisagreements []domaineval.MultiBackendEvalCaseDelta       `json:"narrowed_disagreements,omitempty"`
	}{
		SchemaVersion:         skillEvalCompareJSONSchemaVersion,
		Base:                  result.Comparison.Base,
		Candidate:             result.Comparison.Candidate,
		Suite:                 result.Comparison.Suite,
		Backends:              result.Comparison.Backends,
		AggregateSummary:      result.Comparison.AggregateSummary,
		PerBackendDeltas:      result.Comparison.PerBackend,
		DifferingCases:        result.Comparison.DifferingCases,
		WidenedDisagreements:  result.Comparison.WidenedDisagreements,
		NarrowedDisagreements: result.Comparison.NarrowedDisagreements,
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func writeSkillEvalMultiCompareCaseDeltas(w io.Writer, deltas []domaineval.MultiBackendEvalCaseDelta, limit int) error {
	visible := deltas
	if len(visible) > limit {
		visible = visible[:limit]
	}
	for _, delta := range visible {
		if _, err := fmt.Fprintf(w, "- [%s] %q\n", delta.ID, delta.Prompt); err != nil {
			return err
		}
		for _, backend := range delta.ChangedBackends {
			if _, err := fmt.Fprintf(w, "  %s: %t -> %t", backend.BackendName, backend.BasePassed, backend.CandidatePassed); err != nil {
				return err
			}
			if backend.BaseFailureKind != "" || backend.CandidateFailureKind != "" {
				if _, err := fmt.Fprintf(w, " (%s -> %s)", backend.BaseFailureKind, backend.CandidateFailureKind); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	if len(deltas) > len(visible) {
		if _, err := fmt.Fprintf(w, "- ... %d more changed case(s)\n", len(deltas)-len(visible)); err != nil {
			return err
		}
	}
	return nil
}

type skillEvalCompareJSONReport struct {
	SchemaVersion   string                                  `json:"schema_version"`
	Profile         string                                  `json:"profile"`
	Base            domaineval.RoutingEvalSideSummary       `json:"base"`
	Candidate       domaineval.RoutingEvalSideSummary       `json:"candidate"`
	Suite           domaineval.RoutingEvalSuiteInfo         `json:"suite"`
	Backend         domaineval.RoutingEvalBackendInfo       `json:"backend"`
	Summary         domaineval.RoutingEvalComparisonSummary `json:"summary"`
	FlippedToFail   []domaineval.RoutingEvalCaseChange      `json:"flipped_to_fail,omitempty"`
	FlippedToPass   []domaineval.RoutingEvalCaseChange      `json:"flipped_to_pass,omitempty"`
	ChangedCases    []domaineval.RoutingEvalCaseChange      `json:"changed_cases,omitempty"`
	ByProfileDeltas []domaineval.RoutingEvalBreakdownDelta  `json:"by_profile_deltas,omitempty"`
	ByTagDeltas     []domaineval.RoutingEvalBreakdownDelta  `json:"by_tag_deltas,omitempty"`
}
