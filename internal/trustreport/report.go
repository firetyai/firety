package trustreport

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/artifactview"
	"github.com/firety/firety/internal/domain/attestation"
	"github.com/firety/firety/internal/domain/compatibility"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/evidencepack"
	"github.com/firety/firety/internal/render"
	"github.com/firety/firety/internal/service"
)

const (
	SchemaVersion = "1"
	ReportType    = "firety.trust-report"
)

type BuildOptions struct {
	OutputDir         string
	InputArtifacts    []string
	InputPacks        []string
	Profile           service.SkillLintProfile
	Strictness        lint.Strictness
	FailOn            string
	Explain           bool
	RoutingRisk       bool
	Runner            string
	SuitePath         string
	BackendSelections []service.SkillEvalBackendSelection
	IncludePlan       bool
	IncludeGate       bool
}

type Builder struct {
	application *app.App
}

type Result struct {
	OutputDir string
	Manifest  Manifest
}

type Manifest struct {
	SchemaVersion          string            `json:"schema_version"`
	ReportType             string            `json:"report_type"`
	Tool                   ToolInfo          `json:"tool"`
	Source                 string            `json:"source"`
	Target                 string            `json:"target,omitempty"`
	Summary                string            `json:"summary"`
	SupportPosture         string            `json:"support_posture,omitempty"`
	EvidenceLevel          string            `json:"evidence_level,omitempty"`
	QualityGateDecision    string            `json:"quality_gate_decision,omitempty"`
	TestedProfiles         []string          `json:"tested_profiles,omitempty"`
	TestedBackends         []string          `json:"tested_backends,omitempty"`
	Claims                 []string          `json:"claims,omitempty"`
	Strengths              []string          `json:"strengths,omitempty"`
	Limitations            []string          `json:"limitations,omitempty"`
	CautionAreas           []string          `json:"caution_areas,omitempty"`
	RecommendedEntrypoints []string          `json:"recommended_entrypoints,omitempty"`
	Context                BuildContext      `json:"context"`
	EvidencePacks          []EvidencePackRef `json:"evidence_packs,omitempty"`
	Artifacts              []ArtifactRef     `json:"artifacts,omitempty"`
	Pages                  []PageRef         `json:"pages,omitempty"`
}

type ToolInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
}

type BuildContext struct {
	Profile        string   `json:"profile,omitempty"`
	Strictness     string   `json:"strictness,omitempty"`
	FailOn         string   `json:"fail_on,omitempty"`
	Explain        bool     `json:"explain,omitempty"`
	RoutingRisk    bool     `json:"routing_risk,omitempty"`
	SuitePath      string   `json:"suite_path,omitempty"`
	Backends       []string `json:"backends,omitempty"`
	InputArtifacts []string `json:"input_artifacts,omitempty"`
	InputPacks     []string `json:"input_packs,omitempty"`
	IncludePlan    bool     `json:"include_plan,omitempty"`
	IncludeGate    bool     `json:"include_gate,omitempty"`
}

type EvidencePackRef struct {
	Path          string `json:"path"`
	Summary       string `json:"summary,omitempty"`
	ArtifactCount int    `json:"artifact_count"`
	ReportCount   int    `json:"report_count"`
}

type ArtifactRef struct {
	Path            string   `json:"path"`
	ArtifactType    string   `json:"artifact_type"`
	Origin          string   `json:"origin,omitempty"`
	Target          string   `json:"target,omitempty"`
	BaseTarget      string   `json:"base_target,omitempty"`
	CandidateTarget string   `json:"candidate_target,omitempty"`
	Summary         string   `json:"summary,omitempty"`
	Context         []string `json:"context,omitempty"`
}

type PageRef struct {
	Path               string `json:"path"`
	Title              string `json:"title"`
	Summary            string `json:"summary,omitempty"`
	SourceArtifact     string `json:"source_artifact"`
	SourceArtifactType string `json:"source_artifact_type"`
}

type packBundle struct {
	relativeRoot string
	manifest     evidencepack.Manifest
	artifacts    []bundleArtifact
}

type bundleArtifact struct {
	relativePath string
	absolutePath string
	inspection   artifactview.Inspection
}

type knownArtifacts struct {
	attestation   *artifact.SkillAttestationArtifact
	compatibility *artifact.SkillCompatibilityArtifact
	gate          *artifact.SkillGateArtifact
	benchmark     *artifact.BenchmarkArtifact
}

type overviewData struct {
	Title                  string
	Summary                string
	Target                 string
	SupportPosture         string
	EvidenceLevel          string
	QualityGateDecision    string
	TestedProfiles         []string
	TestedBackends         []string
	Claims                 []string
	Strengths              []string
	Limitations            []string
	CautionAreas           []string
	RecommendedEntrypoints []entrypointLink
	Pages                  []pageLink
	EvidencePacks          []EvidencePackRef
	Artifacts              []ArtifactRef
}

type pageData struct {
	Title        string
	Summary      string
	ArtifactPath string
	BackLink     string
	Content      string
}

type entrypointLink struct {
	Label string
	Path  string
}

type pageLink struct {
	Title   string
	Path    string
	Summary string
}

func NewBuilder(application *app.App) Builder {
	return Builder{application: application}
}

func (b Builder) Build(target string, options BuildOptions) (Result, error) {
	if err := validateOptions(target, options); err != nil {
		return Result{}, err
	}

	root, err := prepareOutputDir(options.OutputDir)
	if err != nil {
		return Result{}, err
	}

	packs, err := b.prepareEvidence(root, target, options)
	if err != nil {
		return Result{}, err
	}

	allArtifacts := flattenArtifacts(packs)
	known, err := loadKnownArtifacts(allArtifacts)
	if err != nil {
		return Result{}, err
	}

	if known.attestation == nil && canDeriveAttestation(allArtifacts) {
		derivedPath := filepath.Join(root, "artifacts", "skill-attestation.json")
		derived, err := b.deriveAttestationArtifact(derivedPath, allArtifacts)
		if err != nil {
			return Result{}, err
		}
		allArtifacts = append(allArtifacts, derived)
		known, err = loadKnownArtifacts(allArtifacts)
		if err != nil {
			return Result{}, err
		}
	}

	pages, err := buildPages(root, allArtifacts)
	if err != nil {
		return Result{}, err
	}

	manifest := buildManifest(b.application, options, packs, allArtifacts, pages, known)
	if err := writeManifest(filepath.Join(root, "manifest.json"), manifest); err != nil {
		return Result{}, err
	}
	if err := writeOverviewPage(filepath.Join(root, "index.html"), buildOverviewData(manifest)); err != nil {
		return Result{}, err
	}

	return Result{
		OutputDir: root,
		Manifest:  manifest,
	}, nil
}

func validateOptions(target string, options BuildOptions) error {
	if strings.TrimSpace(options.OutputDir) == "" {
		return fmt.Errorf("--output is required")
	}
	if options.OutputDir == "-" {
		return fmt.Errorf(`output path "-" is not supported; choose a directory path`)
	}
	hasInputs := len(options.InputArtifacts) > 0 || len(options.InputPacks) > 0
	if hasInputs && strings.TrimSpace(target) != "" {
		return fmt.Errorf("trust report accepts either a target path or existing artifact/pack inputs, not both")
	}
	if !hasInputs && strings.TrimSpace(target) == "" {
		return fmt.Errorf("trust report requires a target path or at least one --input-artifact/--input-pack value")
	}
	if options.FailOn != "errors" && options.FailOn != "warnings" {
		return fmt.Errorf(`invalid fail-on %q: must be one of errors, warnings`, options.FailOn)
	}
	if len(options.BackendSelections) > 0 && options.Runner != "" {
		return fmt.Errorf("--runner cannot be combined with --backend")
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

func (b Builder) prepareEvidence(root, target string, options BuildOptions) ([]packBundle, error) {
	if len(options.InputPacks) > 0 {
		return b.copyInputPacks(root, options.InputPacks)
	}

	packRoot := filepath.Join(root, "evidence-pack")
	builder := evidencepack.NewBuilder(b.application)
	result, err := builder.Build(target, evidencepack.PackOptions{
		OutputDir:            packRoot,
		InputArtifacts:       append([]string(nil), options.InputArtifacts...),
		Profile:              options.Profile,
		Strictness:           options.Strictness,
		FailOn:               options.FailOn,
		Explain:              options.Explain,
		RoutingRisk:          options.RoutingRisk,
		Runner:               options.Runner,
		SuitePath:            options.SuitePath,
		BackendSelections:    options.BackendSelections,
		IncludePlan:          options.IncludePlan,
		IncludeCompatibility: true,
		IncludeGate:          options.IncludeGate,
	})
	if err != nil {
		return nil, err
	}

	bundle, err := loadPackBundle(result.OutputDir, filepath.Base(result.OutputDir))
	if err != nil {
		return nil, err
	}
	return []packBundle{bundle}, nil
}

func (b Builder) copyInputPacks(root string, inputs []string) ([]packBundle, error) {
	packsRoot := filepath.Join(root, "evidence-packs")
	if err := os.MkdirAll(packsRoot, 0o755); err != nil {
		return nil, err
	}

	out := make([]packBundle, 0, len(inputs))
	for index, input := range inputs {
		name := fmt.Sprintf("pack-%02d", index+1)
		destination := filepath.Join(packsRoot, name)
		if err := copyDir(input, destination); err != nil {
			return nil, err
		}
		bundle, err := loadPackBundle(destination, filepath.ToSlash(filepath.Join("evidence-packs", name)))
		if err != nil {
			return nil, err
		}
		out = append(out, bundle)
	}
	return out, nil
}

func loadPackBundle(root, relativeRoot string) (packBundle, error) {
	manifestPath := filepath.Join(root, "manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return packBundle{}, err
	}

	var manifest evidencepack.Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return packBundle{}, fmt.Errorf("parse evidence pack manifest: %w", err)
	}
	if manifest.PackType != evidencepack.PackType {
		return packBundle{}, fmt.Errorf("directory %s does not contain a supported Firety evidence pack", root)
	}

	artifacts := make([]bundleArtifact, 0, len(manifest.Artifacts))
	for _, item := range manifest.Artifacts {
		absolutePath := filepath.Join(root, filepath.FromSlash(item.Path))
		inspection, err := artifactview.Inspect(absolutePath)
		if err != nil {
			return packBundle{}, err
		}
		artifacts = append(artifacts, bundleArtifact{
			relativePath: filepath.ToSlash(filepath.Join(relativeRoot, filepath.FromSlash(item.Path))),
			absolutePath: absolutePath,
			inspection:   inspection,
		})
	}

	return packBundle{
		relativeRoot: relativeRoot,
		manifest:     manifest,
		artifacts:    artifacts,
	}, nil
}

func flattenArtifacts(packs []packBundle) []bundleArtifact {
	out := make([]bundleArtifact, 0)
	for _, pack := range packs {
		out = append(out, pack.artifacts...)
	}
	sortArtifacts(out)
	return out
}

func sortArtifacts(values []bundleArtifact) {
	slices.SortFunc(values, func(left, right bundleArtifact) int {
		if left.inspection.ArtifactType == right.inspection.ArtifactType {
			return strings.Compare(left.relativePath, right.relativePath)
		}
		return strings.Compare(left.inspection.ArtifactType, right.inspection.ArtifactType)
	})
}

func canDeriveAttestation(artifacts []bundleArtifact) bool {
	for _, item := range artifacts {
		switch item.inspection.ArtifactType {
		case "firety.skill-lint",
			"firety.skill-analysis",
			"firety.skill-routing-eval",
			"firety.skill-routing-eval-multi",
			"firety.skill-compatibility",
			"firety.skill-quality-gate":
			return true
		}
	}
	return false
}

func (b Builder) deriveAttestationArtifact(path string, artifacts []bundleArtifact) (bundleArtifact, error) {
	inputs := make([]string, 0, len(artifacts))
	for _, item := range artifacts {
		inputs = append(inputs, item.absolutePath)
	}
	includeGate := hasArtifactType(artifacts, "firety.skill-quality-gate") ||
		hasArtifactType(artifacts, "firety.skill-analysis") ||
		hasArtifactType(artifacts, "firety.skill-routing-eval") ||
		hasArtifactType(artifacts, "firety.skill-routing-eval-multi") ||
		hasArtifactType(artifacts, "firety.skill-routing-eval-compare") ||
		hasArtifactType(artifacts, "firety.skill-routing-eval-compare-multi") ||
		hasArtifactType(artifacts, "firety.skill-lint-compare") ||
		hasArtifactType(artifacts, "firety.skill-baseline-compare")
	result, err := b.application.Services.SkillAttest.Generate("", service.SkillAttestOptions{
		InputArtifacts: inputs,
		IncludeGate:    includeGate,
	})
	if err != nil {
		return bundleArtifact{}, err
	}

	value := artifact.BuildSkillAttestationArtifact(b.application.Version, result.Report, artifact.SkillAttestationArtifactOptions{
		Format:         "json",
		InputArtifacts: relativeArtifactPaths(artifacts),
		IncludeGate:    includeGate,
	})
	if err := artifact.WriteSkillAttestationArtifact(path, value); err != nil {
		return bundleArtifact{}, err
	}
	inspection, err := artifactview.Inspect(path)
	if err != nil {
		return bundleArtifact{}, err
	}
	return bundleArtifact{
		relativePath: filepath.ToSlash(filepath.Join("artifacts", filepath.Base(path))),
		absolutePath: path,
		inspection:   inspection,
	}, nil
}

func relativeArtifactPaths(values []bundleArtifact) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.relativePath)
	}
	slices.Sort(out)
	return out
}

func loadKnownArtifacts(values []bundleArtifact) (knownArtifacts, error) {
	out := knownArtifacts{}
	for _, item := range values {
		content, err := os.ReadFile(item.absolutePath)
		if err != nil {
			return knownArtifacts{}, err
		}

		var header struct {
			ArtifactType string `json:"artifact_type"`
		}
		if err := json.Unmarshal(content, &header); err != nil {
			return knownArtifacts{}, fmt.Errorf("parse artifact envelope: %w", err)
		}

		switch header.ArtifactType {
		case "firety.skill-attestation":
			var value artifact.SkillAttestationArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return knownArtifacts{}, err
			}
			out.attestation = &value
		case "firety.skill-compatibility":
			var value artifact.SkillCompatibilityArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return knownArtifacts{}, err
			}
			out.compatibility = &value
		case "firety.skill-quality-gate":
			var value artifact.SkillGateArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return knownArtifacts{}, err
			}
			out.gate = &value
		case "firety.benchmark-report":
			var value artifact.BenchmarkArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return knownArtifacts{}, err
			}
			out.benchmark = &value
		}
	}
	return out, nil
}

func buildPages(root string, artifacts []bundleArtifact) ([]PageRef, error) {
	pagesDir := filepath.Join(root, "pages")
	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		return nil, err
	}

	typeCounts := make(map[string]int)
	pages := make([]PageRef, 0, len(artifacts))
	for _, item := range artifacts {
		if len(item.inspection.SupportedRenderModes) == 0 {
			continue
		}

		content, err := render.RenderArtifact(item.absolutePath, render.ModeFullReport)
		if err != nil {
			return nil, err
		}

		fileName := pageFileName(item.inspection.ArtifactType, typeCounts)
		relativePath := filepath.ToSlash(filepath.Join("pages", fileName+".html"))
		title := pageTitle(item.inspection)
		if err := writeReportPage(filepath.Join(root, filepath.FromSlash(relativePath)), pageData{
			Title:        title,
			Summary:      item.inspection.Summary,
			ArtifactPath: relativeLink(filepath.Dir(relativePath), item.relativePath),
			BackLink:     relativeLink(filepath.Dir(relativePath), "index.html"),
			Content:      content,
		}); err != nil {
			return nil, err
		}
		pages = append(pages, PageRef{
			Path:               relativePath,
			Title:              title,
			Summary:            item.inspection.Summary,
			SourceArtifact:     item.relativePath,
			SourceArtifactType: item.inspection.ArtifactType,
		})
	}

	slices.SortFunc(pages, func(left, right PageRef) int {
		return strings.Compare(left.Path, right.Path)
	})
	return pages, nil
}

func buildManifest(application *app.App, options BuildOptions, packs []packBundle, artifacts []bundleArtifact, pages []PageRef, known knownArtifacts) Manifest {
	evidencePacks := make([]EvidencePackRef, 0, len(packs))
	for _, pack := range packs {
		evidencePacks = append(evidencePacks, EvidencePackRef{
			Path:          filepath.ToSlash(filepath.Join(pack.relativeRoot, "manifest.json")),
			Summary:       pack.manifest.ReviewSummary,
			ArtifactCount: len(pack.manifest.Artifacts),
			ReportCount:   len(pack.manifest.Reports),
		})
	}

	artifactRefs := make([]ArtifactRef, 0, len(artifacts))
	for _, item := range artifacts {
		artifactRefs = append(artifactRefs, ArtifactRef{
			Path:            item.relativePath,
			ArtifactType:    item.inspection.ArtifactType,
			Origin:          item.inspection.Origin,
			Target:          item.inspection.Target,
			BaseTarget:      item.inspection.BaseTarget,
			CandidateTarget: item.inspection.CandidateTarget,
			Summary:         item.inspection.Summary,
			Context:         append([]string(nil), item.inspection.Context...),
		})
	}

	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		ReportType:    ReportType,
		Tool: ToolInfo{
			Name:      "firety",
			Version:   application.Version.Version,
			Commit:    application.Version.Commit,
			BuildDate: application.Version.Date,
		},
		Source:        manifestSource(options, packs),
		Target:        firstNonEmpty(attestationTarget(known.attestation), compatibilityTarget(known.compatibility), primaryTarget(artifacts)),
		Summary:       manifestSummary(known, packs, artifacts),
		Context:       buildContext(options),
		EvidencePacks: evidencePacks,
		Artifacts:     artifactRefs,
		Pages:         pages,
	}

	if known.attestation != nil {
		manifest.SupportPosture = string(known.attestation.Attestation.SupportPosture)
		manifest.EvidenceLevel = string(known.attestation.Attestation.EvidenceLevel)
		if known.attestation.Attestation.QualityGate != nil {
			manifest.QualityGateDecision = known.attestation.Attestation.QualityGate.Decision
		}
		manifest.TestedProfiles = append([]string(nil), known.attestation.Attestation.TestedProfiles...)
		manifest.TestedBackends = backendNames(known.attestation.Attestation.TestedBackends)
		manifest.Claims = firstClaimStatements(known.attestation.Attestation.Claims, 4)
		manifest.Strengths = firstStrings(known.attestation.Attestation.Strengths, 4)
		manifest.Limitations = firstStrings(known.attestation.Attestation.Limitations, 4)
		manifest.CautionAreas = firstStrings(known.attestation.Attestation.CautionAreas, 4)
	} else if known.compatibility != nil {
		manifest.SupportPosture = string(known.compatibility.Report.SupportPosture)
		manifest.EvidenceLevel = string(known.compatibility.Report.EvidenceLevel)
		manifest.Strengths = firstStrings(known.compatibility.Report.Strengths, 4)
		manifest.Limitations = firstStrings(known.compatibility.Report.Blockers, 4)
	}

	if manifest.QualityGateDecision == "" && known.gate != nil {
		manifest.QualityGateDecision = string(known.gate.Result.Decision)
	}

	manifest.RecommendedEntrypoints = recommendedEntrypoints(manifest, pages, packs)
	return manifest
}

func buildContext(options BuildOptions) BuildContext {
	return BuildContext{
		Profile:        string(options.Profile),
		Strictness:     options.Strictness.DisplayName(),
		FailOn:         options.FailOn,
		Explain:        options.Explain,
		RoutingRisk:    options.RoutingRisk,
		SuitePath:      options.SuitePath,
		Backends:       backendSelectionLabels(options.BackendSelections),
		InputArtifacts: append([]string(nil), options.InputArtifacts...),
		InputPacks:     append([]string(nil), options.InputPacks...),
		IncludePlan:    options.IncludePlan,
		IncludeGate:    options.IncludeGate,
	}
}

func manifestSource(options BuildOptions, packs []packBundle) string {
	switch {
	case len(options.InputPacks) > 0:
		return "existing-packs"
	case len(options.InputArtifacts) > 0:
		return "existing-artifacts"
	case len(packs) > 0:
		return "fresh-analysis"
	default:
		return "unknown"
	}
}

func manifestSummary(known knownArtifacts, packs []packBundle, artifacts []bundleArtifact) string {
	switch {
	case known.attestation != nil:
		return known.attestation.Attestation.Summary
	case known.benchmark != nil:
		return known.benchmark.Summary.Summary
	case known.compatibility != nil:
		return known.compatibility.Report.Summary
	case len(packs) > 0:
		return packs[0].manifest.ReviewSummary
	case len(artifacts) > 0:
		return artifacts[0].inspection.Summary
	default:
		return "Static Firety trust report."
	}
}

func recommendedEntrypoints(manifest Manifest, pages []PageRef, packs []packBundle) []string {
	out := []string{"index.html"}
	for _, preferred := range []string{
		"pages/attestation.html",
		"pages/gate.html",
		"pages/compatibility.html",
		"pages/lint.html",
		"pages/eval.html",
		"pages/benchmark.html",
	} {
		for _, page := range pages {
			if page.Path == preferred {
				out = append(out, page.Path)
				break
			}
		}
	}
	for _, pack := range packs {
		summaryPath := filepath.ToSlash(filepath.Join(pack.relativeRoot, "SUMMARY.md"))
		out = append(out, summaryPath)
	}
	return uniqueStrings(out)
}

func buildOverviewData(manifest Manifest) overviewData {
	entrypoints := make([]entrypointLink, 0, len(manifest.RecommendedEntrypoints))
	for _, item := range manifest.RecommendedEntrypoints {
		entrypoints = append(entrypoints, entrypointLink{
			Label: entrypointLabel(item),
			Path:  item,
		})
	}
	pages := make([]pageLink, 0, len(manifest.Pages))
	for _, page := range manifest.Pages {
		pages = append(pages, pageLink{
			Title:   page.Title,
			Path:    page.Path,
			Summary: page.Summary,
		})
	}
	return overviewData{
		Title:                  "Firety Trust Report",
		Summary:                manifest.Summary,
		Target:                 manifest.Target,
		SupportPosture:         manifest.SupportPosture,
		EvidenceLevel:          manifest.EvidenceLevel,
		QualityGateDecision:    manifest.QualityGateDecision,
		TestedProfiles:         append([]string(nil), manifest.TestedProfiles...),
		TestedBackends:         append([]string(nil), manifest.TestedBackends...),
		Claims:                 append([]string(nil), manifest.Claims...),
		Strengths:              append([]string(nil), manifest.Strengths...),
		Limitations:            append([]string(nil), manifest.Limitations...),
		CautionAreas:           append([]string(nil), manifest.CautionAreas...),
		RecommendedEntrypoints: entrypoints,
		Pages:                  pages,
		EvidencePacks:          append([]EvidencePackRef(nil), manifest.EvidencePacks...),
		Artifacts:              append([]ArtifactRef(nil), manifest.Artifacts...),
	}
}

func writeManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeOverviewPage(path string, data overviewData) error {
	return renderTemplateToFile(path, overviewTemplate, data)
}

func writeReportPage(path string, data pageData) error {
	return renderTemplateToFile(path, pageTemplate, data)
}

func renderTemplateToFile(path, source string, data any) error {
	tmpl, err := template.New(filepath.Base(path)).Parse(source)
	if err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return tmpl.Execute(file, data)
}

func pageFileName(artifactType string, counts map[string]int) string {
	base := pageSlug(artifactType)
	counts[base]++
	if counts[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s-%02d", base, counts[base])
}

func pageSlug(artifactType string) string {
	switch artifactType {
	case "firety.skill-attestation":
		return "attestation"
	case "firety.skill-compatibility":
		return "compatibility"
	case "firety.skill-quality-gate":
		return "gate"
	case "firety.skill-lint":
		return "lint"
	case "firety.skill-routing-eval":
		return "eval"
	case "firety.skill-routing-eval-multi":
		return "eval-multi"
	case "firety.skill-improvement-plan":
		return "plan"
	case "firety.skill-analysis":
		return "analysis"
	case "firety.skill-lint-compare":
		return "lint-compare"
	case "firety.skill-routing-eval-compare":
		return "eval-compare"
	case "firety.skill-routing-eval-compare-multi":
		return "eval-compare-multi"
	case "firety.benchmark-report":
		return "benchmark"
	default:
		return strings.NewReplacer(".", "-", "_", "-", ":", "-").Replace(strings.TrimPrefix(artifactType, "firety."))
	}
}

func pageTitle(info artifactview.Inspection) string {
	switch info.ArtifactType {
	case "firety.skill-attestation":
		return "Support Claims"
	case "firety.skill-compatibility":
		return "Compatibility"
	case "firety.skill-quality-gate":
		return "Quality Gate"
	case "firety.skill-lint":
		return "Lint Summary"
	case "firety.skill-routing-eval":
		return "Routing Eval"
	case "firety.skill-routing-eval-multi":
		return "Multi-Backend Eval"
	case "firety.skill-improvement-plan":
		return "Improvement Plan"
	case "firety.skill-analysis":
		return "Correlation Analysis"
	case "firety.skill-lint-compare":
		return "Lint Comparison"
	case "firety.skill-routing-eval-compare":
		return "Routing Eval Comparison"
	case "firety.skill-routing-eval-compare-multi":
		return "Multi-Backend Eval Comparison"
	case "firety.benchmark-report":
		return "Benchmark Summary"
	default:
		return info.ArtifactType
	}
}

func relativeLink(fromDir, to string) string {
	fromDir = filepath.Clean(filepath.FromSlash(fromDir))
	target := filepath.Clean(filepath.FromSlash(to))
	rel, err := filepath.Rel(fromDir, target)
	if err != nil {
		return filepath.ToSlash(to)
	}
	return filepath.ToSlash(rel)
}

func copyDir(source, destination string) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func backendNames(values []compatibility.BackendSummary) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.BackendName) != "" {
			out = append(out, value.BackendName)
			continue
		}
		if strings.TrimSpace(value.BackendID) != "" {
			out = append(out, value.BackendID)
		}
	}
	return uniqueStrings(out)
}

func backendSelectionLabels(values []service.SkillEvalBackendSelection) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		label := value.ID
		if strings.TrimSpace(value.Runner) != "" {
			label = fmt.Sprintf("%s=%s", value.ID, value.Runner)
		}
		out = append(out, label)
	}
	return out
}

func attestationTarget(value *artifact.SkillAttestationArtifact) string {
	if value == nil {
		return ""
	}
	return firstNonEmpty(value.Attestation.Target, value.Run.Target)
}

func compatibilityTarget(value *artifact.SkillCompatibilityArtifact) string {
	if value == nil {
		return ""
	}
	return firstNonEmpty(value.Report.Target, value.Run.Target)
}

func primaryTarget(values []bundleArtifact) string {
	for _, value := range values {
		if value.inspection.Target != "" {
			return value.inspection.Target
		}
	}
	return ""
}

func firstClaimStatements(values []attestation.Claim, limit int) []string {
	out := make([]string, 0, min(len(values), limit))
	for _, value := range values {
		out = append(out, value.Statement)
		if len(out) == limit {
			break
		}
	}
	return out
}

func firstStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}

func entrypointLabel(path string) string {
	switch path {
	case "index.html":
		return "Overview"
	}
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "-", " ")
	return strings.Title(base)
}

func uniqueStrings(values []string) []string {
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

func hasArtifactType(values []bundleArtifact, artifactType string) bool {
	for _, value := range values {
		if value.inspection.ArtifactType == artifactType {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

const overviewTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; background: #f5f2ea; color: #1f2430; }
    main { max-width: 980px; margin: 0 auto; padding: 40px 24px 56px; }
    h1, h2 { margin: 0 0 12px; }
    p { line-height: 1.55; }
    .lede { font-size: 1.1rem; margin-bottom: 24px; }
    .grid { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); margin: 24px 0; }
    .card, .section { background: white; border: 1px solid #d8d1c4; border-radius: 14px; padding: 18px; box-shadow: 0 8px 20px rgba(31,36,48,0.05); }
    ul { margin: 10px 0 0 20px; padding: 0; }
    li { margin: 6px 0; }
    code, .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
    a { color: #0f5b78; text-decoration: none; }
    a:hover { text-decoration: underline; }
    .kicker { text-transform: uppercase; letter-spacing: 0.08em; font-size: 0.78rem; color: #76553a; margin-bottom: 8px; }
    .muted { color: #5d6470; }
  </style>
</head>
<body>
  <main>
    <div class="kicker">Firety static trust report</div>
    <h1>{{.Title}}</h1>
    {{if .Target}}<p class="muted mono">Target: {{.Target}}</p>{{end}}
    <p class="lede">{{.Summary}}</p>

    <div class="grid">
      {{if .SupportPosture}}<div class="card"><div class="kicker">Support posture</div><strong>{{.SupportPosture}}</strong></div>{{end}}
      {{if .EvidenceLevel}}<div class="card"><div class="kicker">Evidence</div><strong>{{.EvidenceLevel}}</strong></div>{{end}}
      {{if .QualityGateDecision}}<div class="card"><div class="kicker">Quality gate</div><strong>{{.QualityGateDecision}}</strong></div>{{end}}
      {{if .TestedProfiles}}<div class="card"><div class="kicker">Tested profiles</div><strong>{{range $i, $p := .TestedProfiles}}{{if $i}}, {{end}}{{$p}}{{end}}</strong></div>{{end}}
      {{if .TestedBackends}}<div class="card"><div class="kicker">Tested backends</div><strong>{{range $i, $p := .TestedBackends}}{{if $i}}, {{end}}{{$p}}{{end}}</strong></div>{{end}}
    </div>

    {{if .Claims}}
    <section class="section">
      <h2>Support claims</h2>
      <ul>{{range .Claims}}<li>{{.}}</li>{{end}}</ul>
    </section>
    {{end}}

    {{if .Strengths}}
    <section class="section">
      <h2>Notable strengths</h2>
      <ul>{{range .Strengths}}<li>{{.}}</li>{{end}}</ul>
    </section>
    {{end}}

    {{if .Limitations}}
    <section class="section">
      <h2>Known limitations</h2>
      <ul>{{range .Limitations}}<li>{{.}}</li>{{end}}</ul>
    </section>
    {{end}}

    {{if .CautionAreas}}
    <section class="section">
      <h2>Caution areas</h2>
      <ul>{{range .CautionAreas}}<li>{{.}}</li>{{end}}</ul>
    </section>
    {{end}}

    {{if .RecommendedEntrypoints}}
    <section class="section">
      <h2>Read first</h2>
      <ul>{{range .RecommendedEntrypoints}}<li><a href="{{.Path}}">{{.Label}}</a></li>{{end}}</ul>
    </section>
    {{end}}

    {{if .Pages}}
    <section class="section">
      <h2>Rendered pages</h2>
      <ul>{{range .Pages}}<li><a href="{{.Path}}">{{.Title}}</a>{{if .Summary}}: {{.Summary}}{{end}}</li>{{end}}</ul>
    </section>
    {{end}}

    {{if .EvidencePacks}}
    <section class="section">
      <h2>Included evidence packs</h2>
      <ul>{{range .EvidencePacks}}<li><a href="{{.Path}}">{{.Path}}</a>{{if .Summary}}: {{.Summary}}{{end}}</li>{{end}}</ul>
    </section>
    {{end}}

    {{if .Artifacts}}
    <section class="section">
      <h2>Evidence files</h2>
      <ul>{{range .Artifacts}}<li><a href="{{.Path}}">{{.ArtifactType}}</a>{{if .Summary}}: {{.Summary}}{{end}}</li>{{end}}</ul>
    </section>
    {{end}}
  </main>
</body>
</html>
`

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; background: #f5f2ea; color: #1f2430; }
    main { max-width: 980px; margin: 0 auto; padding: 36px 24px 56px; }
    a { color: #0f5b78; text-decoration: none; }
    a:hover { text-decoration: underline; }
    .meta { color: #5d6470; margin-bottom: 18px; }
    .report { background: white; border: 1px solid #d8d1c4; border-radius: 14px; padding: 18px; box-shadow: 0 8px 20px rgba(31,36,48,0.05); }
    pre { white-space: pre-wrap; word-wrap: break-word; margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; line-height: 1.55; }
  </style>
</head>
<body>
  <main>
    <p><a href="{{.BackLink}}">Overview</a>{{if .ArtifactPath}} · <a href="{{.ArtifactPath}}">Artifact JSON</a>{{end}}</p>
    <h1>{{.Title}}</h1>
    {{if .Summary}}<p class="meta">{{.Summary}}</p>{{end}}
    <section class="report"><pre>{{.Content}}</pre></section>
  </main>
</body>
</html>
`
