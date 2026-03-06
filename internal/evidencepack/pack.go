package evidencepack

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/artifactview"
	domaineval "github.com/firety/firety/internal/domain/eval"
	domaingate "github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/provenance"
	"github.com/firety/firety/internal/render"
	"github.com/firety/firety/internal/service"
)

const (
	SchemaVersion = "1"
	PackType      = "firety.evidence-pack"
)

type PackOptions struct {
	OutputDir            string
	InputArtifacts       []string
	Profile              service.SkillLintProfile
	Strictness           lint.Strictness
	FailOn               string
	Explain              bool
	RoutingRisk          bool
	Runner               string
	SuitePath            string
	BackendSelections    []service.SkillEvalBackendSelection
	IncludePlan          bool
	IncludeCompatibility bool
	IncludeGate          bool
}

type Builder struct {
	application *app.App
}

type Result struct {
	OutputDir string
	Manifest  Manifest
}

type Manifest struct {
	SchemaVersion          string             `json:"schema_version"`
	PackType               string             `json:"pack_type"`
	Tool                   ToolInfo           `json:"tool"`
	Source                 string             `json:"source"`
	Target                 string             `json:"target,omitempty"`
	ReviewSummary          string             `json:"review_summary"`
	RecommendedEntrypoints []string           `json:"recommended_entrypoints"`
	Context                PackContext        `json:"context"`
	Provenance             provenance.Record  `json:"provenance"`
	Artifacts              []ManifestArtifact `json:"artifacts"`
	Reports                []ManifestReport   `json:"reports"`
}

type ToolInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
}

type PackContext struct {
	Profile              string   `json:"profile,omitempty"`
	Strictness           string   `json:"strictness,omitempty"`
	FailOn               string   `json:"fail_on,omitempty"`
	Explain              bool     `json:"explain,omitempty"`
	RoutingRisk          bool     `json:"routing_risk,omitempty"`
	SuitePath            string   `json:"suite_path,omitempty"`
	Backends             []string `json:"backends,omitempty"`
	InputArtifacts       []string `json:"input_artifacts,omitempty"`
	IncludePlan          bool     `json:"include_plan,omitempty"`
	IncludeCompatibility bool     `json:"include_compatibility,omitempty"`
	IncludeGate          bool     `json:"include_gate,omitempty"`
}

type ManifestArtifact struct {
	Path            string   `json:"path"`
	ArtifactType    string   `json:"artifact_type"`
	Origin          string   `json:"origin,omitempty"`
	Target          string   `json:"target,omitempty"`
	BaseTarget      string   `json:"base_target,omitempty"`
	CandidateTarget string   `json:"candidate_target,omitempty"`
	Summary         string   `json:"summary,omitempty"`
	Context         []string `json:"context,omitempty"`
}

type ManifestReport struct {
	Path               string `json:"path"`
	SourceArtifact     string `json:"source_artifact"`
	SourceArtifactType string `json:"source_artifact_type"`
	RenderMode         string `json:"render_mode"`
}

type packArtifact struct {
	relativePath string
	sourcePath   string
	inspection   artifactview.Inspection
}

func NewBuilder(application *app.App) Builder {
	return Builder{application: application}
}

func (b Builder) Build(target string, options PackOptions) (Result, error) {
	if err := validateOptions(target, options); err != nil {
		return Result{}, err
	}

	root, err := prepareOutputDir(options.OutputDir)
	if err != nil {
		return Result{}, err
	}

	artifactsDir := filepath.Join(root, "artifacts")
	reportsDir := filepath.Join(root, "reports")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return Result{}, err
	}

	var packArtifacts []packArtifact
	if len(options.InputArtifacts) > 0 {
		packArtifacts, err = b.collectInputArtifacts(root, options.InputArtifacts)
		if err != nil {
			return Result{}, err
		}
	} else {
		packArtifacts, err = b.buildFreshArtifacts(root, target, options)
		if err != nil {
			return Result{}, err
		}
	}

	reports, err := buildRenderedReports(root, packArtifacts)
	if err != nil {
		return Result{}, err
	}

	manifest, err := buildManifest(b.application, target, options, packArtifacts, reports)
	if err != nil {
		return Result{}, err
	}
	if err := writeManifest(filepath.Join(root, "manifest.json"), manifest); err != nil {
		return Result{}, err
	}
	if err := writeSummary(filepath.Join(root, "SUMMARY.md"), manifest); err != nil {
		return Result{}, err
	}

	return Result{
		OutputDir: root,
		Manifest:  manifest,
	}, nil
}

func validateOptions(target string, options PackOptions) error {
	if strings.TrimSpace(options.OutputDir) == "" {
		return fmt.Errorf("output directory must not be empty")
	}
	if len(options.InputArtifacts) > 0 && strings.TrimSpace(target) != "" {
		return fmt.Errorf("evidence pack accepts either a target path or --input-artifact values, not both")
	}
	if len(options.InputArtifacts) == 0 && strings.TrimSpace(target) == "" {
		return fmt.Errorf("evidence pack requires a target path or at least one --input-artifact value")
	}
	if len(options.BackendSelections) > 0 && options.Runner != "" {
		return fmt.Errorf("--runner cannot be combined with --backend in evidence pack mode")
	}
	return nil
}

func prepareOutputDir(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absolute)
	switch {
	case err == nil:
		if !info.IsDir() {
			return "", fmt.Errorf("output path %s is not a directory", absolute)
		}
		entries, err := os.ReadDir(absolute)
		if err != nil {
			return "", err
		}
		if len(entries) > 0 {
			return "", fmt.Errorf("output directory %s must be empty", absolute)
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(absolute, 0o755); err != nil {
			return "", err
		}
	default:
		return "", err
	}

	return absolute, nil
}

func (b Builder) collectInputArtifacts(root string, inputs []string) ([]packArtifact, error) {
	artifacts := make([]packArtifact, 0, len(inputs))
	typeCounts := make(map[string]int)
	currentTarget := ""

	for _, input := range inputs {
		info, err := artifactview.Inspect(input)
		if err != nil {
			return nil, err
		}
		target := currentPackTarget(info)
		if target != "" {
			if currentTarget == "" {
				currentTarget = target
			} else if currentTarget != target {
				return nil, fmt.Errorf("artifact targets are incompatible for a single evidence pack: %q vs %q", currentTarget, target)
			}
		}

		name := artifactFileName(info.ArtifactType, typeCounts)
		relative := filepath.ToSlash(filepath.Join("artifacts", name+".json"))
		destination := filepath.Join(root, relative)
		if err := copyFile(input, destination); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, packArtifact{
			relativePath: relative,
			sourcePath:   destination,
			inspection:   info,
		})
	}

	return maybeAddDerivedArtifacts(b.application, root, artifacts)
}

func maybeAddDerivedArtifacts(_ *app.App, _ string, existing []packArtifact) ([]packArtifact, error) {
	return existing, nil
}

func (b Builder) buildFreshArtifacts(root, target string, options PackOptions) ([]packArtifact, error) {
	artifacts := make([]packArtifact, 0, 5)

	lintReport, err := b.application.Services.SkillLint.LintWithProfileAndStrictness(target, options.Profile, options.Strictness)
	if err != nil {
		return nil, err
	}
	lintArtifact := artifact.BuildSkillLintArtifact(
		b.application.Version,
		lintReport,
		service.SkillFixResult{},
		artifact.SkillLintArtifactOptions{
			Format:      skillArtifactOutputFormat,
			Profile:     string(options.Profile),
			Strictness:  options.Strictness.DisplayName(),
			FailOn:      options.FailOn,
			Explain:     options.Explain,
			RoutingRisk: options.RoutingRisk,
		},
		lintExitCode(lintReport, options.FailOn),
	)
	item, err := writePackArtifact(root, lintArtifact)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, item)

	if len(options.BackendSelections) > 0 {
		report, err := b.application.Services.SkillEval.EvaluateAcrossBackends(target, options.SuitePath, options.BackendSelections)
		if err != nil {
			return nil, err
		}
		multiArtifact := artifact.BuildSkillEvalMultiArtifact(
			b.application.Version,
			report,
			artifact.SkillEvalMultiArtifactOptions{
				Format: skillArtifactOutputFormat,
				Suite:  report.Suite.Path,
			},
			evalMultiExitCode(report),
		)
		item, err := writePackArtifact(root, multiArtifact)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, item)
	} else if options.Runner != "" {
		report, err := b.application.Services.SkillEval.Evaluate(target, service.SkillEvalOptions{
			SuitePath: options.SuitePath,
			Profile:   options.Profile,
			Runner:    options.Runner,
		})
		if err != nil {
			return nil, err
		}
		evalArtifact := artifact.BuildSkillEvalArtifact(
			b.application.Version,
			report,
			artifact.SkillEvalArtifactOptions{
				Format:  skillArtifactOutputFormat,
				Profile: string(options.Profile),
				Suite:   report.Suite.Path,
				Runner:  options.Runner,
			},
			evalExitCode(report),
		)
		item, err := writePackArtifact(root, evalArtifact)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, item)
	}

	if options.IncludePlan {
		planResult, err := b.application.Services.SkillPlan.Build(target, service.SkillPlanOptions{
			Profile:           options.Profile,
			Strictness:        options.Strictness,
			SuitePath:         options.SuitePath,
			Runner:            options.Runner,
			BackendSelections: options.BackendSelections,
		})
		if err != nil {
			return nil, err
		}
		planArtifact := artifact.BuildSkillPlanArtifact(
			b.application.Version,
			planResult,
			artifact.SkillPlanArtifactOptions{
				Format:     skillArtifactOutputFormat,
				Profile:    string(options.Profile),
				Strictness: options.Strictness.DisplayName(),
				FailOn:     options.FailOn,
				Suite:      options.SuitePath,
			},
			0,
		)
		item, err := writePackArtifact(root, planArtifact)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, item)
	}

	if options.IncludeCompatibility {
		inputs := filterArtifactPaths(artifacts, map[string]struct{}{
			"firety.skill-lint":               {},
			"firety.skill-routing-eval":       {},
			"firety.skill-routing-eval-multi": {},
			"firety.skill-analysis":           {},
		})
		compatibilityResult, err := b.application.Services.SkillCompatibility.Analyze("", service.SkillCompatibilityOptions{
			InputArtifacts: inputs,
		})
		if err != nil {
			return nil, err
		}
		compatibilityArtifact := artifact.BuildSkillCompatibilityArtifact(
			b.application.Version,
			compatibilityResult.Report,
			artifact.SkillCompatibilityArtifactOptions{
				Format:         skillArtifactOutputFormat,
				Target:         target,
				Profiles:       []string{string(options.Profile)},
				Strictness:     options.Strictness.DisplayName(),
				SuitePath:      options.SuitePath,
				Backends:       backendIDs(options.BackendSelections),
				InputArtifacts: inputs,
			},
		)
		item, err := writePackArtifact(root, compatibilityArtifact)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, item)
	}

	if options.IncludeGate {
		inputs := filterArtifactPaths(artifacts, map[string]struct{}{
			"firety.skill-lint":                       {},
			"firety.skill-lint-compare":               {},
			"firety.skill-routing-eval":               {},
			"firety.skill-routing-eval-compare":       {},
			"firety.skill-routing-eval-multi":         {},
			"firety.skill-routing-eval-compare-multi": {},
			"firety.skill-analysis":                   {},
		})
		gateResult, err := b.application.Services.SkillGate.Evaluate("", service.SkillGateOptions{
			InputArtifacts: inputs,
			Profile:        options.Profile,
			Strictness:     options.Strictness,
		})
		if err != nil {
			return nil, err
		}
		gateArtifact := artifact.BuildSkillGateArtifact(
			b.application.Version,
			gateResult.Gate,
			artifact.SkillGateArtifactOptions{
				Format:         skillArtifactOutputFormat,
				Target:         target,
				Profile:        string(options.Profile),
				Strictness:     options.Strictness.DisplayName(),
				SuitePath:      options.SuitePath,
				Runner:         options.Runner,
				Backends:       backendIDs(options.BackendSelections),
				InputArtifacts: inputs,
			},
			gateExitCode(gateResult.Gate),
		)
		item, err := writePackArtifact(root, gateArtifact)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, item)
	}

	return artifacts, nil
}

func writePackArtifact(root string, value any) (packArtifact, error) {
	var name string
	var write func(string) error

	switch item := value.(type) {
	case artifact.SkillLintArtifact:
		name = "skill-lint"
		write = func(path string) error { return artifact.WriteSkillLintArtifact(path, item) }
	case artifact.SkillEvalArtifact:
		name = "skill-routing-eval"
		write = func(path string) error { return artifact.WriteSkillEvalArtifact(path, item) }
	case artifact.SkillEvalMultiArtifact:
		name = "skill-routing-eval-multi"
		write = func(path string) error { return artifact.WriteSkillEvalMultiArtifact(path, item) }
	case artifact.SkillPlanArtifact:
		name = "skill-improvement-plan"
		write = func(path string) error { return artifact.WriteSkillPlanArtifact(path, item) }
	case artifact.SkillCompatibilityArtifact:
		name = "skill-compatibility"
		write = func(path string) error { return artifact.WriteSkillCompatibilityArtifact(path, item) }
	case artifact.SkillGateArtifact:
		name = "skill-quality-gate"
		write = func(path string) error { return artifact.WriteSkillGateArtifact(path, item) }
	default:
		return packArtifact{}, fmt.Errorf("unsupported pack artifact type %T", value)
	}

	relative := filepath.ToSlash(filepath.Join("artifacts", name+".json"))
	destination := filepath.Join(root, relative)
	if err := write(destination); err != nil {
		return packArtifact{}, err
	}
	info, err := artifactview.Inspect(destination)
	if err != nil {
		return packArtifact{}, err
	}

	return packArtifact{
		relativePath: relative,
		sourcePath:   destination,
		inspection:   info,
	}, nil
}

func buildRenderedReports(root string, artifacts []packArtifact) ([]ManifestReport, error) {
	reports := make([]ManifestReport, 0, len(artifacts)*2)

	for _, item := range artifacts {
		if len(item.inspection.SupportedRenderModes) == 0 {
			continue
		}

		baseName := strings.TrimSuffix(filepath.Base(item.relativePath), filepath.Ext(item.relativePath))
		for _, mode := range []render.Mode{render.ModeCISummary, render.ModeFullReport} {
			output, err := artifactview.Render(item.sourcePath, mode)
			if err != nil {
				return nil, err
			}
			relative := filepath.ToSlash(filepath.Join("reports", fmt.Sprintf("%s-%s.md", baseName, mode)))
			if err := os.WriteFile(filepath.Join(root, relative), []byte(output), 0o644); err != nil {
				return nil, err
			}
			reports = append(reports, ManifestReport{
				Path:               relative,
				SourceArtifact:     item.relativePath,
				SourceArtifactType: item.inspection.ArtifactType,
				RenderMode:         string(mode),
			})
		}
	}

	sort.Slice(reports, func(i, j int) bool { return reports[i].Path < reports[j].Path })
	return reports, nil
}

func buildManifest(application *app.App, target string, options PackOptions, artifacts []packArtifact, reports []ManifestReport) (Manifest, error) {
	manifestArtifacts := make([]ManifestArtifact, 0, len(artifacts))
	reviewSummaryParts := make([]string, 0, len(artifacts))
	for _, item := range artifacts {
		manifestArtifacts = append(manifestArtifacts, ManifestArtifact{
			Path:            item.relativePath,
			ArtifactType:    item.inspection.ArtifactType,
			Origin:          item.inspection.Origin,
			Target:          item.inspection.Target,
			BaseTarget:      item.inspection.BaseTarget,
			CandidateTarget: item.inspection.CandidateTarget,
			Summary:         item.inspection.Summary,
			Context:         append([]string(nil), item.inspection.Context...),
		})
		if item.inspection.Summary != "" {
			reviewSummaryParts = append(reviewSummaryParts, fmt.Sprintf("%s: %s", item.inspection.ArtifactType, item.inspection.Summary))
		}
	}
	sort.Slice(manifestArtifacts, func(i, j int) bool { return manifestArtifacts[i].Path < manifestArtifacts[j].Path })

	entrypoints := []string{"SUMMARY.md"}
	for _, report := range reports {
		if report.RenderMode != string(render.ModeCISummary) {
			continue
		}
		entrypoints = append(entrypoints, report.Path)
	}
	entrypoints = uniqueStrings(entrypoints)

	reviewSummary := "Firety packaged quality evidence for offline review."
	if len(reviewSummaryParts) > 0 {
		reviewSummary = strings.Join(reviewSummaryParts, " ")
	}

	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		PackType:      PackType,
		Tool: ToolInfo{
			Name:      "firety",
			Version:   application.Version.Version,
			Commit:    application.Version.Commit,
			BuildDate: application.Version.Date,
		},
		Source:                 firstNonEmpty(packSource(options), "fresh-analysis"),
		Target:                 firstNonEmpty(target, primaryTargetFromArtifacts(artifacts)),
		ReviewSummary:          reviewSummary,
		RecommendedEntrypoints: entrypoints,
		Context: PackContext{
			Profile:              string(options.Profile),
			Strictness:           options.Strictness.DisplayName(),
			FailOn:               options.FailOn,
			Explain:              options.Explain,
			RoutingRisk:          options.RoutingRisk,
			SuitePath:            options.SuitePath,
			Backends:             backendIDs(options.BackendSelections),
			InputArtifacts:       append([]string(nil), options.InputArtifacts...),
			IncludePlan:          options.IncludePlan,
			IncludeCompatibility: options.IncludeCompatibility,
			IncludeGate:          options.IncludeGate,
		},
		Provenance: buildPackProvenance(application, target, options, manifestArtifacts),
		Artifacts:  manifestArtifacts,
		Reports:    reports,
	}
	return manifest, nil
}

func packSource(options PackOptions) string {
	if len(options.InputArtifacts) > 0 {
		return "existing-artifacts"
	}
	return "fresh-analysis"
}

func primaryTargetFromArtifacts(values []packArtifact) string {
	for _, value := range values {
		if target := currentPackTarget(value.inspection); target != "" {
			return target
		}
	}
	return ""
}

func currentPackTarget(info artifactview.Inspection) string {
	if strings.TrimSpace(info.CandidateTarget) != "" {
		return info.CandidateTarget
	}
	return strings.TrimSpace(info.Target)
}

func buildPackProvenance(application *app.App, target string, options PackOptions, artifacts []ManifestArtifact) provenance.Record {
	record := provenance.NewRecord()
	record.CommandOrigin = "firety evidence pack"
	record.FiretyVersion = application.Version.Version
	record.FiretyCommit = application.Version.Commit
	record.Target = firstNonEmpty(target, firstNonEmptyArtifactTarget(artifacts))
	record.Profile = string(options.Profile)
	record.Strictness = options.Strictness.DisplayName()
	record.FailOn = options.FailOn
	record.Explain = options.Explain
	record.RoutingRisk = options.RoutingRisk
	record.SuitePath = strings.TrimSpace(options.SuitePath)
	record.Backends = backendIDs(options.BackendSelections)
	record.InputArtifacts = append([]string(nil), options.InputArtifacts...)
	record.ArtifactDependencies = artifactDependencyPaths(artifacts)
	record.ComparableKey = packComparableKey(options)

	if strings.TrimSpace(target) != "" {
		fingerprint, err := provenance.FingerprintDirectory(target)
		if err == nil {
			record.TargetFingerprint = fingerprint
		} else {
			record.ReproducibilityNotes = append(record.ReproducibilityNotes, fmt.Sprintf("target fingerprint could not be computed: %v", err))
		}
	} else {
		record.ReproducibilityNotes = append(record.ReproducibilityNotes, "pack was built from existing artifacts rather than a fresh target run")
	}
	if len(options.InputArtifacts) > 0 {
		record.ReproducibilityNotes = append(record.ReproducibilityNotes, "pack content depends on supplied input artifacts")
	}
	if record.SuitePath == "" && len(record.Backends) > 0 {
		record.ComparabilityNotes = append(record.ComparabilityNotes, "backend evidence is present without an explicit suite path")
	}

	return provenance.NormalizeRecord(record)
}

func artifactDependencyPaths(values []ManifestArtifact) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.Path)
	}
	return out
}

func firstNonEmptyArtifactTarget(values []ManifestArtifact) string {
	for _, value := range values {
		if target := firstNonEmpty(value.CandidateTarget, value.Target); target != "" {
			return target
		}
	}
	return ""
}

func packComparableKey(options PackOptions) string {
	parts := []string{
		"source:" + firstNonEmpty(packSource(options), "fresh-analysis"),
		"profile:" + string(options.Profile),
		"strictness:" + options.Strictness.DisplayName(),
		"fail-on:" + options.FailOn,
		"suite:" + strings.TrimSpace(options.SuitePath),
	}
	backends := backendIDs(options.BackendSelections)
	if len(backends) > 0 {
		parts = append(parts, "backends:"+strings.Join(backends, ","))
	}
	return strings.Join(parts, "|")
}

func writeManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeSummary(path string, manifest Manifest) error {
	var b strings.Builder
	writeLine(&b, "# Firety Evidence Pack")
	writeLine(&b, "")
	writeLine(&b, fmt.Sprintf("- Source: %s", manifest.Source))
	if manifest.Target != "" {
		writeLine(&b, fmt.Sprintf("- Target: %s", manifest.Target))
	}
	writeLine(&b, fmt.Sprintf("- Firety: %s", manifest.Tool.Version))
	writeLine(&b, fmt.Sprintf("- Summary: %s", manifest.ReviewSummary))
	writeLine(&b, "")
	if len(manifest.RecommendedEntrypoints) > 0 {
		writeLine(&b, "## Review First")
		writeLine(&b, "")
		for _, item := range manifest.RecommendedEntrypoints {
			writeLine(&b, fmt.Sprintf("- `%s`", item))
		}
		writeLine(&b, "")
	}
	writeLine(&b, "## Included Artifacts")
	writeLine(&b, "")
	for _, item := range manifest.Artifacts {
		line := fmt.Sprintf("- `%s` (%s)", item.Path, item.ArtifactType)
		if item.Summary != "" {
			line += ": " + item.Summary
		}
		writeLine(&b, line)
	}
	if len(manifest.Reports) > 0 {
		writeLine(&b, "")
		writeLine(&b, "## Rendered Reports")
		writeLine(&b, "")
		for _, item := range manifest.Reports {
			writeLine(&b, fmt.Sprintf("- `%s` (%s from `%s`)", item.Path, item.RenderMode, item.SourceArtifact))
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o644)
}

func artifactFileName(artifactType string, counts map[string]int) string {
	base := strings.TrimPrefix(artifactType, "firety.")
	base = strings.ReplaceAll(base, ".", "-")
	counts[base]++
	if counts[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, counts[base])
}

func backendIDs(values []service.SkillEvalBackendSelection) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.ID)
	}
	return out
}

func packArtifactPaths(values []packArtifact) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.sourcePath)
	}
	return out
}

func filterArtifactPaths(values []packArtifact, allowed map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := allowed[value.inspection.ArtifactType]; !ok {
			continue
		}
		out = append(out, value.sourcePath)
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func lintExitCode(report lint.Report, failOn string) int {
	switch strings.TrimSpace(failOn) {
	case "warnings":
		if report.HasErrors() || report.WarningCount() > 0 {
			return 1
		}
	default:
		if report.HasErrors() {
			return 1
		}
	}
	return 0
}

func evalExitCode(report domaineval.RoutingEvalReport) int {
	if report.Summary.Failed > 0 {
		return 1
	}
	return 0
}

func evalMultiExitCode(report domaineval.MultiBackendEvalReport) int {
	for _, backend := range report.Backends {
		if backend.Summary.Failed > 0 {
			return 1
		}
	}
	return 0
}

func gateExitCode(result domaingate.Result) int {
	if result.Decision == domaingate.DecisionFail {
		return 1
	}
	return 0
}

const skillArtifactOutputFormat = "json"

func writeLine(w io.StringWriter, value string) {
	_, _ = w.WriteString(value + "\n")
}
