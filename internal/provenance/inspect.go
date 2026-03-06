package provenance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/firety/firety/internal/artifact"
	domaineval "github.com/firety/firety/internal/domain/eval"
)

type artifactEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	ArtifactType  string `json:"artifact_type"`
	Tool          struct {
		Version string `json:"version"`
		Commit  string `json:"commit,omitempty"`
	} `json:"tool"`
}

type evidencePackManifest struct {
	SchemaVersion string `json:"schema_version"`
	PackType      string `json:"pack_type"`
	Target        string `json:"target,omitempty"`
	ReviewSummary string `json:"review_summary,omitempty"`
	Provenance    Record `json:"provenance"`
}

type trustReportManifest struct {
	SchemaVersion string `json:"schema_version"`
	ReportType    string `json:"report_type"`
	Target        string `json:"target,omitempty"`
	Summary       string `json:"summary,omitempty"`
	Provenance    Record `json:"provenance"`
}

func Inspect(path string) (Inspection, error) {
	resolved, err := resolveInput(path)
	if err != nil {
		return Inspection{}, err
	}

	if resolved.kind == ObjectKindEvidencePack {
		return inspectEvidencePack(resolved.path)
	}
	if resolved.kind == ObjectKindTrustReport {
		return inspectTrustReport(resolved.path)
	}
	return inspectArtifact(resolved.path)
}

func ComparePaths(basePath, candidatePath string) (Comparison, error) {
	base, err := Inspect(basePath)
	if err != nil {
		return Comparison{}, err
	}
	candidate, err := Inspect(candidatePath)
	if err != nil {
		return Comparison{}, err
	}
	return Compare(base, candidate), nil
}

type resolvedInput struct {
	path string
	kind ObjectKind
}

func resolveInput(path string) (resolvedInput, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return resolvedInput{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return resolvedInput{}, err
	}
	if info.IsDir() {
		manifestPath := filepath.Join(absolute, "manifest.json")
		kind, err := detectManifestKind(manifestPath)
		if err != nil {
			return resolvedInput{}, err
		}
		return resolvedInput{path: manifestPath, kind: kind}, nil
	}

	kind, err := detectFileKind(absolute)
	if err != nil {
		return resolvedInput{}, err
	}
	return resolvedInput{path: absolute, kind: kind}, nil
}

func detectManifestKind(path string) (ObjectKind, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var probe struct {
		PackType   string `json:"pack_type"`
		ReportType string `json:"report_type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", fmt.Errorf("could not parse %s: %w", path, err)
	}
	switch {
	case probe.PackType == "firety.evidence-pack":
		return ObjectKindEvidencePack, nil
	case probe.ReportType == "firety.trust-report":
		return ObjectKindTrustReport, nil
	default:
		return "", fmt.Errorf("%s is not a recognized Firety evidence-pack or trust-report manifest", path)
	}
}

func detectFileKind(path string) (ObjectKind, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var probe struct {
		ArtifactType string `json:"artifact_type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", fmt.Errorf("could not parse %s: %w", path, err)
	}
	if strings.TrimSpace(probe.ArtifactType) == "" {
		return "", fmt.Errorf("%s is not a recognized Firety artifact", path)
	}
	return ObjectKindArtifact, nil
}

func inspectEvidencePack(path string) (Inspection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Inspection{}, err
	}
	var manifest evidencePackManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Inspection{}, fmt.Errorf("could not parse evidence-pack manifest: %w", err)
	}

	record := NormalizeRecord(manifest.Provenance)
	if record.CommandOrigin == "" {
		record.CommandOrigin = "firety evidence pack"
	}
	record.Target = firstNonEmpty(record.Target, manifest.Target)

	return Inspection{
		Path:                  path,
		Kind:                  ObjectKindEvidencePack,
		Type:                  manifest.PackType,
		SchemaVersion:         manifest.SchemaVersion,
		Summary:               manifest.ReviewSummary,
		Provenance:            record,
		SuitableForBaseline:   false,
		SuitableForComparison: record.ComparableKey != "",
	}, nil
}

func inspectTrustReport(path string) (Inspection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Inspection{}, err
	}
	var manifest trustReportManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Inspection{}, fmt.Errorf("could not parse trust-report manifest: %w", err)
	}

	record := NormalizeRecord(manifest.Provenance)
	if record.CommandOrigin == "" {
		record.CommandOrigin = "firety publish report"
	}
	record.Target = firstNonEmpty(record.Target, manifest.Target)

	return Inspection{
		Path:                  path,
		Kind:                  ObjectKindTrustReport,
		Type:                  manifest.ReportType,
		SchemaVersion:         manifest.SchemaVersion,
		Summary:               manifest.Summary,
		Provenance:            record,
		SuitableForBaseline:   false,
		SuitableForComparison: record.ComparableKey != "",
	}, nil
}

func inspectArtifact(path string) (Inspection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Inspection{}, err
	}

	var envelope artifactEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Inspection{}, fmt.Errorf("could not parse Firety artifact: %w", err)
	}

	record, summary, baselineOK, compareOK, err := artifactProvenance(envelope.ArtifactType, data)
	if err != nil {
		return Inspection{}, err
	}
	record.FiretyVersion = firstNonEmpty(record.FiretyVersion, envelope.Tool.Version)
	record.FiretyCommit = firstNonEmpty(record.FiretyCommit, envelope.Tool.Commit)
	record = NormalizeRecord(record)

	return Inspection{
		Path:                  path,
		Kind:                  ObjectKindArtifact,
		Type:                  envelope.ArtifactType,
		SchemaVersion:         envelope.SchemaVersion,
		Summary:               summary,
		Provenance:            record,
		SuitableForBaseline:   baselineOK,
		SuitableForComparison: compareOK,
	}, nil
}

func artifactProvenance(artifactType string, data []byte) (Record, string, bool, bool, error) {
	switch artifactType {
	case "firety.skill-lint":
		var value artifact.SkillLintArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return Record{}, "", false, false, err
		}
		record := NewRecord()
		record.CommandOrigin = "firety skill lint"
		record.Target = value.Run.Target
		record.Profile = value.Run.Profile
		record.Strictness = value.Run.Strictness
		record.FailOn = value.Run.FailOn
		record.Explain = value.Run.Explain
		record.RoutingRisk = value.Run.RoutingRisk
		record.ComparableKey = strings.Join([]string{
			"artifact:" + artifactType,
			"profile:" + value.Run.Profile,
			"strictness:" + value.Run.Strictness,
			"fail-on:" + value.Run.FailOn,
		}, "|")
		return record, lintArtifactSummary(value), true, true, nil
	case "firety.skill-routing-eval":
		var value artifact.SkillEvalArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return Record{}, "", false, false, err
		}
		record := NewRecord()
		record.CommandOrigin = "firety skill eval"
		record.Target = value.Run.Target
		record.Profile = value.Run.Profile
		record.SuitePath = value.Run.SuitePath
		record.Backends = []string{value.Backend.ID}
		record.ComparableKey = strings.Join([]string{
			"artifact:" + artifactType,
			"profile:" + value.Run.Profile,
			"suite:" + value.Run.SuitePath,
			"backends:" + value.Backend.ID,
		}, "|")
		return record, evalArtifactSummary(value), true, true, nil
	case "firety.skill-routing-eval-multi":
		var value artifact.SkillEvalMultiArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return Record{}, "", false, false, err
		}
		record := NewRecord()
		record.CommandOrigin = "firety skill eval"
		record.Target = value.Run.Target
		record.SuitePath = value.Run.SuitePath
		record.Backends = backendInfoIDs(value.Backends)
		record.ComparableKey = strings.Join([]string{
			"artifact:" + artifactType,
			"suite:" + value.Run.SuitePath,
			"backends:" + strings.Join(record.Backends, ","),
		}, "|")
		return record, evalMultiArtifactSummary(value), true, true, nil
	case "firety.skill-quality-gate":
		var value artifact.SkillGateArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return Record{}, "", false, false, err
		}
		record := NewRecord()
		record.CommandOrigin = "firety skill gate"
		record.Target = value.Run.Target
		record.Profile = value.Run.Profile
		record.Strictness = value.Run.Strictness
		record.SuitePath = value.Run.SuitePath
		record.Backends = append([]string(nil), value.Run.Backends...)
		record.InputArtifacts = append([]string(nil), value.Run.InputArtifacts...)
		record.ComparableKey = strings.Join([]string{
			"artifact:" + artifactType,
			"profile:" + value.Run.Profile,
			"strictness:" + value.Run.Strictness,
			"suite:" + value.Run.SuitePath,
			"backends:" + strings.Join(value.Run.Backends, ","),
		}, "|")
		if value.Run.BaseTarget != "" || value.Run.BaselinePath != "" {
			record.ComparabilityNotes = append(record.ComparabilityNotes, "gate artifact includes baseline or compare context")
		}
		return record, fmt.Sprintf("Gate %s with %d blocking reason(s).", strings.ToUpper(string(value.Result.Decision)), len(value.Result.BlockingReasons)), true, true, nil
	case "firety.skill-attestation":
		var value artifact.SkillAttestationArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return Record{}, "", false, false, err
		}
		record := NewRecord()
		record.CommandOrigin = "firety skill attest"
		record.Target = firstNonEmpty(value.Attestation.Target, value.Run.Target)
		record.Strictness = value.Run.Strictness
		record.SuitePath = value.Run.SuitePath
		record.Backends = append([]string(nil), value.Run.Backends...)
		record.InputArtifacts = append([]string(nil), value.Run.InputArtifacts...)
		record.InputPacks = append([]string(nil), value.Run.InputPacks...)
		record.ComparableKey = "artifact:" + artifactType
		if len(value.Attestation.TestedProfiles) == 0 && len(value.Attestation.TestedBackends) == 0 {
			record.ComparabilityNotes = append(record.ComparabilityNotes, "attestation has weak or indirect test evidence")
		}
		return record, value.Attestation.Summary, true, false, nil
	case "firety.skill-compatibility":
		var value artifact.SkillCompatibilityArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return Record{}, "", false, false, err
		}
		record := NewRecord()
		record.CommandOrigin = "firety skill compatibility"
		record.Target = firstNonEmpty(value.Report.Target, value.Run.Target)
		record.Strictness = value.Run.Strictness
		record.SuitePath = value.Run.SuitePath
		record.Backends = append([]string(nil), value.Run.Backends...)
		record.InputArtifacts = append([]string(nil), value.Run.InputArtifacts...)
		record.ComparableKey = "artifact:" + artifactType
		return record, value.Report.Summary, true, false, nil
	case "firety.skill-readiness":
		var value artifact.SkillReadinessArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return Record{}, "", false, false, err
		}
		record := NewRecord()
		record.CommandOrigin = "firety readiness check"
		record.Target = firstNonEmpty(value.Readiness.Target, value.Run.Target)
		record.Strictness = value.Run.Strictness
		record.SuitePath = value.Run.SuitePath
		record.Backends = append([]string(nil), value.Run.Backends...)
		record.InputArtifacts = append([]string(nil), value.Run.InputArtifacts...)
		record.InputPacks = append([]string(nil), value.Run.InputPacks...)
		record.ArtifactDependencies = append([]string(nil), value.Run.InputReports...)
		record.ComparabilityNotes = append(record.ComparabilityNotes, "readiness artifacts are summary decisions and should not be compared directly for regression analysis")
		record.ReproducibilityNotes = append(record.ReproducibilityNotes, "publish context "+value.Run.PublishContext)
		return record, value.Readiness.Summary, false, false, nil
	case "firety.benchmark-report":
		var value artifact.BenchmarkArtifact
		if err := json.Unmarshal(data, &value); err != nil {
			return Record{}, "", false, false, err
		}
		record := NewRecord()
		record.CommandOrigin = "firety benchmark run"
		record.SuitePath = value.Suite.ID
		record.ComparableKey = "artifact:" + artifactType + "|suite:" + value.Suite.ID
		return record, value.Summary.Summary, false, true, nil
	default:
		record := NewRecord()
		record.CommandOrigin = "firety artifact"
		record.ComparabilityNotes = append(record.ComparabilityNotes, "artifact type does not yet expose dedicated provenance mapping")
		return record, "", false, false, nil
	}
}

func lintArtifactSummary(value artifact.SkillLintArtifact) string {
	return fmt.Sprintf("%d error(s), %d warning(s), %d finding(s).", value.Summary.ErrorCount, value.Summary.WarningCount, value.Summary.FindingCount)
}

func evalArtifactSummary(value artifact.SkillEvalArtifact) string {
	return fmt.Sprintf("%d/%d eval cases passed on %s.", value.Summary.Passed, value.Summary.Total, firstNonEmpty(value.Backend.Name, value.Backend.ID))
}

func evalMultiArtifactSummary(value artifact.SkillEvalMultiArtifact) string {
	return fmt.Sprintf("%d backend(s), %d differing case(s).", len(value.Backends), len(value.DifferingCases))
}

func backendInfoIDs(values []domaineval.RoutingEvalBackendInfo) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, firstNonEmpty(value.ID, value.Name))
	}
	return uniqueSorted(out)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
