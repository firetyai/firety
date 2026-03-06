package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/domain/readiness"
	workspacepkg "github.com/firety/firety/internal/domain/workspace"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const workspaceJSONSchemaVersion = "1"

type workspaceSharedOptions struct {
	format     string
	profile    string
	strictness string
	context    string
	suite      string
	runner     string
	backends   []string
	changed    bool
	base       string
	head       string
}

type workspaceGateOptions struct {
	maxNotReadySkills             int
	maxInsufficientEvidenceSkills int
	maxSkillsWithLintErrors       int
	maxDiscoveryWarnings          int
}

type workspaceReportOptions struct {
	artifact string
}

func newWorkspaceCommand(application *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Analyze and summarize repositories that contain multiple skills",
		Long:  "Discover multiple local skills under one root path and aggregate lint, readiness, gate, and report evidence across the workspace.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newWorkspaceChangesCommand(application),
		newWorkspaceLintCommand(application),
		newWorkspaceReadinessCommand(application),
		newWorkspaceGateCommand(application),
		newWorkspaceReportCommand(application),
	)
	return cmd
}

func newWorkspaceChangesCommand(application *app.App) *cobra.Command {
	shared := defaultWorkspaceSharedOptions()
	reportOptions := workspaceReportOptions{}
	cmd := &cobra.Command{
		Use:   "changes [path]",
		Short: "Detect directly changed and impacted skills from local git state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shared.validateChangeScope(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			scope, err := application.Services.Workspace.Changes(workspaceTargetArg(args), service.WorkspaceChangeOptions{
				BaseRev: shared.base,
				HeadRev: shared.head,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeWorkspaceChangesReport(cmd.OutOrStdout(), scope, shared.format); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if reportOptions.artifact != "" {
				value := artifact.BuildWorkspaceChangeScopeArtifact(application.Version, scope, artifact.WorkspaceChangeScopeArtifactOptions{
					Format:        shared.format,
					WorkspaceRoot: workspaceTargetArg(args),
					BaseRev:       shared.base,
					HeadRev:       shared.head,
				})
				if err := artifact.WriteWorkspaceChangeScopeArtifact(reportOptions.artifact, value); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&shared.format, "format", skillLintFormatText, "Output format: text or json")
	addWorkspaceChangeFlags(cmd, &shared)
	cmd.Flags().StringVar(&reportOptions.artifact, "artifact", "", "Write a versioned workspace change-scope artifact to the given file path")
	return cmd
}

func newWorkspaceLintCommand(application *app.App) *cobra.Command {
	shared := defaultWorkspaceSharedOptions()
	cmd := &cobra.Command{
		Use:   "lint [path]",
		Short: "Run Firety lint across all discovered skills in a workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shared.validate(false); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			report, err := analyzeWorkspace(application, args, shared, nil, false)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeWorkspaceLintReport(cmd.OutOrStdout(), report, shared.format); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if report.Summary.SkillsWithLintErrors > 0 {
				return newExitError(ExitCodeLint, nil)
			}
			return nil
		},
	}
	addWorkspaceSharedFlags(cmd, &shared, false)
	addWorkspaceChangeFlags(cmd, &shared)
	return cmd
}

func newWorkspaceReadinessCommand(application *app.App) *cobra.Command {
	shared := defaultWorkspaceSharedOptions()
	cmd := &cobra.Command{
		Use:   "readiness [path]",
		Short: "Evaluate publish and release readiness across all discovered skills",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shared.validate(true); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			report, err := analyzeWorkspace(application, args, shared, nil, true)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeWorkspaceReadinessReport(cmd.OutOrStdout(), report, shared.format); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if report.Summary.NotReadySkills > 0 || report.Summary.InsufficientEvidenceSkills > 0 {
				return newExitError(ExitCodeLint, nil)
			}
			return nil
		},
	}
	addWorkspaceSharedFlags(cmd, &shared, true)
	addWorkspaceChangeFlags(cmd, &shared)
	return cmd
}

func newWorkspaceGateCommand(application *app.App) *cobra.Command {
	shared := defaultWorkspaceSharedOptions()
	gateOptions := defaultWorkspaceGateOptions()
	cmd := &cobra.Command{
		Use:   "gate [path]",
		Short: "Apply an aggregate quality gate across all discovered skills",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shared.validate(true); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			criteria := gateOptions.criteria()
			report, err := analyzeWorkspace(application, args, shared, &criteria, true)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeWorkspaceGateReport(cmd.OutOrStdout(), report, shared.format); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if report.Gate != nil && report.Gate.Decision == gate.DecisionFail {
				return newExitError(ExitCodeLint, nil)
			}
			return nil
		},
	}
	addWorkspaceSharedFlags(cmd, &shared, true)
	addWorkspaceChangeFlags(cmd, &shared)
	addWorkspaceGateFlags(cmd, &gateOptions)
	return cmd
}

func newWorkspaceReportCommand(application *app.App) *cobra.Command {
	shared := defaultWorkspaceSharedOptions()
	gateOptions := defaultWorkspaceGateOptions()
	reportOptions := workspaceReportOptions{}
	cmd := &cobra.Command{
		Use:   "report [path]",
		Short: "Generate an aggregate workspace report and optional workspace artifact",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shared.validate(true); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if reportOptions.artifact == "-" {
				return newExitError(ExitCodeRuntime, fmt.Errorf(`artifact path "-" is not supported; choose a file path`))
			}
			criteria := gateOptions.criteria()
			report, err := analyzeWorkspace(application, args, shared, &criteria, true)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if err := writeWorkspaceFullReport(cmd.OutOrStdout(), report, shared.format); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			if reportOptions.artifact != "" {
				target := workspaceTargetArg(args)
				value := artifact.BuildWorkspaceReportArtifact(application.Version, report, artifact.WorkspaceReportArtifactOptions{
					Format:         shared.format,
					WorkspaceRoot:  target,
					Profile:        shared.profile,
					Strictness:     shared.strictness,
					PublishContext: shared.context,
					SuitePath:      shared.suite,
					Runner:         shared.runner,
					Backends:       append([]string(nil), shared.backends...),
				})
				if err := artifact.WriteWorkspaceReportArtifact(reportOptions.artifact, value); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}
			return nil
		},
	}
	addWorkspaceSharedFlags(cmd, &shared, true)
	addWorkspaceChangeFlags(cmd, &shared)
	addWorkspaceGateFlags(cmd, &gateOptions)
	cmd.Flags().StringVar(&reportOptions.artifact, "artifact", "", "Write a versioned workspace report artifact to the given file path")
	return cmd
}

func defaultWorkspaceSharedOptions() workspaceSharedOptions {
	return workspaceSharedOptions{
		format:     skillLintFormatText,
		profile:    string(service.SkillLintProfileGeneric),
		strictness: string(lint.StrictnessDefault),
		context:    string(readiness.ContextInternal),
	}
}

func defaultWorkspaceGateOptions() workspaceGateOptions {
	return workspaceGateOptions{
		maxNotReadySkills:             0,
		maxInsufficientEvidenceSkills: 0,
		maxSkillsWithLintErrors:       0,
		maxDiscoveryWarnings:          0,
	}
}

func addWorkspaceSharedFlags(cmd *cobra.Command, options *workspaceSharedOptions, includeReadiness bool) {
	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	cmd.Flags().StringVar(&options.profile, "profile", string(service.SkillLintProfileGeneric), "Lint profile: generic, codex, claude-code, copilot, or cursor")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness: default, strict, or pedantic")
	if includeReadiness {
		cmd.Flags().StringVar(&options.context, "context", string(readiness.ContextInternal), "Publish context: internal, merge, release-candidate, public-release, public-attestation, or public-trust-report")
		cmd.Flags().StringVar(&options.suite, "suite", "", "Routing eval suite path for measured readiness evidence")
		cmd.Flags().StringVar(&options.runner, "runner", "", "Single-backend routing eval runner for measured readiness evidence")
		cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection in the form "<id>" or "<id>=/path/to/runner"; repeat for multiple backends`)
	}
}

func addWorkspaceChangeFlags(cmd *cobra.Command, options *workspaceSharedOptions) {
	cmd.Flags().BoolVar(&options.changed, "changed", false, "Limit workspace analysis to directly changed or impacted skills from local git state")
	cmd.Flags().StringVar(&options.base, "base", "", "Base git revision for changed-scope analysis; defaults to working tree vs HEAD when omitted")
	cmd.Flags().StringVar(&options.head, "head", "", "Head git revision for changed-scope analysis; requires --base")
}

func addWorkspaceGateFlags(cmd *cobra.Command, options *workspaceGateOptions) {
	cmd.Flags().IntVar(&options.maxNotReadySkills, "max-not-ready-skills", 0, "Maximum allowed number of not-ready skills")
	cmd.Flags().IntVar(&options.maxInsufficientEvidenceSkills, "max-insufficient-evidence-skills", 0, "Maximum allowed number of insufficient-evidence skills")
	cmd.Flags().IntVar(&options.maxSkillsWithLintErrors, "max-skills-with-lint-errors", 0, "Maximum allowed number of skills with lint errors")
	cmd.Flags().IntVar(&options.maxDiscoveryWarnings, "max-discovery-warnings", 0, "Maximum allowed number of workspace discovery warnings")
}

func (o workspaceSharedOptions) validate(includeReadiness bool) error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
	if _, err := service.ParseSkillLintProfile(o.profile); err != nil {
		return err
	}
	if _, err := lint.ParseStrictness(o.strictness); err != nil {
		return err
	}
	if !includeReadiness {
		return o.validateChangeScope()
	}
	if _, err := readiness.ParsePublishContext(o.context); err != nil {
		return err
	}
	if len(o.backends) > 0 && o.runner != "" {
		return fmt.Errorf("--runner cannot be combined with --backend")
	}
	return o.validateChangeScope()
}

func (o workspaceSharedOptions) validateChangeScope() error {
	if strings.TrimSpace(o.head) != "" && strings.TrimSpace(o.base) == "" {
		return fmt.Errorf("--head requires --base")
	}
	return nil
}

func (o workspaceGateOptions) criteria() workspacepkg.GateCriteria {
	return workspacepkg.GateCriteria{
		MaxNotReadySkills:             o.maxNotReadySkills,
		MaxInsufficientEvidenceSkills: o.maxInsufficientEvidenceSkills,
		MaxSkillsWithLintErrors:       o.maxSkillsWithLintErrors,
		MaxDiscoveryWarnings:          o.maxDiscoveryWarnings,
	}
}

func analyzeWorkspace(
	application *app.App,
	args []string,
	shared workspaceSharedOptions,
	criteria *workspacepkg.GateCriteria,
	includeReadiness bool,
) (workspacepkg.Report, error) {
	profile, err := service.ParseSkillLintProfile(shared.profile)
	if err != nil {
		return workspacepkg.Report{}, err
	}
	strictness, err := lint.ParseStrictness(shared.strictness)
	if err != nil {
		return workspacepkg.Report{}, err
	}
	backends, err := parseSkillEvalBackendSelections(shared.backends)
	if err != nil {
		return workspacepkg.Report{}, err
	}
	contextValue := readiness.ContextInternal
	if includeReadiness {
		contextValue, err = readiness.ParsePublishContext(shared.context)
		if err != nil {
			return workspacepkg.Report{}, err
		}
	}
	target := workspaceTargetArg(args)
	var (
		scope         *workspacepkg.ChangeScope
		selectedPaths []string
	)
	if shared.changed {
		calculated, err := application.Services.Workspace.Changes(target, service.WorkspaceChangeOptions{
			BaseRev: shared.base,
			HeadRev: shared.head,
		})
		if err != nil {
			return workspacepkg.Report{}, err
		}
		scope = &calculated
		selectedPaths = make([]string, 0, len(calculated.SelectedSkills))
		for _, skill := range calculated.SelectedSkills {
			selectedPaths = append(selectedPaths, skill.Path)
		}
	}
	return application.Services.Workspace.Analyze(target, service.WorkspaceAnalyzeOptions{
		Profile:            profile,
		Strictness:         strictness,
		IncludeReadiness:   includeReadiness,
		ReadinessContext:   contextValue,
		SuitePath:          shared.suite,
		Runner:             shared.runner,
		Backends:           backends,
		GateCriteria:       criteria,
		SelectedSkillPaths: selectedPaths,
		ChangeScope:        scope,
	})
}

func workspaceTargetArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}

func writeWorkspaceLintReport(w io.Writer, report workspacepkg.Report, format string) error {
	switch format {
	case skillLintFormatText:
		return writeWorkspaceLintText(w, report)
	case skillLintFormatJSON:
		return writeWorkspaceJSON(w, report)
	default:
		return fmt.Errorf("invalid format %q", format)
	}
}

func writeWorkspaceReadinessReport(w io.Writer, report workspacepkg.Report, format string) error {
	switch format {
	case skillLintFormatText:
		return writeWorkspaceReadinessText(w, report)
	case skillLintFormatJSON:
		return writeWorkspaceJSON(w, report)
	default:
		return fmt.Errorf("invalid format %q", format)
	}
}

func writeWorkspaceGateReport(w io.Writer, report workspacepkg.Report, format string) error {
	switch format {
	case skillLintFormatText:
		return writeWorkspaceGateText(w, report)
	case skillLintFormatJSON:
		return writeWorkspaceJSON(w, report)
	default:
		return fmt.Errorf("invalid format %q", format)
	}
}

func writeWorkspaceFullReport(w io.Writer, report workspacepkg.Report, format string) error {
	switch format {
	case skillLintFormatText:
		return writeWorkspaceFullText(w, report)
	case skillLintFormatJSON:
		return writeWorkspaceJSON(w, report)
	default:
		return fmt.Errorf("invalid format %q", format)
	}
}

func writeWorkspaceJSON(w io.Writer, report workspacepkg.Report) error {
	payload := struct {
		SchemaVersion string              `json:"schema_version"`
		Report        workspacepkg.Report `json:"report"`
	}{
		SchemaVersion: workspaceJSONSchemaVersion,
		Report:        report,
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func writeWorkspaceChangesReport(w io.Writer, scope workspacepkg.ChangeScope, format string) error {
	switch format {
	case skillLintFormatText:
		return writeWorkspaceChangesText(w, scope)
	case skillLintFormatJSON:
		payload := struct {
			SchemaVersion string                   `json:"schema_version"`
			Scope         workspacepkg.ChangeScope `json:"scope"`
		}{
			SchemaVersion: workspaceJSONSchemaVersion,
			Scope:         scope,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q", format)
	}
}

func writeWorkspaceLintText(w io.Writer, report workspacepkg.Report) error {
	lines := workspaceScopeHeader(report.ChangeScope)
	lines = append(lines,
		fmt.Sprintf("Workspace: %s", report.WorkspaceRoot),
		fmt.Sprintf("Skills: %d", report.Summary.SkillCount),
		fmt.Sprintf("Clean: %d", report.Summary.CleanSkills),
		fmt.Sprintf("With warnings: %d", report.Summary.SkillsWithWarnings),
		fmt.Sprintf("With lint errors: %d", report.Summary.SkillsWithLintErrors),
		fmt.Sprintf("Totals: %d lint error(s), %d lint warning(s)", report.Summary.TotalLintErrors, report.Summary.TotalLintWarnings),
	)
	if len(report.Discovery.Warnings) > 0 {
		lines = append(lines, "Discovery warnings:")
		for _, warning := range report.Discovery.Warnings {
			lines = append(lines, "- "+warning.Summary)
		}
	}
	lines = append(lines, "Per skill:")
	for _, skill := range report.Skills {
		lines = append(lines, fmt.Sprintf("- %s: %s", skill.Skill.Name, skill.Lint.Summary))
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

func writeWorkspaceReadinessText(w io.Writer, report workspacepkg.Report) error {
	lines := workspaceScopeHeader(report.ChangeScope)
	lines = append(lines,
		fmt.Sprintf("Workspace: %s", report.WorkspaceRoot),
		fmt.Sprintf("Skills: %d", report.Summary.SkillCount),
		fmt.Sprintf("Ready: %d", report.Summary.ReadySkills),
		fmt.Sprintf("Ready with caveats: %d", report.Summary.ReadyWithCaveatsSkills),
		fmt.Sprintf("Not ready: %d", report.Summary.NotReadySkills),
		fmt.Sprintf("Insufficient evidence: %d", report.Summary.InsufficientEvidenceSkills),
	)
	if len(report.Summary.WorkspaceBlockers) > 0 {
		lines = append(lines, "Top blockers:")
		for _, item := range report.Summary.WorkspaceBlockers[:min(len(report.Summary.WorkspaceBlockers), 5)] {
			lines = append(lines, "- "+item)
		}
	}
	lines = append(lines, "Per skill:")
	for _, skill := range report.Skills {
		if skill.Readiness == nil {
			lines = append(lines, fmt.Sprintf("- %s: %s", skill.Skill.Name, skill.Lint.Summary))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", skill.Skill.Name, skill.Readiness.Summary, strings.ToUpper(string(skill.Readiness.Decision))))
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

func writeWorkspaceGateText(w io.Writer, report workspacepkg.Report) error {
	if report.Gate == nil {
		return fmt.Errorf("workspace gate result is missing")
	}
	lines := workspaceScopeHeader(report.ChangeScope)
	lines = append(lines,
		fmt.Sprintf("Decision: %s", strings.ToUpper(string(report.Gate.Decision))),
		fmt.Sprintf("Workspace: %s", report.WorkspaceRoot),
		fmt.Sprintf("Summary: %s", report.Gate.Summary),
		fmt.Sprintf("Metrics: not-ready=%d insufficient-evidence=%d lint-error-skills=%d discovery-warnings=%d",
			report.Gate.Metrics.NotReadySkills,
			report.Gate.Metrics.InsufficientEvidenceSkills,
			report.Gate.Metrics.SkillsWithLintErrors,
			report.Gate.Metrics.DiscoveryWarnings,
		),
	)
	if len(report.Gate.BlockingReasons) > 0 {
		lines = append(lines, "Blocking reasons:")
		for _, reason := range report.Gate.BlockingReasons {
			lines = append(lines, "- "+reason)
		}
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

func writeWorkspaceFullText(w io.Writer, report workspacepkg.Report) error {
	lines := workspaceScopeHeader(report.ChangeScope)
	lines = append(lines,
		fmt.Sprintf("Workspace: %s", report.WorkspaceRoot),
		fmt.Sprintf("Skills discovered: %d", report.Summary.SkillCount),
		fmt.Sprintf("Lint totals: %d error(s), %d warning(s)", report.Summary.TotalLintErrors, report.Summary.TotalLintWarnings),
		fmt.Sprintf("Readiness: ready=%d, ready-with-caveats=%d, not-ready=%d, insufficient-evidence=%d",
			report.Summary.ReadySkills,
			report.Summary.ReadyWithCaveatsSkills,
			report.Summary.NotReadySkills,
			report.Summary.InsufficientEvidenceSkills,
		),
	)
	if report.Gate != nil {
		lines = append(lines, fmt.Sprintf("Workspace gate: %s", strings.ToUpper(string(report.Gate.Decision))))
	}
	if len(report.Summary.TopPriorities) > 0 {
		lines = append(lines, "Top priorities:")
		for _, item := range report.Summary.TopPriorities {
			lines = append(lines, "- "+item)
		}
	}
	lines = append(lines, "Per skill:")
	for _, skill := range report.Skills {
		line := fmt.Sprintf("- %s: %s", skill.Skill.Name, skill.Lint.Summary)
		if skill.Readiness != nil {
			line += fmt.Sprintf("; readiness=%s", skill.Readiness.Decision)
			if skill.Readiness.SupportPosture != "" {
				line += fmt.Sprintf("; posture=%s", skill.Readiness.SupportPosture)
			}
		}
		lines = append(lines, line)
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

func writeWorkspaceChangesText(w io.Writer, scope workspacepkg.ChangeScope) error {
	lines := []string{
		fmt.Sprintf("Workspace: %s", scope.WorkspaceRoot),
		fmt.Sprintf("Diff: %s", scope.DiffContext.Summary),
		fmt.Sprintf("Summary: %s", scope.Summary),
	}
	if len(scope.DirectlyChangedSkills) > 0 {
		lines = append(lines, "Directly changed skills:")
		for _, skill := range scope.DirectlyChangedSkills {
			lines = append(lines, "- "+skill.Name)
		}
	}
	if len(scope.ImpactedSkills) > 0 {
		lines = append(lines, "Impacted skills:")
		for _, skill := range scope.ImpactedSkills {
			lines = append(lines, "- "+skill.Name)
		}
	}
	if len(scope.Caveats) > 0 {
		lines = append(lines, "Caveats:")
		for _, caveat := range scope.Caveats {
			lines = append(lines, "- "+caveat)
		}
	}
	if len(scope.SelectedSkills) > 0 {
		lines = append(lines, fmt.Sprintf("Selected analysis scope: %d skill(s)", len(scope.SelectedSkills)))
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

func workspaceScopeHeader(scope *workspacepkg.ChangeScope) []string {
	if scope == nil {
		return nil
	}
	lines := []string{
		fmt.Sprintf("Changed scope: %s", scope.Summary),
	}
	if len(scope.Caveats) > 0 {
		lines = append(lines, "Scope caveats:")
		for _, caveat := range scope.Caveats[:min(len(scope.Caveats), 3)] {
			lines = append(lines, "- "+caveat)
		}
	}
	return lines
}
