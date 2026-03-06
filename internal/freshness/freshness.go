package freshness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/evidencepack"
	"github.com/firety/firety/internal/provenance"
	"github.com/firety/firety/internal/trustreport"
)

const SchemaVersion = "1"

type Status string
type IntendedUse string

const (
	StatusFresh                Status = "fresh"
	StatusUsableWithCaveats    Status = "usable-with-caveats"
	StatusStale                Status = "stale"
	StatusInsufficientEvidence Status = "insufficient-evidence"

	UseCompare      IntendedUse = "compare"
	UseBaseline     IntendedUse = "baseline"
	UsePublish      IntendedUse = "publish"
	UseReleaseClaim IntendedUse = "release-claim"
	UseDebug        IntendedUse = "debug"
)

type Options struct {
	Now               time.Time
	MaxAge            time.Duration
	MaxEvalAge        time.Duration
	MaxMultiEvalAge   time.Duration
	MaxBenchmarkAge   time.Duration
	MaxAttestationAge time.Duration
	MaxReportAge      time.Duration
}

type Subject struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Type string `json:"type"`
}

type Component struct {
	Path        string `json:"path"`
	Kind        string `json:"kind,omitempty"`
	Type        string `json:"type"`
	Status      Status `json:"status"`
	GeneratedAt string `json:"generated_at,omitempty"`
	AgeSummary  string `json:"age_summary"`
	Reason      string `json:"reason"`
}

type UseSuitability struct {
	Use    IntendedUse `json:"use"`
	Status Status      `json:"status"`
	Reason string      `json:"reason"`
}

type Report struct {
	SchemaVersion          string            `json:"schema_version"`
	Subject                Subject           `json:"subject"`
	FreshnessStatus        Status            `json:"freshness_status"`
	GeneratedAt            string            `json:"generated_at,omitempty"`
	AgeSummary             string            `json:"age_summary"`
	StaleComponents        []Component       `json:"stale_components,omitempty"`
	CaveatComponents       []Component       `json:"caveat_components,omitempty"`
	Caveats                []string          `json:"caveats,omitempty"`
	IntendedUseSuitability []UseSuitability  `json:"intended_use_suitability,omitempty"`
	RecertificationActions []string          `json:"recertification_actions,omitempty"`
	SupportingPaths        []string          `json:"supporting_paths,omitempty"`
	Provenance             provenance.Record `json:"provenance"`
}

type dependencyRef struct {
	path string
	kind provenance.ObjectKind
	typ  string
}

func DefaultOptions() Options {
	return Options{
		Now:               time.Now().UTC(),
		MaxAge:            7 * 24 * time.Hour,
		MaxEvalAge:        72 * time.Hour,
		MaxMultiEvalAge:   72 * time.Hour,
		MaxBenchmarkAge:   7 * 24 * time.Hour,
		MaxAttestationAge: 72 * time.Hour,
		MaxReportAge:      72 * time.Hour,
	}
}

func (o Options) Validate() error {
	for label, value := range map[string]time.Duration{
		"max-age":             o.MaxAge,
		"max-eval-age":        o.MaxEvalAge,
		"max-multi-eval-age":  o.MaxMultiEvalAge,
		"max-benchmark-age":   o.MaxBenchmarkAge,
		"max-attestation-age": o.MaxAttestationAge,
		"max-report-age":      o.MaxReportAge,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be greater than zero", label)
		}
	}
	return nil
}

func Inspect(path string, options Options) (Report, error) {
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	return inspectPath(path, options, make(map[string]struct{}))
}

func inspectPath(path string, options Options, visited map[string]struct{}) (Report, error) {
	info, err := provenance.Inspect(path)
	if err != nil {
		return Report{}, err
	}
	if _, ok := visited[info.Path]; ok {
		return Report{}, fmt.Errorf("freshness dependency loop detected at %s", info.Path)
	}
	visited[info.Path] = struct{}{}
	defer delete(visited, info.Path)

	stat, err := os.Stat(info.Path)
	if err != nil {
		return Report{}, err
	}
	baseDir := filepath.Dir(info.Path)
	generatedAt := stat.ModTime().UTC()
	age := boundedAge(options.Now, generatedAt)
	threshold := thresholdFor(info.Kind, info.Type, options)
	status := statusForAge(age, threshold)
	report := Report{
		SchemaVersion:   SchemaVersion,
		Subject:         Subject{Path: info.Path, Kind: string(info.Kind), Type: info.Type},
		FreshnessStatus: status,
		GeneratedAt:     generatedAt.Format(time.RFC3339),
		AgeSummary:      ageSummary(age, threshold),
		Provenance:      info.Provenance,
	}

	caveats := make([]string, 0, 8)
	if info.Provenance.CommandOrigin == "" {
		caveats = append(caveats, "command origin is missing")
	}
	if info.Provenance.FiretyVersion == "" {
		caveats = append(caveats, "Firety version is missing")
	}

	deps, depCaveats, err := dependencyRefs(info, baseDir)
	if err != nil {
		return Report{}, err
	}
	caveats = append(caveats, depCaveats...)

	supportingPaths := make([]string, 0, len(deps))
	staleComponents := make([]Component, 0)
	caveatComponents := make([]Component, 0)
	actions := make([]string, 0, 8)
	missingDependency := false

	for _, dep := range deps {
		supportingPaths = append(supportingPaths, dep.path)
		component, ok, componentCaveat := assessDependency(dep, options, visited)
		if componentCaveat != "" {
			caveats = append(caveats, componentCaveat)
		}
		if !ok {
			missingDependency = true
			continue
		}
		switch component.Status {
		case StatusStale:
			staleComponents = append(staleComponents, component)
		case StatusUsableWithCaveats, StatusInsufficientEvidence:
			caveatComponents = append(caveatComponents, component)
		}
		actions = append(actions, recertificationActionFor(component.Type, component.Kind))
	}

	if missingDependency {
		report.FreshnessStatus = StatusInsufficientEvidence
	} else if len(staleComponents) > 0 {
		report.FreshnessStatus = StatusStale
	} else if report.FreshnessStatus == StatusFresh && (len(caveats) > 0 || len(caveatComponents) > 0) {
		report.FreshnessStatus = StatusUsableWithCaveats
	}

	report.StaleComponents = stableComponents(staleComponents)
	report.CaveatComponents = stableComponents(caveatComponents)
	report.Caveats = uniqueSortedStrings(caveats)
	report.SupportingPaths = uniqueSortedStrings(supportingPaths)
	report.RecertificationActions = uniqueSortedStrings(append(actions, recertificationActionFor(info.Type, string(info.Kind))))
	if report.FreshnessStatus == StatusFresh {
		report.RecertificationActions = nil
	}
	report.IntendedUseSuitability = useSuitability(report, info)
	return report, nil
}

func dependencyRefs(info provenance.Inspection, baseDir string) ([]dependencyRef, []string, error) {
	refs := make([]dependencyRef, 0, 8)
	caveats := make([]string, 0, 4)

	switch info.Kind {
	case provenance.ObjectKindEvidencePack:
		var manifest evidencepack.Manifest
		if err := decodeJSONFile(info.Path, &manifest); err != nil {
			return nil, nil, err
		}
		if len(info.Provenance.InputArtifacts) > 0 || len(info.Provenance.InputPacks) > 0 {
			refs = append(refs, externalRefs(baseDir, info.Provenance.InputArtifacts, provenance.ObjectKindArtifact)...)
			refs = append(refs, externalRefs(baseDir, info.Provenance.InputPacks, provenance.ObjectKindEvidencePack)...)
			return uniqueDependencyRefs(refs), caveats, nil
		}
		for _, item := range manifest.Artifacts {
			refs = append(refs, dependencyRef{
				path: resolveDependencyPath(baseDir, item.Path),
				kind: provenance.ObjectKindArtifact,
				typ:  item.ArtifactType,
			})
		}
	case provenance.ObjectKindTrustReport:
		var manifest trustreport.Manifest
		if err := decodeJSONFile(info.Path, &manifest); err != nil {
			return nil, nil, err
		}
		if len(info.Provenance.InputArtifacts) > 0 || len(info.Provenance.InputPacks) > 0 {
			refs = append(refs, externalRefs(baseDir, info.Provenance.InputArtifacts, provenance.ObjectKindArtifact)...)
			refs = append(refs, externalRefs(baseDir, info.Provenance.InputPacks, provenance.ObjectKindEvidencePack)...)
			return uniqueDependencyRefs(refs), caveats, nil
		}
		for _, item := range manifest.Artifacts {
			refs = append(refs, dependencyRef{
				path: resolveDependencyPath(baseDir, item.Path),
				kind: provenance.ObjectKindArtifact,
				typ:  item.ArtifactType,
			})
		}
		for _, item := range manifest.EvidencePacks {
			refs = append(refs, dependencyRef{
				path: filepath.Dir(resolveDependencyPath(baseDir, item.Path)),
				kind: provenance.ObjectKindEvidencePack,
				typ:  evidencepack.PackType,
			})
		}
	case provenance.ObjectKindArtifact:
		artifactRefs, artifactCaveats, err := artifactDependencies(info, baseDir)
		if err != nil {
			return nil, nil, err
		}
		refs = append(refs, artifactRefs...)
		caveats = append(caveats, artifactCaveats...)
	}

	return uniqueDependencyRefs(refs), uniqueSortedStrings(caveats), nil
}

func artifactDependencies(info provenance.Inspection, baseDir string) ([]dependencyRef, []string, error) {
	data, err := os.ReadFile(info.Path)
	if err != nil {
		return nil, nil, err
	}
	refs := make([]dependencyRef, 0, 6)
	caveats := make([]string, 0, 4)

	switch info.Type {
	case "firety.skill-attestation":
		var value artifact.SkillAttestationArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, nil, err
		}
		refs = append(refs, externalRefs(baseDir, value.Run.InputArtifacts, provenance.ObjectKindArtifact)...)
		refs = append(refs, externalRefs(baseDir, value.Run.InputPacks, provenance.ObjectKindEvidencePack)...)
		if len(refs) == 0 {
			caveats = append(caveats, "attestation does not reference supporting artifact or pack paths")
		}
	case "firety.skill-quality-gate":
		var value artifact.SkillGateArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, nil, err
		}
		refs = append(refs, externalRefs(baseDir, value.Run.InputArtifacts, provenance.ObjectKindArtifact)...)
		if strings.TrimSpace(value.Run.BaselinePath) != "" {
			refs = append(refs, dependencyRef{
				path: resolveDependencyPath(baseDir, value.Run.BaselinePath),
				kind: provenance.ObjectKindArtifact,
				typ:  "firety.skill-baseline",
			})
		}
	case "firety.skill-compatibility":
		var value artifact.SkillCompatibilityArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, nil, err
		}
		refs = append(refs, externalRefs(baseDir, value.Run.InputArtifacts, provenance.ObjectKindArtifact)...)
	case "firety.skill-baseline-compare":
		var value artifact.SkillBaselineCompareArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(value.Run.BaselinePath) != "" {
			refs = append(refs, dependencyRef{
				path: resolveDependencyPath(baseDir, value.Run.BaselinePath),
				kind: provenance.ObjectKindArtifact,
				typ:  "firety.skill-baseline",
			})
		}
	}

	return uniqueDependencyRefs(refs), uniqueSortedStrings(caveats), nil
}

func assessDependency(ref dependencyRef, options Options, visited map[string]struct{}) (Component, bool, string) {
	if ref.kind == provenance.ObjectKindEvidencePack || ref.kind == provenance.ObjectKindTrustReport {
		nested, err := inspectPath(ref.path, options, visited)
		if err != nil {
			return Component{}, false, fmt.Sprintf("supporting evidence is missing or unreadable: %s", ref.path)
		}
		reason := nested.AgeSummary
		if len(nested.StaleComponents) > 0 {
			reason = fmt.Sprintf("%s; %d stale component(s)", nested.AgeSummary, len(nested.StaleComponents))
		}
		return Component{
			Path:        ref.path,
			Kind:        string(ref.kind),
			Type:        ref.typ,
			Status:      nested.FreshnessStatus,
			GeneratedAt: nested.GeneratedAt,
			AgeSummary:  nested.AgeSummary,
			Reason:      reason,
		}, true, ""
	}
	info, err := os.Stat(ref.path)
	if err != nil {
		return Component{}, false, fmt.Sprintf("supporting evidence is missing: %s", ref.path)
	}
	modTime := info.ModTime().UTC()
	age := boundedAge(options.Now, modTime)
	threshold := thresholdFor(ref.kind, ref.typ, options)
	status := statusForAge(age, threshold)
	component := Component{
		Path:        ref.path,
		Kind:        string(ref.kind),
		Type:        ref.typ,
		Status:      status,
		GeneratedAt: modTime.Format(time.RFC3339),
		AgeSummary:  ageSummary(age, threshold),
	}
	switch status {
	case StatusStale:
		component.Reason = fmt.Sprintf("%s is older than the selected freshness threshold", ref.typ)
	case StatusFresh:
		component.Reason = fmt.Sprintf("%s is within the selected freshness threshold", ref.typ)
	default:
		component.Reason = fmt.Sprintf("%s has freshness caveats", ref.typ)
	}
	return component, true, ""
}

func useSuitability(report Report, info provenance.Inspection) []UseSuitability {
	compareStatus, compareReason := compareUseStatus(report, info)
	baselineStatus, baselineReason := baselineUseStatus(report, info)
	publishStatus, publishReason := publishUseStatus(report, info)
	releaseStatus, releaseReason := releaseClaimUseStatus(report, info)
	debugStatus, debugReason := debugUseStatus(report)
	uses := []UseSuitability{
		useDecision(UseCompare, compareStatus, compareReason),
		useDecision(UseBaseline, baselineStatus, baselineReason),
		useDecision(UsePublish, publishStatus, publishReason),
		useDecision(UseReleaseClaim, releaseStatus, releaseReason),
		useDecision(UseDebug, debugStatus, debugReason),
	}
	return uses
}

func compareUseStatus(report Report, info provenance.Inspection) (Status, string) {
	if !info.SuitableForComparison {
		return StatusInsufficientEvidence, "saved output is not a compare-ready Firety result"
	}
	switch report.FreshnessStatus {
	case StatusFresh:
		return StatusFresh, "saved result is current enough for direct comparison reuse"
	case StatusUsableWithCaveats:
		return StatusUsableWithCaveats, "comparison is possible, but freshness caveats should be reviewed first"
	case StatusStale:
		return StatusStale, "rerun before relying on compare results"
	default:
		return StatusInsufficientEvidence, "freshness could not be validated well enough for comparison reuse"
	}
}

func baselineUseStatus(report Report, info provenance.Inspection) (Status, string) {
	if !info.SuitableForBaseline {
		return StatusInsufficientEvidence, "saved output is not suitable as a baseline snapshot"
	}
	switch report.FreshnessStatus {
	case StatusFresh:
		return StatusFresh, "saved result is current enough to act as a baseline"
	case StatusUsableWithCaveats:
		return StatusUsableWithCaveats, "baseline reuse is possible, but review caveats before locking it in"
	case StatusStale:
		return StatusStale, "capture a fresh baseline before using it for regressions"
	default:
		return StatusInsufficientEvidence, "baseline freshness could not be validated"
	}
}

func publishUseStatus(report Report, info provenance.Inspection) (Status, string) {
	if info.Kind == provenance.ObjectKindArtifact && info.Type != "firety.skill-attestation" {
		return StatusUsableWithCaveats, "publish via an evidence pack, trust report, or attestation instead of a raw artifact"
	}
	switch report.FreshnessStatus {
	case StatusFresh:
		return StatusFresh, "saved evidence is current enough for publishable summaries"
	case StatusUsableWithCaveats:
		return StatusUsableWithCaveats, "publish only if the documented caveats are acceptable"
	case StatusStale:
		return StatusStale, "rebuild the publishable output from fresh evidence first"
	default:
		return StatusInsufficientEvidence, "publishability cannot be assessed from the available evidence"
	}
}

func releaseClaimUseStatus(report Report, info provenance.Inspection) (Status, string) {
	if info.Type != "firety.skill-attestation" && info.Kind != provenance.ObjectKindTrustReport {
		return StatusInsufficientEvidence, "release claims should be backed by an attestation or trust report"
	}
	switch report.FreshnessStatus {
	case StatusFresh:
		return StatusFresh, "evidence is current enough to back a release claim"
	case StatusUsableWithCaveats:
		return StatusUsableWithCaveats, "release claims should mention the current evidence caveats"
	case StatusStale:
		return StatusStale, "do not reuse these release claims without recertification"
	default:
		return StatusInsufficientEvidence, "release-claim suitability cannot be confirmed from the saved evidence"
	}
}

func debugUseStatus(report Report) (Status, string) {
	switch report.FreshnessStatus {
	case StatusFresh:
		return StatusFresh, "evidence is current enough for local debugging or review"
	case StatusUsableWithCaveats:
		return StatusUsableWithCaveats, "evidence is still usable for local debugging with caveats"
	case StatusStale:
		return StatusUsableWithCaveats, "stale evidence can still help with local debugging, but should not drive release decisions"
	default:
		return StatusUsableWithCaveats, "evidence is incomplete, but still may help with local debugging"
	}
}

func useDecision(use IntendedUse, status Status, reason string) UseSuitability {
	return UseSuitability{Use: use, Status: status, Reason: reason}
}

func thresholdFor(kind provenance.ObjectKind, typ string, options Options) time.Duration {
	switch {
	case kind == provenance.ObjectKindTrustReport || kind == provenance.ObjectKindEvidencePack:
		return options.MaxReportAge
	case typ == "firety.skill-attestation":
		return options.MaxAttestationAge
	case typ == "firety.benchmark-report":
		return options.MaxBenchmarkAge
	case strings.Contains(typ, "routing-eval-compare-multi") || strings.Contains(typ, "routing-eval-multi"):
		return options.MaxMultiEvalAge
	case strings.Contains(typ, "routing-eval"):
		return options.MaxEvalAge
	default:
		return options.MaxAge
	}
}

func statusForAge(age, threshold time.Duration) Status {
	if age > threshold {
		return StatusStale
	}
	return StatusFresh
}

func boundedAge(now, generatedAt time.Time) time.Duration {
	if generatedAt.After(now) {
		return 0
	}
	return now.Sub(generatedAt)
}

func ageSummary(age, threshold time.Duration) string {
	return fmt.Sprintf("%s old (threshold %s)", roundDuration(age), roundDuration(threshold))
}

func roundDuration(value time.Duration) string {
	if value < time.Minute {
		return value.Round(time.Second).String()
	}
	if value < time.Hour {
		return value.Round(time.Minute).String()
	}
	return value.Round(time.Hour).String()
}

func recertificationActionFor(typ, kind string) string {
	switch {
	case typ == "firety.skill-lint":
		return "rerun lint"
	case typ == "firety.skill-routing-eval":
		return "rerun the routing eval suite"
	case typ == "firety.skill-routing-eval-multi":
		return "rerun the selected backend eval suites"
	case typ == "firety.skill-attestation":
		return "regenerate the attestation from fresh evidence"
	case typ == "firety.skill-quality-gate":
		return "rerun the quality gate after refreshing stale evidence"
	case typ == "firety.skill-compatibility":
		return "rerun compatibility analysis from current evidence"
	case typ == "firety.skill-baseline":
		return "capture a new baseline after rerunning current evidence"
	case typ == "firety.skill-baseline-compare":
		return "rerun the baseline comparison from fresh evidence"
	case typ == "firety.benchmark-report":
		return "rerun the benchmark corpus"
	case kind == string(provenance.ObjectKindEvidencePack):
		return "rebuild the evidence pack from fresh artifacts"
	case kind == string(provenance.ObjectKindTrustReport):
		return "rebuild the trust report from fresh evidence"
	default:
		return "rerun the relevant Firety workflow"
	}
}

func externalRefs(baseDir string, values []string, kind provenance.ObjectKind) []dependencyRef {
	out := make([]dependencyRef, 0, len(values))
	for _, value := range values {
		resolved := resolveDependencyPath(baseDir, value)
		typ := ""
		if kind == provenance.ObjectKindEvidencePack {
			typ = evidencepack.PackType
		}
		if kind == provenance.ObjectKindArtifact {
			if info, err := provenance.Inspect(resolved); err == nil {
				typ = info.Type
			}
		}
		out = append(out, dependencyRef{
			path: resolved,
			kind: kind,
			typ:  typ,
		})
	}
	return out
}

func resolveDependencyPath(baseDir, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(baseDir, filepath.FromSlash(value))
}

func stableComponents(values []Component) []Component {
	slices.SortFunc(values, func(left, right Component) int {
		if cmp := strings.Compare(left.Type, right.Type); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.Path, right.Path)
	})
	return values
}

func uniqueDependencyRefs(values []dependencyRef) []dependencyRef {
	seen := make(map[string]dependencyRef, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.path) == "" {
			continue
		}
		key := string(value.kind) + "|" + value.typ + "|" + value.path
		seen[key] = value
	}
	out := make([]dependencyRef, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	slices.SortFunc(out, func(left, right dependencyRef) int {
		if cmp := strings.Compare(left.typ, right.typ); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.path, right.path)
	})
	return out
}

func uniqueSortedStrings(values []string) []string {
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

func decodeJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
