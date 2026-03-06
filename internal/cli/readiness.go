package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/domain/readiness"
	"github.com/firety/firety/internal/freshness"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const readinessJSONSchemaVersion = "1"

type readinessCheckOptions struct {
	format         string
	context        string
	profiles       []string
	strictness     string
	suite          string
	runner         string
	backends       []string
	inputArtifacts []string
	inputPacks     []string
	inputReports   []string
	artifact       string
}

func newReadinessCommand(application *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "Check publish and release readiness from Firety evidence",
		Long:  "Synthesize Firety lint, eval, gate, freshness, compatibility, and attestation evidence into a clear publish or release-readiness decision.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newReadinessCheckCommand(application))
	return cmd
}

func newReadinessCheckCommand(application *app.App) *cobra.Command {
	options := readinessCheckOptions{
		format:     skillLintFormatText,
		context:    string(readiness.ContextInternal),
		strictness: string(lint.StrictnessDefault),
	}

	cmd := &cobra.Command{
		Use:   "check [path]",
		Short: "Evaluate whether a skill is ready for the selected publish context",
		Long:  "Generate a deterministic publish-decision summary for internal use, merge, release-candidate, public release, public attestation, or public trust-report publishing.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(args); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			target := ""
			if len(args) == 1 {
				target = args[0]
			}

			contextValue, err := readiness.ParsePublishContext(options.context)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			profiles, err := service.ParseSkillLintProfiles(options.profiles)
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
			freshnessSummary, err := readinessFreshnessSummary(target, options)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			result, err := application.Services.SkillReadiness.Evaluate(target, service.SkillReadinessOptions{
				Context:        contextValue,
				Profiles:       profiles,
				Strictness:     strictness,
				SuitePath:      options.suite,
				Runner:         options.runner,
				Backends:       backends,
				InputArtifacts: append([]string(nil), options.inputArtifacts...),
				InputPacks:     append([]string(nil), options.inputPacks...),
				InputReports:   append([]string(nil), options.inputReports...),
				Freshness:      freshnessSummary,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeReadinessReport(cmd.OutOrStdout(), result.Readiness, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			exitCode := ExitCodeOK
			if result.Readiness.Decision == readiness.DecisionNotReady || result.Readiness.Decision == readiness.DecisionInsufficient {
				exitCode = ExitCodeLint
			}

			if options.artifact != "" {
				value := artifact.BuildSkillReadinessArtifact(application.Version, result.Readiness, artifact.SkillReadinessArtifactOptions{
					Format:         options.format,
					Target:         target,
					PublishContext: options.context,
					Profiles:       append([]string(nil), options.profiles...),
					Strictness:     strictness.DisplayName(),
					SuitePath:      options.suite,
					Runner:         options.runner,
					Backends:       append([]string(nil), options.backends...),
					InputArtifacts: append([]string(nil), options.inputArtifacts...),
					InputPacks:     append([]string(nil), options.inputPacks...),
					InputReports:   append([]string(nil), options.inputReports...),
				}, exitCode)
				if err := artifact.WriteSkillReadinessArtifact(options.artifact, value); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}

			if exitCode != ExitCodeOK {
				return newExitError(exitCode, nil)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	cmd.Flags().StringVar(&options.context, "context", string(readiness.ContextInternal), "Publish context: internal, merge, release-candidate, public-release, public-attestation, or public-trust-report")
	cmd.Flags().StringArrayVar(&options.profiles, "profile", nil, "Profiles to use for fresh readiness evidence; repeat for multiple profiles")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness for fresh readiness evidence")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Routing eval suite path for fresh measured evidence")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Single-backend routing eval runner for fresh measured evidence")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection in the form "<id>" or "<id>=/path/to/runner"; repeat for multiple backends`)
	cmd.Flags().StringArrayVar(&options.inputArtifacts, "input-artifact", nil, "Use existing Firety artifacts instead of rerunning analysis")
	cmd.Flags().StringArrayVar(&options.inputPacks, "input-pack", nil, "Use one or more Firety evidence packs instead of rerunning analysis")
	cmd.Flags().StringArrayVar(&options.inputReports, "input-report", nil, "Use one or more Firety trust reports instead of rerunning analysis")
	cmd.Flags().StringVar(&options.artifact, "artifact", "", "Write a versioned machine-readable readiness artifact to the given file path")
	return cmd
}

func (o readinessCheckOptions) Validate(args []string) error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
	if _, err := readiness.ParsePublishContext(o.context); err != nil {
		return err
	}
	if _, err := lint.ParseStrictness(o.strictness); err != nil {
		return err
	}
	if _, err := service.ParseSkillLintProfiles(o.profiles); err != nil {
		return err
	}
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	hasInputs := len(o.inputArtifacts) > 0 || len(o.inputPacks) > 0 || len(o.inputReports) > 0
	if hasInputs && len(args) > 0 {
		return fmt.Errorf("readiness accepts either a target path or existing artifact/pack/report inputs, not both")
	}
	if !hasInputs && len(args) == 0 {
		return fmt.Errorf("readiness requires a target path or at least one --input-artifact/--input-pack/--input-report value")
	}
	if hasInputs {
		if len(o.profiles) > 0 || o.strictness != string(lint.StrictnessDefault) || o.suite != "" || o.runner != "" || len(o.backends) > 0 {
			return fmt.Errorf("artifact-based readiness cannot be combined with fresh-run profile, strictness, suite, runner, or backend flags")
		}
		return nil
	}
	if len(o.backends) > 0 && o.runner != "" {
		return fmt.Errorf("--runner cannot be combined with --backend")
	}
	return nil
}

func writeReadinessReport(w io.Writer, result readiness.Result, options readinessCheckOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeReadinessText(w, result)
	case skillLintFormatJSON:
		payload := struct {
			SchemaVersion string           `json:"schema_version"`
			Readiness     readiness.Result `json:"readiness"`
		}{
			SchemaVersion: readinessJSONSchemaVersion,
			Readiness:     result,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q", options.format)
	}
}

func writeReadinessText(w io.Writer, result readiness.Result) error {
	lines := []string{
		fmt.Sprintf("Decision: %s", strings.ToUpper(string(result.Decision))),
		fmt.Sprintf("Publish context: %s", result.PublishContext),
	}
	if result.Target != "" {
		lines = append(lines, fmt.Sprintf("Target: %s", result.Target))
	}
	lines = append(lines, fmt.Sprintf("Summary: %s", result.Summary))
	if result.EvidenceSummary.GateDecision != "" || result.EvidenceSummary.FreshnessStatus != "" || result.EvidenceSummary.SupportPosture != "" {
		lines = append(lines, "Evidence:")
		if result.EvidenceSummary.GateDecision != "" {
			lines = append(lines, "- quality gate: "+strings.ToUpper(result.EvidenceSummary.GateDecision))
		}
		if result.EvidenceSummary.FreshnessStatus != "" {
			lines = append(lines, "- freshness: "+string(result.EvidenceSummary.FreshnessStatus))
		}
		if result.EvidenceSummary.SupportPosture != "" {
			lines = append(lines, "- support posture: "+string(result.EvidenceSummary.SupportPosture))
		}
		if result.EvidenceSummary.EvidenceLevel != "" {
			lines = append(lines, "- evidence level: "+string(result.EvidenceSummary.EvidenceLevel))
		}
	}
	if len(result.Blockers) > 0 {
		lines = append(lines, "Blockers:")
		for _, item := range result.Blockers {
			lines = append(lines, "- "+item.Summary)
		}
	}
	if len(result.Caveats) > 0 {
		lines = append(lines, "Caveats:")
		for _, item := range result.Caveats {
			lines = append(lines, "- "+item.Summary)
		}
	}
	if len(result.RecommendedActions) > 0 {
		lines = append(lines, "Next actions:")
		for _, item := range result.RecommendedActions {
			lines = append(lines, "- "+item)
		}
	}
	if result.AttestationSuitability.Summary != "" || result.TrustReportSuitability.Summary != "" {
		lines = append(lines, "Publish surfaces:")
		lines = append(lines, fmt.Sprintf("- attestation: %s (%s)", result.AttestationSuitability.Suitability, result.AttestationSuitability.Summary))
		lines = append(lines, fmt.Sprintf("- trust report: %s (%s)", result.TrustReportSuitability.Suitability, result.TrustReportSuitability.Summary))
	}
	_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n")
	return err
}

func readinessFreshnessSummary(target string, options readinessCheckOptions) (*readiness.FreshnessSummary, error) {
	if strings.TrimSpace(target) != "" {
		return &readiness.FreshnessSummary{
			Status:          readiness.FreshnessFresh,
			AgeSummary:      "Fresh Firety analysis was run for this readiness decision.",
			SupportingPaths: []string{target},
		}, nil
	}

	subjects := make([]string, 0, len(options.inputArtifacts)+len(options.inputPacks)+len(options.inputReports))
	subjects = append(subjects, options.inputArtifacts...)
	subjects = append(subjects, options.inputPacks...)
	subjects = append(subjects, options.inputReports...)
	if len(subjects) == 0 {
		return nil, nil
	}

	reports := make([]freshness.Report, 0, len(subjects))
	for _, subject := range uniqueReadinessPaths(subjects) {
		report, err := freshness.Inspect(subject, freshness.DefaultOptions())
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}

	worst := reports[0]
	status := worst.FreshnessStatus
	caveats := make([]string, 0)
	actions := make([]string, 0)
	supporting := make([]string, 0)
	for _, report := range reports {
		if readinessFreshnessRank(report.FreshnessStatus) > readinessFreshnessRank(status) {
			status = report.FreshnessStatus
			worst = report
		}
		caveats = append(caveats, report.Caveats...)
		actions = append(actions, report.RecertificationActions...)
		supporting = append(supporting, report.Subject.Path)
		supporting = append(supporting, report.SupportingPaths...)
		for _, item := range report.StaleComponents {
			caveats = append(caveats, item.Reason)
		}
		for _, item := range report.CaveatComponents {
			caveats = append(caveats, item.Reason)
		}
	}

	return &readiness.FreshnessSummary{
		Status:                 readiness.FreshnessStatus(status),
		AgeSummary:             aggregateReadinessAgeSummary(len(reports), status),
		Caveats:                uniqueReadinessStrings(caveats),
		RecertificationActions: uniqueReadinessStrings(actions),
		SupportingPaths:        uniqueReadinessStrings(supporting),
	}, nil
}

func readinessFreshnessRank(status freshness.Status) int {
	switch status {
	case freshness.StatusFresh:
		return 0
	case freshness.StatusUsableWithCaveats:
		return 1
	case freshness.StatusStale:
		return 2
	case freshness.StatusInsufficientEvidence:
		return 3
	default:
		return 4
	}
}

func aggregateReadinessAgeSummary(count int, status freshness.Status) string {
	switch status {
	case freshness.StatusFresh:
		return fmt.Sprintf("All %d supporting evidence item(s) are fresh enough for reuse.", count)
	case freshness.StatusUsableWithCaveats:
		return fmt.Sprintf("Supporting evidence has caveats across %d saved item(s).", count)
	case freshness.StatusStale:
		return fmt.Sprintf("At least one supporting evidence item is stale across %d saved item(s).", count)
	case freshness.StatusInsufficientEvidence:
		return fmt.Sprintf("At least one supporting evidence item lacks enough provenance or freshness data across %d saved item(s).", count)
	default:
		return fmt.Sprintf("Freshness was evaluated across %d saved item(s).", count)
	}
}

func uniqueReadinessPaths(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		absolute, err := filepath.Abs(value)
		if err != nil {
			absolute = value
		}
		if _, ok := seen[absolute]; ok {
			continue
		}
		seen[absolute] = struct{}{}
		out = append(out, absolute)
	}
	slices.Sort(out)
	return out
}

func uniqueReadinessStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}
